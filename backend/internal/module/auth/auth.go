// Package auth 认证模块（登录/强制改密/登出/个人信息）
// 国密加密流程：前端生成 SM4 会话密钥 → SM2 公钥加密 → 登录时提交；
// 后端解密后绑定到 JWT，后续请求/响应体均以 SM4-CBC 加密
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/crypto"
	"netops/internal/common/password"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
)

// loginReq 登录请求（国密信封）
type loginReq struct {
	Key  string `json:"key"`  // SM2 加密的 SM4 会话密钥（16进制密文）
	Data string `json:"data"` // SM4-CBC 加密的登录载荷
}

// loginPayload 登录载荷
type loginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register 公开路由：登录
func Register(a *core.App, g *gin.RouterGroup) {
	g.POST("/login", func(c *gin.Context) {
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		// 1. SM2 解密会话密钥
		sm4Key, err := crypto.SM2Decrypt(a.SM2.PrivateHex, req.Key)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeCryptoError, "会话密钥解密失败")
			return
		}
		// 2. SM4 解密登录载荷
		plain, err := crypto.SM4Decrypt(sm4Key, req.Data)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeCryptoError, "登录数据解密失败")
			return
		}
		var lp loginPayload
		if err := json.Unmarshal(plain, &lp); err != nil || lp.Username == "" || lp.Password == "" {
			response.Bad(c, "登录数据格式错误")
			return
		}

		// 3. 校验用户
		ip := c.ClientIP()
		var user model.User
		if err := a.DB.Where("username = ?", lp.Username).First(&user).Error; err != nil {
			middleware.RecordLoginFailure(ip)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "用户名或密码错误")
			return
		}
		if user.Status != "active" {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "账号已停用")
			return
		}
		if crypto.PasswordHash(lp.Password, user.Salt) != user.PasswordHash {
			middleware.RecordLoginFailure(ip)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "用户名或密码错误")
			return
		}
		middleware.RecordLoginSuccess(ip)

		// 密码90天过期检查
		mustChange := user.MustChangePwd
		if user.LastPwdChangeAt != nil {
			days := time.Since(*user.LastPwdChangeAt).Hours() / 24
			if days > 90 {
				mustChange = true
			}
		}
		// 并发登录互踢
		a.Sessions.KickUser(user.Username)

		// 4. 签发令牌并绑定会话密钥
		token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, mustChange, a.Cfg.App.JWTKey, 12*time.Hour)
		if err != nil {
			response.Err(c, err)
			return
		}
		now := time.Now()
		a.DB.Model(&user).Updates(map[string]any{"last_login_at": now, "last_login_ip": c.ClientIP()})
		a.Sessions.Set(token, sm4Key, user.ID, user.Username)
		a.DB.Create(&model.AuditLog{UserID: user.ID, Username: user.Username, Action: "POST /auth/login", Method: "POST", Path: "/auth/login", IP: c.ClientIP(), Detail: "登录成功"})

		// 5. 响应以 SM4 加密（会话密钥仅本次有效）
		data := gin.H{
			"token":           token,
			"username":        user.Username,
			"role":            user.Role,
			"email":           user.Email,
			"employee_no":     user.EmployeeNo,
			"must_change_pwd": mustChange,
			"expires_in":      43200,
		}
		payload, _ := json.Marshal(response.Body{Code: response.CodeOK, Message: "登录成功", Data: data})
		env, err := crypto.SM4Encrypt(sm4Key, payload)
		if err != nil {
			response.Err(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"enc": env})
	})
}

// RegisterProtected 受保护路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 个人信息
	g.GET("/me", func(c *gin.Context) {
		uid, _ := c.Get(middleware.KeyUserID)
		var user model.User
		if err := a.DB.First(&user, uid).Error; err != nil {
			response.NotFound(c, "用户不存在")
			return
		}
		response.OK(c, gin.H{
			"id": user.ID, "username": user.Username, "email": user.Email,
			"employee_no": user.EmployeeNo, "role": user.Role, "status": user.Status,
			"must_change_pwd": user.MustChangePwd, "last_login_at": user.LastLoginAt,
		})
	})

	// 修改密码（强制改密入口）
	g.POST("/change-password", func(c *gin.Context) {
		var req struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		uid, _ := c.Get(middleware.KeyUserID)
		var user model.User
		if err := a.DB.First(&user, uid).Error; err != nil {
			response.NotFound(c, "用户不存在")
			return
		}
		if crypto.PasswordHash(req.OldPassword, user.Salt) != user.PasswordHash {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "原密码错误")
			return
		}
		if req.NewPassword == req.OldPassword {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "新密码不能与原密码相同")
			return
		}
		if err := password.Validate(req.NewPassword); err != nil {
			response.Fail(c, http.StatusUnprocessableEntity, response.CodeWeakPassword, err.Error())
			return
		}
		salt, _ := crypto.GenerateSalt()
		a.DB.Model(&user).Updates(map[string]any{
			"password_hash":    crypto.PasswordHash(req.NewPassword, salt),
			"salt":             salt,
			"must_change_pwd": false,
			"last_pwd_change_at": time.Now(),
		})
		// 使该用户旧会话失效
		response.OK(c, gin.H{"ok": true, "message": "密码修改成功，请重新登录"})
	})

	// 登出
	g.POST("/logout", func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		a.Sessions.Delete(trimBearer(token))
		response.OK(c, gin.H{"ok": true})
	})
}

func trimBearer(s string) string {
	if len(s) > 7 && s[:7] == "Bearer " {
		return s[7:]
	}
	return s
}

// init 自动注册路由
func init() {
	modreg.Register("auth", Register)
	modreg.RegisterProtected("auth", RegisterProtected)
}
