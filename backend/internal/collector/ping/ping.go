// Package ping 链路探测（ICMP 原生 / TCP 拨测 / 系统 ping 命令）
// 并行探测：每个检测任务独立 goroutine + 时间片，探测结果写入时序库并实时推送
package ping

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"netops/internal/notify"
	"netops/internal/tsdb"
)

// Result 单次探测结果
type Result struct {
	TaskID   uint    `json:"task_id"`
	Name     string  `json:"name"`
	Target   string  `json:"target"`
	Method   string  `json:"method"`
	Up       bool    `json:"up"`
	RT       float64 `json:"rt_ms"`   // 平均往返时间 ms，失败为0
	Loss     float64 `json:"loss_pct"` // 丢包率 %
	Message  string  `json:"message"`
	TS       int64   `json:"ts"`
	Color    string  `json:"color"`
}

// Probe 探测执行器
type Probe interface {
	Run(target string, port int, timeout time.Duration) (rtMs float64, lossPct float64, err error)
	Name() string
}

// CMDProbe 系统 ping 命令探测（跨平台，调用系统自带工具）
type CMDProbe struct{}

func (CMDProbe) Name() string { return "cmd" }

var (
	reLoss = regexp.MustCompile(`(\d+)\s*%`)
	reTime = regexp.MustCompile(`[=<]([\d.]+)\s*ms`)
	reAvg  = regexp.MustCompile(`[=:]\s*([\d.]+)\s*ms`)
)

func (CMDProbe) Run(target string, _ int, timeout time.Duration) (float64, float64, error) {
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", fmt.Sprintf("%d", int(timeout.Milliseconds())), target}
	} else {
		args = []string{"-c", "1", "-W", "3", target}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ping", args...)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil && !strings.Contains(text, "TTL=") && !strings.Contains(text, "TTL =") {
		loss := 100.0
		if m := reLoss.FindStringSubmatch(text); len(m) > 1 {
			loss, _ = strconv.ParseFloat(m[1], 64)
		}
		return 0, loss, fmt.Errorf("目标不可达: %s", strings.TrimSpace(strings.SplitN(text, "\n", 1)[0]))
	}
	loss := 0.0
	if m := reLoss.FindStringSubmatch(text); len(m) > 1 {
		loss, _ = strconv.ParseFloat(m[1], 64)
	}
	// 收集所有 per-packet 时间值
	var rts []float64
	for _, line := range strings.Split(text, "\n") {
		if m := reTime.FindStringSubmatch(line); len(m) > 1 && m[1] != "" {
			if v, e := strconv.ParseFloat(m[1], 64); e == nil && v > 0 {
				rts = append(rts, v)
			}
		}
	}
	rt := 0.0
	if len(rts) > 0 {
		sum := 0.0
		for _, v := range rts { sum += v }
		rt = sum / float64(len(rts))
	} else {
		// 兜底：从汇总行提取平均
		if m := reAvg.FindStringSubmatch(text); len(m) > 1 {
			rt, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	return rt, loss, nil
}

// TCPProbe TCP 拨测
type TCPProbe struct{}

func (TCPProbe) Name() string { return "tcp" }

func (TCPProbe) Run(target string, port int, timeout time.Duration) (float64, float64, error) {
	if port <= 0 {
		return 0, 100, errors.New("TCP探测需要指定端口")
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, strconv.Itoa(port)), timeout)
	if err != nil {
		return 0, 100, err
	}
	conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, 0, nil
}

// ICMPProbe 原生 ICMP 探测（Linux root / Windows 管理员）
type ICMPProbe struct{}

func (ICMPProbe) Name() string { return "icmp" }

func (ICMPProbe) Run(target string, _ int, timeout time.Duration) (float64, float64, error) {
	addr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return 0, 100, err
	}
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, 100, fmt.Errorf("ICMP需要管理员/root权限: %w", err)
	}
	defer conn.Close()

	start := time.Now()
	msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: int(start.UnixNano() & 0xffff), Seq: 1, Data: []byte("netops")}}
	data, err := msg.Marshal(nil)
	if err != nil {
		return 0, 100, err
	}
	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, 100, err
	}
	if _, err := conn.WriteTo(data, addr); err != nil {
		return 0, 100, err
	}
	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return 0, 100, err
		}
		reply, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}
		if reply.Type == ipv4.ICMPTypeEchoReply {
			rt := time.Since(start).Seconds() * 1000
			return rt, 0, nil
		}
		if reply.Type == ipv4.ICMPTypeDestinationUnreachable {
			return 0, 100, errors.New("目标不可达")
		}
	}
}

// NewProbe 创建探测执行器
func NewProbe(method string) Probe {
	switch method {
	case "tcp":
		return TCPProbe{}
	case "icmp":
		return ICMPProbe{}
	default:
		return CMDProbe{}
	}
}

// Task 运行中的任务
type Task struct {
	ID        uint
	Name      string
	Target    string
	Method    string
	Port      int
	Interval  int
	Timeout   int
	Color     string
	Status    string // up/down/unknown
	LastRT    float64
	LastLoss  float64
	LastTime  time.Time
	mu        sync.RWMutex
	stop      chan struct{}
}

