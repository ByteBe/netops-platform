// Package middleware 防重放攻击：时间戳窗口 + Nonce 一次性校验
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
)

const (
	// ReplayWindow 请求有效期（秒），超过即拒绝
	ReplayWindow = 300 // 5分钟
	// NonceCacheSize Nonce缓存最大条数
	nonceCacheSize = 100000
)

var (
	nonceCache = struct {
		sync.Map
	}{}
)

// ReplayProtection 防重放中间件
// 要求请求头携带：
//   X-Timestamp: Unix秒时间戳
//   X-Nonce:     随机字符串（一次性）
//
// 校验规则：
//  1. 时间戳与服务器时间差超过 ReplayWindow 秒 → 拒绝
//  2. Nonce 在缓存中已存在 → 拒绝（重放）
//  3. 通过后将 Nonce 加入缓存，ReplayWindow 秒后自动过期
func ReplayProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 仅静态资源不校验，所有API（含登录/初始化）都防重放
		if !isAPI(path) {
			c.Next()
			return
		}

		ts := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		if ts == "" || nonce == "" {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "缺少防重放头(X-Timestamp/X-Nonce)")
			c.Abort()
			return
		}

		// 校验时间戳
		var t int64
		for _, ch := range ts {
			if ch < '0' || ch > '9' {
				response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "时间戳格式错误")
				c.Abort()
				return
			}
			t = t*10 + int64(ch-'0')
		}
		now := time.Now().Unix()
		diff := now - t
		if diff < 0 {
			diff = -diff
		}
		if diff > ReplayWindow {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请求已过期，请检查系统时间")
			c.Abort()
			return
		}

		// 校验Nonce（一次性）
		cacheKey := nonce
		if _, loaded := nonceCache.LoadOrStore(cacheKey, time.Now().Unix()); loaded {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "重复请求")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CleanupNonceCache 定期清理过期Nonce（每分钟调用一次）
func CleanupNonceCache() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().Unix()
		nonceCache.Range(func(key, val any) bool {
			if ts, ok := val.(int64); ok && now-ts > ReplayWindow {
				nonceCache.Delete(key)
			}
			return true
		})
	}
}

func isAPI(path string) bool {
	return len(path) > 5 && path[:5] == "/api/"
}
