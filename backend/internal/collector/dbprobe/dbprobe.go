// Package dbprobe 数据库监控探针
// 覆盖四类监控：可用性/状态、容量与存储、事务与日志、错误与异常
// 支持：mysql / oracle / dm(达梦) / kingbase(人大金仓) / sqlite(内置) / tdengine
package dbprobe

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "gitee.com/chunanyong/dm"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/sijms/go-ora/v2"

	"netops/internal/notify"
	"netops/internal/tsdb"
)

var cpuMu sync.Mutex
var prevCPUTime = map[uint]int64{}
var prevCPUIdle = map[uint]int64{}

func hostStats(id uint, host string) (cpuPct float64, memUsed, memTotal float64, diskUsed float64, uptime float64) {
	if host != "127.0.0.1" && host != "localhost" && host != "0.0.0.0" {
		return 0, 0, 0, 0, 0
	}
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		fields := strings.Fields(strings.SplitN(string(data), "\n", 2)[0])
		var total, idle int64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseInt(fields[i], 10, 64)
			total += v
			if i == 4 {
				idle = v
			}
		}
		cpuMu.Lock()
		if prev, ok := prevCPUTime[id]; ok && total > prev {
			dTotal := float64(total - prev)
			dIdle := float64(idle - prevCPUIdle[id])
			if dTotal > 0 {
				cpuPct = (1 - dIdle/dTotal) * 100
			}
		}
		prevCPUTime[id] = total
		prevCPUIdle[id] = idle
		cpuMu.Unlock()
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				v := strings.Fields(line)
				if n, err := strconv.ParseFloat(v[1], 64); err == nil {
					memTotal = n * 1024
				}
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				v := strings.Fields(line)
				if n, err := strconv.ParseFloat(v[1], 64); err == nil && memTotal > 0 {
					memUsed = memTotal - n*1024
				}
			}
		}
	}
	if out, err := exec.Command("df", "-B1", "/").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 3 {
				if n, err := strconv.ParseFloat(fields[2], 64); err == nil {
					diskUsed = n
				}
			}
		}
	}
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if n, err := strconv.ParseFloat(fields[0], 64); err == nil {
			uptime = n
		}
	}
	return
}

// Instance 监控实例配置
type Instance struct {
	ID       uint
	Name     string
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Interval int
}