// Manager 链路检测管理器（多任务并行、常驻监控）
type Manager struct {
	mu     sync.RWMutex
	tasks  map[uint]*Task
	ts     tsdb.Engine
	hub    *notify.Hub
	stopAll chan struct{}
	wg     sync.WaitGroup
}

// NewManager 创建管理器
func NewManager(ts tsdb.Engine, hub *notify.Hub) *Manager {
	return &Manager{tasks: map[uint]*Task{}, ts: ts, hub: hub, stopAll: make(chan struct{})}
}

// StartTask 启动单个任务（幂等：已存在则重启）
func (m *Manager) StartTask(id uint, name, target, method string, port, interval, timeout int, color string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		close(t.stop)
	}
	t := &Task{
		ID: id, Name: name, Target: target, Method: method, Port: port,
		Interval: interval, Timeout: timeout, Color: color, Status: "unknown",
		stop: make(chan struct{}),
	}
	m.tasks[id] = t
	m.wg.Add(1)
	go m.runLoop(t)
}

// StopTask 停止任务
func (m *Manager) StopTask(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		close(t.stop)
		delete(m.tasks, id)
	}
}

// Snapshot 全量状态快照
func (m *Manager) Snapshot() []Result {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Result, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t.snapshot())
	}
	return out
}

// Get 单个状态
func (m *Manager) Get(id uint) (Result, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	if !ok {
		return Result{}, false
	}
	return t.snapshot(), true
}

func (t *Task) snapshot() Result {
	t.mu.RLock()
	defer t.mu.RUnlock()
	up := t.Status == "up"
	return Result{
		TaskID: t.ID, Name: t.Name, Target: t.Target, Method: t.Method,
		Up: up, RT: t.LastRT, Loss: t.LastLoss, Message: t.statusMsg(), TS: t.LastTime.UnixMilli(), Color: t.Color,
	}
}

func (t *Task) statusMsg() string {
	if t.Status == "up" {
		return "正常"
	}
	if t.Status == "down" {
		return "中断"
	}
	return "未知"
}

func (m *Manager) runLoop(t *Task) {
	defer m.wg.Done()
	interval := time.Duration(max(t.Interval, 3)) * time.Second
	timeout := time.Duration(max(t.Timeout, 500)) * time.Millisecond
	probe := NewProbe(t.Method)

	tick := time.NewTicker(interval)
	defer tick.Stop()
	// 启动即探测一次
	m.doProbe(t, probe, timeout)
	for {
		select {
		case <-t.stop:
			return
		case <-m.stopAll:
			return
		case <-tick.C:
			m.doProbe(t, probe, timeout)
		}
	}
}

func (m *Manager) doProbe(t *Task, probe Probe, timeout time.Duration) {
	rt, loss, err := probe.Run(t.Target, t.Port, timeout)
	now := time.Now()
	newStatus := "up"
	msg := "正常"
	if err != nil {
		newStatus = "down"
		msg = err.Error()
		rt = 0
	}
	t.mu.Lock()
	changed := t.Status != newStatus
	t.Status = newStatus
	t.LastRT = rt
	t.LastLoss = loss
	t.LastTime = now
	t.mu.Unlock()

	// 写入时序库
	rows := []tsdb.Row{
		{Metric: "link_rt", Field: "value", Tags: map[string]string{"task_id": fmt.Sprint(t.ID), "name": t.Name}, Value: rt, TS: now},
		{Metric: "link_loss", Field: "value", Tags: map[string]string{"task_id": fmt.Sprint(t.ID), "name": t.Name}, Value: loss, TS: now},
		{Metric: "link_status", Field: "value", Tags: map[string]string{"task_id": fmt.Sprint(t.ID), "name": t.Name}, Value: boolToF(newStatus == "up"), TS: now},
	}
	if m.ts != nil {
		_ = m.ts.Write(rows)
	}

	// 实时推送（含状态变化事件）
	res := t.snapshot()
	res.Message = msg
	m.hub.Publish("linkdetect", "result", res)
	if changed {
		m.hub.Publish("linkdetect", "status_change", res)
	}
}

func boolToF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// StopAll 停止全部任务
func (m *Manager) StopAll() {
	close(m.stopAll)
	m.wg.Wait()
}

// Reset 全量重置（配置变更后调用）
func (m *Manager) Reset(tasks []TaskConfig) {
	m.mu.Lock()
	for _, t := range m.tasks {
		close(t.stop)
	}
	m.tasks = map[uint]*Task{}
	m.mu.Unlock()
	for _, tc := range tasks {
		if tc.Enabled {
			m.StartTask(tc.ID, tc.Name, tc.Target, tc.Method, tc.Port, tc.Interval, tc.Timeout, tc.Color)
		}
	}
}

// TaskConfig 任务配置（供模块层同步）
type TaskConfig struct {
	ID       uint
	Name     string
	Target   string
	Method   string
	Port     int
	Interval int
	Timeout  int
	Color    string
	Enabled  bool
}

var _ = math.Max
