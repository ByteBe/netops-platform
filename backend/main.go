// NetOps 网络运维监控平台入口
// 架构：后端二进制 + 外部前端静态资源（./web/dist，与可执行文件同目录）
//   - 外部 Web 服务端口：30821（用户访问：API + 静态页面）
//   - 内部服务端口：30001（模块间通讯 / 数据库探活 / 健康检查）
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"netops/internal/bootstrap"
	"netops/internal/config"
	"netops/internal/model"
	_ "netops/internal/module/aiinspect"
	"netops/internal/module/devicelog"
	"netops/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// 启动横幅
	fmt.Print(`
  ███╗   ██╗███████╗████████╗    ██████╗  ██████╗ ██████╗ ███████╗
  ████╗  ██║██╔════╝╚══██╔══╝    ██╔══██╗██╔═══██╗██╔══██╗██╔════╝
  ██╔██╗ ██║█████╗     ██║       ██║  ██║██║   ██║██║  ██║█████╗
  ██║╚██╗██║██╔══╝     ██║       ██║  ██║██║   ██║██║  ██║██╔══╝
  ██║ ╚████║███████╗   ██║       ██████╔╝╚██████╔╝██████╔╝███████╗
  ╚═╝  ╚═══╝╚══════╝   ╚═╝       ╚═════╝  ╚═════╝ ╚═════╝ ╚══════╝
  NetOps 网络运维监控平台 v2.4.0 | AI巡检 · 日志中心 · 告警通知
`)

	// 1. 加载配置（支持 --config 指定，缺省自动探测 ./config.yaml 与可执行文件同目录）
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[bootstrap] 配置加载失败: %v", err)
	}

	// 2. 初始化应用（数据库选择、时序库选择、种子数据、模块路由注册）
	app, err := bootstrap.Init(cfg)
	if err != nil {
		log.Fatalf("[bootstrap] 应用初始化失败: %v", err)
	}

	// 3. 路由注册（前端静态资源 + API 自动注册）
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 3.2 前端资源：由 API 端口提供静态资源（生产环境由 Nginx 托管前端静态与反代，API 收归本机）
	webDir := "./web"
	useTLS := cfg.Server.TLSCert != "" && cfg.Server.TLSKey != ""
	mountFrontend(engine, webDir)

	// 3.3 注册 API 路由
	router.Register(engine, app)

	// 4. 启动内部服务（30001：模块间通讯 / 探活 / 健康）
	internalSrv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.InternalHost, cfg.Server.InternalPort),
		Handler: app.InternalHandler(),
	}
	go func() {
		log.Printf("[internal] 内部服务监听 %s:%d", cfg.Server.InternalHost, cfg.Server.InternalPort)
		if err := internalSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[internal] 内部服务异常: %v", err)
		}
	}()

	// 5. 启动 API 服务（默认 30821）
	apiHost := cfg.Server.Host
	if cfg.Initialized {
		// 初始化完成后 API 仅绑定本机：外部统一经 Nginx 或前端独立端口反代访问，
		// 防止绕过前端直连 API（未初始化时开放，供向导页面 /setup 完成初始化）
		apiHost = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", apiHost, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: engine}
	go func() {
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		log.Printf("[api] NetOps API 服务启动: %s://%s", scheme, addr)
		var err error
		if useTLS {
			err = srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("[api] 服务异常退出: %v", err)
		}
	}()

	// 6. 审计日志自动清理（保留180天）
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if app.DB != nil {
				cutoff := time.Now().Add(-180 * 24 * time.Hour)
				app.DB.Where("created_at < ?", cutoff).Delete(&model.AuditLog{})
				// 采集日志高频，保留 7 天
				tlCutoff := time.Now().Add(-7 * 24 * time.Hour)
				app.DB.Where("created_at < ?", tlCutoff).Delete(&model.TaskLog{})
			}
		}
	}()

	// 7. 启动 syslog UDP 接收
	devicelog.StartSyslogReceiver(app)

	// 8. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = internalSrv.Shutdown(ctx)
	app.Shutdown()
	log.Println("[web] 服务已安全退出")
}

// mountFrontend 在前端目录存在时挂载 SPA 静态资源（默认由 API 端口提供）
func mountFrontend(engine *gin.Engine, webDir string) {
	if st, err := os.Stat(webDir); err == nil && st.IsDir() {
		sub := os.DirFS(webDir)
		if entries, err := fs.ReadDir(sub, "assets"); err == nil {
			log.Printf("[web] 前端静态资源目录: %s, assets 文件数: %d", webDir, len(entries))
		}
		engine.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			reqPath := path.Clean(c.Request.URL.Path)
			if reqPath != "/" {
				if _, err := fs.Stat(sub, reqPath[1:]); err == nil {
					c.FileFromFS(reqPath, http.FS(sub))
					return
				}
			}
			// SPA 入口：禁止强缓存，确保升级后刷新即拿到最新版本
			c.Header("Cache-Control", "no-cache, must-revalidate")
			c.FileFromFS("/", http.FS(sub))
		})
	} else {
		log.Printf("[web] 警告: 未找到前端目录 %s（请将前端构建产物放在该目录）", webDir)
	}
}
