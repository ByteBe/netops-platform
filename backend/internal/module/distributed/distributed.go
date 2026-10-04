// Package distributed 分布式级联模块（总部-省-市-县）
// 下级节点通过 HTTP/MQTT 把本机采集到的设备/容器/链路快照上报到上级
// 上级节点存储时带 source_node_uuid，前端按节点树展示
package distributed

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

var (
	selfUUID string
	once     sync.Once
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

func RegisterPublic(a *core.App, g *gin.RouterGroup) {
	// 下级注册到上级（公开，靠 token 鉴权）
	g.POST("/register", func(c *gin.Context) {
		var req struct {
			NodeUUID string `json:"node_uuid"`
			Name     string `json:"name"`
			Level    int    `json:"level"`
			Token    string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.NodeUUID == "" {
			response.Bad(c, "参数缺失"); return
		}
		// token 校验：上级在配置里预设一个共享 token（首次注册用）
		expected := getSetting(a, "cluster_token")
		if expected != "" && req.Token != expected {
			response.Unauthorized(c, "token 无效"); return
		}
		var n model.Node
		if err := a.DB.Where("node_uuid = ?", req.NodeUUID).First(&n).Error; err != nil {
			// 新节点：生成独立 token
			b := make([]byte, 16)
			rand.Read(b)
			nodeToken := hex.EncodeToString(b)
			n = model.Node{NodeUUID: req.NodeUUID, Name: req.Name, Level: req.Level, Status: "online", Token: nodeToken}
			a.DB.Create(&n)
			response.OK(c, gin.H{"ok": true, "node_id": n.ID, "node_token": nodeToken})
		} else {
			a.DB.Model(&n).Updates(map[string]interface{}{"name": req.Name, "level": req.Level, "status": "online", "last_seen_at": time.Now()})
			response.OK(c, gin.H{"ok": true, "node_id": n.ID, "node_token": n.Token})
		}
		logger.Infof("[distributed] node registered: %s (%s)", req.Name, req.NodeUUID)
	})

	// 下级上报快照（设备/容器/链路）
	g.POST("/ingest", func(c *gin.Context) {
		var req struct {
			NodeUUID string                 `json:"node_uuid"`
			Token    string                 `json:"token"`
			Name     string                 `json:"name"`
			Devices  []map[string]interface{} `json:"devices"`
			Containers []map[string]interface{} `json:"containers"`
			Links    []map[string]interface{} `json:"links"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.NodeUUID == "" {
			response.Bad(c, "参数缺失"); return
		}
		// 校验：先查节点独立 token，没有则回退全局 cluster_token
		var node model.Node
		if err := a.DB.Where("node_uuid = ?", req.NodeUUID).First(&node).Error; err == nil && node.Token != "" {
			if req.Token != node.Token {
				response.Unauthorized(c, "节点 token 无效"); return
			}
		} else {
			expected := getSetting(a, "cluster_token")
			if expected != "" && req.Token != expected {
				response.Unauthorized(c, "token 无效"); return
			}
		}
		// 更新节点心跳
		a.DB.Model(&model.Node{}).Where("node_uuid = ?", req.NodeUUID).
			Updates(map[string]interface{}{"status": "online", "last_seen_at": time.Now(), "container_cnt": len(req.Containers), "device_cnt": len(req.Devices)})
		// 持久化下级设备到本库（按 node_uuid+IP upsert）
		for _, d := range req.Devices {
			ip, _ := d["ip"].(string)
			if ip == "" { continue }
			var dev model.MonitorDevice
			if err := a.DB.Where("node_uuid = ? AND ip = ?", req.NodeUUID, ip).First(&dev).Error; err != nil {
				dev = model.MonitorDevice{
					Name: fmt.Sprint(d["name"]),
					IP:   ip,
					Type: fmt.Sprint(d["type"]),
					NodeUUID: req.NodeUUID,
				}
				a.DB.Create(&dev)
			} else {
				a.DB.Model(&dev).Updates(map[string]interface{}{"name": d["name"], "type": d["type"]})
			}
		}
		// 删除本节点已不存在的旧设备
		a.DB.Where("node_uuid = ?", req.NodeUUID).Where("ip NOT IN ?", func() []string {
			ips := []string{}
			for _, d := range req.Devices { if ip, ok := d["ip"].(string); ok { ips = append(ips, ip) } }
			return ips
		}()).Delete(&model.MonitorDevice{})
		// 多级转发：本节点如果也有上级，把收到的下级数据再转发给上级
		if parent := getSetting(a, "parent_url"); parent != "" {
			go ForwardToParent(a, req)
		}
		response.OK(c, gin.H{"ok": true, "devices": len(req.Devices), "containers": len(req.Containers), "links": len(req.Links)})
	})
}

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	activeApp = a
	// 本节点信息
	g.GET("/self", func(c *gin.Context) {
		mode := getSetting(a, "deploy_mode")
		if mode == "" {
			mode = "standalone"
		}
		response.OK(c, gin.H{
			"node_uuid": SelfUUID(a),
			"name":      getSetting(a, "self_node_name"),
			"parent":    getSetting(a, "parent_url"),
			"token":     getSetting(a, "cluster_token"),
			"mqtt_broker":   getSetting(a, "mqtt_broker"),
			"mqtt_username": getSetting(a, "mqtt_username"),
			"mqtt_password": getSetting(a, "mqtt_password"),
			"deploy_mode":   mode,
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
				"device_cnt":    nodes[i].DeviceCnt,
				"container_cnt": nodes[i].ContainerCnt,
				"last_seen_at":  nodes[i].LastSeenAt,
			})
		}
		response.OK(c, out)
	})
	g.POST("/nodes", func(c *gin.Context) {
		var n model.Node
		if err := c.ShouldBindJSON(&n); err != nil { response.Bad(c, err.Error()); return }
		a.DB.Create(&n)
		response.OK(c, n)
	})
	g.DELETE("/nodes/:id", func(c *gin.Context) {
		a.DB.Delete(&model.Node{}, c.Param("id"))
		response.OK(c, gin.H{"ok": true})
	})
	// 测试上报链路
	g.POST("/push-now", func(c *gin.Context) {
		go PushOnce(a)
		response.OK(c, gin.H{"ok": true})
	})
	// 节点分组列表
	g.GET("/groups", func(c *gin.Context) {
		var groups []string
		a.DB.Model(&model.Node{}).Where("`group` != ''").Distinct("`group`").Pluck("`group`", &groups)
		// 合并 system_settings 里预建的分组
		var ss model.SystemSetting
		extra := ""
		if a.DB.Where("`key` = ?", "node_groups").First(&ss).Error == nil {
			extra = ss.Value
		}
		merged := map[string]bool{}
		for _, g := range groups { merged[g] = true }
		for _, g := range strings.Split(extra, ",") { if g = strings.TrimSpace(g); g != "" { merged[g] = true } }
		out := []string{}
		for g := range merged { out = append(out, g) }
		response.OK(c, out)
	})
	// 预建分组
	g.POST("/groups", func(c *gin.Context) {
		var req struct{ Group string `json:"group"` }
		c.ShouldBindJSON(&req)
		var ss model.SystemSetting
		cur := ""
		if a.DB.Where("`key` = ?", "node_groups").First(&ss).Error == nil { cur = ss.Value }
		existing := strings.Split(cur, ",")
		for _, e := range existing { if strings.TrimSpace(e) == req.Group { response.OK(c, gin.H{"ok":true}); return } }
		newVal := req.Group
		if cur != "" { newVal = cur + "," + req.Group }
		if a.DB.Where("`key` = ?", "node_groups").First(&ss).Error != nil {
			a.DB.Create(&model.SystemSetting{Key: "node_groups", Value: newVal})
		} else {
			a.DB.Model(&ss).Update("value", newVal)
		}
		response.OK(c, gin.H{"ok": true})
	})
	// 删除预建分组
	g.POST("/groups/delete", func(c *gin.Context) {
		var req struct{ Group string `json:"group"` }
		c.ShouldBindJSON(&req)
		a.DB.Model(&model.Node{}).Where("`group` = ?", req.Group).Update("group", "")
		var ss model.SystemSetting
		if a.DB.Where("`key` = ?", "node_groups").First(&ss).Error == nil {
			parts := []string{}
			for _, p := range strings.Split(ss.Value, ",") { if strings.TrimSpace(p) != req.Group && strings.TrimSpace(p) != "" { parts = append(parts, strings.TrimSpace(p)) } }
			a.DB.Model(&ss).Update("value", strings.Join(parts, ","))
		}
		response.OK(c, gin.H{"ok": true})
	})
	// 修改节点分组
	g.POST("/nodes/:id/group", func(c *gin.Context) {
		var req struct {
			Group string `json:"group"`
		}
		c.ShouldBindJSON(&req)
		a.DB.Model(&model.Node{}).Where("id = ?", c.Param("id")).Update("group", req.Group)
		response.OK(c, gin.H{"ok": true})
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
		if err := c.ShouldBindJSON(&req); err != nil { response.Bad(c, err.Error()); return }
		if req.Mode != "standalone" && req.Mode != "distributed" {
			response.Bad(c, "mode 必须为 standalone 或 distributed"); return
		}
		var s model.SystemSetting
		if err := a.DB.Where("`key` = ?", "deploy_mode").First(&s).Error; err != nil {
			a.DB.Create(&model.SystemSetting{Key: "deploy_mode", Value: req.Mode})
		} else {
			a.DB.Model(&s).Update("value", req.Mode)
		}
		response.OK(c, gin.H{"ok": true, "mode": req.Mode})
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
		opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID("netops-check-"+time.Now().Format("150405")).SetConnectTimeout(5 * time.Second)
		if user != "" {
			opts.SetUsername(user).SetPassword(pass)
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
	if err := a.DB.Where("`key` = ?", k).First(&s).Error; err != nil { return "" }
	return s.Value
}

func init() {
	modreg.Register("distributed", RegisterPublic)
	modreg.RegisterProtected("distributed", RegisterProtected)
}
