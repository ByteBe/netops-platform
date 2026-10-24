// Package aiinspect AI 巡检：汇总系统状态/告警/日志/指标，调大模型生成巡检报告
package aiinspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/ai"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.POST("/run", func(c *gin.Context) {
		if a.DB == nil {
			response.Fail(c, 500, 50000, "数据库未就绪")
			return
		}
		// 1. 收集系统快照（Go 采集）
		snapshot := collectSnapshot(a)

		// 2. 优先转发 Python 智能外围服务（AI 巡检增强：pandas 统计 + TDengine 趋势 + LLM）
		if a.Cfg.Server.PythonSvcURL != "" {
			if report, stats, err := callPythonSvc(a, snapshot); err == nil {
				// 存历史
				snapJSON, _ := json.Marshal(snapshot)
				a.DB.Create(&model.AIInspectReport{Report: report, Snapshot: string(snapJSON), CreatedAt: time.Now()})
				response.OK(c, gin.H{"report": report, "snapshot": snapshot, "stats": stats, "time": time.Now().Format("2006-01-02 15:04:05"), "source": "python"})
				return
			} else {
				fmt.Printf("[aiinspect] python 服务调用失败，回退内置 AI: %v\n", err)
			}
		}

		// 3. 找启用的 AI 配置
		var cfg model.AIConfig
		if a.DB.Where("enable = ?", true).Order("priority desc").First(&cfg).Error != nil {
			response.Fail(c, 400, 40000, "未配置启用的 AI，请先在系统设置中配置")
			return
		}

		// 3. 构造 prompt
		prompt := buildPrompt(snapshot)

		// 4. 调 AI
		provider := ai.Provider{
			Name: cfg.Name, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey,
			Model: cfg.Model, Temperature: cfg.Temperature,
		}
		result, err := ai.Chat(provider, []ai.Message{
			{Role: "system", Content: "你是资深网络运维专家，根据提供的系统数据生成简洁专业的中文巡检报告，包含：总体评价、发现的问题、风险点、建议措施。用 Markdown 格式。"},
			{Role: "user", Content: prompt},
		})
		if err != nil {
			response.Fail(c, 500, 50000, "AI调用失败: "+err.Error())
			return
		}
		// 5. 保存历史
		snapJSON, _ := json.Marshal(snapshot)
		a.DB.Create(&model.AIInspectReport{Report: result, Snapshot: string(snapJSON), CreatedAt: time.Now()})

		response.OK(c, gin.H{"report": result, "snapshot": snapshot, "time": time.Now().Format("2006-01-02 15:04:05")})
	})

	// 历史列表
	g.GET("/list", func(c *gin.Context) {
		var rows []model.AIInspectReport
		a.DB.Order("created_at desc").Limit(50).Find(&rows)
		type item struct {
			ID        uint      `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			Report    string    `json:"report"`
			Snapshot  string    `json:"snapshot"`
		}
		out := []item{}
		for _, r := range rows {
			out = append(out, item{ID: r.ID, CreatedAt: r.CreatedAt, Report: r.Report, Snapshot: r.Snapshot})
		}
		response.OK(c, out)
	})

	// 删除历史
	g.DELETE("/:id", func(c *gin.Context) {
		a.DB.Delete(&model.AIInspectReport{}, c.Param("id"))
		response.OK(c, gin.H{"ok": true})
	})
}

// callPythonSvc 调用 Python 智能外围服务（FastAPI /internal/aiinspect/run）
// 成功返回（报告, 统计增强数据, nil）；失败返回 error（由调用方回退内置 AI）
func callPythonSvc(a *core.App, snapshot Snapshot) (string, any, error) {
	url := strings.TrimRight(a.Cfg.Server.PythonSvcURL, "/") + "/internal/aiinspect/run"
	body, err := json.Marshal(map[string]any{
		"snapshot": snapshot,
		"time":     time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return "", nil, err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("python svc status %d", resp.StatusCode)
	}
	var out struct {
		Report   string `json:"report"`
		Stats    any    `json:"stats"`
		Time     string `json:"time"`
		Source   string `json:"source"`
		Snapshot any    `json:"snapshot"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(out.Report) == "" {
		return "", nil, fmt.Errorf("python svc empty report")
	}
	return out.Report, out.Stats, nil
}

type Snapshot struct {
	DeviceTotal   int64        `json:"device_total"`
	DeviceUp      int64        `json:"device_up"`
	DeviceDown    int64        `json:"device_down"`
	DeviceUnknown int64        `json:"device_unknown"`
	AlertActive   int64        `json:"alert_active"`
	AlertToday    int64        `json:"alert_today"`
	DockerTotal   int64        `json:"docker_total"`
	DockerRunning int64        `json:"docker_running"`
	LogError24h   int64        `json:"log_error_24h"`
	LogWarn24h    int64        `json:"log_warn_24h"`
	TopDevices    []string     `json:"top_devices"`
	Devices       []DeviceStat `json:"devices"` // 各设备资源明细（含本服务器）
}

// DeviceStat 单台设备（含本服务器）巡检明细
type DeviceStat struct {
	Name     string  `json:"name"`
	IP       string  `json:"ip"`
	Up       bool    `json:"up"`
	Status   string  `json:"status"` // up/down/unknown（来自设备表，供报告明细展示）
	CPU      float64 `json:"cpu"`
	Mem      float64 `json:"mem"`
	Disk     float64 `json:"disk"` // 最大挂载点使用率
	Uptime   float64 `json:"uptime_s"`
	IsLocal  bool    `json:"is_local"`
	Message  string  `json:"message"`
}

func collectSnapshot(a *core.App) Snapshot {
	var s Snapshot
	a.DB.Model(&model.MonitorDevice{}).Count(&s.DeviceTotal)
	a.DB.Model(&model.MonitorDevice{}).Where("status = ?", "up").Count(&s.DeviceUp)
	a.DB.Model(&model.MonitorDevice{}).Where("status = ?", "down").Count(&s.DeviceDown)
	a.DB.Model(&model.MonitorDevice{}).Where("status NOT IN (?)", []string{"up", "down"}).Count(&s.DeviceUnknown)
	a.DB.Model(&model.AlertEvent{}).Where("acked = ?", false).Count(&s.AlertActive)
	today := time.Now().Format("2006-01-02")
	a.DB.Model(&model.AlertEvent{}).Where("DATE(created_at) = ?", today).Count(&s.AlertToday)
	a.DB.Model(&model.DeviceLog{}).Where("created_at > ? AND level = ?", time.Now().Add(-24*time.Hour), "critical").Count(&s.LogError24h)
	a.DB.Model(&model.DeviceLog{}).Where("created_at > ? AND level = ?", time.Now().Add(-24*time.Hour), "warning").Count(&s.LogWarn24h)

	// 各设备（含本机，若已纳管）资源明细：CPU/内存/磁盘/运行时间
	// 先取设备表状态（ip -> status），供报告明细区分 unknown
	statusByIP := map[string]string{}
	{
		var rows []struct {
			IP     string
			Status string
		}
		if err := a.DB.Model(&model.MonitorDevice{}).Select("ip, status").Find(&rows).Error; err == nil {
			for _, r := range rows {
				statusByIP[r.IP] = r.Status
			}
		}
	}
	hasLocal := false
	for _, snap := range a.SnmpMgr.Snapshot() {
		ds := DeviceStat{
			Name: snap.Name, IP: snap.IP, Up: snap.Up,
			CPU: snap.CPU, Mem: snap.MemUsed, Uptime: snap.Uptime, Message: snap.Message,
		}
		ds.Status = statusByIP[snap.IP]
		if ds.Status == "" {
			if snap.Up {
				ds.Status = "up"
			} else {
				ds.Status = "unknown"
			}
		}
		if snap.IP == "127.0.0.1" || snap.IP == "::1" || snap.IP == "localhost" {
			ds.IsLocal = true
			hasLocal = true
		}
		for _, dk := range snap.Disks {
			if dk.UsedPct > ds.Disk {
				ds.Disk = dk.UsedPct
			}
		}
		s.Devices = append(s.Devices, ds)
	}
	// 本机未纳管时单独采集本机资源
	if !hasLocal {
		if h := collectLocalHost(); h != nil {
			s.Devices = append(s.Devices, *h)
		}
	}

	// 列出 down 的设备
	for _, d := range s.Devices {
		if !d.Up {
			s.TopDevices = append(s.TopDevices, fmt.Sprintf("%s(%s)", d.Name, d.IP))
		}
	}
	return s
}

// collectLocalHost 采集本服务器资源（/proc），用于本机未纳管时兜底
func collectLocalHost() *DeviceStat {
	ds := &DeviceStat{Name: "本服务器", IP: "127.0.0.1", Up: true, Status: "up", IsLocal: true}
	// CPU 使用率：两帧 /proc/stat
	if c1, t1, ok := readCPU(); ok {
		time.Sleep(100 * time.Millisecond)
		if c2, t2, ok := readCPU(); ok && t2 > t1 {
			ds.CPU = (1 - float64(c2-c1)/float64(t2-t1)) * 100
		}
	}
	// 内存
	if mem, ok := readMem(); ok {
		ds.Mem = mem
	}
	// 磁盘：取使用率最大的挂载点
	if pct, ok := readDisk(); ok {
		ds.Disk = pct
	}
	// 运行时间
	if up, ok := readUptime(); ok {
		ds.Uptime = up
	}
	return ds
}

func buildPrompt(s Snapshot) string {
	var b strings.Builder
	b.WriteString("## 系统巡检数据\n\n")
	fmt.Fprintf(&b, "- 设备总数：%d，在线：%d，离线：%d，未知：%d\n", s.DeviceTotal, s.DeviceUp, s.DeviceDown, s.DeviceUnknown)
	fmt.Fprintf(&b, "- 活跃告警：%d，今日告警：%d\n", s.AlertActive, s.AlertToday)
	fmt.Fprintf(&b, "- 24h 严重日志：%d 条，警告日志：%d 条\n", s.LogError24h, s.LogWarn24h)

	b.WriteString("\n### 设备资源明细：\n")
	if len(s.Devices) == 0 {
		b.WriteString("- 暂无设备数据\n")
	}
	for _, d := range s.Devices {
		who := "远程设备"
		if d.IsLocal {
			who = "本服务器"
		}
		status := "未知"
		switch d.Status {
		case "up":
			status = "在线"
		case "down":
			status = "离线"
		}
		fmt.Fprintf(&b, "- [%s] %s(%s)：%s，CPU %.1f%%，内存 %.1f%%，磁盘 %.1f%%，运行 %.0f 小时",
			who, d.Name, d.IP, status, d.CPU, d.Mem, d.Disk, d.Uptime/3600)
		if d.Message != "" {
			b.WriteString("，" + d.Message)
		}
		b.WriteString("\n")
	}

	if len(s.TopDevices) > 0 {
		b.WriteString("\n### 离线设备：\n")
		for _, d := range s.TopDevices {
			b.WriteString("- " + d + "\n")
		}
	}
	b.WriteString("\n请根据以上数据生成巡检报告，重点关注各设备的资源使用情况和离线设备。")
	return b.String()
}

func init() { modreg.RegisterProtected("aiinspect", RegisterProtected) }
