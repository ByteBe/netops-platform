// Package builtin 内置时序引擎（本地 SQLite 文件存储，零依赖、跨平台）
// 生产环境建议使用 TDengine / InfluxDB
package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"netops/internal/tsdb"
)

type point struct {
	ID     uint    `gorm:"primaryKey"`
	Metric string  `gorm:"index:idx_metric_ts;size:64"`
	Field  string  `gorm:"size:64"`
	Tags   string  `gorm:"size:512"`
	Value  float64
	TS     int64   `gorm:"index:idx_metric_ts"` // UnixMilli
}

// Engine 内置时序引擎
type Engine struct {
	db  *gorm.DB
	mu  sync.Mutex
	dir string
}

// NewBuiltin 创建内置时序引擎（db 参数为数据目录，缺省 data）
func NewBuiltin(db string) (*Engine, error) {
	if db == "" {
		db = "data"
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		return nil, err
	}
	g, err := gorm.Open(sqlite.Open(filepath.Join(db, "tsdb.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := g.AutoMigrate(&point{}); err != nil {
		return nil, err
	}
	return &Engine{db: g, dir: db}, nil
}

// Name 引擎名
func (e *Engine) Name() string { return "builtin" }

// Ping 探活
func (e *Engine) Ping() error {
	sqlDB, err := e.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Write 批量写入
func (e *Engine) Write(rows []tsdb.Row) error {
	if len(rows) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	pts := make([]point, 0, len(rows))
	for _, r := range rows {
		pts = append(pts, point{
			Metric: r.Metric,
			Field:  r.Field,
			Tags:   tsdb.TagsKey(r.Tags),
			Value:  r.Value,
			TS:     r.TS.UnixMilli(),
		})
	}
	const batch = 500
	for i := 0; i < len(pts); i += batch {
		end := i + batch
		if end > len(pts) {
			end = len(pts)
		}
		if err := e.db.CreateInBatches(pts[i:end], batch).Error; err != nil {
			return err
		}
	}
	go e.cleanup()
	return nil
}

func (e *Engine) cleanup() {
	cut := time.Now().AddDate(0, 0, -30).UnixMilli()
	e.db.Where("ts < ?", cut).Delete(&point{})
}

// Query 查询
func (e *Engine) Query(q tsdb.Query) ([]tsdb.Series, error) {
	qb := e.db.Model(&point{}).Where("metric = ?", q.Metric)
	if q.Field != "" {
		qb = qb.Where("field = ?", q.Field)
	}
	if len(q.Tags) > 0 {
		for k, v := range q.Tags {
			qb = qb.Where("tags LIKE ?", "%"+k+"="+v+";%")
		}
	}
	if !q.Start.IsZero() {
		qb = qb.Where("ts >= ?", q.Start.UnixMilli())
	}
	if !q.End.IsZero() {
		qb = qb.Where("ts <= ?", q.End.UnixMilli())
	}

	var pts []point
	if q.Limit > 0 {
		qb = qb.Limit(q.Limit)
	}
	if err := qb.Order("ts asc").Find(&pts).Error; err != nil {
		return nil, err
	}
	if len(pts) == 0 {
		return nil, tsdb.ErrNoData
	}

	groups := map[string][]point{}
	keys := []string{}
	for _, p := range pts {
		k := p.Field + "|" + p.Tags
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], p)
	}
	sort.Strings(keys)

	var out []tsdb.Series
	for _, k := range keys {
		ps := groups[k]
		field := ""
		tags := map[string]string{}
		if idx := strings.IndexByte(k, '|'); idx >= 0 {
			field = k[:idx]
			tags = parseTags(k[idx+1:])
		}
		var series tsdb.Series
		series.Metric = q.Metric
		series.Field = field
		series.Tags = tags
		series.Points = aggregate(ps, q.Bucket, q.Agg)
		out = append(out, series)
	}
	return out, nil
}

func aggregate(pts []point, bucket time.Duration, agg string) [][2]float64 {
	if bucket <= 0 {
		res := make([][2]float64, 0, len(pts))
		for _, p := range pts {
			res = append(res, [2]float64{float64(p.TS), p.Value})
		}
		return res
	}
	type bucketVal struct {
		ts   int64
		sum  float64
		n    int
		min  float64
		max  float64
		last float64
	}
	order := []int64{}
	buckets := map[int64]*bucketVal{}
	for _, p := range pts {
		bt := (p.TS / int64(bucket/time.Millisecond)) * int64(bucket/time.Millisecond)
		bv, ok := buckets[bt]
		if !ok {
			bv = &bucketVal{ts: bt, min: p.Value, max: p.Value, last: p.Value}
			buckets[bt] = bv
			order = append(order, bt)
		} else {
			if p.Value < bv.min {
				bv.min = p.Value
			}
			if p.Value > bv.max {
				bv.max = p.Value
			}
			bv.last = p.Value
		}
		bv.sum += p.Value
		bv.n++
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	res := make([][2]float64, 0, len(order))
	for _, t := range order {
		bv := buckets[t]
		var v float64
		switch agg {
		case "max":
			v = bv.max
		case "min":
			v = bv.min
		case "sum":
			v = bv.sum
		case "count":
			v = float64(bv.n)
		case "last":
			v = bv.last
		default:
			v = bv.sum / float64(bv.n)
		}
		res = append(res, [2]float64{float64(bv.ts), v})
	}
	return res
}

func parseTags(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// Close 关闭
func (e *Engine) Close() error {
	if sqlDB, err := e.db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}

var _ = json.Marshal
var _ = fmt.Sprintf
