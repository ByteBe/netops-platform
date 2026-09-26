// Package dbmonitor 数据库监控模块
// 覆盖：可用性与状态、容量与存储、事务与日志、错误与异常
package dbmonitor

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/collector/dbprobe"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 实例列表
	g.GET("/instances", func(c *gin.Context) {
		var list []model.DBInstance
		a.DB.Order("id asc").Find(&list)
		result := make([]gin.H, 0, len(list))
		for _, d := range list {
			item := gin.H{
				"id": d.ID, "name": d.Name, "type": d.Type, "host": d.Host,
				"port": d.Port, "user": d.User, "interval": d.Interval,
				"enable": d.Enable, "remark": d.Remark, "status": d.Status,
			}
			for _, s := range a.DBProbe.SnapshotAll() {
				if s.InstanceID == d.ID {
					item["online"] = s.Up
					item["version"] = s.Version
					item["db_status"] = s.Status
					item["conns"] = s.Conns
					item["data_size"] = s.DataSize
					item["table_count"] = s.TableCount
					item["tx_per_sec"] = s.TxPerSec
					item["deadlocks"] = s.Deadlocks
					item["errors"] = s.Errors
					item["uptime"] = s.Uptime
					item["log_file"] = s.LogFile
					item["message"] = s.Message
					break
				}
			}
			result = append(result, item)
		}
		response.OK(c, result)
	})

	// 新增实例
	g.POST("/instances", func(c *gin.Context) {
		var d model.DBInstance
		if err := c.ShouldBindJSON(&d); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if d.Name == "" || d.Type == "" {
			response.Bad(c, "名称与类型必填")
			return
		}
		if d.Interval == 0 {
			d.Interval = 60
		}
		if err := a.DB.Create(&d).Error; err != nil {
			response.Err(c, err)
			return
		}
		syncDB(a)
		response.OK(c, d)
	})

	// 修改实例
	g.PUT("/instances/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.DBInstance
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "实例不存在")
			return
		}
		var req model.DBInstance
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		updates := map[string]any{
			"name": req.Name, "type": req.Type, "host": req.Host, "port": req.Port,
			"user": req.User, "interval": req.Interval,
			"enable": req.Enable, "remark": req.Remark,
		}
		if req.Password != "" {
			updates["password"] = req.Password
		}
		a.DB.Model(&d).Updates(updates)
		syncDB(a)
		response.OK(c, gin.H{"ok": true})
	})

	// 删除实例
	g.DELETE("/instances/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.DBInstance{}, id)
		syncDB(a)
		response.OK(c, gin.H{"ok": true})
	})

	// 即时探活
	g.POST("/instances/:id/probe", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.DBInstance
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "实例不存在")
			return
		}
		response.OK(c, gin.H{"ok": true, "message": "已加入采集队列，下一次采集生效"})
	})

	// 测试连接（弹窗即时测试，不保存）
	g.POST("/test", func(c *gin.Context) {
		var req struct {
			Type     string `json:"type"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		snap := a.DBProbe.TestInstance(dbprobe.Instance{
			Type: req.Type, Host: req.Host, Port: req.Port,
			User: req.User, Password: req.Password, DBName: "", Interval: 60,
		})
		if snap.Up {
			response.OK(c, gin.H{"ok": true, "version": snap.Version})
		} else {
			response.OK(c, gin.H{"ok": false, "error": snap.Message})
		}
	})

	// 历史指标
	g.GET("/instances/:id/history", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		tags := map[string]string{"db_id": strconv.FormatUint(id, 10)}
		q := func(metric string) [][2]float64 {
			s, _ := a.TSDB.Query(tsdb.Query{Metric: metric, Tags: tags, Start: start, End: end, Bucket: 5 * time.Minute, Agg: "avg"})
			if len(s) == 0 {
				return [][2]float64{}
			}
			return s[0].Points
		}
		response.OK(c, gin.H{
			"avail": q("db_avail"), "conn": q("db_conn"), "size": q("db_size"),
			"tx": q("db_tx"), "err": q("db_err"),
		})
	})

	// 数据库类型
	g.GET("/types", func(c *gin.Context) {
		response.OK(c, []gin.H{
			{"value": "mysql", "label": "MySQL", "port": 3306},
			{"value": "oracle", "label": "Oracle", "port": 1521},
			{"value": "postgresql", "label": "PostgreSQL", "port": 5432},
			{"value": "sqlserver", "label": "Microsoft SQL Server", "port": 1433},
			{"value": "mongodb", "label": "MongoDB", "port": 27017},
			{"value": "clickhouse", "label": "ClickHouse", "port": 8123},
			{"value": "tidb", "label": "TiDB", "port": 4000},
			{"value": "oceanbase", "label": "OceanBase", "port": 2881},
			{"value": "dm", "label": "达梦 DM", "port": 5236},
			{"value": "kingbase", "label": "人大金仓 KingbaseES", "port": 54321},
			{"value": "opengauss", "label": "openGauss", "port": 5432},
			{"value": "gbase", "label": "GBase 8s/8a", "port": 5258},
			{"value": "db2", "label": "IBM DB2", "port": 50000},
			{"value": "sybase", "label": "SAP Sybase", "port": 5000},
			{"value": "sqlite", "label": "内置数据库", "port": 0},
		})
	})
}

func syncDB(a *core.App) {
	var insts []model.DBInstance
	a.DB.Find(&insts)
	dbs := make([]dbprobe.Instance, 0, len(insts))
	for _, d := range insts {
		if d.Enable {
			dbs = append(dbs, dbprobe.Instance{ID: d.ID, Name: d.Name, Type: d.Type,
				Host: d.Host, Port: d.Port, User: d.User, Password: d.Password,
				DBName: "", Interval: d.Interval})
		}
	}
	a.DBProbe.Sync(dbs)
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("dbmonitor", RegisterProtected)
}
