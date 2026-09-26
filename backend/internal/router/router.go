// Package router 路由注册：API 自动注册 + WebSocket + 静态资源
package router

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/module"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize: 4096,
	WriteBufferSize: 4096,
}

// Register 注册全部路由
func Register(engine *gin.Engine, app *core.App) {
	// 全局中间件
	engine.Use(middleware.CORS())

	// 基础路由
	api := engine.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			response.OK(c, gin.H{
				"name": app.Cfg.App.Name, "ready": app.Ready,
				"version": "1.0.0", "time": app.Started.UnixMilli(),
				"web_port": app.Cfg.Server.Port, "internal_port": app.Cfg.Server.InternalPort,
			})
		})
		// 国密公钥（初始化前也需可用）
		api.GET("/crypto/public-key", func(c *gin.Context) {
			response.OK(c, gin.H{"public_key": app.SM2.PublicHex, "algorithm": "SM2"})
		})
	}

	// 模块路由自动注册（各模块 init() 自注册到 module.Registry）
	module.Install(app, api)

	// 认证路由组（JWT + SM4 会话）
	authGroup := api.Group("")
	authGroup.Use(middleware.Auth(app), middleware.SessionKey(app), middleware.DecryptBody(), middleware.EncryptResponse(app), middleware.Audit(app))
	module.InstallProtected(app, authGroup)

	// MCP 能力端点（外部 Agent 接入：服务器地址+接口）
	if app.MCPServer != nil {
		h := gin.WrapH(http.HandlerFunc(app.MCPServer.HandleHTTP))
		engine.Any("/mcp", h)
		engine.Any("/mcp/tools", h)
		engine.Any("/mcp/call", h)
	}

	// WebSocket（链路实时监控等）
	engine.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		}
		claims, err := middleware.ParseToken(token, app.Cfg.App.JWTKey)
		if err != nil {
			response.Unauthorized(c, "WebSocket 认证失败")
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		channels := strings.Split(c.Query("channels"), ",")
		client := app.Hub.AddClient(conn, channels, claims.Username)
		app.Hub.Subscribe(client)
		go client.SendLoop()
		// 读循环：保持连接并接收心跳
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
		app.Hub.Unsubscribe(client)
		client.Close()
	})
}

var _ = strconv.Itoa
