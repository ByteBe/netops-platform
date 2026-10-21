// Package topology 网络拓扑总览（华为 eSight 风格数据源）
// 节点 = 纳管监控设备 + 本机核心 + 检测目标外部节点；链路 = 链路检测任务
// 实时指标：设备 CPU/内存取 TSDB 最近点，链路时延/丢包取 PingMgr 实时缓存
package topology

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

func init() {
	modreg.RegisterProtected("topology", RegisterProtected)
}

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/overview", func(c *gin.Context) {
		var devs []model.MonitorDevice
		a.DB.Order("id asc").Find(&devs)
		var tasks []model.LinkTask
		a.DB.Order("id asc").Find(&tasks)

		devIP := map[string]uint{}
		nodes := []gin.H{}
		hostID := ""
		for _, d := range devs {
			devIP[d.IP] = d.ID
			n := gin.H{
				"id": fmt.Sprintf("dev_%d", d.ID), "name": d.Name, "ip": d.IP,
				"type": d.Type, "vendor": d.Vendor, "status": d.Status,
				"cpu": latestVal(a, "dev_cpu", map[string]string{"device_id": fmt.Sprint(d.ID)}),
				"mem": latestVal(a, "dev_mem", map[string]string{"device_id": fmt.Sprint(d.ID)}),
				"node_uuid": d.NodeUUID,
			}
			if d.NodeUUID == "" && hostID == "" {
				hostID = fmt.Sprintf("dev_%d", d.ID)
			}
			nodes = append(nodes, n)
		}
		if hostID == "" {
			hostID = "host"
			nodes = append(nodes, gin.H{"id": "host", "name": "本机", "ip": "", "type": "server", "vendor": "", "status": "up", "cpu": 0, "mem": 0, "node_uuid": ""})
		}

		ext := map[string]string{} // 外部目标节点 id -> target
		links := []gin.H{}
		for _, t := range tasks {
			if !t.Enabled {
				continue
			}
			rt, loss, up := 0.0, 0.0, false
			if s, ok := a.PingMgr.Get(t.ID); ok {
				rt, loss, up = s.RT, s.Loss, s.Up
			}
			status := "down"
			if up {
				status = "up"
			} else if rt == 0 && loss == 0 {
				status = "unknown"
			}
			dst := ""
			if did, ok := devIP[t.Target]; ok {
				dst = fmt.Sprintf("dev_%d", did)
			} else if t.Target != "" {
				nid := "ext_" + tsdb.Sanitize(t.Target)
				ext[nid] = t.Target
				dst = nid
			}
			if dst == "" {
				continue // 无目标的链路不渲染
			}
			links = append(links, gin.H{
				"id": fmt.Sprintf("link_%d", t.ID), "name": t.Name,
				"source": hostID, "target": dst,
				"method": t.Method, "port": t.Port,
				"status": status, "rt": rt, "loss": loss,
			})
		}
		for nid, target := range ext {
			nodes = append(nodes, gin.H{"id": nid, "name": target, "ip": target, "type": "target", "vendor": "", "status": "unknown", "cpu": 0, "mem": 0, "node_uuid": ""})
		}

		response.OK(c, gin.H{"nodes": nodes, "links": links})
	})
}

// latestVal 取 TSDB 最近 10 分钟窗口内的最后一条值
func latestVal(a *core.App, metric string, tags map[string]string) float64 {
	s, err := a.TSDB.Query(tsdb.Query{Metric: metric, Tags: tags, Start: time.Now().Add(-10 * time.Minute), End: time.Now(), Agg: "last"})
	if err != nil || len(s) == 0 || len(s[0].Points) == 0 {
		return 0
	}
	return s[0].Points[len(s[0].Points)-1][1]
}
