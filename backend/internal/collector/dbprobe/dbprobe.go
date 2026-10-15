// Package dbprobe 数据库监控探针
// 覆盖四类监控：可用性/状态、容量与存储、事务与日志、错误与异常
// 支持：mysql / oracle / dm(达梦) / kingbase(人大金仓) / sqlite(内置)
package dbprobe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
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
var tdHTTPClient = &http.Client{Timeout: 10 * time.Second}

func hostStats(id uint, host string) (cpuPct float64, memUsed, memTotal float64, diskUsed float64, uptime float64) {
	if host != "127.0.0.1" && host != "localhost" && host != "0.0.0.0" {
		return 0, 0, 0, 0, 0
	}
	// CPU
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
	// Mem
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
	// Disk
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
	// Uptime
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
	DataSize   float64   `json:"data_size"` // 字节
	TableCount float64   `json:"table_count"`
	TxPerSec   float64   `json:"tx_per_sec"`
	Deadlocks  float64   `json:"deadlocks"`
	Errors     float64   `json:"errors"`
	Uptime     float64   `json:"uptime_s"`
	LogFile    string    `json:"log_file"`
	Message    string    `json:"message"`
	CpuPercent float64   `json:"cpu_percent"`
	MemUsed    float64   `json:"mem_used"` // 字节
	MemTotal   float64   `json:"mem_total"`
	TS         time.Time `json:"ts"`
}

