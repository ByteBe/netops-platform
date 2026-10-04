// Package system 系统管理：AI接入 / MCP配置 / 邮箱 / 系统参数 / 数据备份
package system

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/ai"
	"netops/internal/service/email"
	"netops/internal/service/mcp"
)

// RegisterProtected 路由（AI/MCP/邮箱配置仅管理员可写）
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 已注册模块列表（路由自动注册验证）
	g.GET("/modules", func(c *gin.Context) {
		response.OK(c, modreg.RegisteredModules())
	})
	g.GET("/ai/list", func(c *gin.Context) {
		var list []model.AIConfig
		a.DB.Order("priority asc, id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/ai", middleware.RequireRole("admin"), func(c *gin.Context) {
		var cfg model.AIConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if cfg.Name == "" || cfg.BaseURL == "" {
			response.Bad(c, "名称与BaseURL必填")
			return
		}
		if cfg.Model == "" {
			cfg.Model = "gpt-3.5-turbo"
		}
		if err := a.DB.Create(&cfg).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, cfg)
	})
	g.PUT("/ai/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var cfg model.AIConfig
		if err := a.DB.First(&cfg, id).Error; err != nil {
			response.NotFound(c, "AI配置不存在")
			return
		}
		var req model.AIConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&cfg).Updates(map[string]any{
			"name": req.Name, "provider": req.Provider, "base_url": req.BaseURL,
			"api_key": req.APIKey, "model": req.Model, "temperature": req.Temperature,
			"priority": req.Priority, "enable": req.Enable, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/ai/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.AIConfig{}, id)
		response.OK(c, gin.H{"ok": true})
	})
	// AI 测试
	g.POST("/ai/:id/test", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var cfg model.AIConfig
		if err := a.DB.First(&cfg, id).Error; err != nil {
			response.NotFound(c, "AI配置不存在")
			return
		}
		reply, err := ai.Chat(ai.Provider{Name: cfg.Name, BaseURL: cfg.BaseURL,
			APIKey: cfg.APIKey, Model: cfg.Model, Temperature: cfg.Temperature},
			[]ai.Message{{Role: "user", Content: "你好，请回复：NetOps AI 连接正常"}})
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "AI测试失败: "+err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "reply": reply})
	})
	// AI 对话（多AI调度：按优先级选择启用的第一个；失败自动切换下一个）
	g.POST("/ai/chat", func(c *gin.Context) {
		var req struct {
			Messages []ai.Message `json:"messages"`
			Name     string       `json:"name"` // 指定AI名，空则自动调度
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if len(req.Messages) == 0 {
			response.Bad(c, "消息不能为空")
			return
		}
		var cfgs []model.AIConfig
		q := a.DB.Where("enable = ?", true)
		if req.Name != "" {
			q = q.Where("name = ?", req.Name)
		}
		q.Order("priority asc, id asc").Find(&cfgs)
		if len(cfgs) == 0 {
			response.Bad(c, "未配置可用的AI服务")
			return
		}
		var lastErr error
		for _, cfg := range cfgs {
			reply, err := ai.Chat(ai.Provider{Name: cfg.Name, BaseURL: cfg.BaseURL,
				APIKey: cfg.APIKey, Model: cfg.Model, Temperature: cfg.Temperature}, req.Messages)
			if err == nil {
				response.OK(c, gin.H{"reply": reply, "provider": cfg.Name, "model": cfg.Model})
				return
			}
			lastErr = err
		}
		response.Fail(c, 500, response.CodeServerError, "所有AI服务均不可用: "+lastErr.Error())
	})
	// AI 提供方模板
	g.GET("/ai/providers", func(c *gin.Context) {
		response.OK(c, []gin.H{
			{"value": "openai", "label": "OpenAI", "base_url": "https://api.openai.com/v1"},
			{"value": "ollama", "label": "Ollama(本地)", "base_url": "http://127.0.0.1:11434/v1"},
			{"value": "deepseek", "label": "DeepSeek", "base_url": "https://api.deepseek.com/v1"},
			{"value": "qwen", "label": "通义千问", "base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1"},
			{"value": "glm", "label": "智谱GLM", "base_url": "https://open.bigmodel.cn/api/paas/v4"},
			{"value": "doubao", "label": "豆包", "base_url": "https://ark.cn-beijing.volces.com/api/v3"},
			{"value": "azure", "label": "Azure OpenAI", "base_url": "https://<resource>.openai.azure.com/openai/deployments/<deploy>/"},
			{"value": "custom", "label": "自定义(OpenAI兼容)", "base_url": ""},
		})
	})

	// ===== MCP 配置（服务器地址+接口形式接入主流 Agent） =====
	g.GET("/mcp/agents", func(c *gin.Context) {
		var list []model.MCPAgent
		a.DB.Order("id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/mcp/agents", middleware.RequireRole("admin"), func(c *gin.Context) {
		var agent model.MCPAgent
		if err := c.ShouldBindJSON(&agent); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if agent.Name == "" || agent.ServerURL == "" {
			response.Bad(c, "名称与服务器地址必填")
			return
		}
		if err := a.DB.Create(&agent).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, agent)
	})
	g.PUT("/mcp/agents/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var agent model.MCPAgent
		if err := a.DB.First(&agent, id).Error; err != nil {
			response.NotFound(c, "Agent不存在")
			return
		}
		var req model.MCPAgent
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&agent).Updates(map[string]any{
			"name": req.Name, "server_url": req.ServerURL, "endpoint": req.Endpoint,
			"auth_type": req.AuthType, "auth_token": req.AuthToken, "enable": req.Enable, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/mcp/agents/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.MCPAgent{}, id)
		response.OK(c, gin.H{"ok": true})
	})
	// 测试 Agent 连接
	g.POST("/mcp/agents/:id/test", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var agent model.MCPAgent
		if err := a.DB.First(&agent, id).Error; err != nil {
			response.NotFound(c, "Agent不存在")
			return
		}
		out, err := mcp.CallAgent(mcp.Agent{Name: agent.Name, ServerURL: agent.ServerURL,
			Endpoint: agent.Endpoint, AuthToken: agent.AuthToken}, "GET", "")
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "Agent连接失败: "+err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "output": out})
	})
	// 本系统 MCP 端点信息（Agent 接入本系统用）
	g.GET("/mcp/info", func(c *gin.Context) {
		response.OK(c, gin.H{
			"server":         "http://<本机地址>:" + strconv.Itoa(a.Cfg.Server.Port),
			"tools_endpoint": "/mcp/tools",
			"call_endpoint":  "/mcp/call",
			"tools":          mcpToolNames(a),
		})
	})

	// ===== 邮箱 =====
	g.GET("/email/list", func(c *gin.Context) {
		var list []model.EmailConfig
		a.DB.Order("id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/email", middleware.RequireRole("admin"), func(c *gin.Context) {
		var cfg model.EmailConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if cfg.Name == "" || cfg.SMTPHost == "" {
			response.Bad(c, "名称与SMTP服务器必填")
			return
		}
		if cfg.SMTPPort == 0 {
			cfg.SMTPPort = 465
		}
		if err := a.DB.Create(&cfg).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, cfg)
	})
	g.PUT("/email/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var cfg model.EmailConfig
		if err := a.DB.First(&cfg, id).Error; err != nil {
			response.NotFound(c, "邮箱配置不存在")
			return
		}
		var req model.EmailConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&cfg).Updates(map[string]any{
			"name": req.Name, "smtp_host": req.SMTPHost, "smtp_port": req.SMTPPort,
			"user": req.User, "password": req.Password, "use_ssl": req.UseSSL,
			"enable": req.Enable, "default_to": req.DefaultTo, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/email/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.EmailConfig{}, id)
		response.OK(c, gin.H{"ok": true})
	})
	// 测试邮件发送（通过用户填写的邮箱）
	g.POST("/email/:id/test", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var cfg model.EmailConfig
		if err := a.DB.First(&cfg, id).Error; err != nil {
			response.NotFound(c, "邮箱配置不存在")
			return
		}
		to := splitTo(cfg.DefaultTo)
		if len(to) == 0 {
			var users []model.User
			a.DB.Where("email != ''").Limit(5).Find(&users)
			for _, u := range users {
				to = append(to, u.Email)
			}
		}
		if len(to) == 0 {
			response.Bad(c, "没有可用收件人（请填写用户邮箱或默认收件人）")
			return
		}
		err := email.Send(email.Config{Name: cfg.Name, SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort,
			User: cfg.User, Password: cfg.Password, UseSSL: cfg.UseSSL, Enable: true},
			to, "NetOps 邮箱配置测试", "<h3>NetOps 平台邮箱配置测试成功</h3>")
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "邮件发送失败: "+err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "to": to})
	})

	// 系统参数（全局开关）
	g.GET("/settings", func(c *gin.Context) {
		var list []model.SystemSetting
		a.DB.Find(&list)
		response.OK(c, list)
	})
	g.PUT("/settings/:key", middleware.RequireRole("admin"), func(c *gin.Context) {
		key := c.Param("key")
		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		var s model.SystemSetting
		if err := a.DB.Where("`key` = ?", key).First(&s).Error; err != nil {
			a.DB.Create(&model.SystemSetting{Key: key, Value: req.Value})
		} else {
			a.DB.Model(&s).Update("value", req.Value)
		}
		response.OK(c, gin.H{"ok": true})
	})

	// 安全配置
	g.GET("/security", func(c *gin.Context) {
		get := func(key, def string) string {
			var s model.SystemSetting
			if err := a.DB.Where("`key`=?", key).First(&s).Error; err != nil { return def }
			return s.Value
		}
		getInt := func(key string, def int) int {
			v := get(key, "")
			if v == "" { return def }
			n, _ := strconv.Atoi(v)
			return n
		}
		response.OK(c, gin.H{
			"tls_cert": get("tls_cert", ""), "tls_key": get("tls_key", ""),
			"pwd_max_days": getInt("pwd_max_days", 90),
			"login_max_fail": getInt("login_max_fail", 5),
			"login_lock_min": getInt("login_lock_min", 15),
			"audit_retention_days": getInt("audit_retention_days", 180),
			"max_session_per_user": getInt("max_session_per_user", 1),
		})
	})
	g.PUT("/security", middleware.RequireRole("admin"), func(c *gin.Context) {
		var req struct {
			TLSCert string `json:"tls_cert"`
			TLSKey string `json:"tls_key"`
			PwdMaxDays int `json:"pwd_max_days"`
			LoginMaxFail int `json:"login_max_fail"`
			LoginLockMin int `json:"login_lock_min"`
			AuditRetentionDays int `json:"audit_retention_days"`
			MaxSessionPerUser int `json:"max_session_per_user"`
		}
		if err := c.ShouldBindJSON(&req); err != nil { response.Bad(c, "参数错误"); return }
		upd := func(key, val string) {
			var s model.SystemSetting
			if err := a.DB.Where("`key`=?", key).First(&s).Error; err != nil {
				a.DB.Create(&model.SystemSetting{Key: key, Value: val})
			} else {
				a.DB.Model(&s).Update("value", val)
			}
		}
		upd("tls_cert", req.TLSCert)
		upd("tls_key", req.TLSKey)
		upd("pwd_max_days", strconv.Itoa(req.PwdMaxDays))
		upd("login_max_fail", strconv.Itoa(req.LoginMaxFail))
		upd("login_lock_min", strconv.Itoa(req.LoginLockMin))
		upd("audit_retention_days", strconv.Itoa(req.AuditRetentionDays))
		upd("max_session_per_user", strconv.Itoa(req.MaxSessionPerUser))
		response.OK(c, gin.H{"ok": true})
	})

	// 审计日志
	g.GET("/audit", middleware.RequireRole("admin"), func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		var total int64
		a.DB.Model(&model.AuditLog{}).Count(&total)
		var list []model.AuditLog
		a.DB.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
		response.OK(c, gin.H{"total": total, "list": list})
	})

	// 数据备份（下载zip）
	g.GET("/backup", middleware.RequireRole("admin"), func(c *gin.Context) {
		tmp, err := os.CreateTemp("", "netops-backup-*.zip")
		if err != nil {
			response.Err(c, err)
			return
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		zw := zip.NewWriter(tmp)
		filepath.Walk("data", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			w, err := zw.Create(path)
			if err != nil {
				return nil
			}
			io.Copy(w, f)
			return nil
		})
		if _, err := os.Stat("config.yaml"); err == nil {
			f, _ := os.Open("config.yaml")
			defer f.Close()
			w, _ := zw.Create("config.yaml")
			io.Copy(w, f)
		}
		zw.Close()
		filename := fmt.Sprintf("netops-backup-%s.zip", time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Header("Content-Type", "application/zip")
		c.File(tmp.Name())
	})
}

func mcpToolNames(a *core.App) []string {
	if a.MCPServer == nil {
		return []string{}
	}
	return a.MCPServer.ToolNames()
}

func splitTo(s string) []string {
	var out []string
	for _, v := range split(s, ",") {
		v = trim(v)
		if v != "" && containsAt(v) {
			out = append(out, v)
		}
	}
	return out
}

func split(s, sep string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if string(r) == sep {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

func containsAt(s string) bool {
	return strings.Contains(s, "@")
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("system", RegisterProtected)
}

