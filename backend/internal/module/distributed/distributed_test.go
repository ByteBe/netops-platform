package distributed

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"netops/internal/common/secstore"
	"netops/internal/core"
	"netops/internal/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个测试用独立内存库，避免 shared cache 跨测试复用导致数据/唯一索引冲突
	dsn := fmt.Sprintf("file:mem-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MonitorDevice{}, &model.NodeContainer{}, &model.NodeLink{}, &model.Node{}, &model.SystemSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestUpsertNodeDevices 设备明细 upsert：新增、更新详情、删除消失设备
func TestUpsertNodeDevices(t *testing.T) {
	db := newTestDB(t)
	a := &core.App{DB: db}

	// 首次上报 2 台设备
	upsertNodeDevices(a, "node-a", []map[string]interface{}{
		{"name": "核心交换机", "ip": "10.0.0.1", "type": "switch", "vendor": "h3c", "online": true, "cpu": 12.5, "mem": 40.2, "uptime": 86400.0, "message": "ok"},
		{"name": "web服务器", "ip": "10.0.0.2", "type": "server", "vendor": "", "online": false, "cpu": 0.0, "mem": 0.0, "uptime": 0.0, "message": "未采集"},
	})
	var devs []model.MonitorDevice
	db.Find(&devs)
	if len(devs) != 2 {
		t.Fatalf("期望 2 台设备，实际 %d", len(devs))
	}
	// 详情字段持久化
	for _, d := range devs {
		if d.IP == "10.0.0.1" {
			if !d.LastOnline || d.LastCPU != 12.5 || d.LastMem != 40.2 || d.LastUptime != 86400.0 {
				t.Fatalf("设备详情未正确持久化: %+v", d)
			}
			if d.Status != "up" || d.Vendor != "h3c" {
				t.Fatalf("设备状态/厂商错误: %+v", d)
			}
		}
		if d.IP == "10.0.0.2" {
			if d.LastOnline || d.Status != "down" {
				t.Fatalf("离线设备状态错误: %+v", d)
			}
		}
	}

	// 第二次上报：保留 1 台并更新详情，1 台消失
	upsertNodeDevices(a, "node-a", []map[string]interface{}{
		{"name": "核心交换机", "ip": "10.0.0.1", "type": "switch", "vendor": "h3c", "online": true, "cpu": 20.0, "mem": 55.0, "uptime": 90000.0, "message": "ok"},
	})
	db.Find(&devs)
	if len(devs) != 1 {
		t.Fatalf("期望 1 台设备（消失的已删除），实际 %d", len(devs))
	}
	if devs[0].LastCPU != 20.0 || devs[0].LastMem != 55.0 {
		t.Fatalf("设备详情未更新: %+v", devs[0])
	}

	// 空上报：清空该节点全部设备
	upsertNodeDevices(a, "node-a", nil)
	db.Find(&devs)
	if len(devs) != 0 {
		t.Fatalf("空上报后应清空设备，实际 %d", len(devs))
	}
}

// TestUpsertNodeContainersAndLinks 容器与链路明细 upsert
func TestUpsertNodeContainersAndLinks(t *testing.T) {
	db := newTestDB(t)
	a := &core.App{DB: db}

	upsertNodeContainers(a, "node-b", []map[string]interface{}{
		{"name": "nginx", "image": "nginx:1.25", "state": "running", "cpu": 2.5, "mem_pct": 8.1, "host": "10.0.0.2"},
		{"name": "mysql", "image": "mysql:8.0", "state": "running", "cpu": 5.0, "mem_pct": 30.0, "host": "10.0.0.2"},
	})
	var ctns []model.NodeContainer
	db.Find(&ctns)
	if len(ctns) != 2 {
		t.Fatalf("期望 2 个容器，实际 %d", len(ctns))
	}

	upsertNodeLinks(a, "node-b", []map[string]interface{}{
		{"name": "总部链路", "target": "82.156.100.53", "method": "tcp", "port": 40000.0, "status": "up"},
	})
	var links []model.NodeLink
	db.Find(&links)
	if len(links) != 1 {
		t.Fatalf("期望 1 条链路，实际 %d", len(links))
	}
	if links[0].Target != "82.156.100.53" || links[0].Status != "up" || links[0].Port != 40000 {
		t.Fatalf("链路明细错误: %+v", links[0])
	}

	// 容器消失 1 个
	upsertNodeContainers(a, "node-b", []map[string]interface{}{
		{"name": "nginx", "image": "nginx:1.25", "state": "running", "cpu": 3.0, "mem_pct": 9.0, "host": "10.0.0.2"},
	})
	db.Find(&ctns)
	if len(ctns) != 1 || ctns[0].Name != "nginx" || ctns[0].CPU != 3.0 {
		t.Fatalf("容器 upsert/删除错误: %+v", ctns)
	}

	// 不同节点互不影响
	upsertNodeContainers(a, "node-c", []map[string]interface{}{
		{"name": "redis", "image": "redis:7", "state": "running", "cpu": 1.0, "mem_pct": 2.0, "host": "10.0.0.3"},
	})
	db.Find(&ctns)
	if len(ctns) != 2 {
		t.Fatalf("不同节点容器应共存，实际 %d", len(ctns))
	}
}

// TestUpsertNodeKeepsLevel MQTT 心跳上报不应覆盖已注册节点的层级
func TestUpsertNodeKeepsLevel(t *testing.T) {
	db := newTestDB(t)
	a := &core.App{DB: db}

	// 首次注册：省级（L2）
	upsertNode(a, "node-a", "省级中心", 2, 3, 5)
	// 后续心跳上报（payload 未带 level，默认 0）：计数更新，但层级保持
	upsertNode(a, "node-a", "省级中心", 0, 4, 6)
	var n model.Node
	if err := db.Where("node_uuid = ?", "node-a").First(&n).Error; err != nil {
		t.Fatalf("节点未创建: %v", err)
	}
	if n.Level != 2 {
		t.Fatalf("层级被心跳上报覆盖为 %d（应为 2）: %+v", n.Level, n)
	}
	if n.DeviceCnt != 4 || n.ContainerCnt != 6 {
		t.Fatalf("计数未更新: %+v", n)
	}
}

// TestVerifyIngestToken 上报鉴权：节点独立 token 优先，未注册回退全局 token
func TestVerifyIngestToken(t *testing.T) {
	secstore.Init("test-seed-for-distributed") // Node.Token 有 SM4 加密钩子，测试环境需初始化密钥
	db := newTestDB(t)
	a := &core.App{DB: db}
	db.Create(&model.SystemSetting{Key: "cluster_token", Value: "global-secret"})

	if !verifyIngestToken(a, "unknown-node", "global-secret") {
		t.Fatal("未注册节点应回退全局 token")
	}
	if verifyIngestToken(a, "unknown-node", "wrong") {
		t.Fatal("未注册节点错误 token 应被拒绝")
	}
	db.Create(&model.Node{NodeUUID: "node-a", Token: "node-secret"})
	if !verifyIngestToken(a, "node-a", "node-secret") {
		t.Fatal("节点独立 token 应通过")
	}
	if verifyIngestToken(a, "node-a", "global-secret") {
		t.Fatal("已注册节点应优先校验独立 token")
	}
}
