// Package dashboard 数据大屏（聚合接口：KPI / 链路趋势 / 设备健康 / 流量 / 数据库）
package dashboard

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 大屏总览（KPI + 状态聚合）
	g.GET("/overview", func(c *gin.Context) {
		var linkTotal, linkUp int64
		a.DB.Model(&model.LinkTask{}).Count(&linkTotal)
		for _, s := range a.PingMgr.Snapshot() {
			if s.Up {
				linkUp++
			}
		}
		var devTotal, devOnline int64
		a.DB.Model(&model.MonitorDevice{}).Count(&devTotal)
		for _, s := range a.SnmpMgr.Snapshot() {
			if s.Up {
				devOnline++
			}
		}
		var dbTotal, dbUp int64
		a.DB.Model(&model.DBInstance{}).Count(&dbTotal)
		for _, s := range a.DBProbe.SnapshotAll() {
			if s.Up {
				dbUp++
			}
		}
		var userTotal int64
		a.DB.Model(&model.User{}).Count(&userTotal)
		var ipUsed int64
		a.DB.Model(&model.IPRecord{}).Count(&ipUsed)
		var containerCnt int64
		a.DB.Model(&model.TopoDevice{}).Count(&containerCnt)

		response.OK(c, gin.H{
			"kpi": gin.H{
				"link_total": linkTotal, "link_up": linkUp, "link_down": linkTotal - linkUp,
				"device_total": devTotal, "device_online": devOnline, "device_offline": devTotal - devOnline,
				"db_total": dbTotal, "db_up": dbUp, "db_down": dbTotal - dbUp,
				"user_total": userTotal, "ip_used": ipUsed, "topo_devices": containerCnt,
			},
			"links":      a.PingMgr.Snapshot(),
			"devices":    a.SnmpMgr.Snapshot(),
			"dbs":        a.DBProbe.SnapshotAll(),
			"containers": gin.H{"docker": a.DockerM.Enabled(), "k8s": a.K8sM.Enabled()},
		})
	})

	// 链路趋势（最近10分钟原始点，按任务分色）
	g.GET("/link-trends", func(c *gin.Context) {
		end := time.Now()
		start := end.Add(-10 * time.Minute)
		var tasks []model.LinkTask
		a.DB.Find(&tasks)
		series := make([]gin.H, 0, len(tasks))
		for _, t := range tasks {
			tags := map[string]string{"task_id": strconv.FormatUint(uint64(t.ID), 10)}
			rt, err := a.TSDB.Query(tsdb.Query{Metric: "link_rt", Tags: tags, Start: start, End: end, Agg: "avg"})
			if err != nil || len(rt) == 0 {
				continue
			}
			filtered := [][2]float64{}
			for _, p := range rt[0].Points {
				if p[1] > 0 {
					filtered = append(filtered, p)
				} else {
					filtered = append(filtered, [2]float64{p[0], -1})
				}
			}
			series = append(series, gin.H{"name": t.Name, "color": t.Color, "points": filtered})
		}
		response.OK(c, gin.H{"start": start.UnixMilli(), "end": end.UnixMilli(), "series": series})
	})

	// 设备健康聚合（CPU/内存均值）
	g.GET("/device-health", func(c *gin.Context) {
		snaps := a.SnmpMgr.Snapshot()
		out := make([]gin.H, 0, len(snaps))
		for _, s := range snaps {
			out = append(out, gin.H{"name": s.Name, "ip": s.IP, "up": s.Up, "cpu": s.CPU, "mem": s.MemUsed})
		}
		response.OK(c, out)
	})

	// 流量实时（按规则）
	g.GET("/traffic", func(c *gin.Context) {
		var rules []model.TrafficRule
		a.DB.Where("enable = ?", true).Find(&rules)
		out := make([]gin.H, 0, len(rules))
		for _, r := range rules {
			snap, ok := a.SnmpMgr.DeviceSnapshotByID(r.DeviceID)
			if !ok {
				continue
			}
			for _, ifs := range snap.Interfaces {
				if ifs.Name == r.Interface || ifs.Index == r.Interface {
					out = append(out, gin.H{
						"name": r.DisplayName, "color": r.Color, "in_bps": ifs.InBps, "out_bps": ifs.OutBps,
						"line_rate": r.LineRate, "up_rate": r.UpRate, "down_rate": r.DownRate,
						"in_pct": bpsPct(ifs.InBps, r.DownRate), "out_pct": bpsPct(ifs.OutBps, r.UpRate),
					})
					break
				}
			}
		}
		response.OK(c, out)
	})

	// 数据库健康
	g.GET("/db-health", func(c *gin.Context) {
		response.OK(c, a.DBProbe.SnapshotAll())
	})
}

func bpsPct(bps, limitMbps float64) float64 {
	if limitMbps <= 0 {
		return 0
	}
	return bps / (limitMbps * 1e6) * 100
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("dashboard", RegisterProtected)
}
