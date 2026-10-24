package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
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

		stats := map[string]int{"total": 0, "up": 0, "down": 0, "warn": 0}
		content := renderReport(a, title, start, end, include, stats)

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
		// 优先读取系统设置中的报告默认收件人 report_default_to
		var st model.SystemSetting
		if err := a.DB.Where("`key` = ?", "report_default_to").First(&st).Error; err == nil && st.Value != "" {
			for _, addr := range strings.Split(st.Value, ",") {
				if addr = strings.TrimSpace(addr); addr != "" {
					recipients = append(recipients, addr)
				}
			}
		}
		if len(recipients) == 0 && cfg0.DefaultTo != "" {
			for _, addr := range strings.Split(cfg0.DefaultTo, ",") {
				if addr = strings.TrimSpace(addr); addr != "" {
					recipients = append(recipients, addr)
				}
			}
		}
		if len(recipients) == 0 && cfg0.User != "" {
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

// stripBlocks 移除 <tag>...</tag> 块（CSS/JS 非正文，避免占满通知渠道文本截断预算）
func stripBlocks(s, tag string) string {
	start := "<" + tag
	end := "</" + tag + ">"
	for {
		i := strings.Index(s, start)
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], ">")
		if j < 0 {
			return s
		}
		openEnd := i + j + 1
		k := strings.Index(s[openEnd:], end)
		if k < 0 {
			return s
		}
		s = s[:i] + s[openEnd+k+len(end):]
	}
}

func renderLinkSection(a *core.App, start, end time.Time, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【链路检测汇总】</h3><table><thead><tr><th>链路名称</th><th>目标地址</th><th>当前状态</th><th>平均时延</th><th>累计中断</th><th>可用率</th></tr></thead><tbody>`)
	for _, r := range collectLinkRows(a, start, end, stats) {
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.1f ms</td><td>%d 次</td><td>%.2f%%</td></tr>",
			r["name"], r["target"], badge(fmt.Sprint(r["status"]), fmt.Sprint(r["color"])), r["avg_rt"], r["downs"], r["avail"]))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectLinkRows 链路检测数据收集（供 Python 渲染；本地 fallback 同源）
func collectLinkRows(a *core.App, start, end time.Time, stats map[string]int) []map[string]any {
	var tasks []model.LinkTask
	a.DB.Find(&tasks)
	var rows []map[string]any
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
		rows = append(rows, map[string]any{
			"name": t.Name, "target": t.Target, "status": status, "color": color,
			"avg_rt": avgRT, "downs": total - upCnt, "avail": avail,
		})
	}
	return rows
}

func renderDevSection(a *core.App, start, end time.Time, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【服务器资源使用汇总】</h3><table><thead><tr><th>设备</th><th>IP</th><th>状态</th><th>CPU(均值)</th><th>内存使用(均值)</th></tr></thead><tbody>`)
	for _, r := range collectDevRows(a, start, end, stats) {
		cpuColor := "#52c41a"
		cpu := r["cpu"].(float64)
		if cpu > 80 {
			cpuColor = "#f5222d"
		} else if cpu > 60 {
			cpuColor = "#faad14"
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td style='color:%s'>%.1f%%</td><td>%.1f%%</td></tr>",
			r["name"], r["ip"], badge(fmt.Sprint(r["status"]), fmt.Sprint(r["color"])), cpuColor, cpu, r["mem"]))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectDevRows 服务器资源数据收集（供 Python 渲染；本地 fallback 同源）
func collectDevRows(a *core.App, start, end time.Time, stats map[string]int) []map[string]any {
	var devices []model.MonitorDevice
	a.DB.Where("type = ?", "server").Or("type = ?", "other").Find(&devices)
	var rows []map[string]any
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
		rows = append(rows, map[string]any{"name": d.Name, "ip": d.IP, "status": status, "color": color, "cpu": cpu, "mem": mem})
	}
	return rows
}

