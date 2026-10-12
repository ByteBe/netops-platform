// Package configaudit 配置只读审计：SSH 读取运行配置并做差异对比，不下发任何配置
package configaudit

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"strings"

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
		if a.DB.Where("device_ip = ?", d.IP).Order("id desc").First(&prev).Error == nil && prev.Hash != hash {
			diff = diffLines(prev.Content, content)
		}
		rec := model.ConfigBackup{DeviceIP: d.IP, DeviceName: d.Name, Vendor: d.Vendor, Content: content, Hash: hash, Source: "auto"}
		a.DB.Create(&rec)
		response.OK(c, gin.H{"id": rec.ID, "hash": hash, "diff": diff})
	})
}

func diffLines(old, new string) string {
	oldSet := map[string]bool{}
	for _, line := range strings.Split(old, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			oldSet[line] = true
		}
	}
	added := []string{}
	for _, line := range strings.Split(new, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !oldSet[line] {
			added = append(added, line)
		}
	}
	if len(added) == 0 {
		return "no change"
	}
	return strings.Join(added[:min(20, len(added))], "\n")
}

func init() { modreg.RegisterProtected("configaudit", RegisterProtected) }