// Manager 数据库监控管理器
type Manager struct {
	mu       sync.RWMutex
	insts    map[uint]*Instance
	runs     map[uint]chan struct{}
	snap     map[uint]*Snapshot
	pools    map[uint]*sql.DB // 复用连接池
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
	// 字段合并：新值为 0 或 -1（查询失败）时保留旧值，避免刷新丢失数据
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

func (m *Manager) probeTDengine(d Instance, snap Snapshot) Snapshot {
	dbn := d.DBName
	if dbn == "" {
		dbn = "log"
	}
	client := tdHTTPClient
	tdQuery := func(sql string) (any, error) {
		url := fmt.Sprintf("http://%s:%d/rest/sql/%s", d.Host, d.Port, dbn)
		req, _ := http.NewRequest("POST", url, strings.NewReader(sql))
		req.SetBasicAuth(d.User, d.Password)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var body struct {
			Code int             `json:"code"`
			Desc string          `json:"desc"`
			Data [][]interface{} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body.Code != 0 {
			return nil, fmt.Errorf("%s", body.Desc)
		}
		if len(body.Data) > 0 && len(body.Data[0]) > 0 {
			return body.Data[0][0], nil
		}
		return nil, nil
	}
	if v, err := tdQuery("SELECT SERVER_VERSION()"); err == nil && v != nil {
		snap.Version = fmt.Sprint(v)
	}
	snap.Up = true
	snap.Status = "up"
	snap.Conns = 1
	// 存储用量：disk_used（字节）
	if v, err := tdQuery("SELECT COUNT(*) FROM information_schema.ins_tables"); err == nil && v != nil {
		snap.TableCount, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	// TDengine 数据目录大小
	for _, dir := range []string{"/opt/tdengine/data", "/var/lib/taos"} {
		if out, err := exec.Command("du", "-sb", dir).Output(); err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if n, err := strconv.ParseFloat(fields[0], 64); err == nil && n > 0 {
				snap.DataSize = n
				break
			}
		}
	}
	var diskUsed, uptime float64
	snap.CpuPercent, snap.MemUsed, snap.MemTotal, diskUsed, uptime = hostStats(d.ID, d.Host)
	if snap.Uptime == 0 {
		snap.Uptime = uptime
	}
	_ = diskUsed
	return snap
}

// probe 单实例采集
func (m *Manager) probe(d Instance, prev Snapshot) Snapshot {
	snap := Snapshot{InstanceID: d.ID, Name: d.Name, Type: d.Type, TS: time.Now()}
	if strings.ToLower(d.Type) == "tdengine" {
		return m.probeTDengine(d, snap)
	}
	dsn := buildDSN(d)
	if dsn == "" {
		snap.Message = "不支持的数据库类型: " + d.Type
		return snap
	}
	// 复用连接池
	m.mu.RLock()
	db := m.pools[d.ID]
	m.mu.RUnlock()
	if db == nil {
		var err error
		db, err = sql.Open(driverName(d.Type), dsn)
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

	snap.Version = one(db, ctx, versionSQL(d.Type))
	snap.Status = firstNonEmpty(one(db, ctx, statusSQL(d.Type)), snap.Status)
	snap.Conns = num(db, ctx, connSQL(d.Type))
	snap.DataSize = num(db, ctx, sizeSQL(d.Type))
	snap.TableCount = num(db, ctx, tableSQL(d.Type))
	snap.Deadlocks = num(db, ctx, deadlockSQL(d.Type))
	snap.Uptime = num(db, ctx, uptimeSQL(d.Type))
	snap.LogFile = one(db, ctx, logSQL(d.Type))
	tx := num(db, ctx, txSQL(d.Type))
	errCnt := num(db, ctx, errSQL(d.Type))

	now := time.Now()
	m.mu.Lock()
	if prevT, ok := m.prevTx[d.ID]; ok && !m.prevTime[d.ID].IsZero() {
		dt := now.Sub(m.prevTime[d.ID]).Seconds()
		if dt > 0 && tx >= prevT {
			snap.TxPerSec = (tx - prevT) / dt
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

// TestInstance 即时测试一个连接配置（不加入采集循环）
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

// ---- 驱动与 DSN ----

func driverName(t string) string {
	switch strings.ToLower(t) {
	case "oracle":
		return "oracle"
	case "dm", "dameng":
		return "dm"
	case "kingbase", "postgres":
		return "pgx"
	case "sqlite", "builtin":
		return "sqlite"
	case "tdengine":
		return "mysql" // TDengine 兼容 MySQL 协议
	default:
		return "mysql"
	}
}

func buildDSN(d Instance) string {
	switch strings.ToLower(d.Type) {
	case "oracle":
		svc := d.DBName
		if svc == "" {
			svc = "ORCL"
		}
		return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", d.User, d.Password, d.Host, d.Port, svc)
	case "dm", "dameng":
		return fmt.Sprintf("dm://%s:%s@%s:%d?schema=%s", d.User, d.Password, d.Host, d.Port, firstNonEmpty(d.DBName, d.User))
	case "kingbase", "postgres":
		dbn := d.DBName
		if dbn == "" {
			dbn = "postgres"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", d.Host, d.Port, d.User, d.Password, dbn)
	case "sqlite", "builtin":
		return d.Host
	case "tdengine":
		dbn := d.DBName
		if dbn == "" {
			dbn = "data"
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=20s&readTimeout=20s&writeTimeout=20s", d.User, d.Password, d.Host, d.Port, dbn)
	default: // mysql
		dbn := d.DBName
		if dbn == "" {
			dbn = "mysql"
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=20s&readTimeout=20s&writeTimeout=20s", d.User, d.Password, d.Host, d.Port, dbn)
	}
}

// ---- 各类型查询集 ----

func availSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle", "dm", "dameng":
		return "SELECT 1 FROM DUAL"
	default:
		return "SELECT 1"
	}
}

func versionSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle":
		return "SELECT version_full FROM v$instance"
	case "dm", "dameng":
		return "SELECT banner FROM v$version WHERE ROWNUM=1"
	case "kingbase", "postgres":
		return "SELECT version()"
	case "sqlite", "builtin":
		return "SELECT sqlite_version()"
	case "tdengine":
		return "SELECT server_version()"
	default:
		return "SELECT VERSION()"
	}
}

func statusSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle", "dm", "dameng":
		return "SELECT status FROM v$instance"
	case "kingbase", "postgres":
		return "SELECT CASE WHEN pg_is_in_recovery() THEN 'recovery' ELSE 'up' END"
	case "sqlite", "builtin":
		return "SELECT 'up'"
	default:
		return "SELECT 'up'"
	}
}

func connSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle":
		return "SELECT COUNT(*) FROM v$session"
	case "dm", "dameng":
		return "SELECT COUNT(*) FROM v$sessions"
	case "kingbase", "postgres":
		return "SELECT COUNT(*) FROM pg_stat_activity"
	case "tdengine":
		return "SELECT COUNT(*) FROM information_schema.ins_connections"
	default:
		return "SHOW GLOBAL STATUS LIKE 'Threads_connected'"
	}
}

func sizeSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle":
		return "SELECT NVL(SUM(bytes),0) FROM v$datafile"
	case "dm", "dameng":
		return "SELECT NVL(SUM(total_size),0) FROM v$datafile"
	case "kingbase", "postgres":
		return "SELECT pg_database_size(current_database())"
	case "sqlite", "builtin":
		return "SELECT page_count*page_size FROM pragma_page_count(), pragma_page_size()"
	case "tdengine":
		return "SELECT SUM(disk_total) FROM information_schema.ins_dnodes"
	default:
		return "SELECT COALESCE(SUM(data_length+index_length),0) FROM information_schema.tables WHERE table_schema NOT IN ('mysql','sys','information_schema','performance_schema')"
	}
}

func tableSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle", "dm", "dameng":
		return "SELECT COUNT(*) FROM user_tables"
	case "kingbase", "postgres":
		return "SELECT COUNT(*) FROM pg_tables WHERE schemaname='public'"
	case "sqlite", "builtin":
		return "SELECT COUNT(*) FROM sqlite_master WHERE type='table'"
	case "tdengine":
		return "SELECT COUNT(*) FROM information_schema.ins_tables"
	default:
		return "SELECT COUNT(*) FROM information_schema.tables"
	}
}

func txSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle", "dm", "dameng":
		return "SELECT COUNT(*) FROM v$transaction"
	case "kingbase", "postgres":
		return "SELECT xact_commit+xact_rollback FROM pg_stat_database WHERE datname=current_database()"
	case "sqlite", "builtin":
		return "SELECT 0"
	default:
		return "SHOW GLOBAL STATUS LIKE 'Com_commit'"
	}
}

func deadlockSQL(t string) string {
	switch strings.ToLower(t) {
	case "kingbase", "postgres":
		return "SELECT deadlocks FROM pg_stat_database WHERE datname=current_database()"
	case "sqlite", "builtin":
		return "SELECT 0"
	default:
		return "SHOW GLOBAL STATUS LIKE 'Innodb_deadlocks'"
	}
}

func errSQL(t string) string {
	switch strings.ToLower(t) {
	case "kingbase", "postgres":
		return "SELECT COALESCE(SUM(aborted),0) FROM pg_stat_database"
	default:
		return "SELECT 0"
	}
}

func uptimeSQL(t string) string {
	switch strings.ToLower(t) {
	case "oracle":
		return "SELECT (SYSDATE-startup_time)*86400 FROM v$instance"
	case "dm", "dameng":
		return "SELECT (SYSDATE()-startup_time)*86400 FROM v$instance"
	case "kingbase", "postgres":
		return "SELECT EXTRACT(EPOCH FROM now()-pg_postmaster_start_time())"
	case "sqlite", "builtin":
		return "SELECT 0"
	case "tdengine":
		return "SELECT uptime()"
	default:
		return "SHOW GLOBAL STATUS LIKE 'Uptime'"
	}
}

func logSQL(t string) string {
	switch strings.ToLower(t) {
	case "kingbase", "postgres":
		return "SELECT setting FROM pg_settings WHERE name='log_directory'"
	case "sqlite", "builtin", "oracle", "dm", "dameng":
		return ""
	default:
		return "SHOW VARIABLES LIKE 'log_error'"
	}
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

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
