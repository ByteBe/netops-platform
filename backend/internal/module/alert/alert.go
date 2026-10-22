// Package alert 告警中心：规则/事件/通知渠道
package alert

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/email"
	"netops/internal/service/webhook"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 规则列表
	g.GET("/rules", func(c *gin.Context) {
		var list []model.AlertRule
		a.DB.Order("id desc").Find(&list)
		response.OK(c, list)
	})
	// 新建规则
	g.POST("/rules", middleware.RequireRole("admin"), func(c *gin.Context) {
		var r model.AlertRule
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Create(&r)
		response.OK(c, r)
	})
	// 更新规则
	g.PUT("/rules/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
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
	g.DELETE("/rules/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.AlertRule{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 事件列表
	g.GET("/events", func(c *gin.Context) {
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
	g.POST("/events/:id/ack", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		now := time.Now()
		a.DB.Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]any{"acked": true, "acked_at": now})
		response.OK(c, gin.H{"ok": true})
	})

	// 通知渠道 CRUD
	g.GET("/notify", func(c *gin.Context) {
		var list []model.NotifyConfig
		a.DB.Order("id desc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/notify", middleware.RequireRole("admin"), func(c *gin.Context) {
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
		if req.Name == "" || req.Addr == "" {
			response.Bad(c, "名称与地址必填")
			return
		}
		nc := model.NotifyConfig{Name: req.Name, Type: req.Type, Addr: req.Addr, Token: req.Token, Enabled: req.Enabled}
		if err := a.DB.Create(&nc).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, nc)
	})
	g.PUT("/notify/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
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
		var nc model.NotifyConfig
		if err := a.DB.First(&nc, c.Param("id")).Error; err != nil {
			response.NotFound(c, "通知渠道不存在")
			return
		}
		if req.Token == "" {
			req.Token = nc.Token
		}
		a.DB.Model(&nc).Updates(map[string]any{
			"name": req.Name, "type": req.Type, "addr": req.Addr,
			"token": req.Token, "enabled": req.Enabled,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/notify/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
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
		// 飞书/钉钉/企业微信 Webhook：由 webhook 包统一处理消息格式、平台加签与送达校验
		if err := webhook.SendText(nc.Type, url, nc.Token, "["+level+"] "+target+"\n"+msg); err != nil {
			logger.Warnf("[notify] 渠道 %s(%s) 发送失败: %v", nc.Name, nc.Type, err)
		}
	}
}

func init() {
	modreg.RegisterProtected("alert", RegisterProtected)
}

// StartEvaluator 启动阈值告警评估 goroutine（CPU/内存/链路/节点）
func StartEvaluator(app *core.App) {
	overSince := map[string]time.Time{}
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if app.DB == nil {
				continue
			}
			// CPU/内存/磁盘阈值告警
			var rules []model.AlertRule
			app.DB.Where("enabled = ? AND metric IN ?", true, []string{"cpu", "mem", "disk"}).Find(&rules)
			if len(rules) > 0 && app.SnmpMgr != nil {
				var devices []model.MonitorDevice
				app.DB.Where("enable = ?", true).Find(&devices)
				seen := map[string]bool{}
				for _, d := range devices {
					snap, ok := app.SnmpMgr.DeviceSnapshotByID(d.ID)
					if !ok || !snap.Up {
						continue
					}
					for _, r := range rules {
						if r.Metric != "cpu" && r.Metric != "mem" {
							continue
						}
						val := snap.CPU
						if r.Metric == "mem" {
							val = snap.MemUsed
						}
						if val <= 0 {
							continue
						}
						key := fmt.Sprintf("%d|%s", d.ID, r.Metric)
						seen[key] = true
						if val >= r.Threshold {
							if _, exists := overSince[key]; !exists {
								overSince[key] = time.Now()
							}
							if time.Since(overSince[key]) >= time.Duration(r.Duration)*time.Minute {
								Notify(app, r.Level, d.Name, fmt.Sprintf("%s %.1f%% 超过阈值 %.1f%%", r.Metric, val, r.Threshold))
								delete(overSince, key)
							}
						} else {
							delete(overSince, key)
						}
					}
					// 磁盘阈值告警（按挂载点独立计时）
					for _, r := range rules {
						if r.Metric != "disk" {
							continue
						}
						for _, disk := range snap.Disks {
							if disk.UsedPct <= 0 {
								continue
							}
							key := fmt.Sprintf("disk-%d|%s", d.ID, disk.Mount)
							seen[key] = true
							if disk.UsedPct >= r.Threshold {
								if _, exists := overSince[key]; !exists {
									overSince[key] = time.Now()
								}
								if time.Since(overSince[key]) >= time.Duration(r.Duration)*time.Minute {
									Notify(app, r.Level, d.Name, fmt.Sprintf("%s %s 磁盘使用率 %.1f%% 超过阈值 %.1f%%", d.Name, disk.Mount, disk.UsedPct, r.Threshold))
									delete(overSince, key)
								}
							} else {
								delete(overSince, key)
							}
						}
					}
				}
				for k := range overSince {
					if !seen[k] {
						delete(overSince, k)
					}
				}
			}
			// 节点离线告警
			var nodeRules []model.AlertRule
			app.DB.Where("enabled = ? AND metric = ?", true, "node").Find(&nodeRules)
			if len(nodeRules) > 0 {
				var nodes []model.Node
				app.DB.Find(&nodes)
				nodeSeen := map[uint]bool{}
				for _, n := range nodes {
					nodeSeen[n.ID] = true
					key := fmt.Sprintf("node-%d", n.ID)
					if n.Status != "online" {
						if _, exists := overSince[key]; !exists {
							overSince[key] = time.Now()
						}
						if time.Since(overSince[key]) >= time.Duration(nodeRules[0].Duration)*time.Minute {
							Notify(app, nodeRules[0].Level, n.Name, "节点离线")
							delete(overSince, key)
						}
					} else {
						delete(overSince, key)
					}
				}
				// 清理已删除节点的陈旧 key
				for k := range overSince {
					var nid uint
					if strings.HasPrefix(k, "node-") {
						fmt.Sscanf(k, "node-%d", &nid)
						if !nodeSeen[nid] {
							delete(overSince, k)
						}
					}
				}
			}
			// 链路异常告警（ping 探活结果为 down 且持续超过 duration 分钟）
			var linkRules []model.AlertRule
			app.DB.Where("enabled = ? AND metric = ?", true, "link").Find(&linkRules)
			if len(linkRules) > 0 {
				var links []model.LinkTask
				app.DB.Find(&links)
				linkSeen := map[uint]bool{}
				for _, l := range links {
					linkSeen[l.ID] = true
					key := fmt.Sprintf("link-%d", l.ID)
					if l.Status == "down" {
						if _, exists := overSince[key]; !exists {
							overSince[key] = time.Now()
						}
						if time.Since(overSince[key]) >= time.Duration(linkRules[0].Duration)*time.Minute {
							Notify(app, linkRules[0].Level, l.Name, fmt.Sprintf("链路异常: %s %s 不可达", l.Method, l.Target))
							delete(overSince, key)
						}
					} else {
						delete(overSince, key)
					}
				}
				for k := range overSince {
					var lid uint
					if strings.HasPrefix(k, "link-") {
						fmt.Sscanf(k, "link-%d", &lid)
						if !linkSeen[lid] {
							delete(overSince, k)
						}
					}
				}
			}
		}
	}()
}
