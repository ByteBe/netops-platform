package distributed

import (
	"time"

	"netops/internal/common/logger"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/service/certsvc"
)

// lastNoCertWarn 无证书告警去抖：避免每 30 秒刷屏（上报周期内持续提示一次/5分钟）
var lastNoCertWarn time.Time

// PushOnce 把本机采集到的最新快照上报到上级（全部经 MQTT 通道）
func PushOnce(a *core.App) {
	parent := getSetting(a, "parent_url")
	if parent == "" {
		return // 未配置上级（如总部根节点）：无上报需求
	}
	// 证书是必须的，没有明文传输的通道：
	// ① 本机私有证书未生成 → 拒绝上报；
	// ② 下级必须导入上级下发的公有证书（parent_cert）→ 未导入则拒绝上传（HTTPS 上报与 MQTT 发布）
	if selfFP(a) == "" {
		if time.Since(lastNoCertWarn) > 5*time.Minute {
			lastNoCertWarn = time.Now()
			logger.Warnf("[distributed] 本机私有证书未生成，拒绝明文上报：请在系统设置-节点证书点击“生成本机私有证书”")
		}
		return
	}
	if getSetting(a, "parent_cert") == "" {
		if time.Since(lastNoCertWarn) > 5*time.Minute {
			lastNoCertWarn = time.Now()
			logger.Warnf("[distributed] 未导入上级下发的公有证书，拒绝上传（HTTPS/MQTT）：下级必须导入上级公钥后方可上报数据")
		}
		return
	}
	req := IngestRequest{
		NodeUUID:        SelfUUID(a),
		Token:           getSetting(a, "cluster_token"),
		Name:            getSetting(a, "self_node_name"),
		CertPEM:         certsvc.PublicPEM(a.DB), // 本机公证书：上级登记
		CertFingerprint: selfFP(a),               // 证书指纹特征标签
	}

	// 收集本机设备快照（含详细监控信息）
	var devices []model.MonitorDevice
	a.DB.Where("enable = ?", true).Find(&devices)
	devArr := []map[string]interface{}{}
	for _, d := range devices {
		item := map[string]interface{}{
			"name": d.Name, "ip": d.IP, "type": d.Type, "status": d.Status,
			"vendor": d.Vendor,
		}
		if s, ok := a.SnmpMgr.DeviceSnapshotByID(d.ID); ok {
			item["online"] = s.Up
			item["cpu"] = s.CPU
			item["mem"] = s.MemUsed
			item["uptime"] = s.Uptime
			item["message"] = s.Message
			item["if_count"] = len(s.Interfaces)
		} else {
			item["online"] = false
			item["cpu"] = 0
			item["mem"] = 0
			item["uptime"] = 0
			item["message"] = "未采集"
			item["if_count"] = 0
		}
		devArr = append(devArr, item)
	}
	req.Devices = devArr
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
	req.Containers = ctnArr
	// 链路（含状态）
	var links []model.LinkTask
	a.DB.Find(&links)
	linkArr := []map[string]interface{}{}
	for _, l := range links {
		linkArr = append(linkArr, map[string]interface{}{
			"name": l.Name, "target": l.Target, "method": l.Method, "port": l.Port, "status": l.Status,
		})
	}
	req.Links = linkArr

	// 数据上报全部经 MQTT 通道（WebSocket 承载）；HTTPS 级联已移除——
	// 证书是必须的，没有明文传输的通道，不再走 HTTP 上报
	PublishSnapshot(a, req.toPayload())
}

// StartPusher 启动定时上报 goroutine
func StartPusher(a *core.App) {
	StartCert(a) // 证书系统：本机证书初始化 + 每 4 小时周期双向认证
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
		if days == 0 {
			days = 7
		}
		next = next.AddDate(0, 0, days)
		time.Sleep(next.Sub(now))
		a.DB.Exec("INSERT INTO report_records (title, start_time, end_time, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			"每周巡检周报（自动）", now.AddDate(0, 0, -7).Format("2006-01-02 15:04:05"),
			now.Format("2006-01-02 15:04:05"), "done", now, now)
	}
}
