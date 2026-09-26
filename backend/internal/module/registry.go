// Package module 业务模块集合（空白导入全部功能模块触发 init 自注册；路由自动注册）
// 新增功能模块的接入方式：
//  1. 在本目录下新建功能文件夹，实现 Register/RegisterProtected 并在 init() 中调用 modreg.Register / modreg.RegisterProtected
//  2. 在本文件的 import 块中加入对应包的空白导入
//
// 无需修改 router 或其他任何代码，路由即自动挂载到 /api/v1/<模块名>
package module

import (
	"netops/internal/core"
	"netops/internal/modreg"

	"github.com/gin-gonic/gin"

	// 功能模块（空白导入触发 init 自注册）
	_ "netops/internal/module/auth"
	_ "netops/internal/module/containermon"
	_ "netops/internal/module/dashboard"
	_ "netops/internal/module/dbmonitor"
	_ "netops/internal/module/ipam"
	_ "netops/internal/module/linkdetect"
	_ "netops/internal/module/monitor"
	_ "netops/internal/module/report"
	_ "netops/internal/module/resource"
	_ "netops/internal/module/scriptgen"
	_ "netops/internal/module/setup"
	_ "netops/internal/module/system"
	_ "netops/internal/module/topology"
	_ "netops/internal/module/traffic"
	_ "netops/internal/module/update"
	_ "netops/internal/module/user"
)

// Install 挂载公开路由
func Install(a *core.App, root *gin.RouterGroup) { modreg.Install(a, root) }

// InstallProtected 挂载受保护路由
func InstallProtected(a *core.App, root *gin.RouterGroup) { modreg.InstallProtected(a, root) }

// RegisteredModules 已注册模块列表
func RegisteredModules() []string { return modreg.RegisteredModules() }
