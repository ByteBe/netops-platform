// Package influxdb InfluxDB 时序引擎（HTTP API，兼容 v1 写读 / v2 写入端点）
// 度量命名：<metric>_<field>，写入行协议；读取使用 InfluxQL（v1 /query 兼容端点）
package influxdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"netops/internal/tsdb"
)

// Engine Influx 引擎
type Engine struct {
	base   string
	user   string
	pass   string
	token  string // v2 token（可选，优先于 user/pass）
	db     string
	org    string
	bucket string
	client *http.Client
}

// NewInflux 创建 InfluxDB 连接
// params 可携带 org=<org>;bucket=<bucket>;token=<token>
func NewInflux(host string, port int, user, pass, db, params string) (*Engine, error) {
	if port == 0 {
		port = 8086
	}
	if db == "" {
		db = "netops"
	}
	e := &Engine{
		base:   fmt.Sprintf("http://%s:%d", host, port),
		user:   user,
		pass:   pass,
		db:     db,
		org:    db,
		bucket: db,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	for _, kv := range strings.Split(params, ";") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) != 2 {
			continue
		}
		switch p[0] {
		case "token":
			e.token = p[1]
		case "org":
			e.org = p[1]
		case "bucket":
			e.bucket = p[1]
		}
	}
	if err := e.initDB(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) initDB() error {
	// v1 兼容创建数据库
	req, _ := http.NewRequest(http.MethodPost, e.base+"/query", nil)
	q := req.URL.Query()
	q.Set("q", "CREATE DATABASE "+e.db)
	req.URL.RawQuery = q.Encode()
	e.auth(req)
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("InfluxDB 连接失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// Name 引擎名
func (e *Engine) Name() string { return "influxdb" }

// Ping 探活
func (e *Engine) Ping() error {
	resp, err := e.client.Get(e.base + "/ping")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (e *Engine) auth(req *http.Request) {
	if e.token != "" {
		req.Header.Set("Authorization", "Token "+e.token)
	} else if e.user != "" {
		req.SetBasicAuth(e.user, e.pass)
	}
}

// Write 写入（行协议）
func (e *Engine) Write(rows []tsdb.Row) error {
	if len(rows) == 0 {
		return nil
	}
	var sb strings.Builder
	for _, r := range rows {
		measurement := tsdb.Sanitize(r.Metric) + "_" + tsdb.Sanitize(r.Field)
		sb.WriteString(measurement)
		// tags 排序保证写入一致
		keys := make([]string, 0, len(r.Tags))
		for k := range r.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString("," + k + "=" + escapeTag(r.Tags[k]))
		}
		sb.WriteString(" value=" + strconv.FormatFloat(r.Value, 'f', -1, 64))
		sb.WriteString(" " + strconv.FormatInt(r.TS.UnixNano(), 10) + "\n")
	}
	u := fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=ns", e.base, url.QueryEscape(e.org), url.QueryEscape(e.bucket))
	req, _ := http.NewRequest(http.MethodPost, u, strings.NewReader(sb.String()))
	e.auth(req)
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("InfluxDB 写入失败 %d: %s", resp.StatusCode, string(b[:min(200, len(b))]))
	}
	return nil
}

func escapeTag(s string) string {
	s = strings.ReplaceAll(s, " ", "\\ ")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "=", "\\=")
	return s
}

// Query 查询（InfluxQL）
func (e *Engine) Query(q tsdb.Query) ([]tsdb.Series, error) {
	measurement := tsdb.Sanitize(q.Metric)
	if q.Field != "" {
		measurement += "_" + tsdb.Sanitize(q.Field)
	}
	sel := "value"
	if q.Agg != "" {
		sel = fmt.Sprintf("%s(value)", q.Agg)
	}
	sql := fmt.Sprintf("SELECT %s AS v FROM \"%s\"", sel, measurement)
	conds := []string{}
	if !q.Start.IsZero() {
		conds = append(conds, "time >= "+strconv.FormatInt(q.Start.UnixNano(), 10))
	}
	if !q.End.IsZero() {
		conds = append(conds, "time <= "+strconv.FormatInt(q.End.UnixNano(), 10))
	}
	for k, v := range q.Tags {
		conds = append(conds, fmt.Sprintf("\"%s\" = '%s'", k, strings.ReplaceAll(v, "'", "")))
	}
	if len(conds) > 0 {
		sql += " WHERE " + strings.Join(conds, " AND ")
	}
	if q.Bucket > 0 {
		sql += fmt.Sprintf(" GROUP BY time(%dms)", q.Bucket.Milliseconds())
	}
	if q.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	req, _ := http.NewRequest(http.MethodPost, e.base+"/query", strings.NewReader("q="+url.QueryEscape(sql)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.auth(req)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var ir struct {
		Results []struct {
			Series []struct {
				Name    string          `json:"name"`
				Tags    map[string]string `json:"tags"`
				Columns []string        `json:"columns"`
				Values  [][]interface{} `json:"values"`
			} `json:"series"`
			Error string `json:"error"`
		} `json:"results"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &ir); err != nil {
		return nil, err
	}
	if ir.Error != "" {
		return nil, fmt.Errorf("InfluxDB 查询错误: %s", ir.Error)
	}
	out := []tsdb.Series{}
	for _, res := range ir.Results {
		if res.Error != "" {
			return nil, fmt.Errorf("InfluxDB 查询错误: %s", res.Error)
		}
		for _, s := range res.Series {
			series := tsdb.Series{Metric: q.Metric, Field: q.Field, Tags: s.Tags}
			if s.Tags == nil {
				series.Tags = q.Tags
			}
			for _, row := range s.Values {
				if len(row) < 2 {
					continue
				}
				ts, err := parseInfluxTime(fmt.Sprint(row[0]))
				if err != nil {
					continue
				}
				v, err := strconv.ParseFloat(fmt.Sprint(row[1]), 64)
				if err != nil {
					continue
				}
				series.Points = append(series.Points, [2]float64{float64(ts.UnixMilli()), v})
			}
			out = append(out, series)
		}
	}
	if len(out) == 0 {
		return nil, tsdb.ErrNoData
	}
	return out, nil
}

func parseInfluxTime(s string) (time.Time, error) {
	// RFC3339 或纳秒时间戳
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ns), nil
}

// Close 关闭
func (e *Engine) Close() error {
	e.client.CloseIdleConnections()
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
