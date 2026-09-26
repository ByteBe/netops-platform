// Package traffic 流量监控（通过纳管设备SNMP采集；专线大小与接口带宽可不同、上下行速率可不同）
package traffic

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
	// 规则列表
	g.GET("/rules", func(c *gin.Context) {
		var list []model.TrafficRule
		a.DB.Order("id asc").Find(&list)
		result := make([]gin.H, 0, len(list))
		for _, r := range list {
			item := gin.H{
				"id": r.ID, "device_id": r.DeviceID, "interface": r.Interface,
				"display_name": r.DisplayName, "line_rate": r.LineRate, "up_rate": r.UpRate,
				"down_rate": r.DownRate, "color": r.Color, "enable": r.Enable, "remark": r.Remark,
			}
			var dev model.MonitorDevice
			if err := a.DB.First(&dev, r.DeviceID).Error; err == nil {
				item["device_name"] = dev.Name
				item["device_ip"] = dev.IP
			}
			result = append(result, item)
		}
		response.OK(c, result)
	})

	// 新增规则
	g.POST("/rules", func(c *gin.Context) {
		var r model.TrafficRule
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if r.DeviceID == 0 || r.Interface == "" {
			response.Bad(c, "监控设备与接口必填")
			return
		}
		if r.DisplayName == "" {
			r.DisplayName = r.Interface
		}
		if r.LineRate == 0 {
			r.LineRate = 1000
		}
		if r.Color == "" {
			r.Color = "#409EFF"
		}
		if err := a.DB.Create(&r).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, r)
	})

	// 修改规则
	g.PUT("/rules/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var r model.TrafficRule
		if err := a.DB.First(&r, id).Error; err != nil {
			response.NotFound(c, "规则不存在")
			return
		}
		var req model.TrafficRule
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&r).Updates(map[string]any{
			"device_id": req.DeviceID, "interface": req.Interface, "display_name": req.DisplayName,
			"line_rate": req.LineRate, "up_rate": req.UpRate, "down_rate": req.DownRate,
			"color": req.Color, "enable": req.Enable, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})

	// 删除规则
	g.DELETE("/rules/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.TrafficRule{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 规则实时速率（来自SNMP采集快照）
	g.GET("/rules/:id/current", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var r model.TrafficRule
		if err := a.DB.First(&r, id).Error; err != nil {
			response.NotFound(c, "规则不存在")
			return
		}
		snap, ok := a.SnmpMgr.DeviceSnapshotByID(r.DeviceID)
		if !ok {
			response.OK(c, gin.H{"up": false, "in_bps": 0, "out_bps": 0})
			return
		}
		for _, ifs := range snap.Interfaces {
			if ifs.Name == r.Interface || ifs.Index == r.Interface {
				response.OK(c, gin.H{
					"up": snap.Up, "in_bps": ifs.InBps, "out_bps": ifs.OutBps,
					"in_pct": pct(ifs.InBps, r.DownRate), "out_pct": pct(ifs.OutBps, r.UpRate),
					"line_rate": r.LineRate, "up_rate": r.UpRate, "down_rate": r.DownRate,
				})
				return
			}
		}
		response.OK(c, gin.H{"up": snap.Up, "in_bps": 0, "out_bps": 0})
	})

	// 历史流量
	g.GET("/rules/:id/history", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var r model.TrafficRule
		if err := a.DB.First(&r, id).Error; err != nil {
			response.NotFound(c, "规则不存在")
			return
		}
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		tags := map[string]string{"device_id": strconv.FormatUint(uint64(r.DeviceID), 10), "if_name": r.Interface}
		in, _ := a.TSDB.Query(tsdb.Query{Metric: "if_in", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "avg"})
		out, _ := a.TSDB.Query(tsdb.Query{Metric: "if_out", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "avg"})
		response.OK(c, gin.H{"in": seriesPoints(in), "out": seriesPoints(out), "rule": r})
	})

	// 接口列表（从设备快照）
	g.GET("/devices/:id/interfaces", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		snap, ok := a.SnmpMgr.DeviceSnapshotByID(uint(id))
		if !ok {
			response.OK(c, []gin.H{})
			return
		}
		out := make([]gin.H, 0, len(snap.Interfaces))
		for _, ifs := range snap.Interfaces {
			out = append(out, gin.H{"index": ifs.Index, "name": ifs.Name, "speed": ifs.Speed, "oper": ifs.Oper})
		}
		response.OK(c, out)
	})
}

func pct(bps, limitMbps float64) float64 {
	if limitMbps <= 0 {
		return 0
	}
	return bps / (limitMbps * 1e6) * 100
}

func seriesPoints(series []tsdb.Series) [][2]float64 {
	if len(series) == 0 {
		return [][2]float64{}
	}
	return series[0].Points
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("traffic", RegisterProtected)
}