func renderDBSection(a *core.App, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【数据库状态汇总】</h3><table><thead><tr><th>实例</th><th>类型</th><th>状态</th><th>数据库数</th><th>错误数</th></tr></thead><tbody>`)
	for _, r := range collectDBRows(a, stats) {
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td></tr>",
			r["name"], r["type"], badge(fmt.Sprint(r["status"]), fmt.Sprint(r["color"])), r["conns"], r["errors"]))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectDBRows 数据库状态数据收集（供 Python 渲染；本地 fallback 同源）
func collectDBRows(a *core.App, stats map[string]int) []map[string]any {
	var rows []map[string]any
	for _, s := range a.DBProbe.SnapshotAll() {
		stats["total"]++
		status, color := "异常", "#f5222d"
		if s.Up {
			status, color = "正常", "#52c41a"
			stats["up"]++
		} else {
			stats["down"]++
		}
		rows = append(rows, map[string]any{"name": s.Name, "type": s.Type, "status": status, "color": color, "conns": s.Conns, "errors": s.Errors})
	}
	return rows
}

// renderNginxSection Nginx 状态汇总（读取 nginx_instances 表 + 采集器实时快照）
func renderNginxSection(a *core.App, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【Nginx 状态汇总】</h3><table><thead><tr><th>实例</th><th>状态页地址</th><th>状态</th><th>活跃连接</th><th>累计请求</th><th>QPS(次/秒)</th></tr></thead><tbody>`)
	rows := collectNginxRows(a, stats)
	if len(rows) == 0 {
		sb.WriteString("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Nginx 监控实例</td></tr>")
	}
	for _, r := range rows {
		if r["up"].(bool) {
			sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td><td>%.1f</td></tr>",
				r["name"], r["status_url"], badge("正常", "#52c41a"), r["active_conns"], r["requests"], r["qps"]))
		} else {
			sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td colspan='3' style='text-align:center;color:#999'>未采集</td></tr>",
				r["name"], r["status_url"], badge("异常", "#f5222d")))
		}
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectNginxRows Nginx 状态数据收集（供 Python 渲染；本地 fallback 同源）
func collectNginxRows(a *core.App, stats map[string]int) []map[string]any {
	var insts []model.NginxInstance
	a.DB.Order("id asc").Find(&insts)
	snapMap := map[uint]nginxprobe.Snapshot{}
	for _, s := range a.NginxM.SnapshotAll() {
		snapMap[s.InstanceID] = s
	}
	var rows []map[string]any
	for _, d := range insts {
		stats["total"]++
		if s, ok := snapMap[d.ID]; ok && s.Up {
			stats["up"]++
			rows = append(rows, map[string]any{"name": d.Name, "status_url": d.StatusURL, "up": true,
				"active_conns": s.ActiveConns, "requests": s.Requests, "qps": s.QPS})
		} else {
			stats["down"]++
			rows = append(rows, map[string]any{"name": d.Name, "status_url": d.StatusURL, "up": false,
				"active_conns": float64(0), "requests": float64(0), "qps": float64(0)})
		}
	}
	return rows
}

