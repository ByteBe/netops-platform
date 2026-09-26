// Package setup 系统初始化向导（首次启动时选择存储数据库与时序数据库、创建管理员）
package setup

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/appinit"
	"netops/internal/common/crypto"
	"netops/internal/common/password"
	"netops/internal/common/response"
	"netops/internal/config"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/storage"
)

// Register 注册公开路由
func Register(a *core.App, g *gin.RouterGroup) {
	g.GET("/status", func(c *gin.Context) {
		response.OK(c, gin.H{
			"initialized":   a.Ready,
			"db_types":      storage.SupportedTypes(),
			"tsdb_types":    []string{"tdengine", "influxdb", "builtin"},
			"web_port":      a.Cfg.Server.Port,
			"internal_port": a.Cfg.Server.InternalPort,
		})
	})

	// 测试存储数据库连接
	g.POST("/db-test", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		var req config.Database
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if err := storage.TestConnection(req); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true})
	})

	// 测试时序数据库连接
	g.POST("/tsdb-test", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		var req config.TSDB
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		eng, err := appinit.OpenTSDB(&req)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
		if err := eng.Ping(); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "时序库探活失败: "+err.Error())
			return
		}
		eng.Close()
		response.OK(c, gin.H{"ok": true})
	})

	// 完成初始化
	g.POST("/init", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		var req struct {
			Database config.Database `json:"database"`
			TSDB     config.TSDB     `json:"tsdb"`
			Admin    struct {
				Username   string `json:"username"`
				Password   string `json:"password"`
				Email      string `json:"email"`
				EmployeeNo string `json:"employee_no"`
			} `json:"admin"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		// 校验
		if req.Database.Type == "" {
			response.Bad(c, "请选择存储数据库")
			return
		}
		if req.TSDB.Type == "" {
			response.Bad(c, "请选择时序数据库")
			return
		}
		if req.Admin.Username == "" || req.Admin.Password == "" {
			response.Bad(c, "请填写管理员账号与初始密码")
			return
		}
		if err := password.Validate(req.Admin.Password); err != nil {
			response.Fail(c, http.StatusUnprocessableEntity, response.CodeWeakPassword, err.Error())
			return
		}

		// 1. 测试连接
		if err := storage.TestConnection(req.Database); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "存储数据库: "+err.Error())
			return
		}
		if eng, err := appinit.OpenTSDB(&req.TSDB); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "时序数据库: "+err.Error())
			return
		} else if err := eng.Ping(); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "时序数据库: "+err.Error())
			return
		} else {
			eng.Close()
		}

		// 2. 持久化配置
		if err := appinit.EnsureJWTKey(a.Cfg); err != nil {
			response.Err(c, err)
			return
		}
		a.Cfg.Database = req.Database
		a.Cfg.TSDB = req.TSDB
		a.Cfg.Initialized = true
		if err := a.Cfg.Save(); err != nil {
			response.Err(c, fmt.Errorf("保存配置失败: %w", err))
			return
		}

		// 3. 初始化存储与迁移
		if err := appinit.InitRuntimeAndDB(a); err != nil {
			a.Cfg.Initialized = false
			_ = a.Cfg.Save()
			response.Err(c, err)
			return
		}

		// 4. 创建管理员
		salt, _ := crypto.GenerateSalt()
		admin := model.User{
			Username:      req.Admin.Username,
			EmployeeNo:    req.Admin.EmployeeNo,
			Email:         req.Admin.Email,
			PasswordHash:  crypto.PasswordHash(req.Admin.Password, salt),
			Salt:          salt,
			Role:          "admin",
			Status:        "active",
			MustChangePwd: true, // 初始登录强制修改密码
		}
		if err := a.DB.Create(&admin).Error; err != nil {
			response.Err(c, fmt.Errorf("创建管理员失败: %w", err))
			return
		}

		a.Ready = true
		appinit.SyncManagers(a)
		response.OK(c, gin.H{"ok": true, "message": "初始化完成，请使用管理员账号登录并修改初始密码"})
	})
}

var _ = time.Now

// init 自动注册路由
func init() {
	modreg.Register("setup", Register)
}
