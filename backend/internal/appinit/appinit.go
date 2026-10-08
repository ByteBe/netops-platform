// Package appinit 运行时初始化逻辑（数据库/时序库/种子数据/MCP能力）
// 独立于 module 包，避免 bootstrap ↔ module 的 import 循环
package appinit

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"netops/internal/collector/dbprobe"
	"netops/internal/collector/ping"
	"netops/internal/collector/snmp"
	"netops/internal/common/crypto"
	"netops/internal/common/logger"
	"netops/internal/config"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/module/distributed"
	"netops/internal/service/mcp"
	"netops/internal/storage"
	"netops/internal/tsdb"
	"netops/internal/tsdb/builtin"
	"netops/internal/tsdb/influxdb"
	"netops/internal/tsdb/tdengine"
)

// AllModels 全部业务表（统一迁移）
func AllModels() []any {
	return []any{
		&model.User{}, &model.AuditLog{}, &model.DeviceGroup{},
		&model.MonitorDevice{}, &model.LinkTask{}, &model.TopoDevice{},
		&model.TopoLink{}, &model.TopoLinkIP{}, &model.DBInstance{},
		&model.ScriptTemplate{}, &model.ScriptHistory{}, &model.Subnet{},
		&model.IPRecord{}, &model.BindDevice{}, &model.TrafficRule{},
		&model.AIConfig{}, &model.MCPAgent{}, &model.EmailConfig{},
		&model.ReportRecord{}, &model.SystemSetting{}, &model.DockerHost{}, &model.K8sCluster{},
		&model.Node{},
	}
}

