// Package configaudit 配置只读审计：SSH 读取运行配置并做差异对比，不下发任何配置。
// 差异对比由 Python 智能外围（netops-python /internal/configaudit/diff）唯一实现，Go 不做本地兜底。
package configaudit

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/sshsvc"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/conf-audit/backups", func(c *gin.Context) {
		var list []model.ConfigBackup
		a.DB.Order("id desc").Limit(200).Find(&list)
		response.OK(c, list)
	})

	g.POST("/conf-audit/devices/:id/run", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.MonitorDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "device not found")
			return
		}
		cmd := "display current-configuration"
		if d.Vendor == "cisco" {
			cmd = "show running-config"
		}
		content, err := sshsvc.Run(sshsvc.Target{Host: d.IP, Port: d.SshPort, User: d.SshUser, Credential: d.SshPass}, cmd)
		if err != nil {
			response.Fail(c, 500, response.CodeServerError, err.Error())
			return
		}
		var prev model.ConfigBackup
		hashSum := md5.Sum([]byte(content))
		hash := hex.EncodeToString(hashSum[:])
		diff := "no change"
		changed := false
		if a.DB.Where("device_ip = ?", d.IP).Order("id desc").First(&prev).Error == nil && prev.Hash != hash {
			// 差异对比仅走 Python 智能外围；不可用时明确报错（不本地兜底）
			var diffRes struct {
				Changed bool     `json:"changed"`
				Diff    string   `json:"diff"`
				Added   []string `json:"added"`
				Removed []string `json:"removed"`
			}
			if err := a.PySvc.PostJSON("/internal/configaudit/diff",
				map[string]any{"old_content": prev.Content, "new_content": content}, &diffRes, 30*time.Second); err != nil {
				response.Fail(c, 502, 50200, "配置审计服务不可用，请检查 Python 智能外围（netops-python）服务状态")
				return
			}
			changed = diffRes.Changed
			diff = diffRes.Diff
			if diff == "" {
				diff = "no change"
			}
		}
		rec := model.ConfigBackup{DeviceIP: d.IP, DeviceName: d.Name, Vendor: d.Vendor, Content: content, Hash: hash, Source: "auto"}
		a.DB.Create(&rec)
		response.OK(c, gin.H{"id": rec.ID, "hash": hash, "diff": diff, "changed": changed})
	})
}

func init() { modreg.RegisterProtected("configaudit", RegisterProtected) }
