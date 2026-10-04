// Package monitor 监控服务器/交换机/路由器（SNMP 采集）
package monitor

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/collector/snmp"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 设备列表（支持按分组过滤）
	g.GET("/devices", func(c *gin.Context) {
		q := a.DB.Model(&model.MonitorDevice{})
		if gid := c.Query("group_id"); gid != "" && gid != "0" {
			q = q.Where("group_id = ?", gid)
		}
		if tp := c.Query("type"); tp != "" {
			q = q.Where("type = ?", tp)
		}
		if nu := c.Query("node_uuid"); nu != "" {
			q = q.Where("node_uuid = ?", nu)
		}
		var list []model.MonitorDevice
		q.Order("id asc").Find(&list)
		// 附加实时快照
		result := make([]gin.H, 0, len(list))
		for _, d := range list {
			item := gin.H{
				"id": d.ID, "group_id": d.GroupID, "name": d.Name, "ip": d.IP,
				"type": d.Type, "vendor": d.Vendor, "snmp_version": d.SNMPVersion,
				"community": d.Community, "port": d.Port, "interval": d.Interval,
				"enable": d.Enable, "remark": d.Remark, "status": d.Status,
				"username": d.Username, "auth_proto": d.AuthProto, "priv_proto": d.PrivProto,
				"auth_pass": d.AuthPass, "priv_pass": d.PrivPass,
			}
			if s, ok := a.SnmpMgr.DeviceSnapshotByID(d.ID); ok {
				item["online"] = s.Up
				item["cpu"] = s.CPU
				item["mem"] = s.MemUsed
				item["uptime"] = s.Uptime
				item["message"] = s.Message
				item["if_count"] = len(s.Interfaces)
			} else {
				item["online"] = false
				item["cpu"] = 0
				item["mem"] = 0
				item["uptime"] = 0
				item["message"] = "未采集"
				item["if_count"] = 0
			}
			result = append(result, item)
		}
		response.OK(c, result)
	})

	// 新增设备
	g.POST("/devices", func(c *gin.Context) {
		var d model.MonitorDevice
		if err := c.ShouldBindJSON(&d); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if d.Name == "" || d.IP == "" {
			response.Bad(c, "名称与IP必填")
			return
		}
		if d.Interval == 0 {
			d.Interval = 60
		}
		if d.Port == 0 {
			d.Port = 161
		}
		if d.SNMPVersion == "" {
			d.SNMPVersion = "2c"
		}
		if err := a.DB.Create(&d).Error; err != nil {
			response.Err(c, err)
			return
		}
		syncDevice(a, d)
		response.OK(c, d)
	})

	// 修改设备
	g.PUT("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.MonitorDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		var req model.MonitorDevice
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&d).Updates(map[string]any{
			"group_id": req.GroupID, "name": req.Name, "ip": req.IP, "type": req.Type,
			"vendor": req.Vendor, "snmp_version": req.SNMPVersion, "community": req.Community,
			"username": req.Username, "auth_proto": req.AuthProto, "priv_proto": req.PrivProto,
			"auth_pass": req.AuthPass, "priv_pass": req.PrivPass, "port": req.Port,
			"interval": req.Interval, "enable": req.Enable, "remark": req.Remark,
		})
		req.ID = uint(id); syncDevice(a, req)
		response.OK(c, gin.H{"ok": true})
	})

	// 删除设备
	g.DELETE("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.MonitorDevice{}, id)
		a.DB.Delete(&model.TrafficRule{}, "device_id = ?", id)
		refreshSnmp(a)
		response.OK(c, gin.H{"ok": true})
	})

	// 测试连接（弹窗即时测试，不保存）
	g.POST("/test", func(c *gin.Context) {
		var req struct {
			IP          string `json:"ip"`
			SNMPVersion string `json:"snmp_version"`
			Community   string `json:"community"`
			Username    string `json:"username"`
			AuthProto   string `json:"auth_proto"`
			PrivProto   string `json:"priv_proto"`
			AuthPass    string `json:"auth_pass"`
			PrivPass    string `json:"priv_pass"`
			Port        int    `json:"port"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if req.Port == 0 {
			req.Port = 161
		}
		up, uptime, msg := a.SnmpMgr.TestDevice(snmp.Device{
			IP: req.IP, SNMPVersion: req.SNMPVersion, Community: req.Community,
			Username: req.Username, AuthProto: req.AuthProto, PrivProto: req.PrivProto,
			AuthPass: req.AuthPass, PrivPass: req.PrivPass, Port: req.Port,
		})
		if up {
			response.OK(c, gin.H{"ok": true, "uptime": uptime, "message": msg})
		} else {
			response.OK(c, gin.H{"ok": false, "error": msg})
		}
	})

	// 实时快照
	g.GET("/snapshots", func(c *gin.Context) {
		response.OK(c, a.SnmpMgr.Snapshot())
	})

	// 历史指标（CPU/内存）
	g.GET("/devices/:id/history", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		start, end := rangeQuery(c)
		tags := map[string]string{"device_id": strconv.FormatUint(id, 10)}
		cpu, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_cpu", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "avg"})
		mem, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_mem", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "avg"})
		up, _ := a.TSDB.Query(tsdb.Query{Metric: "dev_up", Tags: tags, Start: start, End: end, Bucket: time.Minute * 5, Agg: "last"})
		response.OK(c, gin.H{"cpu": points(cpu), "mem": points(mem), "up": points(up)})
	})

	// 设备类型
	g.GET("/types", func(c *gin.Context) {
		response.OK(c, []gin.H{
			{"value": "server", "label": "服务器"},
			{"value": "switch", "label": "交换机"},
			{"value": "router", "label": "路由器"},
			{"value": "firewall", "label": "防火墙"},
			{"value": "other", "label": "其他"},
		})
	})
}

// syncDevice 新增/修改后同步采集器
func syncDevice(a *core.App, d model.MonitorDevice) {
	if d.Enable {
		a.SnmpMgr.SyncDevices([]snmp.Device{{
			ID: d.ID, Name: d.Name, IP: d.IP, Type: d.Type, SNMPVersion: d.SNMPVersion,
			Community: d.Community, Username: d.Username, AuthProto: d.AuthProto,
			PrivProto: d.PrivProto, AuthPass: d.AuthPass, PrivPass: d.PrivPass,
			Port: d.Port, Interval: d.Interval,
		}})
	} else {
		refreshSnmp(a)
	}
}

// refreshSnmp 全量重载采集器
func refreshSnmp(a *core.App) {
	var devices []model.MonitorDevice
	a.DB.Find(&devices)
	devs := make([]snmp.Device, 0, len(devices))
	for _, d := range devices {
		if d.Enable {
			devs = append(devs, snmp.Device{ID: d.ID, Name: d.Name, IP: d.IP, Type: d.Type, Vendor: d.Vendor,
				SNMPVersion: d.SNMPVersion, Community: d.Community, Username: d.Username,
				AuthProto: d.AuthProto, PrivProto: d.PrivProto, AuthPass: d.AuthPass,
				PrivPass: d.PrivPass, Port: d.Port, Interval: d.Interval})
		}
	}
	a.SnmpMgr.SyncDevices(devs)
}

func rangeQuery(c *gin.Context) (time.Time, time.Time) {
	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if s := c.Query("start"); s != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
			start = t
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", e, time.Local); err == nil {
			end = t
		}
	}
	return start, end
}

func points(series []tsdb.Series) [][2]float64 {
	if len(series) == 0 {
		return [][2]float64{}
	}
	return series[0].Points
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("monitor", RegisterProtected)
}
