// Package nginxprobe Nginx 监控探针
// 通过 ngx_http_stub_status_module 的状态页（/nginx_status）采集连接与请求指标，
// 覆盖：可用性/状态、活跃连接、请求速率(QPS)、Reading/Writing/Waiting 连接数
package nginxprobe

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"netops/internal/notify"
	"netops/internal/tsdb"
)

// Instance 监控实例配置
type Instance struct {
	ID        uint
	Name      string
	StatusURL string // stub_status 页面地址
	Interval  int
}

// Snapshot 实例采集快照
type Snapshot struct {
	InstanceID  uint      `json:"instance_id"`
	Name        string    `json:"name"`
	Up          bool      `json:"up"`
	Status      string    `json:"status"`
	ActiveConns float64   `json:"active_conns"` // Active connections
	Accepts     float64   `json:"accepts"`      // 累计接受连接数
	Handled     float64   `json:"handled"`      // 累计处理连接数
	Requests    float64   `json:"requests"`     // 累计请求数
	Reading     float64   `json:"reading"`      // 正在读取请求头的连接数
	Writing     float64   `json:"writing"`      // 正在写响应的连接数
	Waiting     float64   `json:"waiting"`      // 空闲 keep-alive 连接数
	QPS         float64   `json:"qps"`          // 请求速率（次/秒，由增量计算）
	Message     string    `json:"message"`
	TS          time.Time `json:"ts"`
}

// Manager Nginx 监控管理器
type Manager struct {
	mu       sync.RWMutex
	insts    map[uint]*Instance
	runs     map[uint]chan struct{}
	snap     map[uint]*Snapshot
	prevReq  map[uint]float64
	prevTime map[uint]time.Time
	client   *http.Client
	ts       tsdb.Engine
	hub      *notify.Hub
	stopAll  chan struct{}
	wg       sync.WaitGroup
}

// NewManager 创建管理器
func NewManager(ts tsdb.Engine, hub *notify.Hub) *Manager {
	return &Manager{
		insts:    map[uint]*Instance{},
		runs:     map[uint]chan struct{}{},
		snap:     map[uint]*Snapshot{},
		prevReq:  map[uint]float64{},
		prevTime: map[uint]time.Time{},
		client:   &http.Client{Timeout: 10 * time.Second},
		ts:       ts,
		hub:      hub,
		stopAll:  make(chan struct{}),
	}
}

// Sync 同步实例列表（变更实例重启循环，幂等）
func (m *Manager) Sync(insts []Instance) {
	m.mu.Lock()
	next := map[uint]*Instance{}
	for i := range insts {
		d := insts[i]
		next[d.ID] = &d
	}
	for id, stop := range m.runs {
		if _, ok := next[id]; !ok {
			close(stop)
			delete(m.runs, id)
		}
	}
	m.insts = next
	m.mu.Unlock()

	m.mu.RLock()
	cur := make([]Instance, 0, len(m.insts))
	for _, d := range m.insts {
		cur = append(cur, *d)
	}
	m.mu.RUnlock()
	for _, d := range cur {
		m.mu.Lock()
		if stop, ok := m.runs[d.ID]; ok {
			close(stop)
		}
		stop := make(chan struct{})
		m.runs[d.ID] = stop
		m.mu.Unlock()
		m.wg.Add(1)
		go m.loop(d, stop)
	}
}

// SnapshotAll 全部快照
func (m *Manager) SnapshotAll() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Snapshot, 0, len(m.snap))
	for _, s := range m.snap {
		out = append(out, *s)
	}
	return out
}

func (m *Manager) loop(d Instance, stop chan struct{}) {
	defer m.wg.Done()
	interval := time.Duration(max(d.Interval, 30)) * time.Second
	tick := time.NewTicker(interval)
	defer tick.Stop()
	m.collect(d)
	for {
		select {
		case <-m.stopAll:
			return
		case <-stop:
			return
		case <-tick.C:
			m.collect(d)
		}
	}
}

func (m *Manager) collect(d Instance) {
	snap := m.probe(d, true)
	m.mu.Lock()
	m.snap[d.ID] = &snap
	m.mu.Unlock()

	now := time.Now()
	up := 0.0
	if snap.Up {
		up = 1
	}
	// 只写 nginx_id 作 tags：与查询端保持一致（TDengine 按 tags 精确哈希建表，带 name 会导致历史查不到）
	tags := map[string]string{"nginx_id": fmt.Sprint(d.ID)}
	rows := []tsdb.Row{
		{Metric: "nginx_avail", Field: "value", Tags: tags, Value: up, TS: now},
		{Metric: "nginx_conn", Field: "value", Tags: tags, Value: snap.ActiveConns, TS: now},
		{Metric: "nginx_accepts", Field: "value", Tags: tags, Value: snap.Accepts, TS: now},
		{Metric: "nginx_handled", Field: "value", Tags: tags, Value: snap.Handled, TS: now},
		{Metric: "nginx_req", Field: "value", Tags: tags, Value: snap.Requests, TS: now},
		{Metric: "nginx_qps", Field: "value", Tags: tags, Value: snap.QPS, TS: now},
		{Metric: "nginx_reading", Field: "value", Tags: tags, Value: snap.Reading, TS: now},
		{Metric: "nginx_writing", Field: "value", Tags: tags, Value: snap.Writing, TS: now},
		{Metric: "nginx_waiting", Field: "value", Tags: tags, Value: snap.Waiting, TS: now},
	}
	if m.ts != nil {
		_ = m.ts.Write(rows)
	}
	m.hub.Publish("nginxmon", "nginx_snapshot", snap)
}

