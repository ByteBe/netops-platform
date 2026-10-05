// Package middleware 中间件：CORS / JWT认证 / 国密SM4加解密 / 强制改密 / 审计
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"netops/internal/common/crypto"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
)

// ContextKey 上下文键
const (
	KeyUserID   = "uid"
	KeyUsername = "username"
	KeyRole     = "role"
	KeyNodeScope = "node_scope"
	KeyMustPwd  = "must_change"
	KeySession  = "session_key"
)

// CORS 跨域（开发模式）
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Enc, X-Request-Id")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth JWT 认证 + 强制改密拦截
func Auth(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" {
			response.Unauthorized(c, "未登录或凭证缺失")
			c.Abort()
			return
		}
		claims, err := ParseToken(token, app.Cfg.App.JWTKey)
		if err != nil {
			response.Unauthorized(c, "登录凭证无效或已过期")
			c.Abort()
			return
		}
		c.Set(KeyUserID, claims.UserID)
		c.Set(KeyUsername, claims.Username)
		c.Set(KeyRole, claims.Role)
		c.Set(KeyNodeScope, claims.NodeScope)
		c.Set(KeyMustPwd, claims.MustChangePwd)

		// 强制改密拦截：除白名单外一律拒绝
		if claims.MustChangePwd {
			path := c.Request.URL.Path
			allow := path == "/api/v1/auth/me" ||
				path == "/api/v1/auth/change-password" ||
				path == "/api/v1/auth/logout"
			if !allow {
				response.MustChangePwd(c)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// SessionKey 绑定会话密钥（登录后调用）
// 若 JWT 有效但内存会话密钥缺失（如服务重启导致旧会话失效），
// 视为会话已失效，返回 40100 让前端引导重新登录，避免出现"解密失败/参数错误"等误导性错误
func SessionKey(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.Next()
			return
		}
		if key, _, _, ok := app.Sessions.Get(token); ok {
			c.Set(KeySession, key)
			c.Next()
			return
		}
		response.Unauthorized(c, "会话已失效，请重新登录")
		c.Abort()
	}
}

// DecryptBody SM4 请求体解密（X-Enc: sm4 时启用）
func DecryptBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		enc := c.GetHeader("X-Enc")
		keyAny, ok := c.Get(KeySession)
		if enc == "sm4" && ok && c.Request.Body != nil && c.Request.Body != http.NoBody {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil && len(body) > 0 {
				plain, err := crypto.SM4Decrypt(keyAny.([]byte), string(body))
				if err != nil {
					response.Fail(c, http.StatusBadRequest, response.CodeCryptoError, "请求体解密失败")
					c.Abort()
					return
				}
				c.Request.Body = io.NopCloser(bytes.NewReader(plain))
				// 清理原 Content-Length
				c.Request.Header.Del("Content-Length")
			}
		}
		c.Next()
	}
}

// EncryptResponse SM4 响应体加密（存在会话密钥时对 JSON 响应整体加密）
func EncryptResponse(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasSuffix(c.Request.URL.Path, "/html") {
			c.Next()
			return
		}
		keyAny, ok := c.Get(KeySession)
		if !ok {
			c.Next()
			return
		}
		key := keyAny.([]byte)
		w := &encWriter{ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if w.captured && w.buf.Len() > 0 {
			env, err := crypto.SM4Encrypt(key, w.buf.Bytes())
			if err != nil {
				return
			}
			payload, _ := json.Marshal(map[string]string{"enc": env})
			w.ResponseWriter.Header().Set("Content-Type", "application/json")
			w.ResponseWriter.Header().Del("Content-Length")
			_, _ = w.ResponseWriter.Write(payload)
		}
	}
}

// encWriter 捕获响应体
type encWriter struct {
	gin.ResponseWriter
	buf      bytes.Buffer
	captured bool
}

func (w *encWriter) Write(b []byte) (int, error) {
	w.captured = true
	return w.buf.Write(b)
}

func (w *encWriter) WriteString(s string) (int, error) {
	w.captured = true
	return w.buf.WriteString(s)
}

// WriteHeader 透传
func (w *encWriter) WriteHeader(code int) { w.ResponseWriter.WriteHeader(code) }

// Audit 操作审计
func Audit(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先读取请求体用于审计，然后恢复 body 供后续中间件/handler 使用
		var detail string
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodOptions && c.Request.Body != nil {
			if b, err := io.ReadAll(c.Request.Body); err == nil && len(b) > 0 {
				if len(b) < 1024 {
					detail = string(b)
				} else {
					detail = string(b[:1024]) + "..."
				}
				c.Request.Body = io.NopCloser(bytes.NewReader(b))
			}
		}
		c.Next()
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions {
			return
		}
		uid, _ := c.Get(KeyUserID)
		username, _ := c.Get(KeyUsername)
		if app.DB == nil {
			return
		}
		log := model.AuditLog{
			Action: c.Request.Method + " " + c.Request.URL.Path,
			Method: c.Request.Method, Path: c.Request.URL.Path, IP: c.ClientIP(), Detail: detail,
		}
		if v, ok := uid.(uint); ok {
			log.UserID = v
		}
		if v, ok := username.(string); ok {
			log.Username = v
		}
		app.DB.Create(&log)
	}
}

// RequireRole 角色校验
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(KeyRole)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		response.Forbidden(c, "无权限执行此操作")
		c.Abort()
	}
}

// ScopeNodeID 返回当前用户可见的 node_id 过滤值
// all = 总部管理员，看全部；其他 = 仅本节点（下级数据由 node_id 关联，前端按 node_scope 过滤）
// 返回空字符串表示不过滤（看全部）；返回具体 node_id 表示只看该节点
func ScopeNodeID(c *gin.Context) string {
	v, _ := c.Get(KeyNodeScope)
	s, _ := v.(string)
	if s == "" || s == "all" {
		return ""
	}
	return s
}
