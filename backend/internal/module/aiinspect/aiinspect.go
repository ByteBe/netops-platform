// Package aiinspect AI 巡检：汇总系统状态/告警/日志/指标，调大模型生成巡检报告
package aiinspect

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.POST("/run", func(c *gin.Context) {
		if a.DB == nil {
			response.Fail(c, 500, 50000, "数据库未就绪")
			return
		}
		// 1. 收集系统快照（Go 采集）
		snapshot := collectSnapshot(a)

		// 2. 转发 Python 智能外围服务（唯一实现：pandas 统计 + TDengine 趋势 + LLM）
		report, stats, err := callPythonSvc(a, snapshot)
		if err != nil {
			// Python 智能外围不可用：明确失败（不再内置兜底）
			fmt.Printf("[aiinspect] python 服务不可用: %v\n", err)
			response.Fail(c, 502, 50200, "AI 智能服务不可用，请检查 Python 智能外围（netops-python）服务状态")
			return
		}
		// 存历史
		snapJSON, _ := json.Marshal(snapshot)
		a.DB.Create(&model.AIInspectReport{Report: report, Snapshot: string(snapJSON), CreatedAt: time.Now()})
		response.OK(c, gin.H{"report": report, "snapshot": snapshot, "stats": stats, "time": time.Now().Format("2006-01-02 15:04:05"), "source": "python"})
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

// callPythonSvc 调用 Python 智能外围服务（统一转发层 pysvc，FastAPI /internal/aiinspect/run）
// 成功返回（报告, 统计增强数据, nil）；失败返回 error（由调用方回退内置 AI）
func callPythonSvc(a *core.App, snapshot Snapshot) (string, any, error) {
	var out struct {
		Report   string `json:"report"`
		Stats    any    `json:"stats"`
		Time     string `json:"time"`
		Source   string `json:"source"`
		Snapshot any    `json:"snapshot"`
	}
	err := a.PySvc.PostJSON("/internal/aiinspect/run", map[string]any{
		"snapshot": snapshot,
		"time":     time.Now().Format("2006-01-02 15:04:05"),
	}, &out, 120*time.Second)
	if err != nil {
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

func init() { modreg.RegisterProtected("aiinspect", RegisterProtected) }
