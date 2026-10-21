// Package tdengine TDengine 时序引擎（RESTful API，纯HTTP，跨平台）
// 表模型：每个指标一张表 t_<metric>，列为 (ts TIMESTAMP, fname VARCHAR(64), value DOUBLE)，
//
//	不同标签组合以 t_<metric>_<taghash> 区分，符合 TDengine 一表多标签-多表模型实践
package tdengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"netops/internal/common/logger"
	"netops/internal/tsdb"
)

// Engine TDengine 引擎
type Engine struct {
	base   string
	user   string
	pass   string
	db     string
	client *http.Client
}

// NewTDengine 创建 TDengine REST 连接（host/port 为 taosAdapter 地址，默认 6041）
func NewTDengine(host string, port int, user, pass, db string) (*Engine, error) {
	if port == 0 {
		port = 6041
	}
	if user == "" {
		user = "root"
	}
	if db == "" {
		db = "netops"
	}
	e := &Engine{
		base:   fmt.Sprintf("http://%s:%d", host, port),
		user:   user,
		pass:   pass,
		db:     db,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	if err := e.initDB(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) initDB() error {
	// 建库必须用不带库路径的 /rest/sql：库不存在时带库路径的请求会被 TDengine 拒绝
	if _, err := e.queryRaw(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s KEEP 30 DURATION 1", e.db)); err != nil {
		return fmt.Errorf("TDengine 初始化数据库失败: %w", err)
	}
	return nil
}

// Name 引擎名
func (e *Engine) Name() string { return "tdengine" }

// Ping 探活
func (e *Engine) Ping() error {
	_, err := e.query("SELECT server_version()")
	return err
}

func (e *Engine) exec(sql string) error {
	_, err := e.query(sql)
	return err
}

type taosResp struct {
	Code       int             `json:"code"`
	Desc       string          `json:"desc"`
	ColumnMeta [][]interface{} `json:"column_meta"`
	Data       [][]interface{} `json:"data"`
}

func (e *Engine) query(sql string) ([][]interface{}, error) {
	return e.queryAt("/rest/sql/"+e.db, sql)
}

// queryRaw 不带库路径执行（建库等库不存在场景专用）
func (e *Engine) queryRaw(sql string) ([][]interface{}, error) {
	return e.queryAt("/rest/sql", sql)
}

func (e *Engine) queryAt(path, sql string) ([][]interface{}, error) {
	req, err := http.NewRequest(http.MethodPost, e.base+path, bytes.NewBufferString(sql))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(e.user, e.pass)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr taosResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("TDengine 响应解析失败: %v, body=%s", err, body[:min(200, len(body))])
	}
	if tr.Code != 0 {
		return nil, fmt.Errorf("TDengine 错误: %s", tr.Desc)
	}
	return tr.Data, nil
}

// Write 写入
func (e *Engine) Write(rows []tsdb.Row) error {
	if len(rows) == 0 {
		return nil
	}
	// 按 指标+标签 分组，构造批量 INSERT
	byTable := map[string][]tsdb.Row{}
	order := []string{}
	for _, r := range rows {
		table := e.tableName(r.Metric, r.Tags)
		if _, ok := byTable[table]; !ok {
			order = append(order, table)
		}
		byTable[table] = append(byTable[table], r)
	}
	for _, table := range order {
		rs := byTable[table]
		// 建表
		if err := e.exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (ts TIMESTAMP, fname VARCHAR(64), val DOUBLE)", table)); err != nil {
			logger.Warnf("[tdengine] create table %s error: %v", table, err)
			return err
		}
		var sb strings.Builder
		sb.WriteString("INSERT INTO ")
		sb.WriteString(table)
		sb.WriteString(" VALUES ")
		for i, r := range rs {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(")
			sb.WriteString(strconv.FormatInt(r.TS.UnixMilli(), 10))
			sb.WriteString(",")
			sb.WriteString("'" + strings.ReplaceAll(tsdb.Sanitize(r.Field), "'", "") + "'")
			sb.WriteString(",")
			sb.WriteString(strconv.FormatFloat(r.Value, 'f', -1, 64))
			sb.WriteString(")")
		}
		if err := e.exec(sb.String()); err != nil {
			logger.Warnf("[tdengine] insert %s error: %v sql=%s", table, err, sb.String())
			return err
		}
	}
	return nil
}

func (e *Engine) tableName(metric string, tags map[string]string) string {
	name := "t_" + tsdb.Sanitize(metric)
	tk := tsdb.TagsKey(tags)
	if tk != "" {
		sum := 0
		for _, c := range tk {
			sum = (sum*31 + int(c)) & 0x7fffffff
		}
		name = fmt.Sprintf("%s_%x", name, sum)
	}
	return name
}

// Query 查询
func (e *Engine) Query(q tsdb.Query) ([]tsdb.Series, error) {
	// 无 tags：遍历该 metric 的全部子表（t_<metric> 及 t_<metric>_<taghash>），合并返回
	// 归档/全量查询场景使用；tags 非空时按单表查询
	if len(q.Tags) == 0 {
		return e.queryAllTables(q)
	}
	table := e.tableName(q.Metric, q.Tags)
	where := ""
	if q.Field != "" {
		where = fmt.Sprintf(" AND fname = '%s'", tsdb.Sanitize(q.Field))
	}
	sql := fmt.Sprintf("SELECT ts, fname, val FROM %s WHERE ts >= %d AND ts <= %d%s",
		table, q.Start.UnixMilli(), q.End.UnixMilli(), where)
	if q.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	// 构造一个通用表，便于查询时统一处理
	data, err := e.query(sql)
	if err != nil {
		if strings.Contains(err.Error(), "Table does not exist") {
			return nil, tsdb.ErrNoData
		}
		return nil, err
	}
	groups := map[string]*tsdb.Series{}
	order := []string{}
	for _, row := range data {
		if len(row) < 3 {
			continue
		}
		tsMs := parseTsMs(row[0])
		field := fmt.Sprint(row[1])
		val, _ := strconv.ParseFloat(fmt.Sprint(row[2]), 64)
		key := field
		s, ok := groups[key]
		if !ok {
			s = &tsdb.Series{Metric: q.Metric, Field: field, Tags: q.Tags}
			groups[key] = s
			order = append(order, key)
		}
		s.Points = append(s.Points, [2]float64{float64(tsMs), val})
	}
	if len(order) == 0 {
		return nil, tsdb.ErrNoData
	}
	out := make([]tsdb.Series, 0, len(order))
	for _, k := range order {
		s := groups[k]
		if q.Bucket > 0 {
			s.Points = tsdb.AggregatePoints(s.Points, q.Bucket, q.Agg)
		}
		out = append(out, *s)
	}
	return out, nil
}

// queryAllTables 无 tags 查询：枚举该 metric 的全部表（含无标签主表 t_<metric>），逐表查询并合并
// 不同标签组合以 Tags={"__table": 表名} 标记分组（TDengine 标签 hash 不可逆，无法还原原始标签）
func (e *Engine) queryAllTables(q tsdb.Query) ([]tsdb.Series, error) {
	prefix := "t_" + tsdb.Sanitize(q.Metric)
	tables, err := e.listMetricTables(prefix)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, tsdb.ErrNoData
	}
	out := []tsdb.Series{}
	for _, tbl := range tables {
		where := ""
		if q.Field != "" {
			where = fmt.Sprintf(" AND fname = '%s'", tsdb.Sanitize(q.Field))
		}
		sql := fmt.Sprintf("SELECT ts, fname, val FROM %s WHERE ts >= %d AND ts <= %d%s",
			tbl, q.Start.UnixMilli(), q.End.UnixMilli(), where)
		if q.Limit > 0 {
			sql += fmt.Sprintf(" LIMIT %d", q.Limit)
		}
		data, err := e.query(sql)
		if err != nil {
			// 表可能在枚举后被删除，跳过
			if strings.Contains(err.Error(), "Table does not exist") {
				continue
			}
			return nil, err
		}
		groups := map[string]*tsdb.Series{}
		order := []string{}
		for _, row := range data {
			if len(row) < 3 {
				continue
			}
			tsMs := parseTsMs(row[0])
			field := fmt.Sprint(row[1])
			val, _ := strconv.ParseFloat(fmt.Sprint(row[2]), 64)
			key := field + "|" + tbl
			s, ok := groups[key]
			if !ok {
				s = &tsdb.Series{Metric: q.Metric, Field: field, Tags: map[string]string{"__table": tbl}}
				groups[key] = s
				order = append(order, key)
			}
			s.Points = append(s.Points, [2]float64{float64(tsMs), val})
		}
		for _, k := range order {
			s := groups[k]
			if q.Bucket > 0 {
				s.Points = tsdb.AggregatePoints(s.Points, q.Bucket, q.Agg)
			}
			out = append(out, *s)
		}
	}
	if len(out) == 0 {
		return nil, tsdb.ErrNoData
	}
	return out, nil
}

