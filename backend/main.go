// NetOps 网络运维监控平台入口
// 架构：单体二进制，内嵌前端静态资源（web/dist）
//   - 外部 Web 服务端口：30821（用户访问：API + 静态页面）
//   - 内部服务端口：30001（模块间通讯 / 数据库探活 / 健康检查）
package main

import (
	"context"
	"embed"
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
	"netops/internal/router"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist
var webFS embed.FS

func main() {
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

	// 3.2 内嵌前端资源（构建产物）
	if sub, err := fs.Sub(webFS, "web/dist"); err == nil {
		entries, _ := fs.ReadDir(sub, "assets")
		log.Printf("[embed] web/dist/assets 文件数: %d", len(entries))
		engine.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			path := path.Clean(c.Request.URL.Path)
			if path != "/" {
				if _, err := fs.Stat(sub, path[1:]); err == nil {
					c.FileFromFS(path, http.FS(sub))
					return
				}
			}
			c.FileFromFS("/", http.FS(sub))
		})
	}

	// 3.2 注册 API 路由（含前端路由自动注册 & 内嵌静态资源）
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

	// 5. 启动外部 Web 服务（30821）
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: engine}
	go func() {
		log.Printf("[web] NetOps 平台启动: http://%s  (前端静态资源已内嵌)", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[web] 服务异常退出: %v", err)
		}
	}()

	// 6. 优雅退出
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
