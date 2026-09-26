// Package k8s Kubernetes 监控（调用 kube-apiserver REST API，支持多集群）
package k8s

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"netops/internal/notify"
	"netops/internal/tsdb"
)

type Node struct {
	Name      string  `json:"name"`
	Ready     bool    `json:"ready"`
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	CPUCores  string  `json:"cpu_cores"`
	MemBytes  string  `json:"mem_bytes"`
	Role      string  `json:"role"`
	K8sVer    string  `json:"k8s_version"`
	Cluster   string  `json:"cluster"`
}

type PodSummary struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
}

type Cluster struct {
	Name      string
	APIServer string
	Token     string
}

type Monitor struct {
	mu       sync.RWMutex
	enable   bool
	interval int
	clusters []Cluster
	nodes    map[string]Node
	clusterStatus map[string]bool
	ts       tsdb.Engine
	hub      *notify.Hub
	client   *http.Client
	stop     chan struct{}
	wg       sync.WaitGroup
}

func NewMonitor(ts tsdb.Engine, hub *notify.Hub) *Monitor {
	return &Monitor{
		nodes:  map[string]Node{},
		clusterStatus: map[string]bool{},
		ts:     ts,
		hub:    hub,
		client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}},
		interval: 60,
		stop:   make(chan struct{}),
	}
}

// SetConfigJSON 传入 JSON 数组 [{name,api_server,token}]
func (m *Monitor) SetConfigJSON(enable bool, interval int, clustersJSON string) {
	m.mu.Lock()
	start := enable && !m.enable
	stop := !enable && m.enable
	m.enable = enable
	if interval > 0 { m.interval = interval }
	m.clusters = nil
	if clustersJSON != "" {
		var arr []struct {
			Name      string `json:"name"`
			APIServer string `json:"api_server"`
			Token     string `json:"token"`
		}
		if err := json.Unmarshal([]byte(clustersJSON), &arr); err == nil {
			for _, c := range arr {
				m.clusters = append(m.clusters, Cluster{
					Name: c.Name, APIServer: strings.TrimRight(c.APIServer, "/"), Token: c.Token,
				})
			}
		}
	}
	m.mu.Unlock()
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

func (m *Monitor) get(c Cluster, path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.APIServer+path, nil)
	if err != nil { return err }
	if c.Token != "" { req.Header.Set("Authorization", "Bearer "+c.Token) }
	resp, err := m.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (m *Monitor) Collect() {
	m.mu.RLock()
	clusters := m.clusters
	m.mu.RUnlock()

	allNodes := []Node{}
	for _, cl := range clusters {
		m.collectCluster(cl, &allNodes)
	}

	now := time.Now()
	rows := []tsdb.Row{}
	m.mu.Lock()
	for k := range m.nodes { delete(m.nodes, k) }
	for _, n := range allNodes {
		m.nodes[n.Cluster+"/"+n.Name] = n
		rows = append(rows,
			tsdb.Row{Metric: "k8s_cpu", Field: "value", Tags: map[string]string{"node": n.Name, "cluster": n.Cluster}, Value: n.CPU, TS: now},
			tsdb.Row{Metric: "k8s_mem", Field: "value", Tags: map[string]string{"node": n.Name, "cluster": n.Cluster}, Value: n.Mem, TS: now},
		)
	}
	m.mu.Unlock()
	if m.ts != nil { _ = m.ts.Write(rows) }
	m.hub.Publish("containermon", "k8s_snapshot", map[string]any{"nodes": m.Snapshot()})
}

func (m *Monitor) collectCluster(cl Cluster, allNodes *[]Node) {
	type nodeList struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				Capacity map[string]string `json:"capacity"`
				NodeInfo struct {
					KubeletVersion string `json:"kubeletVersion"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}
	var nodes nodeList
	if err := m.get(cl, "/api/v1/nodes", &nodes); err != nil {
		m.mu.Lock(); m.clusterStatus[cl.Name] = false; m.mu.Unlock()
		m.hub.Publish("containermon", "k8s_error", map[string]any{"error": err.Error(), "cluster": cl.Name})
		return
	}
	m.mu.Lock(); m.clusterStatus[cl.Name] = true; m.mu.Unlock()

	type metricList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage map[string]string `json:"usage"`
		} `json:"items"`
	}
	var metrics metricList
	_ = m.get(cl, "/apis/metrics.k8s.io/v1beta1/nodes", &metrics)

	usage := map[string]map[string]string{}
	for _, it := range metrics.Items {
		usage[it.Metadata.Name] = it.Usage
	}

	for _, n := range nodes.Items {
		capCPU, _ := parseQuantity(n.Status.Capacity["cpu"])
		capMem, _ := parseQuantity(n.Status.Capacity["memory"])
		node := Node{
			Name: n.Metadata.Name, Cluster: cl.Name,
			CPUCores: n.Status.Capacity["cpu"], MemBytes: n.Status.Capacity["memory"],
			K8sVer: n.Status.NodeInfo.KubeletVersion,
		}
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" { node.Ready = c.Status == "True" }
		}
		if n.Metadata.Labels["node-role.kubernetes.io/control-plane"] != "" ||
			n.Metadata.Labels["node-role.kubernetes.io/master"] != "" {
			node.Role = "master"
		} else {
			node.Role = "worker"
		}
		if u, ok := usage[n.Metadata.Name]; ok {
			uCPU, _ := parseQuantity(u["cpu"])
			uMem, _ := parseQuantity(u["memory"])
			if capCPU > 0 { node.CPU = uCPU / capCPU * 100 }
			if capMem > 0 { node.Mem = uMem / capMem * 100 }
		}
		*allNodes = append(*allNodes, node)
	}
}

// ClusterStatus 返回各集群在线状态
func (m *Monitor) ClusterStatus() map[string]bool {
	m.mu.RLock(); defer m.mu.RUnlock()
	out := map[string]bool{}
	for k, v := range m.clusterStatus { out[k] = v }
	return out
}

func (m *Monitor) Snapshot() []Node {
	m.mu.RLock(); defer m.mu.RUnlock()
	out := make([]Node, 0, len(m.nodes))
	for _, n := range m.nodes { out = append(out, n) }
	return out
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	if m.enable { close(m.stop); m.enable = false }
	m.mu.Unlock()
	m.wg.Wait()
}

func parseQuantity(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" { return 0, nil }
	mult := 1.0
	lower := strings.ToLower(s)
	switch {
	case strings.HasSuffix(lower, "ki"): mult, s = 1024, s[:len(s)-2]
	case strings.HasSuffix(lower, "mi"): mult, s = 1024*1024, s[:len(s)-2]
	case strings.HasSuffix(lower, "gi"): mult, s = 1024*1024*1024, s[:len(s)-2]
	case strings.HasSuffix(lower, "ti"): mult, s = 1024*1024*1024*1024, s[:len(s)-2]
	case strings.HasSuffix(lower, "k"): mult, s = 1000, s[:len(s)-1]
	case strings.HasSuffix(lower, "m"): mult, s = 0.001, s[:len(s)-1]
	case strings.HasSuffix(lower, "g"): mult, s = 1e9, s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil { return 0, err }
	return v * mult, nil
}