// Snapshot 实例采集快照
type Snapshot struct {
	InstanceID uint      `json:"instance_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Up         bool      `json:"up"`
	Version    string    `json:"version"`
	Status     string    `json:"status"`
	Conns      float64   `json:"conns"`
	MaxConns   float64   `json:"max_conns"`
	DataSize   float64   `json:"data_size"`
	TableCount float64   `json:"table_count"`
	TxPerSec   float64   `json:"tx_per_sec"`
	Deadlocks  float64   `json:"deadlocks"`
	Errors     float64   `json:"errors"`
	Uptime     float64   `json:"uptime_s"`
	LogFile    string    `json:"log_file"`
	Message    string    `json:"message"`
	CpuPercent float64   `json:"cpu_percent"`
	MemUsed    float64   `json:"mem_used"`
	MemTotal   float64   `json:"mem_total"`
	TS         time.Time `json:"ts"`
}

// Manager 数据库监控管理器
type Manager struct {
	mu       sync.RWMutex
	insts    map[uint]*Instance
	runs     map[uint]chan struct{}
	snap     map[uint]*Snapshot
	pools    map[uint]*sql.DB
	prevTx   map[uint]float64
	prevErr  map[uint]float64
	prevTime map[uint]time.Time
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
		pools:    map[uint]*sql.DB{},
		prevTx:   map[uint]float64{},
		prevErr:  map[uint]float64{},
		prevTime: map[uint]time.Time{},
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
			if p, ok := m.pools[id]; ok {
				p.Close()
				delete(m.pools, id)
			}
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
	m.mu.Lock()
	var prev Snapshot
	if p := m.snap[d.ID]; p != nil {
		prev = *p
	}
	m.mu.Unlock()
	snap := m.probe(d, prev)
	if snap.Up {
		keep := func(newVal, oldVal float64) float64 {
			if newVal <= 0 {
				return oldVal
			}
			return newVal
		}
		if snap.Version == "" {
			snap.Version = prev.Version
		}
		snap.DataSize = keep(snap.DataSize, prev.DataSize)
		snap.TableCount = keep(snap.TableCount, prev.TableCount)
		snap.Conns = keep(snap.Conns, prev.Conns)
		snap.Uptime = keep(snap.Uptime, prev.Uptime)
		snap.CpuPercent = keep(snap.CpuPercent, prev.CpuPercent)
		snap.MemUsed = keep(snap.MemUsed, prev.MemUsed)
		snap.MemTotal = keep(snap.MemTotal, prev.MemTotal)
	}
	m.mu.Lock()
	m.snap[d.ID] = &snap
	m.mu.Unlock()

	now := time.Now()
	up := 0.0
	if snap.Up {
		up = 1
	}
	tags := map[string]string{"db_id": fmt.Sprint(d.ID), "name": d.Name}
	rows := []tsdb.Row{
		{Metric: "db_avail", Field: "value", Tags: tags, Value: up, TS: now},
		{Metric: "db_conn", Field: "value", Tags: tags, Value: snap.Conns, TS: now},
		{Metric: "db_size", Field: "value", Tags: tags, Value: snap.DataSize, TS: now},
		{Metric: "db_tx", Field: "value", Tags: tags, Value: snap.TxPerSec, TS: now},
		{Metric: "db_err", Field: "value", Tags: tags, Value: snap.Errors, TS: now},
	}
	if snap.CpuPercent > 0 {
		rows = append(rows, tsdb.Row{Metric: "db_cpu", Field: "value", Tags: tags, Value: snap.CpuPercent, TS: now})
	}
	if snap.MemUsed > 0 {
		rows = append(rows, tsdb.Row{Metric: "db_mem", Field: "used", Tags: tags, Value: snap.MemUsed, TS: now})
		rows = append(rows, tsdb.Row{Metric: "db_mem", Field: "total", Tags: tags, Value: snap.MemTotal, TS: now})
	}
	if m.ts != nil {
		_ = m.ts.Write(rows)
	}
	m.hub.Publish("dbmonitor", "db_snapshot", snap)
}

// probe 单实例采集入口
func (m *Manager) probe(d Instance, prev Snapshot) Snapshot {
	snap := Snapshot{InstanceID: d.ID, Name: d.Name, Type: d.Type, TS: time.Now()}
	if strings.ToLower(d.Type) == "tdengine" {
		return m.probeTDengine(d, snap)
	}
	dt := lookupDialect(d.Type)
	dsn := dt.buildDSN(d)
	m.mu.RLock()
	db := m.pools[d.ID]
	m.mu.RUnlock()
	if db == nil {
		var err error
		db, err = sql.Open(dt.driver, dsn)
		if err != nil {
			snap.Message = err.Error()
			return snap
		}
		db.SetMaxIdleConns(2)
		db.SetMaxOpenConns(3)
		db.SetConnMaxLifetime(5 * time.Minute)
		m.mu.Lock()
		m.pools[d.ID] = db
		m.mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		snap.Message = "连接失败: " + err.Error()
		return snap
	}
	snap.Up = true
	snap.Status = "up"

	snap.Version = one(db, ctx, dt.version)
	if s := one(db, ctx, dt.status); s != "" {
		snap.Status = s
	}
	snap.Conns = num(db, ctx, dt.conn)
	snap.MaxConns = num(db, ctx, dt.maxConn)
	snap.DataSize = num(db, ctx, dt.size)
	snap.TableCount = num(db, ctx, dt.tables)
	snap.Deadlocks = num(db, ctx, dt.deadlock)
	snap.Uptime = num(db, ctx, dt.uptime)
	snap.LogFile = one(db, ctx, dt.log)
	tx := num(db, ctx, dt.tx)
	errCnt := num(db, ctx, dt.err)

	now := time.Now()
	m.mu.Lock()
	if prevT, ok := m.prevTx[d.ID]; ok && !m.prevTime[d.ID].IsZero() {
		dtSec := now.Sub(m.prevTime[d.ID]).Seconds()
		if dtSec > 0 && tx >= prevT {
			snap.TxPerSec = (tx - prevT) / dtSec
		}
	}
	prevE, hasErr := m.prevErr[d.ID]
	m.prevTx[d.ID] = tx
	m.prevErr[d.ID] = errCnt
	m.prevTime[d.ID] = now
	m.mu.Unlock()
	if hasErr && errCnt >= prevE {
		snap.Errors = errCnt - prevE
	}
	snap.CpuPercent, snap.MemUsed, snap.MemTotal, _, _ = hostStats(d.ID, d.Host)
	return snap
}

// TestInstance 即时测试一个连接配置
func (m *Manager) TestInstance(d Instance) Snapshot {
	snap := m.probe(d, Snapshot{})
	if d.ID > 0 {
		m.mu.Lock()
		m.snap[d.ID] = &snap
		m.mu.Unlock()
		m.hub.Publish("dbmonitor", "db_snapshot", snap)
	}
	return snap
}

// StopAll 停止
func (m *Manager) StopAll() {
	close(m.stopAll)
	m.wg.Wait()
}

// ---- 查询辅助（兼容 SHOW 双列与单值）----

func one(db *sql.DB, ctx context.Context, query string) string {
	if query == "" {
		return ""
	}
	var out string
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SHOW") {
		var a, b string
		if err := db.QueryRowContext(ctx, query).Scan(&a, &b); err == nil {
			return b
		}
		if err := db.QueryRowContext(ctx, query).Scan(&out); err == nil {
			return out
		}
		return ""
	}
	if err := db.QueryRowContext(ctx, query).Scan(&out); err != nil {
		return ""
	}
	return out
}

func num(db *sql.DB, ctx context.Context, query string) float64 {
	if query == "" {
		return -1
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SHOW") {
		var a, b string
		if err := db.QueryRowContext(ctx, query).Scan(&a, &b); err != nil {
			return -1
		}
		v, err := strconv.ParseFloat(b, 64)
		if err != nil {
			return -1
		}
		return v
	}
	var v interface{}
	if err := db.QueryRowContext(ctx, query).Scan(&v); err != nil {
		return -1
	}
	switch n := v.(type) {
	case int64:
		return float64(n)
	case float64:
		return n
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return -1
}
