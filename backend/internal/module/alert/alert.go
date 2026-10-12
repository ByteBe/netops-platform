// Package alert 告警中心：规则/事件/通知渠道
package alert

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/email"
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
		if level != "" {
			q = q.Where("level = ?", level)
		}
		if ack != "" {
			if ack == "true" {
				q = q.Where("acked = ?", true)
			} else {
				q = q.Where("acked = ?", false)
			}
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
		var req struct {
			Config  string `json:"config"`
			Enabled bool   `json:"enabled"`
		}
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

// Notify 发送通知（供其他模块调用），带告警收敛、同设备同指标抑制和未确认升级
func Notify(app *core.App, level, target, msg string) {
	key := fmt.Sprintf("%s|%s|%s", level, target, msg)
	sum := md5.Sum([]byte(key))
	groupKey := hex.EncodeToString(sum[:])
	var existing model.AlertEvent
	cutoff := time.Now().Add(-5 * time.Minute)
	err := app.DB.Where("group_key = ? AND acked = ? AND created_at > ?", groupKey, false, cutoff).First(&existing).Error
	if err == nil {
		app.DB.Model(&existing).Update("count", gorm.Expr("count + 1"))
		shouldEscalate := existing.Count >= 3 && time.Since(existing.CreatedAt) >= 10*time.Minute && !existing.Escalated
		if shouldEscalate {
			app.DB.Model(&existing).Update("escalated", true)
			sendNotify(app, "escalated", target, "ALARM ESCALATED: "+msg)
		}
		return
	}
	ev := model.AlertEvent{Level: level, Target: target, Message: msg, GroupKey: groupKey, Count: 1, CreatedAt: time.Now()}
	app.DB.Create(&ev)
	sendNotify(app, level, target, msg)
}

func sendNotify(app *core.App, level, target, msg string) {
	client := &http.Client{Timeout: 8 * time.Second}
	var ncs []model.NotifyConfig
	app.DB.Where("enabled = ?", true).Find(&ncs)
	for _, nc := range ncs {
		url := nc.Config
		if url == "" {
			url = nc.Addr
		}
		if nc.Type == "email" {
			var cfgs []model.EmailConfig
			app.DB.Where("enable = ?", true).Find(&cfgs)
			for _, cfg := range cfgs {
				_ = email.Send(email.Config{Name: cfg.Name, SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort, User: cfg.User, Password: cfg.Password, UseSSL: cfg.UseSSL, Enable: true}, []string{nc.Addr}, "["+level+"] NetOps Alert "+target, msg)
			}
			continue
		}
		if url == "" {
			continue
		}
		var out []byte
		if nc.Type == "feishu" {
			out, _ = json.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": "[" + level + "] " + target + "\n" + msg}})
		} else {
			out, _ = json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": "[" + level + "] " + target + "\n" + msg}})
		}
		for i := 0; i < 3; i++ {
			req, _ := http.NewRequest("POST", url, bytes.NewReader(out))
			req.Header.Set("Content-Type", "application/json")
			if nc.Token != "" {
				req.Header.Set("X-Signature", nc.Token)
			}
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				break
			}
			time.Sleep(time.Second)
		}
	}
}

func init() {
	modreg.RegisterProtected("alert", RegisterProtected)
}
