// Package tsarchive 时序数据每日归档：每天凌晨 0 点把时序库当日聚合数据写入存储库（MySQL），
// 解决时序库保留期（TDengine KEEP 30 / 内置 30 天）导致的历史数据丢失问题，保证监控数据永久保存。
package tsarchive

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"netops/internal/common/logger"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/tsdb"
)

// 全部监控指标（与各采集器写入 tsdb 的 metric 保持一致）
var metrics = []string{
	// SNMP 设备
	"dev_up", "dev_cpu", "dev_mem", "if_in", "if_out",
	// 链路
	"link_rt", "link_loss", "link_status",
	// 数据库
	"db_avail", "db_conn", "db_size", "db_tx", "db_err", "db_cpu", "db_mem",
	// 容器 / K8s
	"docker_cpu", "docker_mem", "k8s_cpu", "k8s_mem",
	// Nginx
	"nginx_avail", "nginx_conn", "nginx_accepts", "nginx_handled", "nginx_req",
	"nginx_qps", "nginx_reading", "nginx_writing", "nginx_waiting",
	// BGP
	"bgp_peer_state",
}

var startOnce sync.Once

// 补传策略
const (
	retryMax       = 5  // 单条记录最大自动重试次数，超过则标记 discard（原始数据保留，需人工核查）
	retryWindowDay = 7  // 只补传最近 N 天内失败的归档（更早的时序数据可能已被保留期清理）
)

// Start 启动每日归档调度：
// - 每天 0:00-0:04 窗口归档前一天数据
// - 每 5 分钟补传失败记录（重传机制），保证瞬时故障自动恢复
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
					runOnce(a, now)
				} else {
					retryFailed(a, now)
				}
			}
		}()
		logger.Infof("[tsarchive] 每日归档已启动（每天 00:00 归档前一天，失败自动重传补传）")
	})
}

// runOnce 执行一次归档：先补传欠账，再归档前一天
func runOnce(a *core.App, now time.Time) {
	if a.TSDB == nil {
		logger.Warnf("[tsarchive] 时序库未初始化，跳过归档")
		return
	}
	if !settingBool(a.DB, "ts_archive_enable", true) {
		logger.Infof("[tsarchive] 归档已通过配置关闭，跳过")
		return
	}
	retryFailed(a, now)
	day := now.AddDate(0, 0, -1)
	dayStr := day.Format("2006-01-02")
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24*time.Hour - time.Millisecond)

	total := 0
	for _, m := range metrics {
		n, err := archiveMetric(a, m, start, end, dayStr)
		if err != nil {
			logger.Warnf("[tsarchive] 归档 %s(%s) 失败（已登记待补传）: %v", m, dayStr, err)
			continue
		}
		total += n
	}
	if total > 0 {
		logger.Infof("[tsarchive] 归档完成 %s：%d 条", dayStr, total)
	}
}

// retryFailed 补传失败归档（重传机制）：处理 pending 记录，成功删除、失败计数，
// 超过上限标记 discard（保留时序库原始数据，需人工核查）
func retryFailed(a *core.App, now time.Time) {
	if a.TSDB == nil {
		return
	}
	cutoff := now.AddDate(0, 0, -retryWindowDay).Format("2006-01-02")
	var items []model.TsArchiveRetry
	a.DB.Where("status = ? AND day >= ?", "pending", cutoff).Order("day asc").Limit(50).Find(&items)
	for _, item := range items {
		day, perr := time.Parse("2006-01-02", item.Day)
		if perr != nil {
			a.DB.Model(&item).Update("status", "discard")
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())
		end := start.Add(24*time.Hour - time.Millisecond)
		archiveMetric(a, item.Metric, start, end, item.Day)
		// 以记录是否残留判断成败：archiveMetric 全部校验通过时会 clearRetry 删除记录
		var cur model.TsArchiveRetry
		if a.DB.Where("metric = ? AND day = ?", item.Metric, item.Day).First(&cur).Error == nil {
			cur.Retries++
			if cur.Retries >= retryMax {
				cur.Status = "discard"
				logger.Warnf("[tsarchive] 补传 %s(%s) 已达 %d 次上限，放弃自动重试（原始数据保留在时序库）", item.Metric, item.Day, retryMax)
			}
			a.DB.Save(&cur)
		} else {
			logger.Infof("[tsarchive] 补传成功 %s(%s)", item.Metric, item.Day)
		}
	}
}

// upsertRetry 记录归档失败待补传（同 metric+day 更新失败原因）
func upsertRetry(db *gorm.DB, metric, day, reason string) {
	var r model.TsArchiveRetry
	err := db.Where("metric = ? AND day = ?", metric, day).First(&r).Error
	if err != nil {
		db.Create(&model.TsArchiveRetry{Metric: metric, Day: day, Reason: reason, Status: "pending"})
		return
	}
	db.Model(&r).Updates(map[string]any{"reason": reason, "updated_at": time.Now()})
}

// clearRetry 归档成功 → 删除对应补传记录
func clearRetry(db *gorm.DB, metric, day string) {
	db.Where("metric = ? AND day = ?", metric, day).Delete(&model.TsArchiveRetry{})
}

