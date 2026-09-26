// Package setup 系统初始化向导（首次启动时选择存储数据库与时序数据库、创建管理员）
package setup

import (
	"encoding/json"
	"fmt"
	"net/http"

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

type encReq struct {
	Key  string `json:"key"`
	Data string `json:"data"`
}

// doEnc 处理加密请求：解密 body -> fn(plain) -> 加密响应
func doEnc(a *core.App, c *gin.Context, fn func(plain []byte) (any, error)) {
	var req encReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Bad(c, "请求格式错误")
		return
	}
	sm4Key, err := crypto.SM2Decrypt(a.SM2.PrivateHex, req.Key)
	if err != nil {
		response.Bad(c, "密钥解密失败")
		return
	}
	plain, err := crypto.SM4Decrypt(sm4Key, req.Data)
	if err != nil {
		response.Bad(c, "数据解密失败")
		return
	}
	result, err := fn(plain)
	if err != nil {
		if bizErr, ok := err.(*bizError); ok {
			response.Fail(c, bizErr.status, bizErr.code, bizErr.msg)
		} else {
			response.Bad(c, err.Error())
		}
		return
	}
	raw, _ := json.Marshal(result)
	env, _ := crypto.SM4Encrypt(sm4Key, raw)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"enc": env}})
}

type bizError struct {
	status int
	code   int
	msg    string
}

func (e *bizError) Error() string { return e.msg }

func bizFail(status, code int, msg string) error { return &bizError{status, code, msg} }

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

	g.POST("/db-test", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		doEnc(a, c, func(plain []byte) (any, error) {
			var req config.Database
			if err := json.Unmarshal(plain, &req); err != nil {
				return nil, fmt.Errorf("参数错误: %w", err)
			}
			if err := storage.TestConnection(req); err != nil {
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, err.Error())
			}
			return gin.H{"ok": true}, nil
		})
	})

	g.POST("/tsdb-test", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		doEnc(a, c, func(plain []byte) (any, error) {
			var req config.TSDB
			if err := json.Unmarshal(plain, &req); err != nil {
				return nil, fmt.Errorf("参数错误: %w", err)
			}
			eng, err := appinit.OpenTSDB(&req)
			if err != nil {
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, err.Error())
			}
			if err := eng.Ping(); err != nil {
				eng.Close()
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, "时序库探活失败: "+err.Error())
			}
			eng.Close()
			return gin.H{"ok": true}, nil
		})
	})

	g.POST("/init", func(c *gin.Context) {
		if a.Ready {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "系统已初始化")
			return
		}
		doEnc(a, c, func(plain []byte) (any, error) {
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
			if err := json.Unmarshal(plain, &req); err != nil {
				return nil, fmt.Errorf("参数错误: %w", err)
			}
			if req.Database.Type == "" {
				return nil, fmt.Errorf("请选择存储数据库")
			}
			if req.TSDB.Type == "" {
				return nil, fmt.Errorf("请选择时序数据库")
			}
			if req.Admin.Username == "" || req.Admin.Password == "" {
				return nil, fmt.Errorf("请填写管理员账号与初始密码")
			}
			if err := password.Validate(req.Admin.Password); err != nil {
				return nil, bizFail(http.StatusUnprocessableEntity, response.CodeWeakPassword, err.Error())
			}
			if err := storage.TestConnection(req.Database); err != nil {
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, "存储数据库: "+err.Error())
			}
			eng, err := appinit.OpenTSDB(&req.TSDB)
			if err != nil {
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, "时序数据库: "+err.Error())
			}
			if err := eng.Ping(); err != nil {
				eng.Close()
				return nil, bizFail(http.StatusBadRequest, response.CodeBadRequest, "时序数据库: "+err.Error())
			}
			eng.Close()

			if err := appinit.EnsureJWTKey(a.Cfg); err != nil {
				return nil, err
			}
			a.Cfg.Database = req.Database
			a.Cfg.TSDB = req.TSDB
			a.Cfg.Initialized = true
			if err := a.Cfg.Save(); err != nil {
				return nil, fmt.Errorf("保存配置失败: %w", err)
			}
			if err := appinit.InitRuntimeAndDB(a); err != nil {
				a.Cfg.Initialized = false
				_ = a.Cfg.Save()
				return nil, err
			}
			salt, _ := crypto.GenerateSalt()
			a.DB.Where("username = ?", req.Admin.Username).Delete(&model.User{})
			admin := model.User{
				Username:      req.Admin.Username,
				EmployeeNo:    req.Admin.EmployeeNo,
				Email:         req.Admin.Email,
				PasswordHash:  crypto.PasswordHash(req.Admin.Password, salt),
				Salt:          salt,
				Role:          "admin",
				Status:        "active",
				MustChangePwd: false,
			}
			if err := a.DB.Create(&admin).Error; err != nil {
				return nil, fmt.Errorf("创建管理员失败: %w", err)
			}
			a.Ready = true
			appinit.SyncManagers(a)
			return gin.H{"ok": true, "message": "初始化完成，请使用管理员账号登录"}, nil
		})
	})
}

func init() {
	modreg.Register("setup", Register)
}
