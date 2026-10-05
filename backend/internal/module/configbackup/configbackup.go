// Package configbackup 网络设备配置备份/对比/回滚
package configbackup

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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
	// 备份列表
	g.GET("/config-backups", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		ip := c.Query("device_ip")
		q := a.DB.Model(&model.ConfigBackup{})
		if ip != "" {
			q = q.Where("device_ip = ?", ip)
		}
		// 多租户数据范围
		if ns := middleware.ScopeNodeID(c); ns != "" {
			q = q.Where("node_id = ?", ns)
		}
		var total int64
		q.Count(&total)
		var list []model.ConfigBackup
		q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
		response.OK(c, gin.H{"total": total, "list": list})
	})

	// 手动触发备份（模拟：记录当前时间戳作为配置内容占位，实际应通过SSH/Telnet抓取）
	g.POST("/config-backups", func(c *gin.Context) {
		var req struct {
			DeviceIP   string `json:"device_ip"`
			DeviceName string `json:"device_name"`
			Vendor     string `json:"vendor"`
			Content    string `json:"content"` // 直接传入配置文本
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.DeviceIP == "" {
			response.Bad(c, "device_ip 必填")
			return
		}
		// 最新版本号
		var latest model.ConfigBackup
		a.DB.Where("device_ip = ?", req.DeviceIP).Order("version desc").First(&latest)
		hash := md5.Sum([]byte(req.Content))
		b := model.ConfigBackup{
			NodeID:     middleware.ScopeNodeID(c),
			DeviceIP:   req.DeviceIP,
			DeviceName: req.DeviceName,
			Vendor:     req.Vendor,
			Version:    latest.Version + 1,
			Content:    req.Content,
			Hash:       hex.EncodeToString(hash[:]),
			Source:     "manual",
			CreatedAt:  time.Now(),
		}
		a.DB.Create(&b)
		response.OK(c, b)
	})

	// 查看备份内容
	g.GET("/config-backups/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var b model.ConfigBackup
		if err := a.DB.First(&b, id).Error; err != nil {
			response.NotFound(c, "备份不存在")
			return
		}
		response.OK(c, b)
	})

	// 对比两个版本
	g.GET("/config-backups/diff", func(c *gin.Context) {
		id1, _ := strconv.Atoi(c.Query("id1"))
		id2, _ := strconv.Atoi(c.Query("id2"))
		var a1, a2 model.ConfigBackup
		if err := a.DB.First(&a1, id1).Error; err != nil {
			response.NotFound(c, "版本1不存在")
			return
		}
		if err := a.DB.First(&a2, id2).Error; err != nil {
			response.NotFound(c, "版本2不存在")
			return
		}
		response.OK(c, gin.H{"left": a1, "right": a2, "same": a1.Hash == a2.Hash})
	})

	// 回滚（标记当前应使用版本，实际回滚由管理员手动执行）
	g.POST("/config-backups/:id/rollback", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var b model.ConfigBackup
		if err := a.DB.First(&b, id).Error; err != nil {
			response.NotFound(c, "备份不存在")
			return
		}
		response.OK(c, gin.H{"message": "回滚包已生成，请手动下发设备", "device_ip": b.DeviceIP, "content": b.Content})
	})

	// 删除备份
	g.DELETE("/config-backups/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.ConfigBackup{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 定时备份配置
	g.GET("/configbackup/schedule", func(c *gin.Context) {
		var s model.SystemSetting
		if err := a.DB.Where("key = ?", "config_backup_schedule").First(&s).Error; err != nil {
			response.OK(c, map[string]any{"enabled": false, "cron": "0 2 * * *", "interval_hours": 24, "devices": "all", "keep": 30})
			return
		}
		response.OK(c, s.Value)
	})
	g.POST("/configbackup/schedule", func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		b, _ := json.Marshal(body)
		s := model.SystemSetting{Key: "config_backup_schedule", Value: string(b)}
		a.DB.Where("key = ?", s.Key).Assign(s).FirstOrCreate(&s)
		response.OK(c, nil)
	})
}
	modreg.RegisterProtected("configbackup", RegisterProtected)
}
