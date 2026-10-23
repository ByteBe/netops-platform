package report

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/collector/nginxprobe"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/email"
	"netops/internal/service/webhook"
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
			include = map[string]bool{"link": true, "monitor": true, "db": true, "nginx": true}
		}

		var linkSection, devSection, dbSection, containerSection, nodeSection, nginxSection string
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
		if include["nginx"] {
			nginxSection = renderNginxSection(a, stats)
		}

		content := renderHTML(title, start, end, linkSection, devSection, dbSection, containerSection, nodeSection, nginxSection, stats)

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
			if err := sendReportViaChannels(a, title, content); err != nil {
				response.Fail(c, 500, response.CodeServerError, "报告已生成，但发送失败: "+err.Error())
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

	g.GET("/:id/pdf", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.ReportRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			c.String(404, "报告不存在")
			return
		}
		pdf, err := renderPDF(rec.Title, rec.Content)
		if err != nil {
			c.JSON(500, gin.H{"code": 40000, "message": "PDF生成失败: " + err.Error()})
			return
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.pdf\"", sanitizeFilename(rec.Title)))
		c.Data(200, "application/pdf", pdf)
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
		if err := sendReportViaChannels(a, rec.Title, rec.Content); err != nil {
			response.Fail(c, 500, response.CodeServerError, "报告发送失败: "+err.Error())
			return
		}
		a.DB.Model(&rec).Updates(map[string]any{"send_email": true})
		response.OK(c, gin.H{"ok": true})
	})

	g.DELETE("/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.ReportRecord{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	startReportScheduler(a)
}

// sendReportViaChannels 通过系统管理中配置的所有启用通知渠道发送报告
func sendReportViaChannels(a *core.App, title, content string) error {
	var ncs []model.NotifyConfig
	a.DB.Where("enabled = ?", true).Find(&ncs)
	sent := 0
	// 收集所有 email 渠道的收件地址
	var recipients []string
	for _, nc := range ncs {
		if nc.Type == "email" {
			recipients = append(recipients, nc.Addr)
		}
	}
	// 启用中的邮箱配置（发件账号）
	var cfgs []model.EmailConfig
	a.DB.Where("enable = ?", true).Find(&cfgs)
	// 未配置 email 通知渠道时，兜底用邮箱配置（默认收件人，缺省则发件账号自己）发送
	if len(recipients) == 0 && len(cfgs) > 0 {
		cfg0 := cfgs[0]
		if cfg0.DefaultTo != "" {
			for _, addr := range strings.Split(cfg0.DefaultTo, ",") {
				if addr = strings.TrimSpace(addr); addr != "" {
					recipients = append(recipients, addr)
				}
			}
		} else if cfg0.User != "" {
			recipients = []string{cfg0.User}
		}
	}
	// 通过邮箱配置 SMTP 发送
	if len(recipients) > 0 {
		if len(cfgs) == 0 {
			return fmt.Errorf("未配置发件邮箱（系统管理-邮箱）")
		}
		cfg := cfgs[0]
		if err := email.Send(email.Config{Name: cfg.Name, SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort, User: cfg.User, Password: cfg.Password, UseSSL: cfg.UseSSL, Enable: true}, recipients, title, content); err == nil {
			sent++
		} else {
			return err
		}
	}
	// 推送 webhook 渠道（飞书/钉钉/企业微信）：发送纯文本摘要而非完整 HTML——
	// 各平台文本消息均有长度限制（企业微信 2048 字节、钉钉约 5000 字符），完整报告必被拒收；
	// 飞书/钉钉开启加签时由 webhook 包自动签名，未配置密钥则明文发送
	text := channelText(title, content)
	var errs []string
	for _, nc := range ncs {
		if nc.Type == "email" {
			continue
		}
		url := nc.Config
		if url == "" {
			url = nc.Addr
		}
		if url == "" {
			continue
		}
		if err := webhook.SendText(nc.Type, url, nc.Token, text); err != nil {
			errs = append(errs, fmt.Sprintf("%s(%s): %v", nc.Name, nc.Type, err))
			continue
		}
		sent++
	}
	if len(errs) > 0 {
		return fmt.Errorf("部分通知渠道发送失败: %s", strings.Join(errs, "; "))
	}
	if sent == 0 {
		if len(ncs) == 0 && len(cfgs) == 0 {
			return fmt.Errorf("未配置任何通知渠道（系统管理-通知渠道）")
		}
		return fmt.Errorf("所有通知渠道发送失败")
	}
	return nil
}

