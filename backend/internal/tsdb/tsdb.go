// Package tsdb 时序数据库统一抽象（TDengine / InfluxDB / 内置SQLite）
// 设计：扁平行模型（metric + field + tags），适配三种引擎的统一写入与聚合查询
package tsdb

import (
	"errors"
	"strings"
	"time"
)

// Row 单条指标行
type Row struct {
	Metric string            `json:"metric"` // 指标名，如 link_rt / dev_cpu / if_in
	Field  string            `json:"field"`  // 字段名，如 value / rt / loss
	Tags   map[string]string `json:"tags"`   // 标签，如 device_id / task_id / name
	Value  float64           `json:"value"`
	TS     time.Time         `json:"ts"`
}

// Series 时间序列（Points 为 [ts_ms, value] 对）
type Series struct {
	Metric string            `json:"metric"`
	Field  string            `json:"field"`
	Tags   map[string]string `json:"tags"`
	Points [][2]float64      `json:"points"`
}

// Query 聚合查询
type Query struct {
	Metric string
	Field  string
	Tags   map[string]string // 精确匹配（AND）
	Start  time.Time
	End    time.Time
	Bucket time.Duration // 聚合桶，0 表示不聚合
	Agg    string        // avg|max|min|sum|count|last
	Limit  int
}

// Engine 时序引擎接口
type Engine interface {
	Name() string
	Write(rows []Row) error
	Query(q Query) ([]Series, error)
	Ping() error
	Close() error
}

// ErrNoData 查询无数据
var ErrNoData = errors.New("no data")

// Factory 由 appinit.OpenTSDB 提供（避免子包循环引用）

// TagsKey 生成稳定的标签签名（用于建表/分组）
func TagsKey(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	var sb strings.Builder
	for k, v := range tags {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(v)
		sb.WriteString(";")
	}
	return sb.String()
}

// Sanitize 清理标识符
func Sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}
