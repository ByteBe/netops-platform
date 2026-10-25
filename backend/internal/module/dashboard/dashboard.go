// Package dashboard 数据大屏（聚合接口：KPI / 链路趋势 / 设备健康 / 流量 / 数据库 / 大屏总览）
package dashboard

import (
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

		response.OK(c, gin.H{
			"kpi": gin.H{
				"link_total": linkTotal, "link_up": linkUp, "link_down": linkTotal - linkUp,
				"device_total": devTotal, "device_online": devOnline, "device_offline": devTotal - devOnline,
				"db_total": dbTotal, "db_up": dbUp, "db_down": dbTotal - dbUp,
				"user_total": userTotal, "ip_used": ipUsed,
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
			merged := map[int64]float64{}
			for _, s := range rt {
				for _, p := range s.Points {
					merged[int64(p[0])] = p[1]
				}
			}
			filtered := [][2]float64{}
			keys := make([]int64, 0, len(merged))
			for k := range merged {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
			for _, k := range keys {
				v := merged[k]
				if v > 0 {
					filtered = append(filtered, [2]float64{float64(k), v})
				} else {
					filtered = append(filtered, [2]float64{float64(k), -1})
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

	// 大屏总览（参考运维可视化大屏：KPI + 分布 + 主机资源 + 趋势/热力图 + 变更动态 + 摘要）
	g.GET("/screen", func(c *gin.Context) {
		now := time.Now()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		weekStart := now.AddDate(0, 0, -6)
		weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

		// ---- KPI ----
		var devTotal int64
		a.DB.Model(&model.MonitorDevice{}).Count(&devTotal)
		devOnline := int64(0)
		snaps := a.SnmpMgr.Snapshot()
		for _, s := range snaps {
			if s.Up {
				devOnline++
			}
		}
		typeCnt := map[string]int64{}
		vendorCnt := map[string]int64{}
		devName := map[uint]string{}
		devType := map[uint]string{}
		var devs []model.MonitorDevice
		a.DB.Model(&model.MonitorDevice{}).Find(&devs)
		for _, d := range devs {
			if d.Type != "" {
				typeCnt[d.Type]++
			}
			if d.Vendor != "" {
				vendorCnt[d.Vendor]++
			}
			devName[d.ID] = d.Name
			devType[d.ID] = d.Type
		}

		var alertTotal, alertCritical, alertToday, alertActive int64
		a.DB.Model(&model.AlertEvent{}).Count(&alertTotal)
		a.DB.Model(&model.AlertEvent{}).Where("level = ?", "critical").Count(&alertCritical)
		a.DB.Model(&model.AlertEvent{}).Where("created_at >= ?", dayStart).Count(&alertToday)
		a.DB.Model(&model.AlertEvent{}).Where("acked = ?", false).Count(&alertActive)

		var ipUsed int64
		a.DB.Model(&model.IPRecord{}).Count(&ipUsed)
		ipTotal := int64(0)
		var subs []model.Subnet
		a.DB.Find(&subs)
		for _, s := range subs {
			ipTotal += subnetCapacity(s.CIDR)
		}

		ifUp, ifTotal := int64(0), int64(0)
		for _, s := range snaps {
			for _, ifs := range s.Interfaces {
				ifTotal++
				if strings.EqualFold(ifs.Oper, "up") {
					ifUp++
				}
			}
		}

		var tplTotal int64
		a.DB.Model(&model.ScriptTemplate{}).Count(&tplTotal)

		// ---- 告警分级分布（近7天） ----
		levelAgg := map[string]int64{}
		var levelRows []struct {
			Level string
			N     int64
		}
		a.DB.Model(&model.AlertEvent{}).
			Select("level, count(*) as n").
			Where("created_at >= ?", weekStart).
			Group("level").Scan(&levelRows)
		for _, r := range levelRows {
			levelAgg[r.Level] = r.N
		}
		alertLevel := make([]gin.H, 0, len(levelAgg))
		for lv, n := range levelAgg {
			alertLevel = append(alertLevel, gin.H{"name": lv, "value": n})
		}
		sort.Slice(alertLevel, func(i, j int) bool { return alertLevel[i]["value"].(int64) > alertLevel[j]["value"].(int64) })

		// ---- 近7天告警趋势（按天） ----
		trend := make([]gin.H, 0, 7)
		for i := 0; i < 7; i++ {
			ds := weekStart.AddDate(0, 0, i)
			de := ds.AddDate(0, 0, 1)
			var n int64
			a.DB.Model(&model.AlertEvent{}).Where("created_at >= ? AND created_at < ?", ds, de).Count(&n)
			trend = append(trend, gin.H{"date": ds.Format("01-02"), "count": n})
		}

		// ---- 告警热力图（设备 × 小时，近7天，Top 10 设备） ----
		devAlerts := map[string]int64{}
		var targetRows []struct {
			Target string
			N      int64
		}
		a.DB.Model(&model.AlertEvent{}).
			Select("target, count(*) as n").
			Where("created_at >= ?", weekStart).
			Group("target").Scan(&targetRows)
		for _, r := range targetRows {
			devAlerts[r.Target] = r.N
		}
		topDevs := make([]string, 0, len(devAlerts))
		for d := range devAlerts {
			topDevs = append(topDevs, d)
		}
		sort.Slice(topDevs, func(i, j int) bool { return devAlerts[topDevs[i]] > devAlerts[topDevs[j]] })
		if len(topDevs) > 10 {
			topDevs = topDevs[:10]
		}
		heatIdx := map[string]int{}
		heatData := make([][3]int64, 0, 64)
		for _, tgt := range topDevs {
			heatIdx[tgt] = len(heatIdx)
			var hr []model.AlertEvent
			a.DB.Where("target = ? AND created_at >= ?", tgt, weekStart).Find(&hr)
			hourCnt := map[int]int64{}
			for _, e := range hr {
				hourCnt[e.CreatedAt.Hour()]++
			}
			for h, n := range hourCnt {
				heatData = append(heatData, [3]int64{int64(heatIdx[tgt]), int64(h), n})
			}
		}

		// ---- 近7天流量趋势（TSDB if_in/if_out 按天聚合，跨接口/设备求和） ----
		// 注意：必须传 Bucket 让 TSDB 按天 INTERVAL 聚合，否则拿到原始采样点直接相加会虚高数十倍
		trafficX := make([]string, 0, 7)
		trafficIn := make([]float64, 7)
		trafficOut := make([]float64, 7)
		for i := 0; i < 7; i++ {
			trafficX = append(trafficX, weekStart.AddDate(0, 0, i).Format("01-02"))
		}
		dayIdx := map[string]int{}
		for i, x := range trafficX {
			dayIdx[x] = i
		}
		for _, metric := range []string{"if_in", "if_out"} {
			rt, err := a.TSDB.Query(tsdb.Query{Metric: metric, Start: weekStart, End: now, Bucket: time.Hour * 24, Agg: "avg"})
			if err != nil {
				continue
			}
			for _, s := range rt {
				for _, p := range s.Points {
					day := time.UnixMilli(int64(p[0])).Format("01-02")
					if idx, ok := dayIdx[day]; ok {
						if metric == "if_in" {
							trafficIn[idx] += p[1]
						} else {
							trafficOut[idx] += p[1]
						}
					}
				}
			}
		}

		// ---- 主机资源（在线设备 CPU/内存/磁盘均值；磁盘只统计有磁盘数据的设备） ----
		cpuSum, memSum, diskSum, hostN, diskN := 0.0, 0.0, 0.0, 0.0, 0.0
		deviceRank := make([]gin.H, 0, len(snaps))
		for _, s := range snaps {
			disk := 0.0
			if len(s.Disks) > 0 {
				var sum float64
				for _, d := range s.Disks {
					sum += d.UsedPct
				}
				disk = sum / float64(len(s.Disks))
			}
			deviceRank = append(deviceRank, gin.H{
				"name": s.Name, "ip": s.IP, "up": s.Up, "device_id": s.DeviceID,
				"cpu": math.Round(s.CPU*10) / 10, "mem": math.Round(s.MemUsed*10) / 10, "disk": math.Round(disk*10) / 10,
			})
			if s.Up {
				cpuSum += s.CPU
				memSum += s.MemUsed
				hostN++
				if len(s.Disks) > 0 {
					diskSum += disk
					diskN++
				}
			}
		}
		hostRes := gin.H{"cpu": 0, "mem": 0, "disk": 0}
		if hostN > 0 {
			hostRes = gin.H{
				"cpu":  math.Round(cpuSum/hostN*10) / 10,
				"mem":  math.Round(memSum/hostN*10) / 10,
				"disk": math.Round(diskSum/diskN*10) / 10,
			}
			if diskN == 0 {
				hostRes["disk"] = 0
			}
		}

		// ---- 设备变更动态（审计日志最近 6 条设备相关操作） ----
		var logs []model.AuditLog
		a.DB.Order("created_at desc").Limit(6).Find(&logs)
		changeFeed := make([]gin.H, 0, len(logs))
		for _, l := range logs {
			changeFeed = append(changeFeed, gin.H{
				"user": l.Username, "action": l.Action, "time": l.CreatedAt.Format("01-02 15:04"),
			})
		}

		// ---- 运维摘要 ----
		var taskTotal, taskOK int64
		a.DB.Model(&model.TaskLog{}).Where("created_at >= ?", weekStart).Count(&taskTotal)
		a.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND success = ?", weekStart, true).Count(&taskOK)
		summary := gin.H{
			"alert_active":  alertActive,
			"alert_today":   alertToday,
			"task_total":    taskTotal,
			"task_success":  taskOK,
			"dev_online":    devOnline,
			"dev_total":     devTotal,
			"ip_used":       ipUsed,
			"if_up":         ifUp,
			"if_total":      ifTotal,
		}

		response.OK(c, gin.H{
			"kpi": gin.H{
				"device_total": devTotal, "device_online": devOnline, "device_offline": devTotal - devOnline,
				"dev_type":  typeCnt,
				"alert_today": alertToday, "alert_total": alertTotal, "alert_critical": alertCritical,
				"ip_used": ipUsed, "ip_total": ipTotal,
				"if_up": ifUp, "if_total": ifTotal,
				"template_total": tplTotal,
			},
			"vendor_dist":  vendorPie(vendorCnt),
			"type_dist":    vendorPie(typeCnt),
			"alert_level":  alertLevel,
			"host_res":     hostRes,
			"alert_trend":  trend,
			"alert_heat":   gin.H{"devices": topDevs, "data": heatData},
			"traffic_trend": gin.H{"x": trafficX, "in": trafficIn, "out": trafficOut},
			"device_rank":  deviceRank,
			"change_feed":  changeFeed,
			"summary":      summary,
		})
	})

	registerGeoMap(a, g)
}

// subnetCapacity 子网可用 IP 数（去网络号与广播地址）
func subnetCapacity(cidr string) int64 {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0
	}
	ones, bits := ipNet.Mask.Size()
	total := int64(1) << (bits - ones)
	if total > 2 {
		return total - 2
	}
	return total
}

// vendorPie map[string]int64 -> 饼图数据（按值降序）
func vendorPie(m map[string]int64) []gin.H {
	out := make([]gin.H, 0, len(m))
	for k, v := range m {
		out = append(out, gin.H{"name": k, "value": v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["value"].(int64) > out[j]["value"].(int64) })
	return out
}

// ---------- 设备地理图（底图导入 + 点位配置） ----------

// ---------- 设备地理图（底图导入 + 点位配置） ----------

const geoMapDir = "data/geomap"

func geoMapImagePath(a *core.App) string {
	var cfg model.GeoMapConfig
	if err := a.DB.First(&cfg).Error; err != nil || cfg.Image == "" {
		return ""
	}
	return filepath.Join(geoMapDir, cfg.Image)
}

// registerGeoMap 设备地理图路由：底图上传/访问 + 点位查询/保存/删除
func registerGeoMap(a *core.App, g *gin.RouterGroup) {
	// 查询：底图 + 点位（带设备名/在线状态）
	g.GET("/geomap", func(c *gin.Context) {
		image := ""
		if p := geoMapImagePath(a); p != "" {
			if _, err := os.Stat(p); err == nil {
				image = "/api/v1/dashboard/geomap/image"
			}
		}
		upMap := map[uint]bool{}
		for _, s := range a.SnmpMgr.Snapshot() {
			upMap[s.DeviceID] = s.Up
		}
		var pts []model.GeoMapPoint
		a.DB.Find(&pts)
		nameMap := map[uint]string{}
		for _, d := range devsAll(a) {
			nameMap[d.ID] = d.Name
		}
		out := make([]gin.H, 0, len(pts))
		for _, p := range pts {
			out = append(out, gin.H{
				"device_id": p.DeviceID,
				"name":      nameMap[p.DeviceID],
				"x":         p.X, "y": p.Y,
				"up": upMap[p.DeviceID],
			})
		}
		response.OK(c, gin.H{"image": image, "points": out})
	})

	// 上传底图（png/jpg/jpeg/webp，≤5MB）
	g.POST("/geomap/upload", func(c *gin.Context) {
		f, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "缺少文件字段 file")
			return
		}
		if f.Size > 5*1024*1024 {
			response.Bad(c, "图片不能超过 5MB")
			return
		}
		ext := strings.ToLower(filepath.Ext(f.Filename))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
		default:
			response.Bad(c, "仅支持 png/jpg/jpeg/webp 图片")
			return
		}
		if err := os.MkdirAll(geoMapDir, 0o755); err != nil {
			response.Err(c, err)
			return
		}
		old := geoMapImagePath(a)
		name := fmt.Sprintf("geo_%d%s", time.Now().UnixNano(), ext)
		dst := filepath.Join(geoMapDir, name)
		if err := c.SaveUploadedFile(f, dst); err != nil {
			response.Err(c, err)
			return
		}
		if old != "" && old != dst {
			_ = os.Remove(old)
		}
		cfg := model.GeoMapConfig{ID: 1, Image: name}
		a.DB.Where("id = ?", 1).Assign(cfg).FirstOrCreate(&cfg)
		response.OK(c, gin.H{"image": "/api/v1/dashboard/geomap/image"})
	})

	// 底图访问
	g.GET("/geomap/image", func(c *gin.Context) {
		p := geoMapImagePath(a)
		if p == "" {
			c.Status(404)
			return
		}
		c.File(p)
	})

	// 批量保存点位（upsert，按 device_id）
	g.PUT("/geomap/points", func(c *gin.Context) {
		var req []struct {
			DeviceID uint    `json:"device_id"`
			X        float64 `json:"x"`
			Y        float64 `json:"y"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		for _, r := range req {
			if r.X < 0 || r.X > 100 || r.Y < 0 || r.Y > 100 {
				continue
			}
			pt := model.GeoMapPoint{DeviceID: r.DeviceID, X: r.X, Y: r.Y}
			a.DB.Where("device_id = ?", r.DeviceID).Assign(model.GeoMapPoint{X: r.X, Y: r.Y}).FirstOrCreate(&pt)
		}
		response.OK(c, gin.H{"ok": true})
	})

	// 删除点位
	g.DELETE("/geomap/points/:deviceId", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("deviceId"), 10, 64)
		a.DB.Where("device_id = ?", id).Delete(&model.GeoMapPoint{})
		response.OK(c, gin.H{"ok": true})
	})
}

// devsAll 全部监控设备（ID->Name 映射）
func devsAll(a *core.App) []model.MonitorDevice {
	var devs []model.MonitorDevice
	a.DB.Model(&model.MonitorDevice{}).Find(&devs)
	return devs
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
