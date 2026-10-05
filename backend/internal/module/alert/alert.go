// Package alert 告警中心：规则/事件/通知渠道
package alert

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 规则列表
	g.GET("/alert/rules", func(c *gin.Context) {
		var list []model.AlertRule
		a.DB.Order("id desc").Find(&list)
		response.OK(c, list)
	})
	// 新建规则
	g.POST("/alert/rules", middleware.RequireRole("admin"), func(c *gin.Context) {
		var r model.AlertRule
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Create(&r)
		response.OK(c, r)
	})
	// 更新规则
	g.PUT("/alert/rules/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var r model.AlertRule
		if err := a.DB.First(&r, id).Error; err != nil {
			response.NotFound(c, "规则不存在")
			return
		}
		c.ShouldBindJSON(&r)
		a.DB.Save(&r)
		response.OK(c, r)
	})
	// 删除规则
	g.DELETE("/alert/rules/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.AlertRule{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 事件列表
	g.GET("/alert/events", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		level := c.Query("level")
		ack := c.Query("acked")
		q := a.DB.Model(&model.AlertEvent{})
		if level != "" { q = q.Where("level = ?", level) }
		if ack != "" {
			if ack == "true" { q = q.Where("acked = ?", true) } else { q = q.Where("acked = ?", false) }
		}
		var total int64
		q.Count(&total)
		var list []model.AlertEvent
		q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
		response.OK(c, gin.H{"total": total, "list": list})
	})
	// 确认告警
	g.POST("/alert/events/:id/ack", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		now := time.Now()
		a.DB.Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]any{"acked": true, "acked_at": now})
		response.OK(c, gin.H{"ok": true})
	})

	// 通知渠道配置
	g.GET("/alert/notifiers", func(c *gin.Context) {
		var list []model.NotifyConfig
		a.DB.Find(&list)
		response.OK(c, list)
	})
	g.PUT("/alert/notifiers/:type", middleware.RequireRole("admin"), func(c *gin.Context) {
		t := c.Param("type")
		var req struct { Config string `json:"config"`; Enabled bool `json:"enabled"` }
		c.ShouldBindJSON(&req)
		var nc model.NotifyConfig
		if err := a.DB.Where("type = ?", t).First(&nc).Error; err != nil {
			nc = model.NotifyConfig{Type: t, Config: req.Config, Enabled: req.Enabled}
			a.DB.Create(&nc)
		} else {
			a.DB.Model(&nc).Updates(map[string]any{"config": req.Config, "enabled": req.Enabled})
		}
		response.OK(c, nc)
	})

	// 通知渠道 CRUD
	g.GET("/alert/notify", func(c *gin.Context) {
		var list []model.NotifyConfig
		a.DB.Order("id desc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/alert/notify", middleware.RequireRole("admin"), func(c *gin.Context) {
		var req struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Addr    string `json:"addr"`
			Token   string `json:"token"`
			Enabled bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		nc := model.NotifyConfig{Type: req.Type, Config: req.Addr, Enabled: req.Enabled}
		nc.Name = req.Name
		nc.Token = req.Token
		a.DB.Create(&nc)
		response.OK(c, nc)
	})
	g.DELETE("/alert/notify/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		a.DB.Delete(&model.NotifyConfig{}, c.Param("id"))
		response.OK(c, nil)
	})
}

// Notify 发送通知（供其他模块调用）
func Notify(app *core.App, level, target, msg string) {
	// 记录事件
	ev := model.AlertEvent{Level: level, Target: target, Message: msg, CreatedAt: time.Now()}
	app.DB.Create(&ev)
	// 查渠道
	var ncs []model.NotifyConfig
	app.DB.Where("enabled = ?", true).Find(&ncs)
	for _, nc := range ncs {
		switch nc.Type {
		case "webhook":
			payload, _ := json.Marshal(map[string]string{"level": level, "target": target, "message": msg})
			http.Post(nc.Config, "application/json", bytes.NewReader(payload))
		case "dingtalk":
			payload, _ := json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": "["+level+"] "+target+"\n"+msg}})
			http.Post(nc.Config, "application/json", bytes.NewReader(payload))
		}
	}
}

func init() {
	modreg.RegisterProtected("alert", RegisterProtected)
}
