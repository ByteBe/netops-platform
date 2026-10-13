// Package bgpmon BGP / VPNv4 邻居只读监控（SSH 采集，不对设备做任何修改）
package bgpmon

import (
	"log"
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
	IsRR       bool   `json:"is_rr"`     // 自动识别：对端为路由反射器
	OutIface   string `json:"out_iface"` // 到邻居的出接口
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[bgpmon] collect skipped: %v", r)
		}
	}()
	if a.DB == nil {
		return
	}
	var devs []model.MonitorDevice
	a.DB.Where("bgp_enable = ?", true).Find(&devs)
	log.Printf("[bgpmon] collect tick, devices_with_bgp=%d", len(devs))
	keep := map[uint]bool{}
	for _, d := range devs {
		keep[d.ID] = true
		ps, err := pollDevice(d)
		if err != nil {
			log.Printf("[bgpmon] device=%s(%s) poll failed: %v", d.Name, d.IP, err)
			continue
		}
		log.Printf("[bgpmon] device=%s(%s) peers=%d", d.Name, d.IP, len(ps))
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
	mu.Lock()
	for id := range snap {
		if !keep[id] {
			delete(snap, id)
		}
	}
	mu.Unlock()
}

func pollDevice(d model.MonitorDevice) ([]Peer, error) {
	return pollDeviceDeep(d, true)
}

func pollDeviceDeep(d model.MonitorDevice, deep bool) ([]Peer, error) {
	log.Printf("[bgpmon] poll device=%s ip=%s user=%s passLen=%d port=%d vendor=%s deep=%v", d.Name, d.IP, d.SshUser, len(d.SshPass), d.SshPort, d.Vendor, deep)
	vendor := strings.ToLower(d.Vendor)
	cmd := "display bgp peer vpnv4"
	if strings.Contains(vendor, "cisco") || strings.Contains(vendor, "思科") {
		cmd = "show bgp vpnv4 all summary"
	} else if strings.Contains(vendor, "huawei") || strings.Contains(d.Vendor, "华为") {
		cmd = "display bgp vpnv4 all peer"
	}
	out, err := sshsvc.Run(sshsvc.Target{
		Host: d.IP, Port: orInt(d.SshPort, 22), User: d.SshUser, Credential: d.SshPass,
	}, cmd)
	if err != nil {
		return nil, err
	}
	log.Printf("[bgpmon] device=%s raw output (%d bytes):\n%s", d.IP, len(out), out)
	ps := parsePeers(out, d)
	if deep {
		for i := range ps {
			ps[i].OutIface = lookupOutIface(d, ps[i].PeerIP)
		}
	}
	return ps, nil
}

func lookupOutIface(d model.MonitorDevice, peer string) string {
	cmd := "display ip routing-table " + peer
	if strings.Contains(strings.ToLower(d.Vendor), "cisco") {
		cmd = "show ip route " + peer
	}
	out, err := sshsvc.Run(sshsvc.Target{
		Host: d.IP, Port: orInt(d.SshPort, 22), User: d.SshUser, Credential: d.SshPass,
	}, cmd)
	if err != nil {
		log.Printf("[bgpmon] route lookup peer=%s failed: %v", peer, err)
		return ""
	}
	log.Printf("[bgpmon] route lookup peer=%s output:\n%s", peer, out)
	lines := strings.Split(out, "\n")
	for _, ln := range lines {
		// H3C/Huawei 路由表：... NextHop Interface 最后一列为接口
		if strings.Contains(ln, peer+"/") || strings.Contains(ln, peer+" ") {
			fields := strings.Fields(ln)
			if len(fields) >= 2 {
				return fields[len(fields)-1]
			}
		}
		if strings.Contains(ln, "Interface") {
			parts := strings.SplitN(ln, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

var (
	// 华为/华三：Peer ASN MsgRcvd MsgSent ... Up/Down State PrefRcv
	reHuawei = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+(\d)\s+(\d+)\s+(\d+)\s+(\d+)\s+\S+\s+(\S+)\s+(\S+)\s+(\d+)`)
	// H3C: Peer AS MsgRcvd MsgSent OutQ PrefRcv Up/Down State
	reH3c = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\S+)\s+(\S+)`)
	// 思科 summary：Neighbor V AS ... Up/Down State/PfxRcd（State/PfxRcd 为数字=Established，否则为状态词）
	reCisco = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+(\d)\s+(\d+)\s+\d+\s+\d+\s+\d+\s+\d+\s+\S+\s+(\S+)\s+(\S+)`)
)

func parsePeers(out string, d model.MonitorDevice) []Peer {
	isCisco := strings.Contains(strings.ToLower(d.Vendor), "cisco") || strings.Contains(d.Vendor, "思科")
	isH3c := strings.Contains(strings.ToLower(d.Vendor), "h3c") || strings.Contains(d.Vendor, "华三")
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
		} else if isH3c {
			m := reH3c.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			rcvd, _ := strconv.ParseInt(m[6], 10, 64)
			sent, _ := strconv.ParseInt(m[4], 10, 64)
			list = append(list, tmpP{m[1], m[2], "v4", m[8], m[7], rcvd, sent})
			asnCount[m[2]]++
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

	g.GET("/peers", func(c *gin.Context) {
		mu.RLock()
		out := []Peer{}
		for _, ps := range snap {
			out = append(out, ps...)
		}
		mu.RUnlock()
		log.Printf("[bgpmon] GET /peers -> %d peers, snap keys=%v", len(out), func() []uint {
			ks := []uint{}
			for k := range snap {
				ks = append(ks, k)
			}
			return ks
		}())
		response.OK(c, out)
	})

	g.GET("/history", func(c *gin.Context) {
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

	// 测试 BGP SSH 采集
	g.POST("/test", func(c *gin.Context) {
		var req struct {
			DeviceID uint   `json:"device_id"`
			IP       string `json:"ip"`
			User     string `json:"ssh_user"`
			Pass     string `json:"ssh_pass"`
			Port     int    `json:"ssh_port"`
			Vendor   string `json:"vendor"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		d := model.MonitorDevice{}
		if req.DeviceID > 0 {
			if err := a.DB.First(&d, req.DeviceID).Error; err != nil {
				response.OK(c, gin.H{"ok": false, "error": "设备不存在"})
				return
			}
		} else {
			d.IP = req.IP
			d.SshUser = req.User
			d.SshPass = req.Pass
			d.SshPort = req.Port
			d.Vendor = req.Vendor
		}
		if d.SshPort == 0 {
			d.SshPort = 22
		}
		ps, err := pollDeviceDeep(d, false)
		if err != nil {
			response.OK(c, gin.H{"ok": false, "error": err.Error()})
			return
		}
		response.OK(c, gin.H{"ok": true, "peers": ps, "count": len(ps)})
	})
}

func init() {
	modreg.RegisterProtected("bgpmon", RegisterProtected)
}
