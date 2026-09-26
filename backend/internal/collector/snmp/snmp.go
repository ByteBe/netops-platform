// Package snmp SNMP 采集器（服务器/交换机/路由器监控）
// 采集指标：CPU、内存、Uptime、接口流量（供流量监控模块使用）
package snmp

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"

	"netops/internal/notify"
	"netops/internal/tsdb"
)

// OIDs
const (
	oidSysUpTime        = "1.3.6.1.2.1.1.3.0"
	oidIfIndex          = "1.3.6.1.2.1.2.2.1.1"
	oidIfDescr          = "1.3.6.1.2.1.2.2.1.2"
	oidIfSpeed          = "1.3.6.1.2.1.2.2.1.5"
	oidIfOperStatus     = "1.3.6.1.2.1.2.2.1.8"
	oidIfInOctets       = "1.3.6.1.2.1.2.2.1.10"
	oidIfOutOctets      = "1.3.6.1.2.1.2.2.1.16"
	oidHrProcessorLoad  = "1.3.6.1.2.1.25.3.3.1.2"
	oidHrStorageIndex   = "1.3.6.1.2.1.25.2.3.1.1"
	oidHrStorageType    = "1.3.6.1.2.1.25.2.3.1.2"
	oidHrStorageSize    = "1.3.6.1.2.1.25.2.3.1.5"
	oidHrStorageUsed    = "1.3.6.1.2.1.25.2.3.1.6"
	hrStorageRam        = "1.3.6.1.2.1.25.2.1.2"
	oidUcdCpuRawIdle    = "1.3.6.1.4.1.2021.11.11.0"
	oidUcdCpuRawSystem  = "1.3.6.1.4.1.2021.11.10.0"
	oidUcdCpuRawUser    = "1.3.6.1.4.1.2021.11.9.0"
)

// Device 采集设备配置
type Device struct {
	ID          uint
	Name        string
	IP          string
	Type        string
	SNMPVersion string // 1/2c/3
	Community   string
	Username    string
	AuthProto   string
	PrivProto   string
	AuthPass    string
	PrivPass    string
	Port        int
	Interval    int
}

// IfStats 接口统计（瞬时）
type IfStats struct {
	Index    string
	Name     string
	Speed    uint64
	Oper     string
	InBps    float64
	OutBps   float64
	InOctets uint64
	OutOctets uint64
}

