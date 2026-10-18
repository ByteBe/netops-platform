// NetOps 网络运维监控平台入口
// 架构：后端二进制 + 外部前端静态资源（./web/dist，与可执行文件同目录）
//   - 外部 Web 服务端口：30821（用户访问：API + 静态页面）
//   - 内部服务端口：30001（模块间通讯 / 数据库探活 / 健康检查）
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
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
  NetOps 网络运维监控平台 v2.2.1 | AI巡检 · 日志中心 · 告警通知
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

	// 3.1 上传文件静态服务（/uploads/ 映射到 data/uploads/）
	engine.Static("/uploads", "./data/uploads")

	// 3.2 前端资源：默认由 API 端口提供；配置了独立前端端口(web_port>0)时改由独立端口提供
	webDir := "./web"
	var webSrv *http.Server
	useTLS := cfg.Server.TLSCert != "" && cfg.Server.TLSKey != ""
	if err := validateWebPort(cfg); err != nil {
		log.Fatalf("[bootstrap] %v", err)
	}
	webEnabled := cfg.Server.WebPort > 0
	if !webEnabled {
		mountFrontend(engine, webDir)
	}

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
	if cfg.Initialized && cfg.Server.WebPort > 0 {
		// 已完成初始化且启用独立前端端口：API 仅绑定本机（供前端反代与内部调用），
		// 外部只能经前端端口访问，防止绕过前端直连 API
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

	// 5.1 独立前端服务（web_port>0 时启用：前端静态资源 + /api /ws /mcp 反向代理）
	if webEnabled {
		webAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.WebPort)
		webSrv = &http.Server{Addr: webAddr, Handler: buildWebHandler(webDir, cfg.Server.Port, useTLS)}
		go func() {
			log.Printf("[web] 前端独立服务启动: http://%s（API 端口 %d）", webAddr, cfg.Server.Port)
			if err := webSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// 端口被占用等场景：降级警告而非退出整个进程（API 服务仍可用）
				log.Printf("[web] 警告: 前端服务异常（%v），请检查端口占用；API 端口 %d 仍可访问（未初始化时请访问 http://IP:%d/setup 完成初始化）", err, cfg.Server.Port, cfg.Server.Port)
			}
		}()
	}

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
	if webSrv != nil {
		_ = webSrv.Shutdown(ctx)
	}
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
			c.FileFromFS("/", http.FS(sub))
		})
	} else {
		log.Printf("[web] 警告: 未找到前端目录 %s（请将前端构建产物放在该目录）", webDir)
	}
}

// validateWebPort 校验前端独立端口配置：范围 0-65535，且不能与 API/内部端口冲突
func validateWebPort(cfg *config.Config) error {
	wp := cfg.Server.WebPort
	if wp < 0 || wp > 65535 {
		return fmt.Errorf("配置错误: web_port=%d 超出有效范围 0-65535", wp)
	}
	if wp == 0 {
		return nil
	}
	if wp == cfg.Server.Port {
		return fmt.Errorf("配置错误: web_port=%d 不能与 API 端口(port=%d)相同（填 0 表示前端由 API 端口提供）", wp, cfg.Server.Port)
	}
	if wp == cfg.Server.InternalPort {
		return fmt.Errorf("配置错误: web_port=%d 不能与内部端口(internal_port=%d)相同", wp, cfg.Server.InternalPort)
	}
	return nil
}

// buildWebHandler 构建独立前端服务 handler：
//   - 前端 SPA 静态资源（./web，回退 index.html）
//   - /uploads 上传文件静态资源
//   - /api /ws /mcp 反向代理到 API 端口（TLS 模式下走 https 并信任本机证书）
func buildWebHandler(webDir string, apiPort int, useTLS bool) http.Handler {
	mux := http.NewServeMux()

	// API / WebSocket / MCP 反向代理到 API 端口
	if apiPort > 0 {
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		target := &url.URL{Scheme: scheme, Host: fmt.Sprintf("127.0.0.1:%d", apiPort)}
		rp := httputil.NewSingleHostReverseProxy(target)
		if useTLS {
			// 本机回环反代：信任本机 API 服务证书（含自签名），仅限回环地址，风险可控
			rp.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		}
		for _, p := range []string{"/api", "/api/", "/ws", "/mcp", "/mcp/"} {
			mux.Handle(p, rp)
		}
	}

	// 上传文件静态资源
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./data/uploads"))))

	// 前端 SPA 静态资源（回退 index.html）
	if st, err := os.Stat(webDir); err == nil && st.IsDir() {
		sub := os.DirFS(webDir)
		if entries, err := fs.ReadDir(sub, "assets"); err == nil {
			log.Printf("[web] 前端静态资源目录: %s, assets 文件数: %d", webDir, len(entries))
		}
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
			if reqPath != "" && reqPath != "." {
				if _, err := fs.Stat(sub, reqPath); err == nil {
					http.FileServer(http.FS(sub)).ServeHTTP(w, r)
					return
				}
			}
			http.ServeFileFS(w, r, sub, "index.html")
		}))
	} else {
		log.Printf("[web] 警告: 未找到前端目录 %s（请将前端构建产物放在该目录）", webDir)
	}
	return mux
}
