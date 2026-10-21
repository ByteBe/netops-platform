package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"netops/internal/common/logger"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/service/certsvc"
)

// httpClientForParent 构造上报用的 HTTP 客户端：
// 配置了 MQTT TLS CA 时将其作为信任锚（自签上级也可验证），确保 https:// 上级地址
// 的数据上报走 TLS 加密；未配置或 CA 无效时用系统根证书池（https 默认校验）。
// 信任锚构建复用 tlsConfig()，与 MQTT 通道保持同一套 CA 校验逻辑。
// 每次调用新建 client：TLS CA 可能运行时变更，不缓存。
func httpClientForParent(a *core.App) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tc, err := tlsConfig(getSetting(a, "mqtt_tls_ca")); err == nil {
		transport.TLSClientConfig = tc
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: transport}
}

// lastNoCertWarn 无证书告警去抖：避免每 30 秒刷屏（上报周期内持续提示一次/5分钟）
var lastNoCertWarn time.Time

// PushOnce 把本机采集到的最新快照上报到上级
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

	// 优先 MQTT 发布，再 HTTP 上报（MQTT 失败时 HTTP 兜底）
	PublishSnapshot(a, req.toPayload())
	postIngestToParent(a, req)
}

// postIngestToParent 把上报载荷 POST 到上级 ingest 接口（PushOnce 与 ForwardToParent 共用）
func postIngestToParent(a *core.App, req IngestRequest) {
	parent := getSetting(a, "parent_url")
	if parent == "" {
		return
	}
	// 证书是必须的，没有明文传输的通道：上报必须携带证书指纹（特征标签）
	if req.CertFingerprint == "" {
		logger.Warnf("[distributed] 上报缺少证书指纹（证书必须），拒绝明文通道 node=%s", shortFP(req.NodeUUID))
		return
	}
	// 下级必须导入上级下发的公有证书后才能上传（含多级转发路径）
	if getSetting(a, "parent_cert") == "" {
		logger.Warnf("[distributed] 未导入上级下发的公有证书，拒绝上传（HTTPS）node=%s", shortFP(req.NodeUUID))
		return
	}
	// 转发载荷：身份与证书必须一致——
	// PushOnce 场景 req 为本机构造（node_uuid=本机+本机证书）；
	// ForwardToParent 场景 req 为下级原始上报（node_uuid=下级+下级证书），
	// 因此证书一律取自 req，保证上级按 node_uuid 校验指纹时匹配
	payload := map[string]interface{}{
		"node_uuid":        req.NodeUUID,
		"token":            getSetting(a, "cluster_token"),
		"name":             req.Name,
		"cert_pem":         req.CertPEM,
		"cert_fingerprint": req.CertFingerprint,
		"via_uuid":         req.ViaUUID,
		"devices":          req.Devices,
		"containers":       req.Containers,
		"links":            req.Links,
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v1/distributed/ingest", strings.TrimRight(parent, "/"))
	client := httpClientForParent(a)
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Warnf("[distributed] push to parent failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Warnf("[distributed] push to parent http %d", resp.StatusCode)
		return
	}
	// 证书交换：解析上级下发的公证书并保存，完成互信（上级→下级方向）
	var ack struct {
		ParentUUID        string `json:"parent_uuid"`
		ParentCert        string `json:"parent_cert"`
		ParentFingerprint string `json:"parent_fingerprint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		logger.Warnf("[distributed] 上级响应解析失败: %v", err)
	} else if ack.ParentCert != "" {
		if ack.ParentUUID != "" {
			setSetting(a, "parent_node_uuid", ack.ParentUUID)
		}
		// 上级公钥同时写入 parent_cert 配置与 peer 证书表：
		// parent_cert 是本机"上级公钥已导入"的权威判定，页面显示与上传强制均读它
		setSetting(a, "parent_cert", ack.ParentCert)
		savePeerCert(a, ack.ParentUUID, "上级节点", ack.ParentCert)
	}
	logger.Infof("[distributed] pushed to parent: devices=%d containers=%d links=%d", len(req.Devices), len(req.Containers), len(req.Links))
}

// ForwardToParent 把下级上报的数据再转发给本节点的上级（多级级联）
func ForwardToParent(a *core.App, req IngestRequest) {
	// 标记转发者：上级只对直接下级做 3 分钟证书强制，间接节点由本级互认
	req.ViaUUID = SelfUUID(a)
	postIngestToParent(a, req)
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
