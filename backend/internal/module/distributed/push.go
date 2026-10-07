package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"netops/internal/common/logger"
	"netops/internal/core"
	"netops/internal/model"
)

// PushOnce 把本机采集到的最新快照上报到上级
func PushOnce(a *core.App) {
	parent := getSetting(a, "parent_url")
	if parent == "" {
		return
	}
	token := getSetting(a, "cluster_token")
	name := getSetting(a, "self_node_name")
	uuid := SelfUUID(a)

	// 收集本机设备快照
	var devices []model.MonitorDevice
	a.DB.Where("enable = ?", true).Find(&devices)
	devArr := []map[string]interface{}{}
	for _, d := range devices {
		devArr = append(devArr, map[string]interface{}{
			"name": d.Name, "ip": d.IP, "type": d.Type, "status": d.Status,
		})
	}
	// 容器快照
	var ctnArr []map[string]interface{}
	if a.DockerM != nil {
		for _, c := range a.DockerM.Snapshot() {
			ctnArr = append(ctnArr, map[string]interface{}{
				"name": c.Names, "image": c.Image, "state": c.State,
				"cpu": c.CPU, "mem_pct": c.MemPct, "host": c.Host,
			})
		}
	}
	// 链路
	var links []model.LinkTask
	a.DB.Find(&links)
	linkArr := []map[string]interface{}{}
	for _, l := range links {
		linkArr = append(linkArr, map[string]interface{}{"name": l.Name, "target": l.Target})
	}

	payload := map[string]interface{}{
		"node_uuid":  uuid,
		"token":      token,
		"name":       name,
		"devices":    devArr,
		"containers": ctnArr,
		"links":      linkArr,
	}
	body, _ := json.Marshal(payload)
	// 优先 MQTT，失败回退 HTTP
	PublishSnapshot(a, payload)
	url := fmt.Sprintf("%s/api/v1/distributed/ingest", parent)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Warnf("[distributed] push failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Warnf("[distributed] push http %d", resp.StatusCode)
		return
	}
	logger.Infof("[distributed] pushed: devices=%d containers=%d links=%d", len(devArr), len(ctnArr), len(linkArr))
}

// ForwardToParent 把下级上报的数据再转发给本节点的上级（多级级联）
func ForwardToParent(a *core.App, req struct {
	NodeUUID  string                   `json:"node_uuid"`
	Token     string                   `json:"token"`
	Name      string                   `json:"name"`
	Devices   []map[string]interface{} `json:"devices"`
	Containers []map[string]interface{} `json:"containers"`
	Links     []map[string]interface{} `json:"links"`
}) {
	parent := getSetting(a, "parent_url")
	if parent == "" {
		return
	}
	token := getSetting(a, "cluster_token")
	payload := map[string]interface{}{
		"node_uuid":  req.NodeUUID,
		"token":      token,
		"name":       req.Name,
		"devices":    req.Devices,
		"containers": req.Containers,
		"links":      req.Links,
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v1/distributed/ingest", parent)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Warnf("[distributed] forward to parent failed: %v", err)
		return
	}
	defer resp.Body.Close()
	logger.Infof("[distributed] forwarded %s -> parent: %d devices", req.Name, len(req.Devices))
}

// StartPusher 启动定时上报 goroutine
func StartPusher(a *core.App) {
	StartMQTT(a)
	go startWeeklyReport(a)
	interval := 30 * time.Second
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		PushOnce(a)
		for range t.C {
			PushOnce(a)
		}
	}()
}

// startWeeklyReport 每周一 09:00 自动生成巡检周报记录
func startWeeklyReport(a *core.App) {
	defer func() { recover() }()
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		days := (8 - int(now.Weekday())) % 7
		if days == 0 { days = 7 }
		next = next.AddDate(0, 0, days)
		time.Sleep(next.Sub(now))
		a.DB.Exec("INSERT INTO report_records (title, start_time, end_time, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			"每周巡检周报（自动）", now.AddDate(0, 0, -7).Format("2006-01-02 15:04:05"),
			now.Format("2006-01-02 15:04:05"), "done", now, now)
	}
}
