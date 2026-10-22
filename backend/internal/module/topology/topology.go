// Package topology 网络拓扑总览（华为 eSight 风格数据源）
// 节点 = 纳管监控设备 + 本机核心 + 检测目标外部节点 + 手工节点；链路 = 链路检测任务 + 手工链路
// 实时指标：设备 CPU/内存取 TSDB 最近点，链路时延/丢包取 PingMgr 实时缓存
// 深度视图：类型统计 / 设备告警数 / 在线时长 / 设备与链路历史趋势 / 活跃告警
// 编辑能力：手工添加/修改/删除节点与链路、拖拽位置持久化（TopoPos）
package topology

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// 拓扑总览（含类型统计 + 设备告警数 + 活跃告警预览 + 手工节点/链路 + 位置）
	g.GET("/overview", func(c *gin.Context) {
		var devs []model.MonitorDevice
		a.DB.Order("id asc").Find(&devs)
		var tasks []model.LinkTask
		a.DB.Order("id asc").Find(&tasks)

		since24h := time.Now().Add(-24 * time.Hour)
		typeStats := map[string]int{}
		devAlerts := map[string]int{} // device name -> 24h 未确认告警数
		devIP := map[string]uint{}
		nodes := []gin.H{}
		hostID := ""
		for _, d := range devs {
			devIP[d.IP] = d.ID
			typeStats[d.Type]++
			var cnt int64
			a.DB.Model(&model.AlertEvent{}).Where("target = ? AND acked = ? AND created_at > ?", d.Name, false, since24h).Count(&cnt)
			devAlerts[d.Name] = int(cnt)
			n := gin.H{
				"id": fmt.Sprintf("dev_%d", d.ID), "name": d.Name, "ip": d.IP,
				"type": d.Type, "vendor": d.Vendor, "status": d.Status,
				"cpu": latestVal(a, "dev_cpu", map[string]string{"device_id": fmt.Sprint(d.ID)}),
				"mem": latestVal(a, "dev_mem", map[string]string{"device_id": fmt.Sprint(d.ID)}),
				"node_uuid": d.NodeUUID,
				"alerts":    devAlerts[d.Name],
				"uptime":    d.LastUptime,
			}
			if d.NodeUUID == "" && hostID == "" {
				hostID = fmt.Sprintf("dev_%d", d.ID)
			}
			nodes = append(nodes, n)
		}
		if hostID == "" {
			hostID = "host"
			nodes = append(nodes, gin.H{"id": "host", "name": "本机", "ip": "", "type": "server", "vendor": "", "status": "up", "cpu": 0, "mem": 0, "node_uuid": "", "alerts": 0, "uptime": 0})
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
			nodes = append(nodes, gin.H{"id": nid, "name": target, "ip": target, "type": "target", "vendor": "", "status": "unknown", "cpu": 0, "mem": 0, "node_uuid": "", "alerts": 0, "uptime": 0})
		}

		// 位置记忆（任意节点 id → x/y）
		var poss []model.TopoPos
		a.DB.Find(&poss)
		posMap := map[string][2]float64{}
		for _, p := range poss {
			posMap[p.NodeID] = [2]float64{p.X, p.Y}
		}
		applyPos := func(id string, h gin.H) {
			if p, ok := posMap[id]; ok {
				h["x"], h["y"] = p[0], p[1]
			}
		}
		for _, n := range nodes {
			applyPos(fmt.Sprint(n["id"]), n)
		}

		// 手工节点
		var tnodes []model.TopoNode
		a.DB.Order("id asc").Find(&tnodes)
		for _, tn := range tnodes {
			typeStats[tn.Type]++
			n := gin.H{
				"id": fmt.Sprintf("tn_%d", tn.ID), "name": tn.Name, "ip": tn.IP,
				"type": tn.Type, "vendor": "", "status": tn.Status,
				"cpu": 0, "mem": 0, "node_uuid": "", "alerts": 0, "uptime": 0,
				"manual": true, "x": tn.X, "y": tn.Y,
			}
			nodes = append(nodes, n)
		}
		// 手工链路（引用不存在的节点则跳过）
		nodeIDs := map[string]bool{}
		for _, n := range nodes {
			nodeIDs[fmt.Sprint(n["id"])] = true
		}
		var tlinks []model.TopoLink
		a.DB.Order("id asc").Find(&tlinks)
		for _, tl := range tlinks {
			if !nodeIDs[tl.Source] || !nodeIDs[tl.Target] {
				continue
			}
			links = append(links, gin.H{
				"id": fmt.Sprintf("ml_%d", tl.ID), "name": tl.Name,
				"source": tl.Source, "target": tl.Target,
				"method": tl.Method, "port": tl.Port,
				"status": tl.Status, "rt": 0, "loss": 0, "manual": true, "color": tl.Color,
			})
		}

		// 活跃告警（24h 内未确认，倒序最多 8 条预览 + 统计）
		alerts := []gin.H{}
		var evs []model.AlertEvent
		a.DB.Where("acked = ? AND created_at > ?", false, since24h).Order("created_at desc").Limit(8).Find(&evs)
		for _, e := range evs {
			alerts = append(alerts, gin.H{"id": e.ID, "level": e.Level, "metric": e.Metric, "target": e.Target, "message": e.Message, "created_at": e.CreatedAt})
		}
		var alertTotal, alertCritical int64
		a.DB.Model(&model.AlertEvent{}).Where("acked = ? AND created_at > ?", false, since24h).Count(&alertTotal)
		a.DB.Model(&model.AlertEvent{}).Where("acked = ? AND created_at > ? AND level = ?", false, since24h, "critical").Count(&alertCritical)

		response.OK(c, gin.H{
			"nodes":      nodes,
			"links":      links,
			"typeStats":  typeStats,
			"alertStats": gin.H{"total": alertTotal, "critical": alertCritical},
			"alerts":     alerts,
		})
	})

	// 历史趋势：kind=device → cpu/mem；kind=link → rt/loss/status
	g.GET("/history", func(c *gin.Context) {
		kind := c.Query("kind")
		idStr := c.Query("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			response.Bad(c, "参数错误")
			return
		}
		start, end := parseRange(c)
		bucket := parseBucket(c, start, end)
		if kind == "link" {
			var t model.LinkTask
			if err := a.DB.First(&t, id).Error; err != nil {
				response.NotFound(c, "任务不存在")
				return
			}
			tags := map[string]string{"task_id": idStr}
			rt, _ := a.TSDB.Query(tsdb.Query{Metric: "link_rt", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
			loss, _ := a.TSDB.Query(tsdb.Query{Metric: "link_loss", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
			status, _ := a.TSDB.Query(tsdb.Query{Metric: "link_status", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "last"})
			response.OK(c, gin.H{"rt": firstPoints(rt), "loss": firstPoints(loss), "status": firstPoints(status)})
			return
		}
		// device
		var d model.MonitorDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		tags := map[string]string{"device_id": idStr}
		cpu, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_cpu", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
		mem, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_mem", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
		response.OK(c, gin.H{"cpu": firstPoints(cpu), "mem": firstPoints(mem)})
	})

	// 活跃告警列表（24h 内未确认，倒序，默认 50 条）
	g.GET("/alerts", func(c *gin.Context) {
		size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
		if size <= 0 || size > 200 {
			size = 50
		}
		evs := []model.AlertEvent{}
		a.DB.Where("acked = ? AND created_at > ?", false, time.Now().Add(-24*time.Hour)).Order("created_at desc").Limit(size).Find(&evs)
		out := make([]gin.H, 0, len(evs))
		for _, e := range evs {
			out = append(out, gin.H{"id": e.ID, "level": e.Level, "metric": e.Metric, "target": e.Target, "message": e.Message, "created_at": e.CreatedAt})
		}
		response.OK(c, gin.H{"list": out})
	})

	// ===== 手工节点 CRUD =====
	g.POST("/nodes", func(c *gin.Context) {
		var in model.TopoNode
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			response.Bad(c, "节点名称不能为空")
			return
		}
		if in.Type == "" {
			in.Type = "other"
		}
		if in.Status == "" {
			in.Status = "unknown"
		}
		if err := a.DB.Create(&in).Error; err != nil {
			response.Bad(c, "创建失败")
			return
		}
		// 位置一并入库，保证手工节点坐标持久化
		a.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}}, DoUpdates: clause.AssignmentColumns([]string{"x", "y", "updated_at"})}).Create(&model.TopoPos{
			NodeID: fmt.Sprintf("tn_%d", in.ID), X: in.X, Y: in.Y, UpdatedAt: time.Now(),
		})
		response.OK(c, in)
	})
	g.PUT("/nodes/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var tn model.TopoNode
		if err := a.DB.First(&tn, id).Error; err != nil {
			response.NotFound(c, "节点不存在")
			return
		}
		var in model.TopoNode
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if strings.TrimSpace(in.Name) != "" {
			tn.Name = strings.TrimSpace(in.Name)
		}
		tn.IP = in.IP
		if in.Type != "" {
			tn.Type = in.Type
		}
		if in.Status != "" {
			tn.Status = in.Status
		}
		tn.Remark = in.Remark
		a.DB.Model(&tn).Select("name", "ip", "type", "status", "remark").Updates(tn)
		response.OK(c, tn)
	})
	g.DELETE("/nodes/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var tn model.TopoNode
		if err := a.DB.First(&tn, id).Error; err != nil {
			response.NotFound(c, "节点不存在")
			return
		}
		nid := fmt.Sprintf("tn_%d", tn.ID)
		a.DB.Where("source = ? OR target = ?", nid, nid).Delete(&model.TopoLink{})
		a.DB.Where("node_id = ?", nid).Delete(&model.TopoPos{})
		a.DB.Delete(&tn)
		response.OK(c, gin.H{"ok": true})
	})

	// ===== 手工链路 CRUD =====
	g.POST("/links", func(c *gin.Context) {
		var in model.TopoLink
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			response.Bad(c, "链路名称不能为空")
			return
		}
		if in.Source == "" || in.Target == "" {
			response.Bad(c, "源/目标节点不能为空")
			return
		}
		if in.Source == in.Target {
			response.Bad(c, "源和目标不能相同")
			return
		}
		if in.Method == "" {
			in.Method = "manual"
		}
		if in.Status == "" {
			in.Status = "unknown"
		}
		if in.Color == "" {
			in.Color = "#3d9df2"
		}
		if err := a.DB.Create(&in).Error; err != nil {
			response.Bad(c, "创建失败")
			return
		}
		response.OK(c, in)
	})
	g.PUT("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var tl model.TopoLink
		if err := a.DB.First(&tl, id).Error; err != nil {
			response.NotFound(c, "链路不存在")
			return
		}
		var in model.TopoLink
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if strings.TrimSpace(in.Name) != "" {
			tl.Name = strings.TrimSpace(in.Name)
		}
		if in.Source != "" {
			tl.Source = in.Source
		}
		if in.Target != "" {
			tl.Target = in.Target
		}
		if in.Method != "" {
			tl.Method = in.Method
		}
		if in.Status != "" {
			tl.Status = in.Status
		}
		if in.Color != "" {
			tl.Color = in.Color
		}
		tl.Port = in.Port
		tl.Remark = in.Remark
		a.DB.Model(&tl).Select("name", "source", "target", "method", "port", "status", "color", "remark").Updates(tl)
		response.OK(c, tl)
	})
	g.DELETE("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var tl model.TopoLink
		if err := a.DB.First(&tl, id).Error; err != nil {
			response.NotFound(c, "链路不存在")
			return
		}
		a.DB.Delete(&tl)
		response.OK(c, gin.H{"ok": true})
	})

	// 批量保存节点位置（拖拽后持久化）
	g.PUT("/positions", func(c *gin.Context) {
		var in struct {
			Positions []struct {
				ID string  `json:"id"`
				X  float64 `json:"x"`
				Y  float64 `json:"y"`
			} `json:"positions"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		now := time.Now()
		err := a.DB.Transaction(func(tx *gorm.DB) error {
			for _, p := range in.Positions {
				if strings.TrimSpace(p.ID) == "" {
					continue
				}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}}, DoUpdates: clause.AssignmentColumns([]string{"x", "y", "updated_at"})}).Create(&model.TopoPos{
					NodeID: p.ID, X: p.X, Y: p.Y, UpdatedAt: now,
				}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			response.Bad(c, "保存失败")
			return
		}
		response.OK(c, gin.H{"ok": true, "saved": len(in.Positions)})
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

// parseRange 历史范围（复用链路检测的解析约定）
func parseRange(c *gin.Context) (time.Time, time.Time) {
	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			if v > 168 {
				v = 168
			}
			start = end.Add(-time.Duration(v) * time.Hour)
		}
	}
	if s := c.Query("start"); s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			start = time.UnixMilli(ms)
		}
	}
	if e := c.Query("end"); e != "" {
		if ms, err := strconv.ParseInt(e, 10, 64); err == nil {
			end = time.UnixMilli(ms)
		}
	}
	if !end.After(start) {
		end = start.Add(time.Hour)
	}
	return start, end
}

// parseBucket 按跨度自动选择聚合粒度
func parseBucket(c *gin.Context, start, end time.Time) time.Duration {
	if b := c.Query("bucket"); b != "" {
		if v, err := strconv.Atoi(b); err == nil && v > 0 {
			return time.Duration(v) * time.Second
		}
	}
	span := end.Sub(start)
	switch {
	case span <= 2*time.Hour:
		return time.Minute
	case span <= 24*time.Hour:
		return 5 * time.Minute
	case span <= 7*24*time.Hour:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

// firstPoints 多序列按时间戳合并升序
func firstPoints(series []tsdb.Series) [][2]float64 {
	if len(series) == 0 {
		return [][2]float64{}
	}
	merged := map[int64]float64{}
	for _, s := range series {
		for _, p := range s.Points {
			merged[int64(p[0])] = p[1]
		}
	}
	out := make([][2]float64, 0, len(merged))
	for ts, v := range merged {
		out = append(out, [2]float64{float64(ts), v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