// Delete 按 metric + 时间范围删除（归档转存校验通过后清理已归档数据）
func (e *Engine) Delete(q tsdb.Query) (int64, error) {
	tables := []string{}
	if tbl, ok := q.Tags["__table"]; ok {
		// 无 tags 查询返回的序列带 __table 标记：直接按表名删除
		tables = []string{tbl}
	} else if len(q.Tags) > 0 {
		tables = []string{e.tableName(q.Metric, q.Tags)}
	} else {
		t, err := e.listMetricTables("t_" + tsdb.Sanitize(q.Metric))
		if err != nil {
			return 0, err
		}
		tables = t
	}
	var total int64
	for _, tbl := range tables {
		sql := fmt.Sprintf("DELETE FROM %s WHERE ts >= %d AND ts <= %d", tbl, q.Start.UnixMilli(), q.End.UnixMilli())
		if _, err := e.query(sql); err != nil {
			// 表可能已被删除，跳过
			if strings.Contains(err.Error(), "Table does not exist") {
				continue
			}
			return total, err
		}
		total++
	}
	return total, nil
}

// listMetricTables 枚举前缀匹配的表名（TDengine 3.x information_schema，失败回退 SHOW TABLES）
func (e *Engine) listMetricTables(prefix string) ([]string, error) {
	rows, err := e.query(fmt.Sprintf(
		"SELECT table_name FROM information_schema.ins_tables WHERE db_name = '%s' AND table_name LIKE '%s%%'",
		e.db, prefix))
	if err != nil {
		// 兼容旧版本：SHOW TABLES 后过滤前缀
		rows, err = e.query("SHOW TABLES")
		if err != nil {
			return nil, err
		}
	}
	names := []string{}
	for _, r := range rows {
		if len(r) == 0 {
			continue
		}
		name := fmt.Sprint(r[0])
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Close 关闭
func (e *Engine) Close() error {
	e.client.CloseIdleConnections()
	return nil
}

// parseTsMs 把 TDengine REST 返回的 ts 列解析成 UnixMilli
// 可能是: int64 毫秒、float64 毫秒、"2026-10-02 16:05:34.000"、"2026-10-02T16:05:34.000+0800"
func parseTsMs(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
	case string:
		s := strings.TrimSpace(n)
		// 纯数字
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			// 超过 1e12 视为毫秒，否则秒
			if i > 1e12 {
				return i
			}
			return i * 1000
		}
		// 时间字符串
		layouts := []string{
			"2006-01-02 15:04:05.000",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05.000Z0700",
			"2006-01-02T15:04:05Z0700",
			"2006-01-02T15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				return t.UnixMilli()
			}
		}
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = sort.Strings
