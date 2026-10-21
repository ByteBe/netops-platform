// Package distributed 分布式级联模块（总部-省-市-县）
// 下级节点通过 HTTP/MQTT 把本机采集到的设备/容器/链路快照上报到上级
// 上级节点存储时带 source_node_uuid，前端按节点树展示
package distributed

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/certsvc"
)

var (
	selfUUID  string
	once      sync.Once
	activeApp *core.App
)

// SelfUUID 返回本节点 UUID（首次启动生成并持久化到 system_settings）
func SelfUUID(a *core.App) string {
	once.Do(func() {
		var s model.SystemSetting
		if err := a.DB.Where("`key` = ?", "self_node_uuid").First(&s).Error; err != nil || s.Value == "" {
			b := make([]byte, 16)
			rand.Read(b)
			selfUUID = hex.EncodeToString(b)
			a.DB.Where("`key` = ?", "self_node_uuid").Assign(model.SystemSetting{Value: selfUUID}).FirstOrCreate(&model.SystemSetting{Key: "self_node_uuid", Value: selfUUID})
		} else {
			selfUUID = s.Value
		}
	})
	return selfUUID
}

// IngestRequest 下级上报快照的统一载荷（HTTP 与 MQTT 通道共用）
// 证书字段：下级携带本机公证书（cert_pem）与指纹特征标签（cert_fingerprint），
// 上级据此登记/校验节点证书，并在响应中下发上级公证书完成交换
// ViaUUID：多级转发标记——中间节点转发间接下级数据时填自身 UUID，
// 上级据此只对直接下级做"3 分钟证书待导入"强制（证书互信发生在相邻级）
type IngestRequest struct {
	NodeUUID        string                   `json:"node_uuid"`
	Token           string                   `json:"token"`
	Name            string                   `json:"name"`
	Level           int                      `json:"level"`
	CertPEM         string                   `json:"cert_pem"`         // 本机公证书（上级登记用）
	CertFingerprint string                   `json:"cert_fingerprint"` // 证书指纹特征标签
	ViaUUID         string                   `json:"via_uuid"`
	Devices         []map[string]interface{} `json:"devices"`
	Containers      []map[string]interface{} `json:"containers"`
	Links           []map[string]interface{} `json:"links"`
}

// toPayload 转换为 MQTT 发布等场景使用的 map 载荷
func (r IngestRequest) toPayload() map[string]interface{} {
	return map[string]interface{}{
		"node_uuid": r.NodeUUID, "token": r.Token, "name": r.Name, "level": r.Level,
		"cert_pem": r.CertPEM, "cert_fingerprint": r.CertFingerprint, "via_uuid": r.ViaUUID,
		"devices": r.Devices, "containers": r.Containers, "links": r.Links,
	}
}

// verifyIngestToken 校验上报 token：节点独立 token 优先，未注册节点回退全局 cluster_token
func verifyIngestToken(a *core.App, nodeUUID, token string) bool {
	var node model.Node
	if err := a.DB.Where("node_uuid = ?", nodeUUID).First(&node).Error; err == nil && node.Token != "" {
		return token == node.Token
	}
	expected := getSetting(a, "cluster_token")
	// 集群 token 未配置时拒绝未注册节点的上报，防止伪造 node_uuid 注入数据
	if expected == "" {
		return false
	}
	return token == expected
}

