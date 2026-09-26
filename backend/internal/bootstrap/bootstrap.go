// Package bootstrap 应用初始化入口（薄封装：依赖 appinit 与 module，避免循环引用）
package bootstrap

import (
	"os"

	"netops/internal/appinit"
	"netops/internal/common/logger"
	"netops/internal/config"
	"netops/internal/core"
	"netops/internal/module"
)

// AllModels 全部业务表
func AllModels() []any { return appinit.AllModels() }

// EnsureJWTKey 确保 JWT 密钥存在
func EnsureJWTKey(cfg *config.Config) error { return appinit.EnsureJWTKey(cfg) }

// InitRuntimeAndDB 运行时初始化
func InitRuntimeAndDB(a *core.App) error { return appinit.InitRuntimeAndDB(a) }

// SyncManagers 同步采集任务
func SyncManagers(a *core.App) { appinit.SyncManagers(a) }

// InitMCPServer 注册MCP能力
func InitMCPServer(a *core.App) { appinit.InitMCPServer(a) }

// Init 初始化应用
func Init(cfg *config.Config) (*core.App, error) {
	// 自动创建运行时目录（无论选择什么数据库）
	os.MkdirAll("data", 0o755)
	os.MkdirAll("data/uploads", 0o755)
	os.MkdirAll("logs", 0o755)

	if err := logger.Init("logs"); err != nil {
		return nil, err
	}
	app := core.New(cfg)
	if err := app.EnsureCrypto(); err != nil {
		logger.Warnf("国密密钥持久化失败: %v", err)
	}
	if cfg.Initialized {
		if err := appinit.InitStorage(app, cfg); err != nil {
			return nil, err
		}
		if err := appinit.InitTSDB(app, cfg); err != nil {
			return nil, err
		}
		app.Ready = true
		app.InitRuntime()
		appinit.InitMCPServer(app)
		appinit.SyncManagers(app)
		logger.Infof("平台初始化完成：数据库=%s 时序库=%s", cfg.Database.Type, cfg.TSDB.Type)
	} else {
		logger.Infof("平台尚未初始化，等待初始化向导（数据库/时序库/管理员）")
	}
	app.Modules = module.RegisteredModules()
	return app, nil
}
