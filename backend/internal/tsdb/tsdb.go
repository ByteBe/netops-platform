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
	Delete(q Query) (int64, error) // 删除匹配时间范围的数据，返回删除条数
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

// AggregatePoints 按 bucket 对 [ts_ms, value] 点做窗口聚合，返回每个非空窗口一个点 [windowStartMs, 聚合值]
// bucket<=0 或 agg 为空时原样返回；agg 支持 avg|min|max|sum|count|last，其余按 avg 处理
// 窗口与 TDengine INTERVAL 一致按 epoch 对齐，输出保持时间升序
func AggregatePoints(points [][2]float64, bucket time.Duration, agg string) [][2]float64 {
	if bucket <= 0 || len(points) == 0 {
		return points
	}
	bms := int64(bucket / time.Millisecond)
	if bms <= 0 {
		bms = 1
	}
	type acc struct {
		start int64
		sum   float64
		cnt   int
		minV  float64
		maxV  float64
		lastV float64
	}
	buckets := map[int64]*acc{}
	order := make([]int64, 0, len(points))
	for _, p := range points {
		b := int64(p[0]) / bms
		a, ok := buckets[b]
		if !ok {
			a = &acc{start: b * bms, minV: p[1], maxV: p[1]}
			buckets[b] = a
			order = append(order, b)
		}
		a.sum += p[1]
		a.cnt++
		a.lastV = p[1]
		if p[1] < a.minV {
			a.minV = p[1]
		}
		if p[1] > a.maxV {
			a.maxV = p[1]
		}
	}
	out := make([][2]float64, 0, len(order))
	agg = strings.ToLower(agg)
	for _, b := range order {
		a := buckets[b]
		var v float64
		switch agg {
		case "min":
			v = a.minV
		case "max":
			v = a.maxV
		case "sum":
			v = a.sum
		case "count":
			v = float64(a.cnt)
		case "last":
			v = a.lastV
		default:
			v = a.sum / float64(a.cnt)
		}
		out = append(out, [2]float64{float64(a.start), v})
	}
	return out
}