func renderContainerSection(a *core.App, stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>【容器与 Kubernetes 汇总】</h3><table><thead><tr><th>节点</th><th>IP/地址</th><th>类型</th><th>状态</th><th>容器/Pod 数</th><th>异常数</th></tr></thead><tbody>`)
	rows := collectContainerRows(a, stats)
	if len(rows) == 0 {
		sb.WriteString("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Docker/K8s 节点监控</td></tr>")
	}
	for _, r := range rows {
		cnt, abnormal := "-", "-"
		if r["count"] != nil {
			cnt = fmt.Sprint(r["count"])
		}
		if r["abnormal"] != nil {
			abnormal = fmt.Sprint(r["abnormal"])
		}
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			r["name"], r["addr"], r["kind"], badge(fmt.Sprint(r["status"]), fmt.Sprint(r["color"])), cnt, abnormal))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectContainerRows 容器/K8s 状态数据收集（供 Python 渲染；本地 fallback 同源）
func collectContainerRows(a *core.App, stats map[string]int) []map[string]any {
	var hosts []model.DockerHost
	a.DB.Order("id asc").Find(&hosts)
	var clusters []model.K8sCluster
	a.DB.Order("id asc").Find(&clusters)

	dockerStatus := a.DockerM.HostStatus()
	containers := a.DockerM.Snapshot()
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

	var rows []map[string]any
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
		rows = append(rows, map[string]any{"name": h.Name, "addr": addr, "kind": "docker",
			"status": status, "color": color, "count": containerCount[key], "abnormal": abnormalCount[key]})
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
		rows = append(rows, map[string]any{"name": k.Name, "addr": k.APIServer, "kind": "k8s",
			"status": status, "color": color, "count": nil, "abnormal": nil})
	}
	return rows
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

// collectReportData 组装报告渲染所需全部结构化数据（Python 渲染使用；本地 fallback 同源）
func collectReportData(a *core.App, start, end time.Time, include map[string]bool, stats map[string]int) map[string]any {
	sections := []string{}
	if include["link"] {
		sections = append(sections, "link")
	}
	if include["monitor"] {
		sections = append(sections, "dev")
	}
	if include["db"] {
		sections = append(sections, "db")
	}
	if include["container"] {
		sections = append(sections, "container")
	}
	if include["node"] {
		sections = append(sections, "node")
	}
	if include["nginx"] {
		sections = append(sections, "nginx")
	}
	payload := map[string]any{
		"title":  "",
		"start":  start.Format("2006-01-02 15:04"),
		"end":    end.Format("2006-01-02 15:04"),
		"now":    time.Now().Format("2006-01-02 15:04:05"),
		"stats":  map[string]int{"total": stats["total"], "up": stats["up"], "down": stats["down"], "warn": stats["warn"]},
		"sections": sections,
		"link": collectLinkRows(a, start, end, stats),
		"dev": collectDevRows(a, start, end, stats),
		"db":  collectDBRows(a, stats),
		"nginx": collectNginxRows(a, stats),
		"container": collectContainerRows(a, stats),
		"node": collectNodeRows(a, stats),
	}
	return payload
}

// callPythonRender 转发 Python 渲染报告 HTML；失败返回 error（调用方回退本地渲染）
func callPythonRender(a *core.App, payload map[string]any) (string, error) {
	base := strings.TrimRight(a.Cfg.Server.PythonSvcURL, "/")
	if base == "" {
		return "", fmt.Errorf("python svc url empty")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(base+"/internal/report/render", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("python svc status %d", resp.StatusCode)
	}
	var out struct {
		HTML string `json:"html"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.HTML) < 200 {
		return "", fmt.Errorf("python render empty")
	}
	return out.HTML, nil
}

// renderReport 渲染报告 HTML：优先 Python（Jinja 模板渲染），失败自动回退本地渲染。
// 注意：Python 失败时本地渲染会再次更新 stats，故使用独立 stats 副本避免重复计数。
func renderReport(a *core.App, title string, start, end time.Time, include map[string]bool, stats map[string]int) string {
	pyStats := map[string]int{}
	payload := collectReportData(a, start, end, include, pyStats)
	payload["title"] = title
	if html, err := callPythonRender(a, payload); err == nil {
		for k, v := range pyStats {
			stats[k] = v
		}
		return html
	}
	linkS := renderLinkSection(a, start, end, stats)
	devS := renderDevSection(a, start, end, stats)
	dbS := renderDBSection(a, stats)
	containerS := renderContainerSection(a, stats)
	nodeS := renderNodeSection(a, stats)
	nginxS := renderNginxSection(a, stats)
	return renderHTML(title, start, end, linkS, devS, dbS, containerS, nodeS, nginxS, stats)
}

