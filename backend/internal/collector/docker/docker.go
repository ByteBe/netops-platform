// Package docker Docker 监控（通过 Docker Engine HTTP API 直连）
package docker

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"netops/internal/common/logger"
	"netops/internal/notify"
	"netops/internal/tsdb"
)

type Container struct {
	ID     string  `json:"id"`
	Names  string  `json:"names"`
	Image  string  `json:"image"`
	State  string  `json:"state"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MemPct float64 `json:"mem_pct"`
	Host   string  `json:"host"`
}

type Monitor struct {
	mu         sync.RWMutex
	enable     bool
	interval   int
	hosts      []string
	containers map[string]Container
	hostStatus map[string]bool
	lastStats  map[string]cpuStats
	ts         tsdb.Engine
	hub        *notify.Hub
	stop       chan struct{}
	wg         sync.WaitGroup
}

func NewMonitor(ts tsdb.Engine, hub *notify.Hub) *Monitor {
	return &Monitor{containers: map[string]Container{}, hostStatus: map[string]bool{}, lastStats: map[string]cpuStats{}, ts: ts, hub: hub, interval: 30, stop: make(chan struct{})}
}

func (m *Monitor) SetConfig(enable bool, interval int, hosts string) {
	m.mu.Lock()
	start := enable && !m.enable
	stop := !enable && m.enable
	m.enable = enable
	if interval > 0 { m.interval = interval }
	m.hosts = nil
	if hosts != "" {
		var arr []string
		if err := json.Unmarshal([]byte(hosts), &arr); err == nil {
			for _, h := range arr {
				h = strings.TrimSpace(h)
				if h != "" { m.hosts = append(m.hosts, h) }
			}
		} else {
			for _, h := range strings.Split(hosts, ",") {
				h = strings.TrimSpace(h)
				if h != "" { m.hosts = append(m.hosts, h) }
			}
		}
	}
	hostsSnap := append([]string{}, m.hosts...)
	m.mu.Unlock()
	logger.Infof("[docker] SetConfig enable=%v interval=%d hosts=%v start=%v stop=%v", enable, interval, hostsSnap, start, stop)
	if start { m.wg.Add(1); go m.loop() }
	if stop { close(m.stop); m.stop = make(chan struct{}) }
}

func (m *Monitor) Enabled() bool {
	m.mu.RLock(); defer m.mu.RUnlock()
	return m.enable
}

func (m *Monitor) loop() {
	defer m.wg.Done()
	tick := time.NewTicker(time.Duration(m.interval) * time.Second)
	defer tick.Stop()
	m.Collect()
	for {
		select {
		case <-m.stop: return
		case <-tick.C: m.Collect()
		}
	}
}

func dockerURL(host, path string) string {
	addr := strings.TrimPrefix(host, "tcp://")
	if addr == "" { addr = "127.0.0.1:2375" }
	return "http://" + addr + path
}

func hostName(host string) string {
	if host == "" { return "本机" }
	return host
}

func (m *Monitor) Collect() {
	m.mu.RLock()
	hosts := m.hosts
	enable := m.enable
	m.mu.RUnlock()
	if !enable {
		logger.Warnf("[docker] Collect called but monitor disabled")
		return
	}
	if len(hosts) == 0 { hosts = []string{""} }
	logger.Infof("[docker] Collect hosts=%v", hosts)

	allContainers := []Container{}
	for _, host := range hosts {
		list, err := m.listContainers(host)
		m.mu.Lock()
		if err != nil {
			m.hostStatus[hostName(host)] = false
			m.mu.Unlock()
			logger.Warnf("[docker] host=%s listContainers error: %v", host, err)
			m.hub.Publish("containermon", "docker_error", map[string]any{"error": err.Error(), "host": host})
			continue
		}
		m.hostStatus[hostName(host)] = true
		m.mu.Unlock()
		logger.Infof("[docker] host=%s online, containers=%d", host, len(list))
		for i := range list {
			list[i].Host = hostName(host)
			allContainers = append(allContainers, list[i])
		}
	}

	now := time.Now()
	rows := []tsdb.Row{}
	m.mu.Lock()
	for k := range m.containers { delete(m.containers, k) }
	for _, c := range allContainers {
		m.containers[c.Host+"/"+c.ID] = c
		rows = append(rows,
			tsdb.Row{Metric: "docker_cpu", Field: "value", Tags: map[string]string{"container": c.Names, "host": c.Host}, Value: c.CPU, TS: now},
			tsdb.Row{Metric: "docker_mem", Field: "value", Tags: map[string]string{"container": c.Names, "host": c.Host}, Value: c.MemPct, TS: now},
		)
	}
	m.mu.Unlock()
	if m.ts != nil { _ = m.ts.Write(rows) }
	m.hub.Publish("containermon", "docker_snapshot", allContainers)
}

// HostStatus 返回各主机在线状态
func (m *Monitor) HostStatus() map[string]bool {
	m.mu.RLock(); defer m.mu.RUnlock()
	out := map[string]bool{}
	for k, v := range m.hostStatus { out[k] = v }
	return out
}

func (m *Monitor) Snapshot() []Container {
	m.mu.RLock(); defer m.mu.RUnlock()
	out := make([]Container, 0, len(m.containers))
	for _, c := range m.containers { out = append(out, c) }
	return out
}

func (m *Monitor) listContainers(host string) ([]Container, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(dockerURL(host, "/containers/json?all=1"))
	if err != nil { return nil, fmt.Errorf("连接失败: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil, fmt.Errorf("HTTP %d", resp.StatusCode) }
	var raw []struct {
		ID     string   `json:"Id"`
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil { return nil, err }
	list := []Container{}
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 { name = strings.TrimPrefix(r.Names[0], "/") }
		cid := r.ID
		short := cid
		if len(short) > 12 { short = short[:12] }
		c := Container{ID: short, Names: name, Image: r.Image, State: r.State, Status: r.Status, Host: hostName(host)}
		if r.State == "running" {
			if cpu, mem, err := m.containerStats(client, host, cid); err == nil {
				c.CPU = cpu
				c.MemPct = mem
			}
		}
		list = append(list, c)
	}
	return list, nil
}

// containerStats 拉取单次（非流式）容器统计
func (m *Monitor) containerStats(client *http.Client, host, cid string) (cpuPct, memPct float64, err error) {
	u := dockerURL(host, "/containers/"+cid+"/stats?stream=false")
	resp, err := client.Get(u)
	if err != nil { return 0, 0, err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return 0, 0, fmt.Errorf("HTTP %d", resp.StatusCode) }
	var st struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCpus     uint   `json:"online_cpus"`
		} `json:"cpu_stats"`
		PrecpuStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil { return 0, 0, err }
	// CPU%: 单次 stats 接口返回的 precpu 与 cpu 是同一瞬间，差值为 0；
	// 这里直接按 (total/system)*online_cpus*100 的近似（Docker 官方推荐公式需要两次采样）。
	// 因为每次 Collect 间隔固定，我们用差值缓存算。
	key := host + "/" + cid
	m.mu.Lock()
	prev, hasPrev := m.lastStats[key]
	m.lastStats[key] = cpuStats{
		cpuTotal:  st.CPUStats.CPUUsage.TotalUsage,
		sysCPU:    st.CPUStats.SystemCPUUsage,
		online:    st.CPUStats.OnlineCpus,
		timestamp: time.Now(),
	}
	m.mu.Unlock()
	if hasPrev {
		dCPU := float64(st.CPUStats.CPUUsage.TotalUsage - prev.cpuTotal)
		dSys := float64(st.CPUStats.SystemCPUUsage - prev.sysCPU)
		n := float64(st.CPUStats.OnlineCpus)
		if n <= 0 { n = float64(prev.online) }
		if n <= 0 { n = 1 }
		if dSys > 0 {
			cpuPct = dCPU / dSys * n * 100.0
		}
	}
	if st.MemoryStats.Limit > 0 {
		memPct = float64(st.MemoryStats.Usage) / float64(st.MemoryStats.Limit) * 100.0
	}
	if cpuPct < 0 { cpuPct = 0 }
	if cpuPct > 100 { cpuPct = math.Round(cpuPct*10) / 10 } else { cpuPct = math.Round(cpuPct*10) / 10 }
	memPct = math.Round(memPct*10) / 10
	return cpuPct, memPct, nil
}

type cpuStats struct {
	cpuTotal uint64
	sysCPU   uint64
	online   uint
	timestamp time.Time
}

func TestHost(host string) (string, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(dockerURL(host, "/version"))
	if err != nil { return "", fmt.Errorf("连接失败: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return "", fmt.Errorf("HTTP %s", resp.Status) }
	var v struct{ Version string `json:"Version"` }
	json.NewDecoder(resp.Body).Decode(&v)
	return v.Version, nil
}

func DockerAvailable() (string, error) {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil { return "", err }
	return strings.TrimSpace(string(out)), nil
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	if m.enable { close(m.stop); m.enable = false }
	m.mu.Unlock()
	m.wg.Wait()
}