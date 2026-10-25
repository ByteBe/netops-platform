// Package cleaner 存量数据清洗服务（Python 智能外围唯一实现）：对 MySQL / TDengine 已存数据做
// 低频批量清洗（告警去重、日志规范化、IP 登记 MAC 规范化、时序异常值统计）。
// 默认 dry_run=true 只统计不修改；dry_run=false 才执行清洗。Go 仅负责鉴权路由转发，不做本地兜底。
package cleaner

import (
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/modreg"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.POST("/clean/run", func(c *gin.Context) {
		var req struct {
			Mode   string `json:"mode"`    // mysql | tdengine | all
			DryRun bool   `json:"dry_run"` // 默认 true 只统计
			Metric string `json:"metric"`  // tdengine 模式指定指标
			Days   int    `json:"days"`    // tdengine 时间窗口
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if req.Mode == "" {
			req.Mode = "all"
		}
		if req.Metric == "" {
			req.Metric = "dev_cpu"
		}
		if req.Days <= 0 {
			req.Days = 7
		}
		var res struct {
			OK      bool  `json:"ok"`
			DryRun  bool  `json:"dry_run"`
			Reports []any `json:"reports"`
		}
		payload := map[string]any{"mode": req.Mode, "dry_run": req.DryRun, "metric": req.Metric, "days": req.Days}
		if err := a.PySvc.PostJSON("/internal/clean/run", payload, &res, 120*time.Second); err != nil {
			response.Fail(c, 502, 50200, "数据清洗服务不可用，请检查 Python 智能外围（netops-python）服务状态")
			return
		}
		response.OK(c, gin.H{"ok": res.OK, "dry_run": res.DryRun, "reports": res.Reports})
	})
}

func init() { modreg.RegisterProtected("cleaner", RegisterProtected) }
