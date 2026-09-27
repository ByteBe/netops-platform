// Package linkdetect 链路检测模块
// 功能：多地址长时间并行监控、实时监控数据、历史数据查询、每条链路独立曲线颜色、支持修改
package linkdetect

import (
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/collector/ping"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/tsdb"
)

// Palette 曲线配色（每条检测链路分配一种颜色用于区分）
var Palette = []string{
	"#409EFF", "#67C23A", "#E6A23C", "#F56C6C", "#909399",
	"#9C27B0", "#00BCD4", "#FF9800", "#795548", "#607D8B",
	"#E91E63", "#3F51B5", "#009688", "#FF5722", "#8BC34A",
}

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 任务列表
	g.GET("/tasks", func(c *gin.Context) {
		var list []model.LinkTask
		q := a.DB.Model(&model.LinkTask{})
		if gid := c.Query("group_id"); gid != "" {
			q = q.Where("group_id = ?", gid)
		}
		q.Order("id asc").Find(&list)
		// 附加实时状态
		result := make([]gin.H, 0, len(list))
		for _, t := range list {
			item := gin.H{
				"id": t.ID, "name": t.Name, "target": t.Target, "method": t.Method,
				"port": t.Port, "interval": t.Interval, "timeout": t.Timeout,
				"group_id": t.GroupID, "color": t.Color, "enabled": t.Enabled,
				"created_at": t.CreatedAt,
			}
			if s, ok := a.PingMgr.Get(t.ID); ok {
				item["status"] = s.Up
				item["rt"] = s.RT
				item["loss"] = s.Loss
				item["message"] = s.Message
				item["last_ts"] = s.TS
			} else {
				item["status"] = false
				item["rt"] = 0
				item["loss"] = 0
				item["message"] = "未启用"
				item["last_ts"] = 0
			}
			result = append(result, item)
		}
		response.OK(c, result)
	})

	// 创建任务
	g.POST("/tasks", func(c *gin.Context) {
		var t model.LinkTask
		if err := c.ShouldBindJSON(&t); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if t.Name == "" || t.Target == "" {
			response.Bad(c, "名称与目标地址必填")
			return
		}
		if t.Interval == 0 {
			t.Interval = 10
		}
		if t.Timeout == 0 {
			t.Timeout = 5
		}
		if t.Method == "" {
			t.Method = "cmd"
		}
		if t.Color == "" {
			t.Color = nextColor(a)
		}
		if err := a.DB.Create(&t).Error; err != nil {
			response.Err(c, err)
			return
		}
		if t.Enabled {
			a.PingMgr.StartTask(t.ID, t.Name, t.Target, t.Method, t.Port, t.Interval, t.Timeout, t.Color)
		}
		response.OK(c, t)
	})

	// 修改任务
	g.PUT("/tasks/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var t model.LinkTask
		if err := a.DB.First(&t, id).Error; err != nil {
			response.NotFound(c, "任务不存在")
			return
		}
		var req model.LinkTask
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		updates := map[string]any{
			"name": req.Name, "target": req.Target, "method": req.Method,
			"port": req.Port, "interval": req.Interval, "timeout": req.Timeout,
			"group_id": req.GroupID, "color": req.Color, "enabled": req.Enabled,
		}
		a.DB.Model(&t).Updates(updates)
		// 重启任务
		a.PingMgr.StopTask(t.ID)
		if req.Enabled {
			a.PingMgr.StartTask(t.ID, req.Name, req.Target, req.Method, req.Port, req.Interval, req.Timeout, req.Color)
		}
		response.OK(c, gin.H{"ok": true})
	})

	// 删除任务
	g.DELETE("/tasks/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.PingMgr.StopTask(uint(id))
		a.DB.Delete(&model.LinkTask{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 启停
	g.POST("/tasks/:id/toggle", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var t model.LinkTask
		if err := a.DB.First(&t, id).Error; err != nil {
			response.NotFound(c, "任务不存在")
			return
		}
		t.Enabled = !t.Enabled
		a.DB.Model(&t).Update("enabled", t.Enabled)
		if t.Enabled {
			a.PingMgr.StartTask(t.ID, t.Name, t.Target, t.Method, t.Port, t.Interval, t.Timeout, t.Color)
		} else {
			a.PingMgr.StopTask(t.ID)
		}
		response.OK(c, gin.H{"ok": true, "enabled": t.Enabled})
	})

	// 实时监控数据（全量快照）
	g.GET("/status", func(c *gin.Context) {
		response.OK(c, a.PingMgr.Snapshot())
	})

	// 历史数据查询（时序库聚合）
	g.GET("/tasks/:id/history", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		start, end := parseRange(c)
		bucket := parseBucket(c, start, end)
		var t model.LinkTask
		if err := a.DB.First(&t, id).Error; err != nil {
			response.NotFound(c, "任务不存在")
			return
		}
		tags := map[string]string{"task_id": strconv.FormatUint(id, 10)}
		rt, _ := a.TSDB.Query(tsdb.Query{Metric: "link_rt", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
		loss, _ := a.TSDB.Query(tsdb.Query{Metric: "link_loss", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "avg"})
		status, _ := a.TSDB.Query(tsdb.Query{Metric: "link_status", Tags: tags, Start: start, End: end, Bucket: bucket, Agg: "last"})
		response.OK(c, gin.H{
			"task": t, "rt": firstPoints(rt), "loss": firstPoints(loss), "status": firstPoints(status),
		})
	})

	// 实时曲线（最近N个点）
	g.GET("/tasks/:id/realtime", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		end := time.Now()
		start := end.Add(-time.Duration(30) * time.Minute)
		tags := map[string]string{"task_id": strconv.FormatUint(id, 10)}
		rt, _ := a.TSDB.Query(tsdb.Query{Metric: "link_rt", Tags: tags, Start: start, End: end, Agg: "avg"})
		status, _ := a.TSDB.Query(tsdb.Query{Metric: "link_status", Tags: tags, Start: start, End: end, Agg: "last"})
		response.OK(c, gin.H{"rt": firstPoints(rt), "status": firstPoints(status)})
	})

	// 调色板
	g.GET("/colors", func(c *gin.Context) {
		response.OK(c, Palette)
	})
}

// 为新建链路分配调色板中未使用的颜色
func nextColor(a *core.App) string {
	var used []string
	a.DB.Model(&model.LinkTask{}).Where("color != ''").Pluck("color", &used)
	usedSet := map[string]bool{}
	for _, u := range used {
		usedSet[u] = true
	}
	for _, c := range Palette {
		if !usedSet[c] {
			return c
		}
	}
	return Palette[time.Now().Unix()%int64(len(Palette))]
}

func parseRange(c *gin.Context) (time.Time, time.Time) {
	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if s := c.Query("start"); s != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
			start = t
		} else if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			start = time.UnixMilli(ms)
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", e, time.Local); err == nil {
			end = t
		} else if ms, err := strconv.ParseInt(e, 10, 64); err == nil {
			end = time.UnixMilli(ms)
		}
	}
	if !end.After(start) {
		end = start.Add(time.Hour)
	}
	return start, end
}

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

var _ = ping.TaskConfig{}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("linkdetect", RegisterProtected)
}
