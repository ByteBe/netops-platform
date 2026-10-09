// Package bgpmon BGP / VPNv4 邻居只读监控（SSH 采集，不对设备做任何修改）
package bgpmon

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/sshsvc"
	"netops/internal/tsdb"
)

// Peer BGP/VPNv4 邻居快照
type Peer struct {
	DeviceID   uint   `json:"device_id"`
	DeviceName string `json:"device_name"`
	PeerIP     string `json:"peer_ip"`
	ASN        string `json:"asn"`
	V          string `json:"v"`
	State      string `json:"state"` // Established / Idle / Active / Connect ...
	UpDown     string `json:"up_down"`
	PrefixRcvd int64  `json:"prefix_rcvd"`
	PrefixSent int64  `json:"prefix_sent"`
	IsRR       bool   `json:"is_rr"` // 自动识别：对端为路由反射器
	UpdatedAt  string `json:"updated_at"`
}

var (
	mu      sync.RWMutex
	snap    = map[uint][]Peer{} // device_id -> peers
	started bool
)

func start(a *core.App) {
	if started {
		return
	}
	started = true
	go func() {
		collect(a)
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			collect(a)
		}
	}()
}

func collect(a *core.App) {
	var devs []model.MonitorDevice
	a.DB.Where("bgp_enable = ?", true).Find(&devs)
	for _, d := range devs {
		ps, err := pollDevice(d)
		if err != nil {
			continue
		}
		mu.Lock()
		snap[d.ID] = ps
		mu.Unlock()
		// 写时序库
		now := time.Now()
		rows := make([]tsdb.Row, 0, len(ps)*2)
		for _, p := range ps {
			stateVal := 0.0
			if strings.EqualFold(p.State, "Established") {
				stateVal = 1
			}
			rows = append(rows,
				tsdb.Row{Metric: "bgp_peer_state", Field: "value", Tags: map[string]string{"device_id": strconv.Itoa(int(d.ID)), "peer": p.PeerIP}, Value: stateVal, TS: now},
				tsdb.Row{Metric: "bgp_prefix_rcvd", Field: "value", Tags: map[string]string{"device_id": strconv.Itoa(int(d.ID)), "peer": p.PeerIP}, Value: float64(p.PrefixRcvd), TS: now},
				tsdb.Row{Metric: "bgp_prefix_sent", Field: "value", Tags: map[string]string{"device_id": strconv.Itoa(int(d.ID)), "peer": p.PeerIP}, Value: float64(p.PrefixSent), TS: now},
			)
		}
		_ = a.TSDB.Write(rows)
	}
}

func pollDevice(d model.MonitorDevice) ([]Peer, error) {
	vendor := strings.ToLower(d.Vendor)
	cmd := "display bgp vpnv4 all peer"
	if strings.Contains(vendor, "cisco") || strings.Contains(vendor, "思科") {
		cmd = "show bgp vpnv4 all summary"
	}
	out, err := sshsvc.Exec(sshsvc.Target{
		Host: d.IP, Port: orInt(d.SshPort, 22), User: d.SshUser, Credential: d.SshPass,
	}, []string{cmd})
	if err != nil {
		return nil, err
	}
	ps := parsePeers(out, d)
	return ps, nil
}

var (
	// 华为/华三：Peer ASN MsgRcvd MsgSent ... Up/Down State PrefRcv
	reHuawei = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+(\d)\s+(\d+)\s+(\d+)\s+(\d+)\s+\S+\s+(\S+)\s+(\S+)\s+(\d+)`)
	// 思科 summary：Neighbor V AS ... Up/Down State/PfxRcd（State/PfxRcd 为数字=Established，否则为状态词）
	reCisco = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+(\d)\s+(\d+)\s+\d+\s+\d+\s+\d+\s+\d+\s+\S+\s+(\S+)\s+(\S+)`)
)

func parsePeers(out string, d model.MonitorDevice) []Peer {
	isCisco := strings.Contains(strings.ToLower(d.Vendor), "cisco") || strings.Contains(d.Vendor, "思科")
	lines := strings.Split(out, "\n")
	// 统计 ASN 出现次数，自动识别 RR：同 ASN 下收到 Prefix 远大于普通对端
	asnCount := map[string]int{}
	type tmpP struct {
		PeerIP, ASN, V, State, UpDown string
		Rcvd, Sent                    int64
	}
	var list []tmpP
	for _, ln := range lines {
		if isCisco {
			m := reCisco.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			rcvd, _ := strconv.ParseInt(m[5], 10, 64)
			state := "Established"
			if _, err := strconv.ParseInt(m[5], 10, 64); err != nil {
				state = m[5]
			}
			list = append(list, tmpP{m[1], m[3], m[2], state, m[4], rcvd, 0})
			asnCount[m[3]]++
		} else {
			m := reHuawei.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			rcvd, _ := strconv.ParseInt(m[8], 10, 64)
			sent, _ := strconv.ParseInt(m[5], 10, 64)
			list = append(list, tmpP{m[1], m[3], m[2], m[7], m[6], rcvd, sent})
			asnCount[m[3]]++
		}
	}
	// RR 判定：同 ASN 只有一个邻居（典型 PE→RR）即视为 RR
	now := time.Now().Format("2006-01-02 15:04:05")
	out2 := make([]Peer, 0, len(list))
	for _, p := range list {
		out2 = append(out2, Peer{
			DeviceID: d.ID, DeviceName: d.Name, PeerIP: p.PeerIP, ASN: p.ASN, V: p.V,
			State: p.State, UpDown: p.UpDown, PrefixRcvd: p.Rcvd, PrefixSent: p.Sent,
			IsRR: asnCount[p.ASN] == 1, UpdatedAt: now,
		})
	}
	return out2
}

func orInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	start(a)

	g.GET("/bgpmon/peers", func(c *gin.Context) {
		mu.RLock()
		defer mu.RUnlock()
		out := []Peer{}
		for _, ps := range snap {
			out = append(out, ps...)
		}
		response.OK(c, out)
	})

	g.GET("/bgpmon/history", func(c *gin.Context) {
		deviceID := c.Query("device_id")
		peer := c.Query("peer")
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		tags := map[string]string{"device_id": deviceID, "peer": peer}
		rcvd, _ := a.TSDB.Query(tsdb.Query{Metric: "bgp_prefix_rcvd", Field: "value", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "last"})
		state, _ := a.TSDB.Query(tsdb.Query{Metric: "bgp_peer_state", Field: "value", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "last"})
		points := func(s []tsdb.Series) [][2]float64 {
			if len(s) == 0 {
				return [][2]float64{}
			}
			return s[0].Points
		}
		response.OK(c, gin.H{"prefix_rcvd": points(rcvd), "state": points(state)})
	})
}

func init() {
	modreg.RegisterProtected("bgpmon", RegisterProtected)
}