func RegisterPublic(a *core.App, g *gin.RouterGroup) {
	// 下级注册到上级（公开，靠 token 鉴权）
	g.POST("/register", func(c *gin.Context) {
		var req struct {
			NodeUUID string `json:"node_uuid"`
			Name     string `json:"name"`
			Level    int    `json:"level"`
			Token    string `json:"token"`
			CertPEM  string `json:"cert_pem"` // 下级公证书（登记用）
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.NodeUUID == "" {
			response.Bad(c, "参数缺失")
			return
		}
		// token 校验：必须配置共享 cluster_token 且匹配（首次注册用）；
		// 未配置视为集群安全未就绪，拒绝注册，避免任意节点混入
		expected := getSetting(a, "cluster_token")
		if expected == "" || req.Token != expected {
			response.Unauthorized(c, "token 无效")
			return
		}
		// 证书是必须的，没有明文传输的通道：注册必须携带本机公证书（上级据此登记）；
		// 本机证书可独立生成（不依赖上级公钥），请在系统设置-节点证书点击"生成本机私有证书"
		if strings.TrimSpace(req.CertPEM) == "" {
			response.Bad(c, "注册必须携带本机证书（证书必须，无明文通道）；请在系统设置-节点证书点击“生成本机私有证书”")
			return
		}
		var n model.Node
		if err := a.DB.Where("node_uuid = ?", req.NodeUUID).First(&n).Error; err != nil {
			// 新节点：生成独立 token
			b := make([]byte, 16)
			rand.Read(b)
			nodeToken := hex.EncodeToString(b)
			n = model.Node{NodeUUID: req.NodeUUID, Name: req.Name, Level: req.Level, Status: "online", Token: nodeToken}
			a.DB.Create(&n)
		} else {
			a.DB.Model(&n).Updates(map[string]interface{}{"name": req.Name, "level": req.Level, "status": "online", "last_seen_at": time.Now()})
		}
		// 证书交换：登记下级公证书，并把本机公证书下发给下级（先登记，导入即停止计时）
		savePeerCert(a, req.NodeUUID, req.Name, req.CertPEM)
		// 证书待导入时限：已导入证书则清除标记，未导入则从首次注册/上报起计时（超 3 分钟断开删除）
		touchCertWait(a, req.NodeUUID)
		// 重新注册成功：清除断开黑名单，允许重新上线
		clearExpelled(a, req.NodeUUID)
		logger.Infof("[distributed] node registered: %s (%s)", req.Name, req.NodeUUID)
		response.OK(c, gin.H{
			"ok": true, "node_id": n.ID, "node_token": n.Token,
			"parent_uuid": SelfUUID(a),
			"parent_cert": certsvc.PublicPEM(a.DB), "parent_fingerprint": selfFP(a),
		})
	})

	// 下级上报快照（设备/容器/链路）
	g.POST("/ingest", func(c *gin.Context) {
		var req IngestRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.NodeUUID == "" {
			response.Bad(c, "参数缺失")
			return
		}
		if !verifyIngestToken(a, req.NodeUUID, req.Token) {
			response.Unauthorized(c, "token 无效")
			return
		}
		// 断开黑名单：因证书未导入被删除的节点拒绝上报，必须重新注册上线
		if isExpelled(a, req.NodeUUID) {
			response.Unauthorized(c, "节点已因证书未导入被断开删除，请重新注册上线")
			return
		}
		// 证书指纹校验（特征标签）：已登记证书的节点必须携带匹配指纹，防伪造
		if !verifyCertFingerprint(a, req.NodeUUID, req.CertFingerprint) {
			response.Unauthorized(c, "证书指纹不匹配")
			return
		}
		// 证书交换：登记下级公证书（若携带），再维护待导入时限（导入即停止计时）
		savePeerCert(a, req.NodeUUID, req.Name, req.CertPEM)
		// 仅直接下级做 3 分钟证书待导入强制；多级转发（via_uuid 非空）的节点
		// 由转发节点负责互认，上级不强制（避免总部误删三级单位）
		if req.ViaUUID == "" {
			touchCertWait(a, req.NodeUUID)
		}
		// 更新节点心跳
		a.DB.Model(&model.Node{}).Where("node_uuid = ?", req.NodeUUID).
			Updates(map[string]interface{}{"status": "online", "last_seen_at": time.Now(), "container_cnt": len(req.Containers), "device_cnt": len(req.Devices)})
		// 持久化下级设备到本库（按 node_uuid+IP upsert，含详细监控信息）
		upsertNodeDevices(a, req.NodeUUID, req.Devices)
		// 持久化下级容器明细（按 node_uuid+name upsert）
		upsertNodeContainers(a, req.NodeUUID, req.Containers)
		// 持久化下级链路明细（按 node_uuid+name upsert）
		upsertNodeLinks(a, req.NodeUUID, req.Links)
		// 多级转发：本节点如果也有上级，把收到的下级数据再转发给上级
		if parent := getSetting(a, "parent_url"); parent != "" {
			go ForwardToParent(a, req)
		}
		response.OK(c, gin.H{
			"ok": true, "devices": len(req.Devices), "containers": len(req.Containers), "links": len(req.Links),
			// 证书交换：响应下发的上级公证书，下级保存后完成互信
			"parent_uuid": SelfUUID(a),
			"parent_cert": certsvc.PublicPEM(a.DB), "parent_fingerprint": selfFP(a),
		})
	})
}

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	activeApp = a
	registerCertHandlers(a, g)
	// 本节点信息
	g.GET("/self", func(c *gin.Context) {
		mode := getSetting(a, "deploy_mode")
		if mode == "" {
			mode = "standalone"
		}
		// 公有证书（上级 CA）不回传明文：仅返回是否已配置与摘要信息
		caSet, caInfo := certSummary(a, "mqtt_tls_ca")
		response.OK(c, gin.H{
			"node_uuid":      SelfUUID(a),
			"name":           getSetting(a, "self_node_name"),
			"parent":         getSetting(a, "parent_url"),
			"token":          getSetting(a, "cluster_token"),
			"mqtt_broker":    getSetting(a, "mqtt_broker"),
			"mqtt_username":  getSetting(a, "mqtt_username"),
			"mqtt_password":  getSetting(a, "mqtt_password"),
			"mqtt_tls_ca_set": caSet,
			"mqtt_tls_ca_info": caInfo,
			"deploy_mode":    mode,
		})
	})
	// 下级节点列表
	g.GET("/nodes", func(c *gin.Context) {
		var nodes []model.Node
		a.DB.Order("level asc, id asc").Find(&nodes)
		now := time.Now()
		out := []gin.H{}
		for i := range nodes {
			st := "offline"
			if nodes[i].LastSeenAt != nil && now.Sub(*nodes[i].LastSeenAt) < 60*time.Second {
				st = "online"
			}
			out = append(out, gin.H{
				"id":            nodes[i].ID,
				"node_uuid":     nodes[i].NodeUUID,
				"name":          nodes[i].Name,
				"level":         nodes[i].Level,
				"status":        st,
				"group":         nodes[i].Group,
				"device_cnt":    nodes[i].DeviceCnt,
				"container_cnt": nodes[i].ContainerCnt,
				"last_seen_at":  nodes[i].LastSeenAt,
			})
		}
		response.OK(c, out)
	})
	g.POST("/nodes", func(c *gin.Context) {
		var n model.Node
		if err := c.ShouldBindJSON(&n); err != nil {
			response.Bad(c, err.Error())
			return
		}
		a.DB.Create(&n)
		response.OK(c, n)
	})
	g.DELETE("/nodes/:id", func(c *gin.Context) {
		var n model.Node
		if err := a.DB.First(&n, c.Param("id")).Error; err != nil {
			response.Bad(c, "节点不存在")
			return
		}
		// 级联清理该下级节点上报的设备（避免删除节点后残留孤儿设备）
		if n.NodeUUID != "" {
			a.DB.Where("node_uuid = ?", n.NodeUUID).Delete(&model.MonitorDevice{})
		}
		a.DB.Delete(&model.Node{}, n.ID)
		response.OK(c, gin.H{"ok": true})
	})
	// 测试上报链路
	g.POST("/push-now", func(c *gin.Context) {
		go PushOnce(a)
		response.OK(c, gin.H{"ok": true})
	})
	// 下级节点容器明细
	g.GET("/nodes/:id/containers", func(c *gin.Context) {
		var n model.Node
		if err := a.DB.First(&n, c.Param("id")).Error; err != nil {
			response.NotFound(c, "节点不存在")
			return
		}
		var list []model.NodeContainer
		a.DB.Where("node_uuid = ?", n.NodeUUID).Order("id asc").Find(&list)
		response.OK(c, list)
	})
	// 下级节点链路明细
	g.GET("/nodes/:id/links", func(c *gin.Context) {
		var n model.Node
		if err := a.DB.First(&n, c.Param("id")).Error; err != nil {
			response.NotFound(c, "节点不存在")
			return
		}
		var list []model.NodeLink
		a.DB.Where("node_uuid = ?", n.NodeUUID).Order("id asc").Find(&list)
		response.OK(c, list)
	})
	// 节点健康度汇总
	g.GET("/health", func(c *gin.Context) {
		var nodes []model.Node
		a.DB.Find(&nodes)
		now := time.Now()
		online, offline := 0, 0
		devTotal, ctnTotal := 0, 0
		for _, n := range nodes {
			if n.LastSeenAt != nil && now.Sub(*n.LastSeenAt) < 60*time.Second {
				online++
			} else {
				offline++
			}
			devTotal += n.DeviceCnt
			ctnTotal += n.ContainerCnt
		}
		response.OK(c, gin.H{
			"total": len(nodes), "online": online, "offline": offline,
			"device_total": devTotal, "container_total": ctnTotal,
		})
	})
	// 切换部署模式后重启进程
	g.POST("/restart", func(c *gin.Context) {
		response.OK(c, gin.H{"ok": true})
		go func() {
			time.Sleep(500 * time.Millisecond)
			exe, _ := os.Executable()
			cmd := exec.Command(exe)
			cmd.Start()
			os.Exit(0)
		}()
	})
	g.POST("/deploy-mode", func(c *gin.Context) {
		var req struct {
			Mode string `json:"mode"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, err.Error())
			return
		}
		if req.Mode != "standalone" && req.Mode != "distributed" {
			response.Bad(c, "mode 必须为 standalone 或 distributed")
			return
		}
		var s model.SystemSetting
		if err := a.DB.Where("`key` = ?", "deploy_mode").First(&s).Error; err != nil {
			a.DB.Create(&model.SystemSetting{Key: "deploy_mode", Value: req.Mode})
		} else {
			a.DB.Model(&s).Update("value", req.Mode)
		}
		response.OK(c, gin.H{"ok": true, "mode": req.Mode})
	})
	// 校验并保存 MQTT TLS CA 证书（PEM）：上传后保存到系统，供 MQTT 连接使用
	g.POST("/mqtt-ca-upload", func(c *gin.Context) {
		handleMqttCaUpload(a, c)
	})
	// 移除 MQTT TLS CA 证书配置（公有证书）
	g.POST("/mqtt-ca-remove", func(c *gin.Context) {
		a.DB.Where("`key` = ?", "mqtt_tls_ca").Delete(&model.SystemSetting{})
		response.OK(c, gin.H{"ok": true})
	})
	// 检测 MQTT Broker 连通性
	g.POST("/mqtt-check", func(c *gin.Context) {
		var req struct {
			Broker string `json:"broker"`
		}
		c.ShouldBindJSON(&req)
		broker := req.Broker
		if broker == "" {
			broker = getSetting(a, "mqtt_broker")
		}
		if broker == "" {
			response.OK(c, gin.H{"ok": false, "error": "未配置 Broker 地址"})
			return
		}
		user := getSetting(a, "mqtt_username")
		pass := getSetting(a, "mqtt_password")
		opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID("netops-check-" + time.Now().Format("150405")).SetConnectTimeout(5 * time.Second)
		if user != "" {
			opts.SetUsername(user).SetPassword(pass)
		}
		// TLS：按配置的 CA（或系统根证书）校验，与启动链路一致
		if strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") {
			if tc, terr := tlsConfig(getSetting(a, "mqtt_tls_ca")); terr == nil {
				opts.SetTLSConfig(tc)
			}
		}
		cli := mqtt.NewClient(opts)
		if tok := cli.Connect(); tok.Wait() && tok.Error() == nil {
			cli.Disconnect(100)
			response.OK(c, gin.H{"ok": true, "broker": broker})
		} else {
			response.OK(c, gin.H{"ok": false, "error": tok.Error().Error()})
		}
	})
}

func getSetting(a *core.App, k string) string {
	var s model.SystemSetting
	if err := a.DB.Where("`key` = ?", k).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// setSetting 保存系统设置（不存在则创建）；写库失败需记录，避免"谎报成功"
func setSetting(a *core.App, k, v string) {
	if v == "" {
		return
	}
	if err := a.DB.Where("`key` = ?", k).Assign(model.SystemSetting{Value: v}).FirstOrCreate(&model.SystemSetting{Key: k, Value: v}).Error; err != nil {
		logger.Errorf("[distributed] 保存系统设置 %s 失败: %v", k, err)
	}
}

// upsertNodeDevices 持久化下级设备（含详细监控信息），并删除已消失设备
func upsertNodeDevices(a *core.App, nodeUUID string, devices []map[string]interface{}) {
	liveIPs := make([]string, 0, len(devices))
	for _, d := range devices {
		ip, _ := d["ip"].(string)
		if ip == "" {
			continue
		}
		liveIPs = append(liveIPs, ip)
		status := "unknown"
		if st, ok := d["status"].(string); ok && st != "" {
			status = st
		} else if on, ok := d["online"].(bool); ok {
			if on {
				status = "up"
			} else {
				status = "down"
			}
		}
		var dev model.MonitorDevice
		if err := a.DB.Where("node_uuid = ? AND ip = ?", nodeUUID, ip).First(&dev).Error; err != nil {
			dev = model.MonitorDevice{Name: firstStr(d["name"]), IP: ip, Type: firstStr(d["type"]), NodeUUID: nodeUUID}
			if err := a.DB.Create(&dev).Error; err != nil {
				continue
			}
		}
		cpu, _ := d["cpu"].(float64)
		mem, _ := d["mem"].(float64)
		up, _ := d["uptime"].(float64)
		online, _ := d["online"].(bool)
		a.DB.Model(&dev).Updates(map[string]interface{}{
			"name": firstStr(d["name"]), "type": firstStr(d["type"]), "vendor": firstStr(d["vendor"]),
			"status": status, "last_online": online, "last_cpu": cpu, "last_mem": mem,
			"last_uptime": up, "last_msg": firstStr(d["message"]),
		})
	}
	if len(liveIPs) == 0 {
		a.DB.Where("node_uuid = ?", nodeUUID).Delete(&model.MonitorDevice{})
		return
	}
	a.DB.Where("node_uuid = ?", nodeUUID).Where("ip NOT IN ?", liveIPs).Delete(&model.MonitorDevice{})
}

// upsertNodeContainers 持久化下级容器明细，并删除已消失容器
func upsertNodeContainers(a *core.App, nodeUUID string, containers []map[string]interface{}) {
	liveNames := make([]string, 0, len(containers))
	for _, ct := range containers {
		name, _ := ct["name"].(string)
		if name == "" {
			continue
		}
		liveNames = append(liveNames, name)
		var row model.NodeContainer
		if err := a.DB.Where("node_uuid = ? AND name = ?", nodeUUID, name).First(&row).Error; err != nil {
			row = model.NodeContainer{NodeUUID: nodeUUID, Name: name}
			if err := a.DB.Create(&row).Error; err != nil {
				continue
			}
		}
		cpu, _ := ct["cpu"].(float64)
		mem, _ := ct["mem_pct"].(float64)
		a.DB.Model(&row).Updates(map[string]interface{}{
			"image": firstStr(ct["image"]), "state": firstStr(ct["state"]),
			"cpu": cpu, "mem_pct": mem, "host": firstStr(ct["host"]),
		})
	}
	if len(liveNames) == 0 {
		a.DB.Where("node_uuid = ?", nodeUUID).Delete(&model.NodeContainer{})
		return
	}
	a.DB.Where("node_uuid = ?", nodeUUID).Where("name NOT IN ?", liveNames).Delete(&model.NodeContainer{})
}

// upsertNodeLinks 持久化下级链路明细，并删除已消失链路
func upsertNodeLinks(a *core.App, nodeUUID string, links []map[string]interface{}) {
	liveNames := make([]string, 0, len(links))
	for _, l := range links {
		name, _ := l["name"].(string)
		if name == "" {
			continue
		}
		liveNames = append(liveNames, name)
		var row model.NodeLink
		if err := a.DB.Where("node_uuid = ? AND name = ?", nodeUUID, name).First(&row).Error; err != nil {
			row = model.NodeLink{NodeUUID: nodeUUID, Name: name}
			if err := a.DB.Create(&row).Error; err != nil {
				continue
			}
		}
		port, _ := l["port"].(float64)
		a.DB.Model(&row).Updates(map[string]interface{}{
			"target": firstStr(l["target"]), "method": firstStr(l["method"]),
			"port": int(port), "status": firstStr(l["status"]),
		})
	}
	if len(liveNames) == 0 {
		a.DB.Where("node_uuid = ?", nodeUUID).Delete(&model.NodeLink{})
		return
	}
	a.DB.Where("node_uuid = ?", nodeUUID).Where("name NOT IN ?", liveNames).Delete(&model.NodeLink{})
}

// firstStr 安全取字符串（nil/类型不符返回空串）
func firstStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func init() {
	modreg.Register("distributed", RegisterPublic)
	modreg.RegisterProtected("distributed", RegisterProtected)
}