// probe 单实例采集：GET 状态页并解析 stub_status 文本。
// record=true 时更新 QPS 增量状态（实例定时采集）；false 用于即时测试，不污染增量计算。
func (m *Manager) probe(d Instance, record bool) Snapshot {
	snap := Snapshot{InstanceID: d.ID, Name: d.Name, Status: "unknown", TS: time.Now()}
	if strings.TrimSpace(d.StatusURL) == "" {
		snap.Message = "状态页地址为空"
		return snap
	}
	u, err := url.Parse(d.StatusURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		snap.Message = "状态页地址必须为 http/https 协议"
		return snap
	}
	resp, err := m.client.Get(d.StatusURL)
	if err != nil {
		snap.Message = "请求失败: " + err.Error()
		return snap
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snap.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return snap
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		snap.Message = "读取响应失败: " + err.Error()
		return snap
	}
	active, accepts, handled, requests, reading, writing, waiting, err := parseStubStatus(string(body))
	if err != nil {
		snap.Message = "解析状态页失败: " + err.Error()
		return snap
	}
	snap.Up = true
	snap.Status = "up"
	snap.ActiveConns = active
	snap.Accepts = accepts
	snap.Handled = handled
	snap.Requests = requests
	snap.Reading = reading
	snap.Writing = writing
	snap.Waiting = waiting

	if record {
		// QPS：请求数增量 / 时间差
		now := time.Now()
		m.mu.Lock()
		if prev, ok := m.prevReq[d.ID]; ok && !m.prevTime[d.ID].IsZero() {
			dt := now.Sub(m.prevTime[d.ID]).Seconds()
			if dt > 0 && requests >= prev {
				snap.QPS = (requests - prev) / dt
			}
		}
		m.prevReq[d.ID] = requests
		m.prevTime[d.ID] = now
		m.mu.Unlock()
	}
	return snap
}

// Test 即时测试一个状态页地址（不写入时序库，不更新 QPS 增量状态）
func (m *Manager) Test(d Instance) Snapshot {
	return m.probe(d, false)
}

// StopAll 停止
func (m *Manager) StopAll() {
	close(m.stopAll)
	m.wg.Wait()
}

// parseStubStatus 解析 ngx_http_stub_status_module 标准文本：
//
//	Active connections: 291
//	server accepts handled requests
//	 16630948 16630948 31070465
//	Reading: 6 Writing: 179 Waiting: 106
func parseStubStatus(body string) (active, accepts, handled, requests, reading, writing, waiting float64, err error) {
	lines := strings.Split(body, "\n")
	found := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "active connections:"):
			found = true
			if v, e := strconv.ParseFloat(strings.TrimSpace(line[len("Active connections:"):]), 64); e == nil {
				active = v
			}
		case strings.HasPrefix(lower, "reading:"):
			// Reading: r Writing: w Waiting: wa
			found = true
			fields := strings.Fields(line)
			for i := 0; i+1 < len(fields); i++ {
				key := strings.TrimSuffix(strings.ToLower(fields[i]), ":")
				val, e := strconv.ParseFloat(fields[i+1], 64)
				if e != nil {
					continue
				}
				switch key {
				case "reading":
					reading = val
				case "writing":
					writing = val
				case "waiting":
					waiting = val
				}
			}
		case len(strings.Fields(line)) == 3 && looksNumeric(line):
			// accepts handled requests 三列数字
			found = true
			fields := strings.Fields(line)
			accepts, _ = strconv.ParseFloat(fields[0], 64)
			handled, _ = strconv.ParseFloat(fields[1], 64)
			requests, _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	// 以“是否识别到状态页行”为准，避免空闲/全新 Nginx（数值全 0）被误判
	if !found {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("未识别到 stub_status 指标，请确认该地址为 ngx_http_stub_status_module 状态页")
	}
	return active, accepts, handled, requests, reading, writing, waiting, nil
}

// looksNumeric 判断一行是否全部为数字（用于识别 accepts/handled/requests 数据行）
func looksNumeric(s string) bool {
	for _, f := range strings.Fields(s) {
		if _, err := strconv.ParseFloat(f, 64); err != nil {
			return false
		}
	}
	return true
}