var reportTpl = template.Must(template.New("report").Parse(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>
*{box-sizing:border-box}
body{font-family:"Noto Sans CJK SC","WenQuanYi Micro Hei","Microsoft YaHei",sans-serif;margin:0;padding:0;background:#eef1f6;color:#2c3e50;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.wrap{max-width:960px;margin:0 auto;background:#fff;min-height:100vh;box-shadow:0 2px 16px rgba(15,76,129,.10)}
.header{position:relative;padding:34px 44px 26px;background:#0d3b66;background:linear-gradient(135deg,#0d3b66 0%,#1a73e8 55%,#3f9bff 100%);color:#fff;overflow:hidden}
.header:before{content:"";position:absolute;right:-70px;top:-70px;width:230px;height:230px;border-radius:50%;background:rgba(255,255,255,.09)}
.header:after{content:"";position:absolute;right:70px;bottom:-90px;width:170px;height:170px;border-radius:50%;background:rgba(255,255,255,.06)}
.header h1{margin:0;font-size:25px;font-weight:700;letter-spacing:2px;position:relative}
.header .meta{margin-top:10px;font-size:12.5px;color:#d3e3ff;position:relative}
.header .meta span{margin-right:22px}
.header .tag{display:inline-block;margin-top:14px;padding:4px 14px;border-radius:20px;background:rgba(255,255,255,.16);font-size:12px;color:#fff;border:1px solid rgba(255,255,255,.35);position:relative}
.summary{width:calc(100% - 80px);margin:22px 40px 6px;border-collapse:separate;border-spacing:12px 0}
.summary td{width:25%;text-align:center;padding:16px 8px;background:#f8fbff;border:1px solid #e3edfc;border-radius:10px}
.summary .num{font-size:30px;font-weight:800;line-height:1.15}
.summary .lbl{font-size:12px;color:#7a8798;margin-top:5px}
.section{padding:18px 40px 4px}
h3{margin:0 0 14px;font-size:15px;color:#0d3b66;padding:8px 14px;border-left:4px solid #1a73e8;background:#eef4ff;background:linear-gradient(90deg,#eef4ff 0%,#f8fbff 100%);border-radius:0 6px 6px 0;letter-spacing:1px}
table{width:100%;border-collapse:collapse;font-size:12.5px;box-shadow:0 1px 5px rgba(15,76,129,.07);border-radius:8px;overflow:hidden}
thead th{background:#0d3b66;background:linear-gradient(180deg,#1a73e8 0%,#0d3b66 100%);padding:10px 12px;text-align:left;font-weight:600;color:#fff;border:none}
tbody td{padding:9px 12px;border-bottom:1px solid #eef1f5}
tbody tr:nth-child(even){background:#f7f9fc}
tbody tr:hover{background:#eef4ff}
.footer{padding:16px 40px;text-align:center;color:#9aa7b8;font-size:11px;letter-spacing:1px;border-top:1px solid #eef1f5;background:#fafbfd}
@media print{body{background:#fff}.wrap{box-shadow:none}}
</style></head><body><div class="wrap">
<div class="header">
<h1>{{.Title}}</h1>
<div class="meta"><span>巡检周期：{{.Start}} 至 {{.End}}</span><span>生成时间：{{.Now}}</span></div>
<div class="tag">NetOps 网络运维监控平台 · 自动生成</div>
</div>
<table class="summary"><tr>
<td><div class="num" style="color:#1a73e8">{{.Total}}</div><div class="lbl">监控对象总数</div></td>
<td><div class="num" style="color:#34a853">{{.Up}}</div><div class="lbl">正常</div></td>
<td><div class="num" style="color:#ea4335">{{.Down}}</div><div class="lbl">异常/离线</div></td>
<td><div class="num" style="color:#f9ab00">{{.Warn}}</div><div class="lbl">告警</div></td>
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
	var sb strings.Builder
	sb.WriteString(`<div class="section"><h3>节点健康性</h3><table><thead><tr><th>节点名称</th><th>UUID</th><th>层级</th><th>状态</th><th>设备数</th><th>容器数</th><th>最后上报</th></tr></thead><tbody>`)
	for _, r := range collectNodeRows(a, stats) {
		sb.WriteString(fmt.Sprintf("<tr><td><b>%s</b></td><td><code>%s</code></td><td>%s</td><td><span style='color:%s'>%s</span></td><td>%d</td><td>%d</td><td>%s</td></tr>",
			r["name"], r["uuid"], r["level"], r["color"], r["status"], r["device_cnt"], r["container_cnt"], r["last_seen"]))
	}
	sb.WriteString("</tbody></table></div>")
	return sb.String()
}

// collectNodeRows 节点健康数据收集（供 Python 渲染；本地 fallback 同源）
func collectNodeRows(a *core.App, stats map[string]int) []map[string]any {
	var nodes []model.Node
	a.DB.Order("level asc, id asc").Find(&nodes)
	var rows []map[string]any
	for _, n := range nodes {
		stats["total"]++
		color := "#52c41a"
		status := "在线"
		if n.Status != "online" {
			color = "#f5222d"
			status = "离线"
			stats["down"]++
		} else {
			stats["up"]++
		}
		level := []string{"总部", "省级", "市级", "县级"}[n.Level]
		if n.Level >= 4 {
			level = fmt.Sprintf("L%d", n.Level)
		}
		rows = append(rows, map[string]any{"name": n.Name, "uuid": n.NodeUUID, "level": level,
			"status": status, "color": color, "device_cnt": n.DeviceCnt, "container_cnt": n.ContainerCnt,
			"last_seen": n.LastSeenAt.Format("2006-01-02 15:04:05")})
	}
	return rows
}

func init() {
	modreg.RegisterProtected("report", RegisterProtected)
}

// stripHTML 去掉 HTML 标签，提取纯文本行（先剥离 style/script 块）
func stripHTML(html string) []string {
	html = stripBlocks(html, "style")
	html = stripBlocks(html, "script")
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
//   - 每 10 分钟检查一次，每天首次检查（时间 >= 20:00）即执行当天任务，
//     服务在晚间任意时刻启动/重启都不会错过（不再依赖整点 tick 对齐）
//   - lastRunDay 防同一天重复触发，DB 日期范围去重兜底
//   - 错过补发：周五未生成周报时（近 7 天无自动周报且系统已运行超一周），
//     其余日期自动补发；月报同理（近 30 天无自动月报且运行超 30 天）
//   - 自动生成的报告同步通过已配置通知渠道发送（失败仅记日志，不影响入库）
func startReportScheduler(a *core.App) {
	go func() {
		tk := time.NewTicker(10 * time.Minute)
		defer tk.Stop()
		lastRunDay := ""
		for now := range tk.C {
			day := now.Format("2006-01-02")
			if now.Hour() < 20 || day == lastRunDay {
				continue
			}
			lastRunDay = day
			// 周报：周五 20:00 正常生成；其余日期但近 7 天无自动周报且系统运行超一周 → 补发
			if !hasRecentAutoReport(a, "自动巡检周报", 7*24*time.Hour, now) {
				if now.Weekday() == time.Friday || sysDataAge(a) >= 7*24*time.Hour {
					generateAutoReport(a, "weekly", now)
				}
			}
			// 月报：每月 1 号生成；错过且近 30 天无自动月报且系统运行超 30 天 → 补发
			if now.Day() == 1 || (!hasRecentAutoReport(a, "自动巡检月报", 30*24*time.Hour, now) && sysDataAge(a) >= 30*24*time.Hour) {
				generateAutoReport(a, "monthly", now)
			}
		}
	}()
}

// hasRecentAutoReport 最近 dur 内是否存在指定前缀的自动报告
func hasRecentAutoReport(a *core.App, prefix string, dur time.Duration, now time.Time) bool {
	var cnt int64
	a.DB.Model(&model.ReportRecord{}).Where("title LIKE ? AND created_at >= ?", prefix+"%", now.Add(-dur)).Count(&cnt)
	return cnt > 0
}

// sysDataAge 系统数据最早时间（以系统设置表最早记录时间近似），用于判断是否有足够历史补发报告
func sysDataAge(a *core.App) time.Duration {
	var st model.SystemSetting
	if err := a.DB.Order("updated_at asc").First(&st).Error; err != nil {
		return 0
	}
	return time.Since(st.UpdatedAt)
}

func generateAutoReport(a *core.App, kind string, now time.Time) {
	end := now
	start := end.Add(-7 * 24 * time.Hour)
	catchup := ""
	if kind == "weekly" && end.Weekday() != time.Friday {
		catchup = "(补发)"
	}
	title := "自动巡检周报" + catchup + " " + end.Format("2006-01-02")
	prefix := "自动巡检周报"
	if kind == "monthly" {
		start = end.Add(-30 * 24 * time.Hour)
		if end.Day() != 1 {
			catchup = "(补发)"
		}
		title = "自动巡检月报" + catchup + " " + end.Format("2006-01")
		prefix = "自动巡检月报"
	}
	// 避免重复生成：当天已生成则跳过（日期范围 + 前缀查询，兼容 SQLite/MySQL/PostgreSQL/国产库）
	dayStart := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	var cnt int64
	a.DB.Model(&model.ReportRecord{}).
		Where("title LIKE ? AND created_at >= ? AND created_at < ?", prefix+"%", dayStart, dayStart.Add(24*time.Hour)).
		Count(&cnt)
	if cnt > 0 {
		return
	}
	stats := map[string]int{}
	content := renderReport(a, title, start, end, nil, stats)
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
