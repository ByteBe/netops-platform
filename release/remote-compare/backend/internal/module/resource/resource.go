// Package resource 设置资源管理：将纳管的监控设备分类管理（分组/分类/颜色）
package resource

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 分组列表（含设备数+数据库数）
	g.GET("/groups", func(c *gin.Context) {
		var list []model.DeviceGroup
		a.DB.Order("type asc, id asc").Find(&list)
		type cntRow struct {
			GroupID uint  `json:"group_id"`
			Cnt     int64 `json:"cnt"`
		}
		// SNMP 设备统计
		var rows []cntRow
		a.DB.Model(&model.MonitorDevice{}).Select("group_id, count(*) as cnt").Group("group_id").Scan(&rows)
		cntMap := map[uint]int64{}
		for _, r := range rows {
			cntMap[r.GroupID] = r.Cnt
		}
		// 数据库统计
		var dbRows []cntRow
		a.DB.Model(&model.DBInstance{}).Select("group_id, count(*) as cnt").Group("group_id").Scan(&dbRows)
		for _, r := range dbRows {
			cntMap[r.GroupID] += r.Cnt
		}
		result := make([]gin.H, 0, len(list))
		for _, grp := range list {
			result = append(result, gin.H{
				"id": grp.ID, "name": grp.Name, "type": grp.Type, "color": grp.Color,
				"remark": grp.Remark, "device_count": cntMap[grp.ID],
			})
		}
		response.OK(c, result)
	})

	g.GET("/group-types", func(c *gin.Context) {
		response.OK(c, []gin.H{
			{"value": "server", "label": "服务器"},
			{"value": "switch", "label": "交换机"},
			{"value": "router", "label": "路由器"},
			{"value": "firewall", "label": "防火墙"},
			{"value": "database", "label": "数据库"},
			{"value": "other", "label": "其他"},
		})
	})

	g.POST("/groups", func(c *gin.Context) {
		var grp model.DeviceGroup
		if err := c.ShouldBindJSON(&grp); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if grp.Name == "" {
			response.Bad(c, "分组名称必填")
			return
		}
		if err := a.DB.Create(&grp).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, grp)
	})

	g.PUT("/groups/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var grp model.DeviceGroup
		if err := a.DB.First(&grp, id).Error; err != nil {
			response.NotFound(c, "分组不存在")
			return
		}
		var req model.DeviceGroup
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&grp).Updates(map[string]any{
			"name": req.Name, "type": req.Type,
			"color": req.Color, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})

	g.DELETE("/groups/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Model(&model.MonitorDevice{}).Where("group_id = ?", id).Update("group_id", 0)
		a.DB.Model(&model.DBInstance{}).Where("group_id = ?", id).Update("group_id", 0)
		a.DB.Delete(&model.DeviceGroup{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 查看组内设备（含 SNMP 设备 + 数据库实例）
	g.GET("/groups/:id/devices", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		out := []gin.H{}
		// SNMP 设备
		var devs []model.MonitorDevice
		a.DB.Where("group_id = ?", id).Order("id asc").Find(&devs)
		for _, d := range devs {
			item := gin.H{
				"id": d.ID, "name": d.Name, "ip": d.IP, "type": d.Type,
				"source": "snmp",
			}
			if s, ok := a.SnmpMgr.DeviceSnapshotByID(d.ID); ok {
				item["up"] = s.Up
				item["cpu"] = s.CPU
				item["mem"] = s.MemUsed
			} else {
				item["up"] = false
			}
			out = append(out, item)
		}
		// 数据库实例
		var dbs []model.DBInstance
		a.DB.Where("group_id = ?", id).Order("id asc").Find(&dbs)
		dbSnaps := a.DBProbe.SnapshotAll()
		for _, d := range dbs {
			item := gin.H{
				"id": d.ID, "name": d.Name, "ip": d.Host, "type": d.Type,
				"source": "db", "up": false,
			}
			for _, s := range dbSnaps {
				if s.InstanceID == d.ID {
					item["up"] = s.Up
					item["conns"] = s.Conns
					break
				}
			}
			out = append(out, item)
		}
		response.OK(c, out)
	})

	// 自动归集：SNMP 设备按类型 + 数据库全部归入 database 组
	g.POST("/auto-assign", func(c *gin.Context) {
		var groups []model.DeviceGroup
		a.DB.Find(&groups)
		typeMap := map[string][]model.DeviceGroup{}
		var dbGroup *model.DeviceGroup
		for i := range groups {
			typeMap[groups[i].Type] = append(typeMap[groups[i].Type], groups[i])
			if groups[i].Type == "database" {
				grp := groups[i]
				dbGroup = &grp
			}
		}
		assigned := 0
		// SNMP 设备按类型匹配
		var devices []model.MonitorDevice
		a.DB.Where("group_id = 0").Find(&devices)
		for _, d := range devices {
			if grps, ok := typeMap[d.Type]; ok && len(grps) > 0 {
				a.DB.Model(&d).Update("group_id", grps[0].ID)
				assigned++
			}
		}
		// 数据库实例全部归入 database 组
		if dbGroup != nil {
			var dbs []model.DBInstance
			a.DB.Where("group_id = 0").Find(&dbs)
			for _, d := range dbs {
				a.DB.Model(&d).Update("group_id", dbGroup.ID)
				assigned++
			}
		}
		response.OK(c, gin.H{"assigned": assigned})
	})
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("resource", RegisterProtected)
}
