// Package nginxmon Nginx 监控模块
// 覆盖：可用性与状态、活跃连接、请求速率(QPS)、Reading/Writing/Waiting
package nginxmon

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/collector/nginxprobe"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

// RegisterProtected 路由（认证组内 /api/v1/nginxmon/...）
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 实例列表（合并实时快照）
	g.GET("/instances", func(c *gin.Context) {
		var list []model.NginxInstance
		a.DB.Order("id asc").Find(&list)
		snapMap := map[uint]nginxprobe.Snapshot{}
		for _, s := range a.NginxM.SnapshotAll() {
			snapMap[s.InstanceID] = s
		}
		result := make([]gin.H, 0, len(list))
		for _, d := range list {
			item := gin.H{
				"id": d.ID, "name": d.Name, "status_url": d.StatusURL,
				"interval": d.Interval, "enable": d.Enable, "status": d.Status,
				"remark": d.Remark, "created_at": d.CreatedAt,
			}
			if s, ok := snapMap[d.ID]; ok {
				item["online"] = s.Up
				item["active_conns"] = s.ActiveConns
				item["requests"] = s.Requests
				item["qps"] = s.QPS
				item["reading"] = s.Reading
				item["writing"] = s.Writing
				item["waiting"] = s.Waiting
				item["message"] = s.Message
				item["ts"] = s.TS
			}
			result = append(result, item)
		}
		response.OK(c, result)
	})

	// 新增实例
	g.POST("/instances", func(c *gin.Context) {
		var d model.NginxInstance
		if err := c.ShouldBindJSON(&d); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if d.Name == "" || d.StatusURL == "" {
			response.Bad(c, "名称与状态页地址必填")
			return
		}
		if d.Interval == 0 {
			d.Interval = 60
		}
		if err := a.DB.Create(&d).Error; err != nil {
			response.Err(c, err)
			return
		}
		syncNginx(a)
		response.OK(c, d)
	})

	// 修改实例
	g.PUT("/instances/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.NginxInstance
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "实例不存在")
			return
		}
		var req model.NginxInstance
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if err := a.DB.Model(&d).Updates(map[string]any{
			"name": req.Name, "status_url": req.StatusURL, "interval": req.Interval,
			"enable": req.Enable, "remark": req.Remark,
		}).Error; err != nil {
			response.Err(c, err)
			return
		}
		syncNginx(a)
		response.OK(c, gin.H{"ok": true})
	})

	// 删除实例
	g.DELETE("/instances/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		res := a.DB.Delete(&model.NginxInstance{}, id)
		if res.Error != nil {
			response.Err(c, res.Error)
			return
		}
		if res.RowsAffected == 0 {
			response.NotFound(c, "实例不存在")
			return
		}
		syncNginx(a)
		response.OK(c, gin.H{"ok": true})
	})

	// 测试状态页（弹窗即时测试，不保存）
	g.POST("/test", func(c *gin.Context) {
		var req struct {
			StatusURL string `json:"status_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		snap := a.NginxM.Test(nginxprobe.Instance{StatusURL: req.StatusURL})
		if snap.Up {
			response.OK(c, gin.H{"ok": true, "active_conns": snap.ActiveConns,
				"reading": snap.Reading, "writing": snap.Writing, "waiting": snap.Waiting})
		} else {
			response.OK(c, gin.H{"ok": false, "error": snap.Message})
		}
	})

	// 历史指标（24 小时，5 分钟聚合）
	// 注意：查询 tags 只传 nginx_id，依赖 builtin 引擎对 tags 做 LIKE 子串匹配（写入含 name），
	// 更换其他 tsdb 引擎时需保证同样的子串匹配语义或补全 tags。
	g.GET("/instances/:id/history", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		tags := map[string]string{"nginx_id": strconv.FormatUint(id, 10)}
		q := func(metric string) [][2]float64 {
			s, _ := a.TSDB.Query(tsdb.Query{Metric: metric, Tags: tags, Start: start, End: end, Bucket: 5 * time.Minute, Agg: "avg"})
			if len(s) == 0 {
				return [][2]float64{}
			}
			return s[0].Points
		}
		response.OK(c, gin.H{
			"avail": q("nginx_avail"), "conn": q("nginx_conn"), "qps": q("nginx_qps"),
			"req": q("nginx_req"), "accepts": q("nginx_accepts"), "handled": q("nginx_handled"),
			"reading": q("nginx_reading"), "writing": q("nginx_writing"), "waiting": q("nginx_waiting"),
		})
	})
}

// syncNginx 将启用的实例同步到采集器
func syncNginx(a *core.App) {
	var insts []model.NginxInstance
	a.DB.Find(&insts)
	nx := make([]nginxprobe.Instance, 0, len(insts))
	for _, d := range insts {
		if d.Enable {
			nx = append(nx, nginxprobe.Instance{
				ID: d.ID, Name: d.Name, StatusURL: d.StatusURL, Interval: d.Interval,
			})
		}
	}
	a.NginxM.Sync(nx)
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("nginxmon", RegisterProtected)
}
