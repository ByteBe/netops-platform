// Package tsarchive 时序数据每日归档调度：每天凌晨 0 点把时序库当日聚合数据写入存储库（MySQL），
// 解决时序库保留期（TDengine KEEP 30 / 内置 30 天）导致的历史数据丢失问题，保证监控数据永久保存。
// 归档执行（查时序库/校验/聚合/写存储库/清理原始数据/补传）由 Python 智能外围唯一实现，
// Go 只负责调度触发与日志，不再本地兜底实现。
package tsarchive

import (
	"sync"
	"time"

	"gorm.io/gorm"

	"netops/internal/common/logger"
	"netops/internal/core"
	"netops/internal/model"
)

var startOnce sync.Once

// Start 启动每日归档调度：
// - 每天 0:00-0:04 窗口归档前一天数据（mode=daily）
// - 每 5 分钟补传失败记录（mode=retry），保证瞬时故障自动恢复
func Start(a *core.App) {
	startOnce.Do(func() {
		go func() {
			tk := time.NewTicker(5 * time.Minute)
			defer tk.Stop()
			for now := range tk.C {
				if !settingBool(a.DB, "ts_archive_enable", true) {
					continue
				}
				if now.Hour() == 0 && now.Minute() < 5 {
					callPython(a, "daily", now)
				} else {
					callPython(a, "retry", now)
				}
			}
		}()
		logger.Infof("[tsarchive] 每日归档已启动（每天 00:00 归档前一天，失败自动重传补传）")
	})
}

// callPython 调用 Python 智能外围执行归档（daily=补传+归档前一天；retry=补传 pending）。
// Python 不可用时仅记录日志，等待下一轮调度触发（归档为后台任务，无需向用户返回错误）。
func callPython(a *core.App, mode string, now time.Time) {
	if a.TSDB == nil {
		logger.Warnf("[tsarchive] 时序库未初始化，跳过归档（%s）", mode)
		return
	}
	var res struct {
		Ok      bool     `json:"ok"`
		Total   int      `json:"total"`
		Retried int      `json:"retried"`
		Day     string   `json:"day"`
		Errors  []string `json:"errors"`
	}
	if err := a.PySvc.PostJSON("/internal/tsarchive/run",
		map[string]any{"mode": mode, "now": now.Format("2006-01-02 15:04:05")}, &res, 120*time.Second); err != nil {
		logger.Warnf("[tsarchive] Python 智能外围不可用（%s 模式）: %v", mode, err)
		return
	}
	if res.Ok {
		logger.Infof("[tsarchive] Python 归档完成（%s）: 归档 %d 条，补传 %d 条，日 %s", mode, res.Total, res.Retried, res.Day)
	} else {
		logger.Warnf("[tsarchive] Python 归档执行异常（%s）: %v", mode, res.Errors)
	}
}

// settingBool 读取布尔系统设置（缺省默认值）
func settingBool(db *gorm.DB, key string, def bool) bool {
	var s model.SystemSetting
	if err := db.Where("`key` = ?", key).First(&s).Error; err != nil {
		return def
	}
	return s.Value == "true"
}