// channelText 通知渠道文本：标题 + 报告纯文本摘要。
// 完整 HTML 报告不适合作为聊天机器人文本消息，先转纯文本再按字节截断
// （企业微信 text 限制 2048 字节，中文 UTF-8 每字符 3 字节，保守取 1900 字节并回退到完整字符边界）
func channelText(title, content string) string {
	lines := stripHTML(content)
	var sb strings.Builder
	sb.WriteString(title)
	for _, l := range lines {
		sb.WriteByte('\n')
		sb.WriteString(l)
	}
	s := sb.String()
	b := []byte(s)
	if len(b) <= 1900 {
		return s
	}
	cut := 0
	for i := 0; i < len(b) && i < 1900; {
		_, size := utf8.DecodeRune(b[i:])
		if i+size > 1900 {
			break
		}
		i += size
		cut = i
	}
	return string(b[:cut]) + "\n…（详见完整报告）"
}

func badge(status, color string) string {
	return fmt.Sprintf("<span style='display:inline-block;padding:2px 10px;border-radius:10px;font-size:12px;font-weight:500;color:#fff;background:%s'>%s</span>", color, status)
}

func renderLinkSection(a *core.App, start, end time.Time, stats map[string]int) string {
	var tasks []model.LinkTask
	a.DB.Find(&tasks)
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【链路检测汇总】</h3><table><thead><tr><th>链路名称</th><th>目标地址</th><th>当前状态</th><th>平均时延</th><th>累计中断</th><th>可用率</th></tr></thead><tbody>`)
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
	sb.WriteString(`<div class="section"><h3>【服务器资源使用汇总】</h3><table><thead><tr><th>设备</th><th>IP</th><th>状态</th><th>CPU(均值)</th><th>内存使用(均值)</th></tr></thead><tbody>`)
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
	sb.WriteString(`<div class="section"><h3>【数据库状态汇总】</h3><table><thead><tr><th>实例</th><th>类型</th><th>状态</th><th>数据库数</th><th>错误数</th></tr></thead><tbody>`)
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

// renderNginxSection Nginx 状态汇总（读取 nginx_instances 表 + 采集器实时快照）
func renderNginxSection(a *core.App, stats map[string]int) string {
	var insts []model.NginxInstance
	a.DB.Order("id asc").Find(&insts)
	snapMap := map[uint]nginxprobe.Snapshot{}
	for _, s := range a.NginxM.SnapshotAll() {
		snapMap[s.InstanceID] = s
	}

	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【Nginx 状态汇总】</h3><table><thead><tr><th>实例</th><th>状态页地址</th><th>状态</th><th>活跃连接</th><th>累计请求</th><th>QPS(次/秒)</th></tr></thead><tbody>`)
	if len(insts) == 0 {
		sb.WriteString("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Nginx 监控实例</td></tr>")
	}
	for _, d := range insts {
		stats["total"]++
		status, color := "异常", "#f5222d"
		if s, ok := snapMap[d.ID]; ok && s.Up {
			status, color = "正常", "#52c41a"
			stats["up"]++
			sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td><td>%.1f</td></tr>",
				d.Name, d.StatusURL, badge(status, color), s.ActiveConns, s.Requests, s.QPS))
		} else {
			stats["down"]++
			sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td colspan='3' style='text-align:center;color:#999'>未采集</td></tr>",
				d.Name, d.StatusURL, badge(status, color)))
		}
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
	sb.WriteString(`<div class="section"><h3>【容器与 Kubernetes 汇总】</h3><table><thead><tr><th>节点</th><th>IP/地址</th><th>类型</th><th>状态</th><th>容器/Pod 数</th><th>异常数</th></tr></thead><tbody>`)

	if len(hosts) == 0 && len(clusters) == 0 {
		sb.WriteString("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Docker/K8s 节点监控</td></tr>")
	}

	dockerStatus := a.DockerM.HostStatus()
	containers := a.DockerM.Snapshot()
	// 统计每个 host 上的容器数和异常数
	containerCount := map[string]int{}
	abnormalCount := map[string]int{}
	for _, c := range containers {
		k := c.Host
		if k == "" {
			k = "本机"
		}
		containerCount[k]++
		if c.State != "running" {
			abnormalCount[k]++
		}
	}

	for _, h := range hosts {
		stats["total"]++
		online := false
		key := h.Address
		if key == "" || key == "本机" || strings.HasPrefix(key, "unix://") {
			key = "本机"
		}
		if v, ok := dockerStatus[key]; ok {
			online = v
		}
		if !online {
			if v, ok := dockerStatus[h.Address]; ok {
				online = v
			}
		}
		status, color := "离线", "#f5222d"
		if online {
			status, color = "在线", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		addr := strings.TrimPrefix(h.Address, "tcp://")
		if addr == "" {
			addr = "本机"
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>docker</td><td>%s</td><td>%d</td><td>%d</td></tr>",
			h.Name, addr, badge(status, color), containerCount[key], abnormalCount[key]))
	}

	k8sStatus := a.K8sM.ClusterStatus()
	for _, k := range clusters {
		stats["total"]++
		online := false
		if v, ok := k8sStatus[k.Name]; ok {
			online = v
		}
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
body{font-family:"Noto Sans CJK SC","WenQuanYi Micro Hei","WenQuanYi Zen Hei","Microsoft YaHei",sans-serif;margin:0;padding:32px;background:linear-gradient(180deg,#e9eef6 0%,#f5f7fa 100%);color:#2c3e50;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.wrap{max-width:920px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 4px 20px rgba(26,115,232,.10);overflow:hidden}
.header{padding:30px 40px 20px;background:linear-gradient(135deg,#1a73e8 0%,#0d47a1 100%);border-radius:12px 12px 0 0}
.header h1{margin:0;font-size:24px;font-weight:700;color:#fff;letter-spacing:1px}
.header .meta{margin-top:8px;font-size:13px;color:#d6e4ff}
.summary{width:calc(100% - 72px);margin:26px 36px 10px;border-collapse:separate;border-spacing:10px 0}
.summary td{width:25%;text-align:center;padding:16px 8px;background:#f6f9ff;border:1px solid #e4ecfb;border-radius:10px}
.summary .num{font-size:28px;font-weight:700;line-height:1.2}
.summary .lbl{font-size:12px;color:#7a8798;margin-top:6px}
.section{padding:22px 40px}
h3{margin:0 0 14px;font-size:16px;color:#1a73e8;padding:7px 12px;border-left:4px solid #1a73e8;background:#eef4ff;border-radius:4px}
table{width:100%;border-collapse:collapse;font-size:13px}
thead th{background:#1a73e8;padding:10px 12px;text-align:left;font-weight:600;color:#fff;border:none}
tbody td{padding:10px 12px;border-bottom:1px solid #eef1f5}
tbody tr:nth-child(even){background:#f7f9fc}
tbody tr:hover{background:#eef4ff}
.footer{padding:18px 40px;text-align:center;color:#8a97a8;font-size:12px;border-top:1px solid #eef1f5;background:#f7f9fc}
</style></head><body><div class="wrap">
<div class="header">
<h1>{{.Title}}</h1>
<div class="meta">巡检周期：{{.Start}} 至 {{.End}} ｜ 生成时间：{{.Now}}</div>
</div>
<table class="summary"><tr>
<td><div class="num" style="color:#1a73e8">{{.Total}}</div><div class="lbl">监控对象总数</div></td>
<td><div class="num" style="color:#52c41a">{{.Up}}</div><div class="lbl">正常</div></td>
<td><div class="num" style="color:#f5222d">{{.Down}}</div><div class="lbl">异常/离线</div></td>
<td><div class="num" style="color:#faad14">{{.Warn}}</div><div class="lbl">告警</div></td>
</tr></table>
{{.LinkSection}}{{.DevSection}}{{.DBSection}}{{.ContainerSection}}{{.NodeSection}}{{.NginxSection}}
<div class="footer">本报告由 NetOps 网络运维监控平台自动生成</div>
</div></body></html>`))

func renderHTML(title string, start, end time.Time, linkS, devS, dbS, containerS, nodeS, nginxS string, stats map[string]int) string {
	var sb strings.Builder
	reportTpl.Execute(&sb, map[string]any{
		"Title": title, "Start": start.Format("2006-01-02 15:04"), "End": end.Format("2006-01-02 15:04"),
		"Now":         time.Now().Format("2006-01-02 15:04:05"),
		"LinkSection": template.HTML(linkS), "DevSection": template.HTML(devS), "DBSection": template.HTML(dbS), "ContainerSection": template.HTML(containerS), "NodeSection": template.HTML(nodeS), "NginxSection": template.HTML(nginxS),
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

// stripHTML 去掉 HTML 标签，提取纯文本行
func stripHTML(html string) []string {
	html = strings.ReplaceAll(html, "<br", "\n<br")
	html = strings.ReplaceAll(html, "</tr>", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n")
	html = strings.ReplaceAll(html, "</h1>", "\n")
	html = strings.ReplaceAll(html, "</h2>", "\n")
	html = strings.ReplaceAll(html, "</h3>", "\n")
	var sb strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				sb.WriteRune(r)
			}
		}
	}
	var lines []string
	for _, l := range strings.Split(sb.String(), "\n") {
		l = strings.TrimSpace(l)
		l = strings.ReplaceAll(l, "&nbsp;", " ")
		l = strings.ReplaceAll(l, "&amp;", "&")
		l = strings.ReplaceAll(l, "&lt;", "<")
		l = strings.ReplaceAll(l, "&gt;", ">")
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// findWkhtmltopdf 查找 wkhtmltopdf：先在程序同目录，再在 PATH
func findWkhtmltopdf() (string, error) {
	name := "wkhtmltopdf"
	if runtime.GOOS == "windows" {
		name = "wkhtmltopdf.exe"
	}
	// 1. 程序同目录
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), name)
		if ifst, e := os.Stat(candidate); e == nil && !ifst.IsDir() {
			return candidate, nil
		}
	}
	// 2. 当前目录
	if ifst, e := os.Stat(name); e == nil && !ifst.IsDir() {
		return name, nil
	}
	// 3. PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 wkhtmltopdf，请安装后重试（Linux: apt/yum install wkhtmltopdf；Windows: 下载 wkhtmltopdf.exe 放到程序同目录）")
}

// renderPDF 调用 wkhtmltopdf 将报告 HTML 渲染为 PDF
func renderPDF(title, htmlContent string) ([]byte, error) {
	tmpDir := os.TempDir()
	htmlFile := filepath.Join(tmpDir, fmt.Sprintf("report-%d.html", time.Now().UnixNano()))
	pdfFile := filepath.Join(tmpDir, fmt.Sprintf("report-%d.pdf", time.Now().UnixNano()))
	defer os.Remove(htmlFile)
	defer os.Remove(pdfFile)
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		return nil, err
	}
	bin, err := findWkhtmltopdf()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin,
		"--quiet", "--disable-smart-shrinking",
		"--margin-top", "15mm", "--margin-bottom", "15mm",
		"--margin-left", "12mm", "--margin-right", "12mm",
		"--encoding", "utf-8",
		htmlFile, pdfFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("wkhtmltopdf: %v: %s", err, out)
	}
	return os.ReadFile(pdfFile)
}

func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, s)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// startReportScheduler 定时生成周报/月报：
//   - 每 10 分钟检查一次，每天首次检查（时间 >= 01:00）即执行当天任务，
//     服务在凌晨任意时刻启动/重启都不会错过（不再依赖整点 tick 对齐）
//   - lastRunDay 防同一天重复触发，DB 日期范围去重兜底
//   - 自动生成的报告同步通过已配置通知渠道发送（失败仅记日志，不影响入库）
func startReportScheduler(a *core.App) {
	go func() {
		tk := time.NewTicker(10 * time.Minute)
		defer tk.Stop()
		lastRunDay := ""
		for now := range tk.C {
			day := now.Format("2006-01-02")
			if now.Hour() < 1 || day == lastRunDay {
				continue
			}
			lastRunDay = day
			// 周一生成周报
			if now.Weekday() == time.Monday {
				generateAutoReport(a, "weekly")
			}
			// 每月 1 号生成月报
			if now.Day() == 1 {
				generateAutoReport(a, "monthly")
			}
		}
	}()
}

func generateAutoReport(a *core.App, kind string) {
	end := time.Now()
	start := end.Add(-7 * 24 * time.Hour)
	title := "自动巡检周报 " + end.Format("2006-01-02")
	if kind == "monthly" {
		start = end.Add(-30 * 24 * time.Hour)
		title = "自动巡检月报 " + end.Format("2006-01")
	}
	// 避免重复生成：当天已生成则跳过（日期范围查询，兼容 SQLite/MySQL/PostgreSQL/国产库）
	dayStart := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	var cnt int64
	a.DB.Model(&model.ReportRecord{}).
		Where("title = ? AND created_at >= ? AND created_at < ?", title, dayStart, dayStart.Add(24*time.Hour)).
		Count(&cnt)
	if cnt > 0 {
		return
	}
	stats := map[string]int{}
	linkS := renderLinkSection(a, start, end, stats)
	devS := renderDevSection(a, start, end, stats)
	dbS := renderDBSection(a, stats)
	containerS := renderContainerSection(a, stats)
	nodeS := renderNodeSection(a, stats)
	nginxS := renderNginxSection(a, stats)
	content := renderHTML(title, start, end, linkS, devS, dbS, containerS, nodeS, nginxS, stats)
	rec := model.ReportRecord{Title: title, StartAt: start, EndAt: end, Content: content, Creator: "system", SendEmail: true}
	if err := a.DB.Create(&rec).Error; err != nil {
		logger.Errorf("[report] 自动报告生成失败 %s: %v", title, err)
		return
	}
	// 自动发送到已配置的通知渠道（邮件 + 飞书/钉钉/企业微信）；失败仅记日志，报告仍入库可手动补发
	if err := sendReportViaChannels(a, title, content); err != nil {
		logger.Warnf("[report] 自动报告已生成但发送失败 %s: %v", title, err)
	} else {
		logger.Infof("[report] 自动报告已生成并发送 %s", title)
	}
}
