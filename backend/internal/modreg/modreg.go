// Package modreg 业务模块注册中心（路由自动注册）
// 独立于 module 包，供各功能模块 init() 自注册，避免模块↔注册中心循环依赖
package modreg

import (
	"sort"

	"github.com/gin-gonic/gin"

	"netops/internal/core"
)

// ModuleFunc 模块路由注册函数
type ModuleFunc func(a *core.App, g *gin.RouterGroup)

var (
	public    = map[string]ModuleFunc{} // 无需认证（登录前可访问）
	protected = map[string]ModuleFunc{} // JWT + 国密会话
)

// Register 注册公开路由（登录前可访问，如 setup/login/crypto）
func Register(name string, f ModuleFunc) {
	if f != nil {
		public[name] = f
	}
}

// RegisterProtected 注册受保护路由（需 JWT 认证 + SM4 会话加密）
func RegisterProtected(name string, f ModuleFunc) {
	if f != nil {
		protected[name] = f
	}
}

// Install 挂载公开路由
func Install(a *core.App, root *gin.RouterGroup) {
	names := funcKeys(public)
	for _, n := range names {
		if f := public[n]; f != nil {
			f(a, root.Group("/"+n))
		}
	}
}

// InstallProtected 挂载受保护路由
func InstallProtected(a *core.App, root *gin.RouterGroup) {
	names := funcKeys(protected)
	for _, n := range names {
		if f := protected[n]; f != nil {
			f(a, root.Group("/"+n))
		}
	}
}

// RegisteredModules 已注册模块列表
func RegisteredModules() []string {
	seen := map[string]bool{}
	for n := range public {
		seen[n] = true
	}
	for n := range protected {
		seen[n] = true
	}
	return sortedKeys(seen)
}

func funcKeys(m map[string]ModuleFunc) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