// archiveMetric 归档单个指标：查询该指标当天全部序列，逐序列校验后聚合写入存储库；
// 校验通过并转存的序列随即清除时序库原始数据；查询/写入/校验失败均登记待补传（重传机制）
func archiveMetric(a *core.App, metric string, start, end time.Time, dayStr string) (int, error) {
	series, err := a.TSDB.Query(tsdb.Query{Metric: metric, Start: start, End: end})
	if err == tsdb.ErrNoData {
		return 0, nil
	}
	if err != nil {
		upsertRetry(a.DB, metric, dayStr, "查询失败: "+err.Error())
		return 0, err
	}
	written := 0
	bad := 0
	failed := false // 任一序列写入/清除失败时禁止清除补传登记
	cleanup := settingBool(a.DB, "ts_archive_cleanup", true)
	for _, s := range series {
		// 完整性/正确性/非乱码性校验：校验不通过的序列不归档（保留时序库原始数据待人工检查）
		if err := validateSeries(&s, start, end); err != nil {
			bad++
			upsertRetry(a.DB, metric, dayStr, "校验未通过: "+err.Error())
			logger.Warnf("[tsarchive] 数据校验不通过，跳过归档 %s/%s: %v", metric, s.Field, err)
			continue
		}
		avg, maxV, minV, lastV, cnt := aggPoints(s.Points)
		tagsJSON, _ := json.Marshal(s.Tags)
		tagsKey := sha1Hex(tsdb.TagsKey(s.Tags))
		// 幂等：同一天同一序列已归档则跳过写入（unique 索引兜底）
		var existed int64
		a.DB.Model(&model.TsArchive{}).
			Where("metric = ? AND field = ? AND tags_key = ? AND day = ?", metric, s.Field, tagsKey, dayStr).
			Count(&existed)
		if existed == 0 {
			rec := model.TsArchive{
				Metric: metric, Field: s.Field, TagsKey: tagsKey,
				Tags: string(tagsJSON), Day: dayStr,
				AvgVal: avg, MaxVal: maxV, MinVal: minV, LastVal: lastV, Count: cnt,
			}
			if err := a.DB.Create(&rec).Error; err != nil {
				// 并发/重复写入触发唯一索引冲突时视为已归档
				failed = true
				upsertRetry(a.DB, metric, dayStr, "写入归档失败: "+err.Error())
				logger.Warnf("[tsarchive] 写入归档失败 %s/%s/%s: %v", metric, s.Field, dayStr, err)
				continue
			}
			written++
		}
		// 校验通过且已转存的序列，清除时序库原始数据（按序列粒度，避免误删异常序列）
		if cleanup {
			delQ := tsdb.Query{Metric: metric, Tags: s.Tags, Start: start, End: end}
			if del, derr := a.TSDB.Delete(delQ); derr != nil {
				failed = true
				upsertRetry(a.DB, metric, dayStr, "清除时序库失败: "+derr.Error())
				logger.Warnf("[tsarchive] 清除时序库失败 %s/%s/%s: %v", metric, s.Field, dayStr, derr)
			} else {
				logger.Infof("[tsarchive] 已清除时序库 %s/%s(%s)（批/表 %d）", metric, s.Field, dayStr, del)
			}
		}
	}
	if bad == 0 && !failed {
		// 全部序列校验通过且无写入/清除失败 → 清除补传登记
		clearRetry(a.DB, metric, dayStr)
	}
	if bad > 0 {
		logger.Warnf("[tsarchive] %s(%s) 有 %d 条序列校验未通过，已登记待补传", metric, dayStr, bad)
	}
	return written, nil
}

// validateSeries 校验序列数据的完整性、正确性与非乱码性
func validateSeries(s *tsdb.Series, start, end time.Time) error {
	if len(s.Points) == 0 {
		return fmt.Errorf("序列无采样点")
	}
	for _, p := range s.Points {
		if math.IsNaN(p[1]) || math.IsInf(p[1], 0) {
			return fmt.Errorf("数值非法(NaN/Inf)")
		}
		// 时间戳必须落在归档日期范围内（防越界/错位数据）
		ts := time.UnixMilli(int64(p[0]))
		if ts.Before(start) || ts.After(end.Add(time.Millisecond)) {
			return fmt.Errorf("采样时间超出归档范围 %s", ts.Format("2006-01-02 15:04:05"))
		}
	}
	// 非乱码性：metric/field/tags 必须为有效 UTF-8 且无可打印异常字符
	for _, s2 := range []string{s.Metric, s.Field} {
		if !isCleanText(s2) {
			return fmt.Errorf("名称含乱码: %q", s2)
		}
	}
	for k, v := range s.Tags {
		if !isCleanText(k) || !isCleanText(v) {
			return fmt.Errorf("标签含乱码: %q=%q", k, v)
		}
	}
	return nil
}

// isCleanText 校验文本为有效 UTF-8、无替换符（utf8.RuneError）、无控制字符（乱码检测）
func isCleanText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == utf8.RuneError || (r < 0x20 && r != '\t') {
			return false
		}
	}
	return true
}

// aggPoints 聚合原始采样点为 avg/max/min/last/count
func aggPoints(pts [][2]float64) (avg, maxV, minV, lastV float64, count int) {
	if len(pts) == 0 {
		return
	}
	sum := 0.0
	minV = pts[0][1]
	maxV = pts[0][1]
	for _, p := range pts {
		v := p[1]
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
		lastV = v
		count++
	}
	avg = sum / float64(count)
	return
}

func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// settingBool 读取布尔系统设置（缺省默认值）
func settingBool(db *gorm.DB, key string, def bool) bool {
	var s model.SystemSetting
	if err := db.Where("`key` = ?", key).First(&s).Error; err != nil {
		return def
	}
	return s.Value == "true"
}