// EnsureJWTKey 确保 JWT 密钥存在（初始化时调用）
func EnsureJWTKey(cfg *config.Config) error {
	if cfg.App.JWTKey != "" {
		return nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	cfg.App.JWTKey = hex.EncodeToString(b)
	return nil
}

// InitRuntimeAndDB 运行时初始化（初始化向导完成后调用：打开存储/时序库、迁移、启动采集器）
func InitRuntimeAndDB(a *core.App) error {
	if err := InitStorage(a, a.Cfg); err != nil {
		return err
	}
	if err := InitTSDB(a, a.Cfg); err != nil {
		return err
	}
	a.InitRuntime()
	InitMCPServer(a)
	return nil
}

// InitStorage 打开存储数据库并迁移
func InitStorage(a *core.App, cfg *config.Config) error {
	db, err := storage.Open(&cfg.Database)
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	a.DB = db
	SeedData(db)
	return nil
}

// InitTSDB 打开时序数据库；连不上时回退到 builtin，不阻塞启动
func InitTSDB(a *core.App, cfg *config.Config) error {
	t, err := OpenTSDB(&cfg.TSDB)
	if err != nil {
		logger.Warnf("[tsdb] %s 不可用，回退到 builtin: %v", cfg.TSDB.Type, err)
		t, err = builtin.NewBuiltin(cfg.TSDB.DB)
		if err != nil {
			return fmt.Errorf("时序数据库初始化失败: %w", err)
		}
	}
	a.TSDB = t
	return nil
}

// OpenTSDB 创建时序引擎（tdengine / influxdb / builtin）
func OpenTSDB(cfg *config.TSDB) (tsdb.Engine, error) {
	switch strings.ToLower(cfg.Type) {
	case "tdengine":
		return tdengine.NewTDengine(cfg.Host, cfg.Port, cfg.User, cfg.Pass, cfg.DB)
	case "influxdb":
		return influxdb.NewInflux(cfg.Host, cfg.Port, cfg.User, cfg.Pass, cfg.DB, cfg.Params)
	case "builtin", "":
		return builtin.NewBuiltin(cfg.DB)
	default:
		return nil, fmt.Errorf("不支持的时序数据库类型: %s（可选 tdengine/influxdb/builtin）", cfg.Type)
	}
}

// SeedData 种子数据（脚本模板/默认分组/系统设置/默认管理员）
func SeedData(db *gorm.DB) {
	var userCnt int64
	db.Model(&model.User{}).Count(&userCnt)
	if userCnt == 0 {
		salt, _ := crypto.GenerateSalt()
		db.Create(&model.User{Username: "admin", EmployeeNo: "admin", Email: "admin@localhost", PasswordHash: crypto.PasswordHash("admin123", salt), Salt: salt, Role: "admin", Status: "active", MustChangePwd: false})
	}
	var cnt int64
	db.Model(&model.DeviceGroup{}).Count(&cnt)
	if cnt == 0 {
		defaults := []model.DeviceGroup{
			{Name: "核心网络设备", Type: "switch", Color: "#409EFF"},
			{Name: "服务器", Type: "server", Color: "#67C23A"},
			{Name: "安全设备", Type: "firewall", Color: "#E6A23C"},
			{Name: "数据库", Type: "database", Color: "#F56C6C"},
			{Name: "其他设备", Type: "other", Color: "#909399"},
		}
		db.Create(&defaults)
	}
	settings := map[string]string{
		"ipam_binding_enable": "true",
		"docker_enable":       "false",
		"docker_interval":     "30",
		"docker_hosts":      "",
		"k8s_enable":          "false",
		"k8s_api_server":      "",
		"k8s_token":           "",
		"k8s_interval":        "60",
		"report_default_to":   "",
		"lang":                "zh-CN",
		"theme":               "light",
	}
	for k, v := range settings {
		var s model.SystemSetting
		if err := db.Where("`key` = ?", k).First(&s).Error; err != nil {
			db.Create(&model.SystemSetting{Key: k, Value: v})
		}
	}
}

// SyncManagers 从数据库同步采集任务到运行时管理器
func SyncManagers(a *core.App) {
	if a.DB == nil {
		return
	}
	var tasks []model.LinkTask
	a.DB.Find(&tasks)
	tcs := make([]ping.TaskConfig, 0, len(tasks))
	for _, t := range tasks {
		tcs = append(tcs, ping.TaskConfig{
			ID: t.ID, Name: t.Name, Target: t.Target, Method: t.Method,
			Port: t.Port, Interval: t.Interval, Timeout: t.Timeout, Color: t.Color, Enabled: t.Enabled,
		})
	}
	a.PingMgr.Reset(tcs)

	var devices []model.MonitorDevice
	a.DB.Find(&devices)
	devs := make([]snmp.Device, 0, len(devices))
	for _, d := range devices {
		if d.Enable {
			devs = append(devs, snmp.Device{
				ID: d.ID, Name: d.Name, IP: d.IP, Type: d.Type, SNMPVersion: d.SNMPVersion,
				Community: d.Community, Username: d.Username, AuthProto: d.AuthProto,
				PrivProto: d.PrivProto, AuthPass: d.AuthPass, PrivPass: d.PrivPass,
				Port: d.Port, Interval: d.Interval,
			})
		}
	}
	a.SnmpMgr.SyncDevices(devs)

	var insts []model.DBInstance
	a.DB.Find(&insts)
	dbs := make([]dbprobe.Instance, 0, len(insts))
	for _, d := range insts {
		if d.Enable {
			dbs = append(dbs, dbprobe.Instance{ID: d.ID, Name: d.Name, Type: d.Type, Host: d.Host,
				Port: d.Port, User: d.User, Password: d.Password, DBName: d.DBName, Interval: d.Interval})
		}
	}
	a.DBProbe.Sync(dbs)

		// 从数据库读取已启用的Docker主机列表
	dockerAddrs := []string{}
	var dockerHosts []model.DockerHost
	a.DB.Where("enabled = ?", true).Find(&dockerHosts)
	for _, h := range dockerHosts {
		if h.Address != "" { dockerAddrs = append(dockerAddrs, h.Address) }
	}
	dockerHostsJSON, _ := json.Marshal(dockerAddrs)
	dockerEnable := SettingBool(a.DB, "docker_enable")
	// 如果已经配置了 Docker 主机，自动启用（避免用户每次重启都要去点保存）
	if !dockerEnable && len(dockerHosts) > 0 {
		dockerEnable = true
		a.DB.Where("`key` = ?", "docker_enable").Assign(model.SystemSetting{Value: "true"}).FirstOrCreate(&model.SystemSetting{Key: "docker_enable"})
		fmt.Printf("[docker] auto-enable because %d host(s) configured\n", len(dockerHosts))
	}
	fmt.Printf("[docker] enable=%v interval=%d hosts=%s\n", dockerEnable, SettingInt(a.DB, "docker_interval", 30), string(dockerHostsJSON))
	a.DockerM.SetConfig(dockerEnable, SettingInt(a.DB, "docker_interval", 30), string(dockerHostsJSON))
	// K8s多集群配置
	var k8sClusters []model.K8sCluster
	a.DB.Where("enabled = ?", true).Find(&k8sClusters)
	type kcInfo struct { Name string; APIServer string; Token string }
	k8sArr := []kcInfo{}
	for _, cl := range k8sClusters { k8sArr = append(k8sArr, kcInfo{Name: cl.Name, APIServer: cl.APIServer, Token: cl.Token}) }
	k8sJSON, _ := json.Marshal(k8sArr)
	a.K8sM.SetConfigJSON(SettingBool(a.DB, "k8s_enable"), SettingInt(a.DB, "k8s_interval", 60), string(k8sJSON))

	// 分布式级联：启动定时上报（如果配置了 parent_url）
	distributed.StartPusher(a)
}

// InitMCPServer 注册系统能力到 MCP 服务（外部 Agent 接入）
func InitMCPServer(a *core.App) {
	if a.MCPServer != nil {
		return
	}
	srv := mcp.NewServer()
	srv.RegisterTool("link_status", "查询全部链路实时监控状态", map[string]any{
		"type": "object", "properties": map[string]any{},
	}, func(args map[string]any) (map[string]any, error) {
		return map[string]any{"links": a.PingMgr.Snapshot()}, nil
	})
	srv.RegisterTool("device_status", "查询纳管设备(SNMP)监控快照", map[string]any{
		"type": "object", "properties": map[string]any{},
	}, func(args map[string]any) (map[string]any, error) {
		return map[string]any{"devices": a.SnmpMgr.Snapshot()}, nil
	})
	srv.RegisterTool("database_status", "查询数据库监控状态", map[string]any{
		"type": "object", "properties": map[string]any{},
	}, func(args map[string]any) (map[string]any, error) {
		return map[string]any{"databases": a.DBProbe.SnapshotAll()}, nil
	})
	srv.RegisterTool("generate_report", "生成巡检报告", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string", "description": "报告标题"},
		},
	}, func(args map[string]any) (map[string]any, error) {
		title, _ := args["title"].(string)
		if title == "" {
			title = "MCP 请求生成巡检报告"
		}
		return map[string]any{"message": "请登录平台在 巡检报告 模块查看与导出，或调用 /api/v1/report/generate 生成"}, nil
	})
	srv.RegisterTool("ipam_subnets", "查询IP地址网段与使用情况", map[string]any{
		"type": "object", "properties": map[string]any{},
	}, func(args map[string]any) (map[string]any, error) {
		var subnets []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
			CIDR string `json:"cidr"`
			VLAN int    `json:"vlan"`
		}
		a.DB.Model(&model.Subnet{}).Scan(&subnets)
		return map[string]any{"subnets": subnets}, nil
	})

	a.MCPServer = srv
}

// Setting 读取系统设置
func Setting(db *gorm.DB, key string) string {
	var s model.SystemSetting
	if err := db.Where("`key` = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// SettingBool 布尔设置
func SettingBool(db *gorm.DB, key string) bool {
	return Setting(db, key) == "true"
}

// SettingInt 整数设置
func SettingInt(db *gorm.DB, key string, def int) int {
	v := Setting(db, key)
	if v == "" {
		return def
	}
	n := 0
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return def
	}
	return n
}

