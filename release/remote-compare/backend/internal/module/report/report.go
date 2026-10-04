package report

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/email"
	"netops/internal/tsdb"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.POST("/generate", func(c *gin.Context) {
		var req struct {
			Title   string   `json:"title"`
			Start   string   `json:"start"`
			End     string   `json:"end"`
			Send    bool     `json:"send_email"`
			EmailTo string   `json:"email_to"`
			Include []string `json:"include"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if req.Start != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", req.Start, time.Local); err == nil {
				start = t
			}
		}
		if req.End != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", req.End, time.Local); err == nil {
				end = t
			}
		}
		title := req.Title
		if title == "" {
			title = fmt.Sprintf("运维巡检报告 %s 至 %s", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
		}
		include := map[string]bool{}
		for _, s := range req.Include {
			include[s] = true
		}
		if len(include) == 0 {
			include = map[string]bool{"link": true, "monitor": true, "db": true}
		}

		var linkSection, devSection, dbSection, containerSection, nodeSection string
		stats := map[string]int{"total": 0, "up": 0, "down": 0, "warn": 0}
		if include["link"] {
			linkSection = renderLinkSection(a, start, end, stats)
		}
		if include["monitor"] {
			devSection = renderDevSection(a, start, end, stats)
		}
		if include["db"] {
			dbSection = renderDBSection(a, stats)
		}
		if include["container"] {
			containerSection = renderContainerSection(a, stats)
		}
		if include["node"] {
			nodeSection = renderNodeSection(a, stats)
		}

		content := renderHTML(title, start, end, linkSection, devSection, dbSection, containerSection, nodeSection, stats)

		username, _ := c.Get(middleware.KeyUsername)
		rec := model.ReportRecord{
			Title: title, StartAt: start, EndAt: end, Content: content,
			Creator: fmt.Sprint(username), SendEmail: req.Send, EmailTo: req.EmailTo,
		}
		if err := a.DB.Create(&rec).Error; err != nil {
			response.Err(c, err)
			return
		}

		if req.Send {
			to := splitTo(req.EmailTo)
			if err := sendReportEmail(a, rec.ID, title, content, to); err != nil {
				response.Fail(c, 500, response.CodeServerError, "报告已生成，但邮件发送失败: "+err.Error())
				return
			}
		}
		response.OK(c, gin.H{"id": rec.ID, "title": title, "message": "报告生成成功"})
	})

	g.GET("/list", func(c *gin.Context) {
		var list []model.ReportRecord
		a.DB.Order("id desc").Limit(100).Find(&list)
		response.OK(c, list)
	})

	g.GET("/:id/html", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.ReportRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			c.String(404, "报告不存在")
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, rec.Content)
	})

	g.GET("/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.ReportRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			response.NotFound(c, "报告不存在")
			return
		}
		response.OK(c, rec)
	})

	g.POST("/:id/send", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.ReportRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			response.NotFound(c, "报告不存在")
			return
		}
		var req struct {
			EmailTo string `json:"email_to"`
		}
		_ = c.ShouldBindJSON(&req)
		to := splitTo(req.EmailTo)
		if len(to) == 0 {
			to = splitTo(rec.EmailTo)
		}
		if err := sendReportEmail(a, rec.ID, rec.Title, rec.Content, to); err != nil {
			response.Fail(c, 500, response.CodeServerError, "邮件发送失败: "+err.Error())
			return
		}
		a.DB.Model(&rec).Updates(map[string]any{"send_email": true, "email_to": strings.Join(to, ",")})
		response.OK(c, gin.H{"ok": true})
	})

	g.DELETE("/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.ReportRecord{}, id)
		response.OK(c, gin.H{"ok": true})
	})
}

func sendReportEmail(a *core.App, id uint, title, content string, to []string) error {
	if len(to) == 0 {
		return fmt.Errorf("收件人列表为空")
	}
	var cfgs []model.EmailConfig
	a.DB.Where("enable = ?", true).Find(&cfgs)
	if len(cfgs) == 0 {
		return fmt.Errorf("未启用任何邮箱配置（系统管理-邮箱）")
	}
	cfg := cfgs[0]
	return email.Send(email.Config{
		Name: cfg.Name, SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort,
		User: cfg.User, Password: cfg.Password, UseSSL: cfg.UseSSL, Enable: true,
	}, to, title, content)
}

func splitTo(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" && strings.Contains(v, "@") {
			out = append(out, v)
		}
	}
	return out
}

func badge(status, color string) string {
	return fmt.Sprintf("<span style='display:inline-block;padding:2px 10px;border-radius:10px;font-size:12px;font-weight:500;color:#fff;background:%s'>%s</span>", color, status)
}

func renderLinkSection(a *core.App, start, end time.Time, stats map[string]int) string {
	var tasks []model.LinkTask
	a.DB.Find(&tasks)
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>📡 链路检测汇总</h3><table><thead><tr><th>链路名称</th><th>目标地址</th><th>当前状态</th><th>平均时延</th><th>累计中断</th><th>可用率</th></tr></thead><tbody>`)
	for _, t := range tasks {
		stats["total"]++
		tags := map[string]string{"task_id": strconv.FormatUint(uint64(t.ID), 10)}
		upSeries, _ := a.TSDB.Query(tsdb.Query{Metric: "link_status", Tags: tags, Start: start, End: end, Agg: "last"})
		rtSeries, _ := a.TSDB.Query(tsdb.Query{Metric: "link_rt", Tags: tags, Start: start, End: end, Agg: "avg"})
		avgRT := 0.0
		upCnt, total := 0, 0
		if len(upSeries) > 0 {
			for _, p := range upSeries[0].Points {
				total++
				if p[1] >= 0.5 {
					upCnt++
				}
			}
		}
		if len(rtSeries) > 0 && len(rtSeries[0].Points) > 0 {
			var sum, n float64
			for _, p := range rtSeries[0].Points {
				if p[1] > 0 {
					sum += p[1]
					n++
				}
			}
			if n > 0 {
				avgRT = sum / n
			}
		}
		avail := 100.0
		if total > 0 {
			avail = float64(upCnt) / float64(total) * 100
		}
		status, color := "未知", "#909399"
		if s, ok := a.PingMgr.Get(t.ID); ok && s.Up {
			status, color = "正常", "#52c41a"
			stats["up"]++
		} else if s, ok := a.PingMgr.Get(t.ID); ok && !s.Up {
			status, color = "中断", "#f5222d"
			stats["down"]++
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.1f ms</td><td>%d 次</td><td>%.2f%%</td></tr>",
			t.Name, t.Target, badge(status, color), avgRT, total-upCnt, avail))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

func renderDevSection(a *core.App, start, end time.Time, stats map[string]int) string {
	var devices []model.MonitorDevice
	a.DB.Where("type = ?", "server").Or("type = ?", "other").Find(&devices)
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>🖥️ 服务器资源使用汇总</h3><table><thead><tr><th>设备</th><th>IP</th><th>状态</th><th>CPU(均值)</th><th>内存使用(均值)</th></tr></thead><tbody>`)
	for _, d := range devices {
		stats["total"]++
		tags := map[string]string{"device_id": strconv.FormatUint(uint64(d.ID), 10)}
		cpuSeries, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_cpu", Tags: tags, Start: start, End: end, Agg: "avg"})
		memSeries, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_mem", Tags: tags, Start: start, End: end, Agg: "avg"})
		cpu := avgOf(cpuSeries)
		mem := avgOf(memSeries)
		status, color := "离线", "#f5222d"
		if s, ok := a.SnmpMgr.DeviceSnapshotByID(d.ID); ok && s.Up {
			status, color = "在线", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		cpuColor := "#52c41a"
		if cpu > 80 {
			cpuColor = "#f5222d"
		} else if cpu > 60 {
			cpuColor = "#faad14"
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td style='color:%s'>%.1f%%</td><td>%.1f%%</td></tr>",
			d.Name, d.IP, badge(status, color), cpuColor, cpu, mem))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

func renderDBSection(a *core.App, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>🗄️ 数据库状态汇总</h3><table><thead><tr><th>实例</th><th>类型</th><th>状态</th><th>连接数</th><th>错误数</th></tr></thead><tbody>`)
	for _, s := range a.DBProbe.SnapshotAll() {
		stats["total"]++
		status, color := "异常", "#f5222d"
		if s.Up {
			status, color = "正常", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td></tr>",
			s.Name, s.Type, badge(status, color), s.Conns, s.Errors))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

func renderContainerSection(a *core.App, stats map[string]int) string {
	var hosts []model.DockerHost
	a.DB.Order("id asc").Find(&hosts)
	var clusters []model.K8sCluster
	a.DB.Order("id asc").Find(&clusters)

	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>🐳 容器与 Kubernetes 汇总</h3><table><thead><tr><th>节点</th><th>IP/地址</th><th>类型</th><th>状态</th><th>容器/Pod 数</th><th>异常数</th></tr></thead><tbody>`)

	if len(hosts) == 0 && len(clusters) == 0 {
		sb.WriteString("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Docker/K8s 节点监控</td></tr>")
	}

	dockerStatus := a.DockerM.HostStatus()
	containers := a.DockerM.Snapshot()
	// 统计每个 host 上的容器数和异常数
	containerCount := map[string]int{}
	abnormalCount := map[string]int{}
	for _, c := range containers {
		containerCount[c.Host]++
		if c.State != "running" {
			abnormalCount[c.Host]++
		}
	}

	for _, h := range hosts {
		stats["total"]++
		online := false
		if v, ok := dockerStatus[h.Address]; ok { online = v }
		status, color := "离线", "#f5222d"
		if online {
			status, color = "在线", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		addr := strings.TrimPrefix(h.Address, "tcp://")
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>docker</td><td>%s</td><td>%d</td><td>%d</td></tr>",
			h.Name, addr, badge(status, color), containerCount[h.Address], abnormalCount[h.Address]))
	}

	k8sStatus := a.K8sM.ClusterStatus()
	for _, k := range clusters {
		stats["total"]++
		online := false
		if v, ok := k8sStatus[k.Name]; ok { online = v }
		status, color := "离线", "#f5222d"
		if online {
			status, color = "在线", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>k8s</td><td>%s</td><td>-</td><td>0</td></tr>",
			k.Name, k.APIServer, badge(status, color)))
	}

	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

func avgOf(series []tsdb.Series) float64 {
	if len(series) == 0 {
		return 0
	}
	var sum, n float64
	for _, p := range series[0].Points {
		sum += p[1]
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

var reportTpl = template.Must(template.New("report").Parse(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>
*{box-sizing:border-box}
body{font-family:"Microsoft YaHei","Segoe UI",Arial,sans-serif;margin:0;padding:32px;background:#f0f2f5;color:#2c3e50}
.wrap{max-width:900px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.08);overflow:hidden}
.header{background:linear-gradient(135deg,#1a73e8,#0d47a1);color:#fff;padding:28px 36px}
.header h1{margin:0;font-size:22px;font-weight:600}
.header .meta{margin-top:8px;font-size:13px;opacity:.85}
.summary{display:flex;gap:16px;padding:20px 36px;background:#fafbfc;border-bottom:1px solid #eee}
.summary .card{flex:1;text-align:center;padding:14px;border-radius:8px;background:#fff;border:1px solid #eee}
.summary .num{font-size:28px;font-weight:700;line-height:1.2}
.summary .lbl{font-size:12px;color:#888;margin-top:4px}
.section{padding:20px 36px}
h3{margin:0 0 14px;font-size:16px;color:#1a73e8;padding-bottom:8px;border-bottom:2px solid #e8f0fe}
table{width:100%;border-collapse:collapse;font-size:13px}
thead th{background:#f5f7fa;padding:10px 12px;text-align:left;font-weight:600;color:#555;border-bottom:2px solid #e8e8e8}
tbody td{padding:10px 12px;border-bottom:1px solid #f0f0f0}
tbody tr:hover{background:#f8faff}
.footer{padding:16px 36px;text-align:center;color:#aaa;font-size:12px;border-top:1px solid #eee}
</style></head><body><div class="wrap">
<div class="header">
<h1>{{.Title}}</h1>
<div class="meta">巡检周期：{{.Start}} 至 {{.End}} ｜ 生成时间：{{.Now}}</div>
</div>
<div class="summary">
<div class="card"><div class="num" style="color:#1a73e8">{{.Total}}</div><div class="lbl">监控对象总数</div></div>
<div class="card"><div class="num" style="color:#52c41a">{{.Up}}</div><div class="lbl">正常</div></div>
<div class="card"><div class="num" style="color:#f5222d">{{.Down}}</div><div class="lbl">异常/离线</div></div>
<div class="card"><div class="num" style="color:#faad14">{{.Warn}}</div><div class="lbl">告警</div></div>
</div>
{{.LinkSection}}{{.DevSection}}{{.DBSection}}{{.ContainerSection}}{{.NodeSection}}
<div class="footer">本报告由 NetOps 网络运维监控平台自动生成</div>
</div></body></html>`))

func renderHTML(title string, start, end time.Time, linkS, devS, dbS, containerS, nodeS string, stats map[string]int) string {
	var sb strings.Builder
	reportTpl.Execute(&sb, map[string]any{
		"Title": title, "Start": start.Format("2006-01-02 15:04"), "End": end.Format("2006-01-02 15:04"),
		"Now":           time.Now().Format("2006-01-02 15:04:05"),
		"LinkSection":   template.HTML(linkS), "DevSection": template.HTML(devS), "DBSection": template.HTML(dbS), "ContainerSection": template.HTML(containerS), "NodeSection": template.HTML(nodeS),
		"Total": stats["total"], "Up": stats["up"], "Down": stats["down"], "Warn": stats["warn"],
	})
	return sb.String()
}

func renderNodeSection(a *core.App, stats map[string]int) string {
	var nodes []model.Node
	a.DB.Order("level asc, id asc").Find(&nodes)
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>节点健康性</h3><table><thead><tr><th>节点名称</th><th>UUID</th><th>层级</th><th>状态</th><th>设备数</th><th>容器数</th><th>最后上报</th></tr></thead><tbody>`)
	for _, n := range nodes {
		stats["total"]++
		color := "#52c41a"
		badge := "在线"
		if n.Status != "online" {
			color = "#f5222d"
			badge = "离线"
			stats["down"]++
		} else {
			stats["up"]++
		}
		level := []string{"总部", "省级", "市级", "县级"}[n.Level]
		if n.Level >= len(level) {
			level = fmt.Sprintf("L%d", n.Level)
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td><code>%s</code></td><td>%s</td><td><span style='color:%s'>%s</span></td><td>%d</td><td>%d</td><td>%s</td></tr>",
			n.Name, n.NodeUUID, level, color, badge, n.DeviceCnt, n.ContainerCnt, n.LastSeenAt.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

func init() {
	modreg.RegisterProtected("report", RegisterProtected)
}
