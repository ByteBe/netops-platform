// Package middleware 等保合规：登录失败锁定 + 接口限流
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
)

const (
	// MaxLoginAttempts 最大登录失败次数
	MaxLoginAttempts = 5
	// LockDuration 锁定时长
	LockDuration = 15 * time.Minute
	// maxRatePerSec 每个IP每秒最大请求数
	maxRatePerSec = 100
)

// loginFailRecord 登录失败记录
type loginFail struct {
	count    int
	lastFail time.Time
	lockedUntil time.Time
}

var (
	loginFails = sync.Map{}
	ipLimits   = sync.Map{}
)

// LoginLockout 登录失败锁定中间件
func LoginLockout() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只对登录接口生效
		if c.Request.URL.Path != "/api/v1/auth/login" {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if v, ok := loginFails.Load(ip); ok {
			r := v.(*loginFail)
			if time.Now().Before(r.lockedUntil) {
				response.Fail(c, http.StatusTooManyRequests, response.CodeForbidden,
					"登录失败次数过多，账户已锁定，请15分钟后重试")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// RecordLoginFailure 记录登录失败
func RecordLoginFailure(ip string) {
	v, _ := loginFails.LoadOrStore(ip, &loginFail{})
	r := v.(*loginFail)
	r.count++
	r.lastFail = time.Now()
	if r.count >= MaxLoginAttempts {
		r.lockedUntil = time.Now().Add(LockDuration)
		r.count = 0
	}
}

// RecordLoginSuccess 清除登录失败记录
func RecordLoginSuccess(ip string) {
	loginFails.Delete(ip)
}

// RateLimit 接口限流中间件（每IP每秒100请求）
func RateLimit() gin.HandlerFunc {
	type bucket struct {
		count    int
		resetAt  time.Time
	}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		v, _ := ipLimits.LoadOrStore(ip, &bucket{})
		b := v.(*bucket)
		if now.After(b.resetAt) {
			b.count = 0
			b.resetAt = now.Add(time.Second)
		}
		b.count++
		if b.count > maxRatePerSec {
			response.Fail(c, http.StatusTooManyRequests, response.CodeBadRequest, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// CleanupSecurityCache 定期清理过期记录
func CleanupSecurityCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		loginFails.Range(func(key, val any) bool {
			r := val.(*loginFail)
			if now.After(r.lockedUntil) && now.Sub(r.lastFail) > LockDuration {
				loginFails.Delete(key)
			}
			return true
		})
		ipLimits.Range(func(key, val any) bool {
			b := val.(*struct {
				count   int
				resetAt time.Time
			})
			if now.After(b.resetAt.Add(time.Minute)) {
				ipLimits.Delete(key)
			}
			return true
		})
	}
}