// DeviceSnapshot 设备采集快照
type DeviceSnapshot struct {
	DeviceID   uint      `json:"device_id"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Up         bool      `json:"up"`
	CPU        float64   `json:"cpu"`     // %
	MemUsed    float64   `json:"mem_used"` // %
	Uptime     float64   `json:"uptime_s"`
	Interfaces []IfStats `json:"interfaces"`
	TS         time.Time `json:"ts"`
	Message    string    `json:"message"`
}

// Manager SNMP 采集管理器
type Manager struct {
	mu       sync.RWMutex
	devices  map[uint]*Device
	runs     map[uint]chan struct{} // 每设备停止信号
	lastOct  map[string]map[string]uint64 // deviceID|ifIndex -> last octets
	lastTime map[string]time.Time
	snap     map[uint]*DeviceSnapshot
	ts       tsdb.Engine
	hub      *notify.Hub
	stopAll  chan struct{}
	wg       sync.WaitGroup
}

// NewManager 创建 SNMP 采集管理器
func NewManager(ts tsdb.Engine, hub *notify.Hub) *Manager {
	return &Manager{
		devices:  map[uint]*Device{},
		runs:     map[uint]chan struct{}{},
		lastOct:  map[string]map[string]uint64{},
		lastTime: map[string]time.Time{},
		snap:     map[uint]*DeviceSnapshot{},
		ts:       ts,
		hub:      hub,
		stopAll:  make(chan struct{}),
	}
}

// SyncDevices 同步设备列表（启用的进入采集，停用/删除的退出；变更设备重启循环）
func (m *Manager) SyncDevices(devs []Device) {
	m.mu.Lock()
	next := map[uint]*Device{}
	for i := range devs {
		d := devs[i]
		next[d.ID] = &d
	}
	// 停止已移除或变更的设备循环
	for id, stop := range m.runs {
		if _, ok := next[id]; !ok {
			close(stop)
			delete(m.runs, id)
			m.hub.Publish("monitor", "device_removed", map[string]any{"device_id": id})
		}
	}
	m.devices = next
	m.mu.Unlock()

	// 为全部启用设备启动采集循环（幂等）
	m.mu.RLock()
	cur := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		cur = append(cur, *d)
	}
	m.mu.RUnlock()
	for _, d := range cur {
		m.mu.Lock()
		if stop, ok := m.runs[d.ID]; ok {
			close(stop) // 重启
		}
		stop := make(chan struct{})
		m.runs[d.ID] = stop
		m.mu.Unlock()
		m.wg.Add(1)
		go m.collectLoop(d, stop)
	}
}

// Snapshot 获取全部设备快照
func (m *Manager) Snapshot() []DeviceSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]DeviceSnapshot, 0, len(m.snap))
	for _, s := range m.snap {
		out = append(out, *s)
	}
	return out
}

// DeviceSnapshotByID 单设备快照
func (m *Manager) DeviceSnapshotByID(id uint) (DeviceSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.snap[id]
	if !ok {
		return DeviceSnapshot{}, false
	}
	return *s, true
}

func (m *Manager) collectLoop(d Device, stop chan struct{}) {
	defer m.wg.Done()
	interval := time.Duration(max(d.Interval, 10)) * time.Second
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

func (m *Manager) collect(d Device) {
	snap, err := m.collectOnce(d)
	if err != nil {
		fmt.Printf("[snmp] device=%s(%s) v=%s user=%s auth=%s priv=%s FAIL: %v\n", d.Name, d.IP, d.SNMPVersion, d.Username, d.AuthProto, d.PrivProto, err)
		snap = &DeviceSnapshot{DeviceID: d.ID, Name: d.Name, IP: d.IP, Up: false, TS: time.Now(), Message: err.Error()}
	}
	m.mu.Lock()
	m.snap[d.ID] = snap
	m.mu.Unlock()

	// 写入时序库
	now := time.Now()
	up := 0.0
	if snap.Up {
		up = 1
	}
	rows := []tsdb.Row{
		{Metric: "dev_up", Field: "value", Tags: map[string]string{"device_id": fmt.Sprint(d.ID), "name": d.Name}, Value: up, TS: now},
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": fmt.Sprint(d.ID), "name": d.Name}, Value: snap.CPU, TS: now},
		{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": fmt.Sprint(d.ID), "name": d.Name}, Value: snap.MemUsed, TS: now},
	}
	for _, ifs := range snap.Interfaces {
		rows = append(rows,
			tsdb.Row{Metric: "if_in", Field: "value", Tags: map[string]string{"device_id": fmt.Sprint(d.ID), "name": d.Name, "if_index": ifs.Index, "if_name": ifs.Name}, Value: ifs.InBps, TS: now},
			tsdb.Row{Metric: "if_out", Field: "value", Tags: map[string]string{"device_id": fmt.Sprint(d.ID), "name": d.Name, "if_index": ifs.Index, "if_name": ifs.Name}, Value: ifs.OutBps, TS: now},
		)
	}
	if m.ts != nil {
		_ = m.ts.Write(rows)
	}

	// 推送
	m.hub.Publish("monitor", "device_snapshot", snap)
	for _, ifs := range snap.Interfaces {
		m.hub.Publish("traffic", "if_rate", map[string]any{
			"device_id": d.ID, "name": d.Name, "if_index": ifs.Index, "if_name": ifs.Name,
			"in_bps": ifs.InBps, "out_bps": ifs.OutBps, "ts": now.UnixMilli(),
		})
	}
}

func (m *Manager) collectOnce(d Device) (*DeviceSnapshot, error) {
	params := &gosnmp.GoSNMP{
		Target:        d.IP,
		Port:          uint16(d.Port),
		Timeout:       time.Duration(4) * time.Second,
		Retries:       1,
		MaxOids:       40,
		Version:       gosnmp.Version2c,
		Community:     d.Community,
		SecurityModel: gosnmp.UserSecurityModel,
	}
	switch d.SNMPVersion {
	case "3":
		params.Version = gosnmp.Version3
		params.MsgFlags = gosnmp.AuthPriv
		params.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 d.Username,
			AuthenticationProtocol:   authProto(d.AuthProto),
			PrivacyProtocol:          privProto(d.PrivProto),
			AuthenticationPassphrase: d.AuthPass,
			PrivacyPassphrase:        d.PrivPass,
		}
	case "1":
		params.Version = gosnmp.Version1
	default:
		params.Version = gosnmp.Version2c
	}
	if params.Community == "" {
		params.Community = "public"
	}
	if params.Port == 0 {
		params.Port = 161
	}
	if err := params.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP连接失败: %w", err)
	}
	defer params.Conn.Close()

	snap := &DeviceSnapshot{DeviceID: d.ID, Name: d.Name, IP: d.IP, Up: true, TS: time.Now()}

	// Uptime
	if uptime, err := params.Get([]string{oidSysUpTime}); err == nil && len(uptime.Variables) > 0 {
		snap.Uptime = float64(uptime.Variables[0].Value.(uint32)) / 100
	}

	// CPU（HR 表，多核取平均）
	loads := map[string]uint32{}
	if rows, err := params.BulkWalkAll(oidHrProcessorLoad); err == nil {
		for _, v := range rows {
			if v.Type == gosnmp.Integer || v.Type == gosnmp.Gauge32 || v.Type == gosnmp.Counter32 {
				loads[oidLast(v.Name)] = toUint32(v.Value)
			}
		}
	}
	if len(loads) > 0 {
		var sum float64
		for _, l := range loads {
			sum += float64(l)
		}
		snap.CPU = math.Round(sum/float64(len(loads))*10) / 10
	} else if raw, err := params.Get([]string{oidUcdCpuRawIdle, oidUcdCpuRawUser, oidUcdCpuRawSystem}); err == nil && len(raw.Variables) == 3 {
		// UCD-SNMP 差值计算：cpu% = (1 - Δidle / Δtotal) * 100
		now := time.Now()
		key := fmt.Sprintf("%d", d.ID)
		m.mu.Lock()
		prevT, hasPrev := m.lastTime[key]
		var prevIdle, prevUser, prevSystem float64
		if prev, ok := m.lastOct[key]; ok {
			prevIdle = float64(prev["idle"])
			prevUser = float64(prev["user"])
			prevSystem = float64(prev["system"])
		}
		idle := float64(toUint64(raw.Variables[0].Value))
		user := float64(toUint64(raw.Variables[1].Value))
		system := float64(toUint64(raw.Variables[2].Value))
		m.lastOct[key] = map[string]uint64{"idle": uint64(idle), "user": uint64(user), "system": uint64(system)}
		m.lastTime[key] = now
		m.mu.Unlock()
		if hasPrev && now.Sub(prevT).Seconds() > 0 {
			dIdle := idle - prevIdle
			dTotal := (idle + user + system) - (prevIdle + prevUser + prevSystem)
			if dTotal > 0 {
				snap.CPU = math.Round((1-dIdle/dTotal)*1000) / 10
			}
		}
	}

	// 内存（HR Storage：取 hrStorageRam 类型）
	types := map[string]string{}
	sizes := map[string]uint64{}
	useds := map[string]uint64{}
	if rows, err := params.BulkWalkAll(oidHrStorageType); err == nil {
		for _, v := range rows {
			types[oidLast(v.Name)] = oidStr(v.Value)
		}
	}
	if rows, err := params.BulkWalkAll(oidHrStorageSize); err == nil {
		for _, v := range rows {
			sizes[oidLast(v.Name)] = toUint64(v.Value)
		}
	}
	if rows, err := params.BulkWalkAll(oidHrStorageUsed); err == nil {
		for _, v := range rows {
			useds[oidLast(v.Name)] = toUint64(v.Value)
		}
	}
	for idx, t := range types {
		if t == hrStorageRam && sizes[idx] > 0 {
			snap.MemUsed = math.Round(float64(useds[idx])/float64(sizes[idx])*1000) / 10
			break
		}
	}

	// 接口表
	m.collectInterfaces(params, d, snap)
	return snap, nil
}

func (m *Manager) collectInterfaces(params *gosnmp.GoSNMP, d Device, snap *DeviceSnapshot) {
	indexes := map[string]string{}
	names := map[string]string{}
	speeds := map[string]uint64{}
	opers := map[string]string{}
	ins := map[string]uint64{}
	outs := map[string]uint64{}

	if rows, err := params.BulkWalkAll(oidIfIndex); err == nil {
		for _, v := range rows {
			indexes[oidLast(v.Name)] = fmt.Sprint(toUint64(v.Value))
		}
	}
	if rows, err := params.BulkWalkAll(oidIfDescr); err == nil {
		for _, v := range rows {
			names[oidLast(v.Name)] = oidStr(v.Value)
		}
	}
	if rows, err := params.BulkWalkAll(oidIfSpeed); err == nil {
		for _, v := range rows {
			speeds[oidLast(v.Name)] = toUint64(v.Value)
		}
	}
	if rows, err := params.BulkWalkAll(oidIfOperStatus); err == nil {
		for _, v := range rows {
			opers[oidLast(v.Name)] = fmt.Sprint(toUint64(v.Value))
		}
	}
	if rows, err := params.BulkWalkAll(oidIfInOctets); err == nil {
		for _, v := range rows {
			ins[oidLast(v.Name)] = toUint64(v.Value)
		}
	}
	if rows, err := params.BulkWalkAll(oidIfOutOctets); err == nil {
		for _, v := range rows {
			outs[oidLast(v.Name)] = toUint64(v.Value)
		}
	}

	now := time.Now()
	key := fmt.Sprintf("%d", d.ID)
	m.mu.Lock()
	prevIns := m.lastOct[key+"_in"]
	prevOuts := m.lastOct[key+"_out"]
	prevT := m.lastTime[key+"_rate"]
	curIns := map[string]uint64{}
	curOuts := map[string]uint64{}
	m.mu.Unlock()

	idxNames := map[string]string{}
	for oid, idx := range indexes {
		idxNames[idx] = oid
	}
	for oid, idx := range indexes {
		name := names[oid]
		if name == "" {
			name = "if" + idx
		}
		speed := speeds[oid]
		oper := "down"
		if op, ok := opers[oid]; ok && op == "1" {
			oper = "up"
		}
		inO, outO := ins[oid], outs[oid]
		curIns[idx] = inO
		curOuts[idx] = outO

		ifs := IfStats{Index: idx, Name: name, Speed: speed, Oper: oper, InOctets: inO, OutOctets: outO}
		if prevIns != nil && prevOuts != nil && !prevT.IsZero() {
			dt := now.Sub(prevT).Seconds()
			if dt > 0 {
				if inO >= prevIns[idx] {
					ifs.InBps = float64(inO-prevIns[idx]) * 8 / dt
				}
				if outO >= prevOuts[idx] {
					ifs.OutBps = float64(outO-prevOuts[idx]) * 8 / dt
				}
			}
		}
		snap.Interfaces = append(snap.Interfaces, ifs)
		_ = idxNames
	}
	m.mu.Lock()
	m.lastOct[key+"_in"] = curIns
	m.lastOct[key+"_out"] = curOuts
	m.lastTime[key+"_rate"] = now
	m.mu.Unlock()
}

// StopAll 停止
// TestDevice 即时测试一个设备配置（不加入采集循环）
func (m *Manager) TestDevice(d Device) (bool, float64, string) {
	snap, err := m.collectOnce(d)
	if err != nil {
		return false, 0, err.Error()
	}
	return true, snap.Uptime, "SNMP连接成功"
}

func (m *Manager) StopAll() {
	close(m.stopAll)
	m.wg.Wait()
}

func authProto(p string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToLower(p) {
	case "sha", "sha1":
		return gosnmp.SHA
	case "sha224":
		return gosnmp.SHA224
	case "sha256":
		return gosnmp.SHA256
	default:
		return gosnmp.MD5
	}
}

func privProto(p string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToLower(p) {
	case "aes", "aes128":
		return gosnmp.AES
	case "aes192":
		return gosnmp.AES192
	case "aes256":
		return gosnmp.AES256
	default:
		return gosnmp.DES
	}
}

func oidLast(oid string) string {
	i := strings.LastIndexByte(oid, '.')
	if i < 0 {
		return oid
	}
	return oid[i+1:]
}

func toUint32(v any) uint32 {
	switch n := v.(type) {
	case uint32:
		return n
	case int:
		return uint32(n)
	case int32:
		return uint32(n)
	case uint64:
		return uint32(n)
	case float64:
		return uint32(n)
	default:
		return 0
	}
}

func toUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case uint32:
		return uint64(n)
	case int:
		return uint64(n)
	case int32:
		return uint64(n)
	case float64:
		return uint64(n)
	default:
		return 0
	}
}

func oidStr(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case []byte:
		return string(n)
	default:
		return fmt.Sprint(v)
	}
}
