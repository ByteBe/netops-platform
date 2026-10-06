package distributed

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"netops/internal/common/logger"
	"netops/internal/core"
)

var (
	mqttOnce   sync.Once
	mqttClient mqtt.Client
)

// MQTT 配置从 system_settings 读取，支持运行时改
func mqttCfg(a *core.App) (broker, user, pass string) {
	broker = getSetting(a, "mqtt_broker")
	user = getSetting(a, "mqtt_username")
	pass = getSetting(a, "mqtt_password")
	return
}

// StartMQTT 启动 MQTT 订阅（作为上级接收下级上报）和发布（作为下级上报）
func StartMQTT(a *core.App) {
	mqttOnce.Do(func() {
		broker, user, pass := mqttCfg(a)
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
		// TLS（mqtts:// 自动启用，跳过校验；生产可指定证书）
		if strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") {
			opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
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
		if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
			logger.Warnf("[mqtt] connect %s failed: %v", broker, t.Error())
			mqttClient = nil
			return
		}
		logger.Infof("[mqtt] connected to %s", broker)
	})
}

// onIngest 处理下级通过 MQTT 上报的快照
func onIngest(c mqtt.Client, msg mqtt.Message) {
	var req struct {
		NodeUUID   string                   `json:"node_uuid"`
		Token      string                   `json:"token"`
		Name       string                   `json:"name"`
		Level      int                      `json:"level"`
		Devices    []map[string]interface{} `json:"devices"`
		Containers []map[string]interface{} `json:"containers"`
		Links      []map[string]interface{} `json:"links"`
	}
	if err := json.Unmarshal(msg.Payload(), &req); err != nil || req.NodeUUID == "" {
		return
	}
	// 从 topic 取 app 引用比较麻烦，这里用全局指针
	if activeApp == nil {
		return
	}
	expected := getSetting(activeApp, "cluster_token")
	if expected != "" && req.Token != expected {
		logger.Warnf("[mqtt] token invalid from %s", req.NodeUUID)
		return
	}
	// 更新或创建节点
	activeApp.DB.Model(nil).Where(nil) // noop, keep
	upsertNode(activeApp, req.NodeUUID, req.Name, req.Level, len(req.Devices), len(req.Containers))
	logger.Infof("[mqtt] ingest from %s: devices=%d containers=%d", req.Name, len(req.Devices), len(req.Containers))
}

func upsertNode(a *core.App, uuid, name string, level, devCnt, ctnCnt int) {
	now := time.Now()
	type nodeRow struct {
		NodeUUID     string
		Name         string
		Level        int
		Status       string
		DeviceCnt    int
		ContainerCnt int
		LastSeenAt   time.Time
	}
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
		a.DB.Table("nodes").Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"name": name, "level": level, "status": "online",
			"device_cnt": devCnt, "container_cnt": ctnCnt, "last_seen_at": now,
		})
	}
	_ = nodeRow{}
}

// PublishSnapshot 通过 MQTT 发布快照到上级（若已配置 MQTT）
func PublishSnapshot(a *core.App, payload map[string]interface{}) {
	if mqttClient == nil || !mqttClient.IsConnected() {
		return
	}
	uuid := payload["node_uuid"].(string)
	topic := fmt.Sprintf("netops/%s/up", uuid)
	body, _ := json.Marshal(payload)
	if t := mqttClient.Publish(topic, 1, false, body); t.Wait() && t.Error() != nil {
		logger.Warnf("[mqtt] publish failed: %v", t.Error())
	}
}
