// Package core 应用核心：全局状态与共享依赖
package core

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"gorm.io/gorm"

	"netops/internal/collector/dbprobe"
	"netops/internal/collector/docker"
	"netops/internal/collector/k8s"
	"netops/internal/collector/ping"
	"netops/internal/collector/snmp"
	"netops/internal/common/crypto"
	"netops/internal/common/session"
	"netops/internal/config"
	"netops/internal/notify"
	"netops/internal/service/mcp"
	"netops/internal/tsdb"
)

// App 全局应用对象
type App struct {
	Cfg      *config.Config
	DB       *gorm.DB
	TSDB     tsdb.Engine
	Sessions *session.Manager
	Hub      *notify.Hub
	SM2      *crypto.SM2Key
	Ready    bool // 是否已完成初始化向导
	Started  time.Time

	PingMgr  *ping.Manager
	SnmpMgr  *snmp.Manager
	DockerM  *docker.Monitor
	K8sM     *k8s.Monitor
	DBProbe  *dbprobe.Manager

	// MCP 能力服务（外部 Agent 接入本系统）
	MCPServer *mcp.Server

	// 已注册模块（由 bootstrap 填充）
	Modules []string

	// 运行状态
	startTime time.Time
}

// New 创建应用实例（未初始化状态）
func New(cfg *config.Config) *App {
	hub := notify.NewHub()
	return &App{
		Cfg:      cfg,
		Hub:      hub,
		Sessions: session.NewManager(12 * time.Hour),
		Ready:    false,
		Started:  time.Now(),
	}
}

// EnsureCrypto 确保国密密钥对存在（缺省生成并持久化到配置）
func (a *App) EnsureCrypto() error {
	if a.Cfg.Crypto.SM2PrivateKey != "" && a.Cfg.Crypto.SM2PublicKey != "" {
		a.SM2 = &crypto.SM2Key{PrivateHex: a.Cfg.Crypto.SM2PrivateKey, PublicHex: a.Cfg.Crypto.SM2PublicKey}
		return nil
	}
	key, err := crypto.GenerateSM2Key()
	if err != nil {
		return err
	}
	a.SM2 = key
	a.Cfg.Crypto.SM2PrivateKey = key.PrivateHex
	a.Cfg.Crypto.SM2PublicKey = key.PublicHex
	return a.Cfg.Save()
}

// InitRuntime 初始化运行时管理器（需要在 TSDB 就绪后调用）
func (a *App) InitRuntime() {
	a.PingMgr = ping.NewManager(a.TSDB, a.Hub)
	a.SnmpMgr = snmp.NewManager(a.TSDB, a.Hub)
	a.DockerM = docker.NewMonitor(a.TSDB, a.Hub)
	a.K8sM = k8s.NewMonitor(a.TSDB, a.Hub)
	a.DBProbe = dbprobe.NewManager(a.TSDB, a.Hub)
}

// Shutdown 优雅关闭
func (a *App) Shutdown() {
	if a.PingMgr != nil {
		a.PingMgr.StopAll()
	}
	if a.SnmpMgr != nil {
		a.SnmpMgr.StopAll()
	}
	if a.DockerM != nil {
		a.DockerM.Stop()
	}
	if a.K8sM != nil {
		a.K8sM.Stop()
	}
	if a.DBProbe != nil {
		a.DBProbe.StopAll()
	}
	if a.TSDB != nil {
		_ = a.TSDB.Close()
	}
	a.Hub.Stop()
}

// Stats 运行时统计（内部服务暴露）
func (a *App) Stats() map[string]any {
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	linkTotal := 0
	if a.PingMgr != nil {
		linkTotal = len(a.PingMgr.Snapshot())
	}
	return map[string]any{
		"app":         a.Cfg.App.Name,
		"ready":       a.Ready,
		"uptime_s":    int(time.Since(a.Started).Seconds()),
		"go_version":  runtime.Version(),
		"goroutines":  runtime.NumGoroutine(),
		"mem_alloc_mb": float64(mem.Alloc) / 1024 / 1024,
		"mem_sys_mb":  float64(mem.Sys) / 1024 / 1024,
		"sessions":    a.Sessions.Count(),
		"link_tasks":  linkTotal,
		"tsdb":        tsdbName(a.TSDB),
	}
}

func tsdbName(e tsdb.Engine) string {
	if e == nil {
		return "none"
	}
	return e.Name()
}

// InternalHandler 内部服务（端口 30001）：模块间通讯与健康检查
func (a *App) InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		code := 200
		if !a.Ready {
			status = "not_initialized"
			code = 200
		}
		_ = writeJSON(w, code, map[string]any{"status": status, "ready": a.Ready, "stats": a.Stats()})
	})
	mux.HandleFunc("/internal/db/ping", func(w http.ResponseWriter, r *http.Request) {
		var errMsg string
		if a.DB != nil {
			if sqlDB, err := a.DB.DB(); err == nil {
				if err := sqlDB.Ping(); err != nil {
					errMsg = err.Error()
				}
			}
		} else {
			errMsg = "database not initialized"
		}
		ok := errMsg == ""
		_ = writeJSON(w, 200, map[string]any{"ok": ok, "error": errMsg})
	})
	mux.HandleFunc("/internal/tsdb/ping", func(w http.ResponseWriter, r *http.Request) {
		var errMsg string
		if a.TSDB != nil {
			if err := a.TSDB.Ping(); err != nil {
				errMsg = err.Error()
			}
		} else {
			errMsg = "tsdb not initialized"
		}
		_ = writeJSON(w, 200, map[string]any{"ok": errMsg == "", "error": errMsg, "type": tsdbName(a.TSDB)})
	})
	mux.HandleFunc("/internal/modules", func(w http.ResponseWriter, r *http.Request) {
		_ = writeJSON(w, 200, map[string]any{"modules": a.Modules})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := fmt.Fprintf(w, "%s", mustJSON(data))
	return err
}

func mustJSON(v any) string {
	b, err := jsonMarshal(v)
	if err != nil {
		return `{"error":"json marshal failed"}`
	}
	return string(b)
}
