package distributed

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/core"
)

var (
	mqttOnce   sync.Once
	mqttClient mqtt.Client
)

// MQTT 配置从 system_settings 读取，支持运行时改
func mqttCfg(a *core.App) (broker, user, pass, ca string) {
	broker = getSetting(a, "mqtt_broker")
	user = getSetting(a, "mqtt_username")
	pass = getSetting(a, "mqtt_password")
	ca = getSetting(a, "mqtt_tls_ca")
	return
}

// tlsConfig 构建 MQTT TLS 配置：配置了自定义 CA（PEM）时用其校验服务端证书，
// 否则使用系统根证书池验证；不再跳过校验
func tlsConfig(caPEM string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if strings.TrimSpace(caPEM) == "" {
		return cfg, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("MQTT TLS CA 解析失败（需为标准 PEM 证书）")
	}
	cfg.RootCAs = pool
	return cfg, nil
}

// handleMqttCaUpload 校验并保存 MQTT TLS CA 证书（PEM）：解析格式、校验有效期，
// 校验通过后保存到系统配置 mqtt_tls_ca，供 MQTT 连接使用
func handleMqttCaUpload(a *core.App, c *gin.Context) {
	var req struct {
		CA string `json:"ca"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Bad(c, "参数错误")
		return
	}
	ca := strings.TrimSpace(req.CA)
	if ca == "" {
		response.OK(c, gin.H{"ok": false, "valid": false, "message": "CA 内容为空"})
		return
	}
	if len(ca) > 512*1024 {
		response.OK(c, gin.H{"ok": false, "valid": false, "message": "CA 内容过大（限 512KB）"})
		return
	}
	// 逐块解析 PEM：用 smx509（兼容国密 SM2 与标准证书）分类给出精确错误
	rest := []byte(ca)
	now := time.Now()
	var certs []gin.H
	var parseErr string
	var blockTypes []string
	var isSM2 bool
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		blockTypes = append(blockTypes, block.Type)
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := smx509.ParseCertificate(block.Bytes)
		if err != nil {
			parseErr = "证书解析失败: " + err.Error()
			break
		}
		// 识别算法：SM2（国密）证书不能用于标准 TLS 加密，需明确提示
		algorithm := "RSA/ECDSA"
		switch cert.PublicKey.(type) {
		case *rsa.PublicKey:
			algorithm = "RSA"
		case *ecdsa.PublicKey:
			if sm2.IsSM2PublicKey(cert.PublicKey) {
				algorithm = "SM2(国密)"
				isSM2 = true
			} else {
				algorithm = "ECDSA"
			}
		}
		st := "valid"
		if now.Before(cert.NotBefore) {
			st = "not-yet-valid"
		} else if now.After(cert.NotAfter) {
			st = "expired"
		}
		certs = append(certs, gin.H{
			"subject":    cert.Subject.String(),
			"issuer":     cert.Issuer.String(),
			"not_before": cert.NotBefore,
			"not_after":  cert.NotAfter,
			"serial":     cert.SerialNumber.String(),
			"status":     st,
			"algorithm":  algorithm,
		})
	}
	if parseErr != "" {
		response.OK(c, gin.H{"ok": false, "valid": false, "message": parseErr, "certs": certs})
		return
	}
	if len(certs) == 0 {
		msg := "未找到 PEM 证书块（BEGIN CERTIFICATE）"
		if len(blockTypes) > 0 {
			seen := make([]string, 0, len(blockTypes))
			uniq := map[string]bool{}
			for _, bt := range blockTypes {
				if !uniq[bt] {
					uniq[bt] = true
					seen = append(seen, bt)
				}
			}
			msg = fmt.Sprintf("文件包含 PEM 块：%s；未找到证书块（BEGIN CERTIFICATE）——请上传 CA 证书文件，而非私钥或证书请求", strings.Join(seen, " / "))
		}
		response.OK(c, gin.H{"ok": false, "valid": false, "message": msg, "certs": certs})
		return
	}
	// 国密 SM2 证书：用于节点身份证书体系（公有/私有证书），不能作为 MQTT TLS 加密的 CA
	if isSM2 {
		response.OK(c, gin.H{"ok": false, "valid": false, "message": "检测到国密 SM2 证书：SM2 证书用于节点身份证书体系（公有/私有证书）；MQTT TLS 加密请上传标准 TLS CA 证书（RSA/ECDSA PEM）", "certs": certs})
		return
	}
	for _, ct := range certs {
		if ct["status"] == "not-yet-valid" {
			response.OK(c, gin.H{"ok": false, "valid": false, "message": "证书尚未生效", "certs": certs})
			return
		}
		if ct["status"] == "expired" {
			response.OK(c, gin.H{"ok": false, "valid": false, "message": "证书已过期", "certs": certs})
			return
		}
	}
	// 校验通过：保存到系统配置（替换旧值），供 MQTT TLS 连接使用
	setSetting(a, "mqtt_tls_ca", ca)
	response.OK(c, gin.H{
		"ok": true, "valid": true, "saved": true,
		"message": "证书有效，已保存到系统",
		"certs":   certs,
	})
}

// StartMQTT 启动 MQTT 订阅（作为上级接收下级上报）和发布（作为下级上报）
func StartMQTT(a *core.App) {
	mqttOnce.Do(func() {
		broker, user, pass, ca := mqttCfg(a)
		if broker == "" {
			logger.Infof("[mqtt] broker 未配置，跳过 MQTT 启动")
			return
		}
		opts := mqtt.NewClientOptions()
		opts.AddBroker(broker)
		opts.SetClientID("netops-" + SelfUUID(a))
		if user != "" {
			opts.SetUsername(user)
			opts.SetPassword(pass)
		}
		opts.SetAutoReconnect(true)
		opts.SetConnectRetry(true)
		opts.SetConnectTimeout(5 * time.Second)
		// TLS（mqtts:// 自动启用）：按配置的 CA（或系统根证书）校验服务端证书
		if strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") {
			tc, terr := tlsConfig(ca)
			if terr != nil {
				logger.Warnf("[mqtt] TLS 配置无效（%v），改用系统根证书校验", terr)
				tc, _ = tlsConfig("")
			}
			opts.SetTLSConfig(tc)
		}
		opts.SetOnConnectHandler(func(c mqtt.Client) {
			topic := fmt.Sprintf("netops/+/up")
			if t := c.Subscribe(topic, 1, onIngest); t.Wait() && t.Error() != nil {
				logger.Warnf("[mqtt] subscribe %s failed: %v", topic, t.Error())
			} else {
				logger.Infof("[mqtt] subscribed %s", topic)
			}
		})
		mqttClient = mqtt.NewClient(opts)
		// ConnectRetry 模式下 Connect token 在失败后持续重试，若用 tok.Wait() 同步等待会无限阻塞主流程
		// （例如跨公网 broker 不可达时导致 web/api 服务无法启动）。改用 WaitTimeout：5 秒内未连上
		// 仅记录警告，paho 后台继续自动重连，连上后由 OnConnectHandler 完成订阅。
		tok := mqttClient.Connect()
		if tok.WaitTimeout(5*time.Second) && tok.Error() == nil {
			logger.Infof("[mqtt] connected to %s", broker)
		} else {
			if tok.Error() != nil {
				logger.Warnf("[mqtt] connect %s failed（后台自动重试中）: %v", broker, tok.Error())
			} else {
				logger.Warnf("[mqtt] connect %s 超时（后台自动重试中）", broker)
			}
		}
	})
}

// onIngest 处理下级通过 MQTT 上报的快照（与 HTTP /ingest 同一校验与持久化逻辑）
func onIngest(c mqtt.Client, msg mqtt.Message) {
	var req IngestRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil || req.NodeUUID == "" {
		return
	}
	// MQTT 回调拿不到 gin.Context，使用注册时保存的全局 app 引用
	if activeApp == nil {
		return
	}
	if !verifyIngestToken(activeApp, req.NodeUUID, req.Token) {
		logger.Warnf("[mqtt] token invalid from %s", req.NodeUUID)
		return
	}
	// 断开黑名单：因证书未导入被删除的节点拒绝上报，必须重新注册上线
	if isExpelled(activeApp, req.NodeUUID) {
		logger.Warnf("[mqtt] node %s expelled (cert not imported), re-register required", req.NodeUUID)
		return
	}
	// 证书指纹校验（特征标签），MQTT 无响应通道，上级证书下发依赖 HTTP /ingest 响应
	if !verifyCertFingerprint(activeApp, req.NodeUUID, req.CertFingerprint) {
		logger.Warnf("[mqtt] cert fingerprint mismatch from %s", req.NodeUUID)
		return
	}
	// 证书交换：登记下级公证书（若携带），再维护待导入时限（导入即停止计时）
	savePeerCert(activeApp, req.NodeUUID, req.Name, req.CertPEM)
	touchCertWait(activeApp, req.NodeUUID)
	// 更新或创建节点（心跳更新不覆盖已注册层级）
	upsertNode(activeApp, req.NodeUUID, req.Name, req.Level, len(req.Devices), len(req.Containers))
	// 与 HTTP 通道一致：持久化下级上报的明细，保证上级可查看
	upsertNodeDevices(activeApp, req.NodeUUID, req.Devices)
	upsertNodeContainers(activeApp, req.NodeUUID, req.Containers)
	upsertNodeLinks(activeApp, req.NodeUUID, req.Links)
	logger.Infof("[mqtt] ingest from %s: devices=%d containers=%d", req.Name, len(req.Devices), len(req.Containers))
}

func upsertNode(a *core.App, uuid, name string, level, devCnt, ctnCnt int) {
	now := time.Now()
	var existing struct {
		ID uint
	}
	if err := a.DB.Table("nodes").Where("node_uuid = ?", uuid).First(&existing).Error; err != nil {
		a.DB.Table("nodes").Create(map[string]interface{}{
			"node_uuid": uuid, "name": name, "level": level,
			"status": "online", "device_cnt": devCnt, "container_cnt": ctnCnt,
			"last_seen_at": now,
		})
	} else {
		// 层级在注册（/register）时确定，心跳上报不覆盖，避免 MQTT 上报把已注册节点层级重置为 0
		a.DB.Table("nodes").Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"name": name, "status": "online",
			"device_cnt": devCnt, "container_cnt": ctnCnt, "last_seen_at": now,
		})
	}
}

// PublishSnapshot 通过 MQTT 发布快照到上级（若已配置 MQTT）
func PublishSnapshot(a *core.App, payload map[string]interface{}) {
	if mqttClient == nil || !mqttClient.IsConnected() {
		return
	}
	// 下级必须导入上级下发的公有证书后才能上传（MQTT 通道同样强制）
	if getSetting(a, "parent_cert") == "" {
		logger.Warnf("[mqtt] 未导入上级下发的公有证书，拒绝 MQTT 发布（下级必须导入上级公钥后方可上传）")
		return
	}
	uuid := payload["node_uuid"].(string)
	topic := fmt.Sprintf("netops/%s/up", uuid)
	body, _ := json.Marshal(payload)
	if t := mqttClient.Publish(topic, 1, false, body); t.Wait() && t.Error() != nil {
		logger.Warnf("[mqtt] publish failed: %v", t.Error())
	}
}
