// Package topology 网络拓扑图模块
// 功能：多IP地址在设备连线间显示、设备名称显示、2D/3D 数据接口
package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 拓扑图数据（节点+连线，含连线IP列表）
	g.GET("/graph", func(c *gin.Context) {
		var devices []model.TopoDevice
		a.DB.Order("id asc").Find(&devices)
		var links []model.TopoLink
		a.DB.Order("id asc").Find(&links)

		nodes := make([]gin.H, 0, len(devices))
		for _, d := range devices {
			nodes = append(nodes, gin.H{
				"id": d.ID, "name": d.Name, "ip": d.IP, "type": d.Type,
				"x": d.X, "y": d.Y, "z": d.Z, "group_id": d.GroupID, "icon": d.Icon,
			})
		}
		edges := make([]gin.H, 0, len(links))
		for _, l := range links {
			var ips []string
			a.DB.Model(&model.TopoLinkIP{}).Where("link_id = ?", l.ID).Pluck("ip", &ips)
			edges = append(edges, gin.H{
				"id": l.ID, "name": l.Name, "source": l.SourceID, "target": l.TargetID,
				"color": l.Color, "bandwidth": l.Bandwidth, "status": l.Status,
				"ips": ips, // 多IP显示在连线上
			})
		}
		response.OK(c, gin.H{"nodes": nodes, "edges": edges})
	})

	// 设备 CRUD
	g.GET("/devices", func(c *gin.Context) {
		var list []model.TopoDevice
		a.DB.Order("id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/devices", func(c *gin.Context) {
		var d model.TopoDevice
		if err := c.ShouldBindJSON(&d); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if d.Name == "" {
			response.Bad(c, "设备名称必填")
			return
		}
		if err := a.DB.Create(&d).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, d)
	})
	g.PUT("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.TopoDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		var req model.TopoDevice
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&d).Updates(map[string]any{
			"name": req.Name, "ip": req.IP, "type": req.Type, "icon": req.Icon,
			"x": req.X, "y": req.Y, "z": req.Z, "group_id": req.GroupID, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		// 级联删除相关连线与IP
		var linkIDs []uint
		a.DB.Model(&model.TopoLink{}).Where("source_id = ? OR target_id = ?", id, id).Pluck("id", &linkIDs)
		if len(linkIDs) > 0 {
			a.DB.Where("link_id IN ?", linkIDs).Delete(&model.TopoLinkIP{})
			a.DB.Delete(&model.TopoLink{}, linkIDs)
		}
		a.DB.Delete(&model.TopoDevice{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 连线 CRUD（支持一条线关联多个IP）
	g.POST("/links", func(c *gin.Context) {
		var req struct {
			Name      string   `json:"name"`
			SourceID  uint     `json:"source_id"`
			TargetID  uint     `json:"target_id"`
			Color     string   `json:"color"`
			Bandwidth string   `json:"bandwidth"`
			Status    string   `json:"status"`
			Remark    string   `json:"remark"`
			IPs       []string `json:"ips"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if req.SourceID == 0 || req.TargetID == 0 {
			response.Bad(c, "连线两端设备必填")
			return
		}
		l := model.TopoLink{Name: req.Name, SourceID: req.SourceID, TargetID: req.TargetID,
			Color: req.Color, Bandwidth: req.Bandwidth, Status: req.Status, Remark: req.Remark}
		if l.Color == "" {
			l.Color = "#67C23A"
		}
		if l.Status == "" {
			l.Status = "up"
		}
		if err := a.DB.Create(&l).Error; err != nil {
			response.Err(c, err)
			return
		}
		saveLinkIPs(a, l.ID, req.IPs)
		response.OK(c, l)
	})
	g.PUT("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var l model.TopoLink
		if err := a.DB.First(&l, id).Error; err != nil {
			response.NotFound(c, "连线不存在")
			return
		}
		var req struct {
			Name      string   `json:"name"`
			SourceID  uint     `json:"source_id"`
			TargetID  uint     `json:"target_id"`
			Color     string   `json:"color"`
			Bandwidth string   `json:"bandwidth"`
			Status    string   `json:"status"`
			Remark    string   `json:"remark"`
			IPs       []string `json:"ips"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&l).Updates(map[string]any{
			"name": req.Name, "source_id": req.SourceID, "target_id": req.TargetID,
			"color": req.Color, "bandwidth": req.Bandwidth, "status": req.Status, "remark": req.Remark,
		})
		saveLinkIPs(a, l.ID, req.IPs)
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Where("link_id = ?", id).Delete(&model.TopoLinkIP{})
		a.DB.Delete(&model.TopoLink{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 上传设备图标（jpg/png/svg/vsdx）
	g.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "未选择文件")
			return
		}
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".svg": true, ".vsdx": true, ".gif": true}
		if !allowed[ext] {
			response.Bad(c, "仅支持 jpg/png/svg/vsdx/gif 格式")
			return
		}
		if file.Size > 5*1024*1024 {
			response.Bad(c, "文件不能超过 5MB")
			return
		}
		uploadDir := filepath.Join("data", "uploads")
		os.MkdirAll(uploadDir, 0755)
		filename := fmt.Sprintf("icon_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			response.Err(c, err)
			return
		}
		url := "/uploads/" + filename
		response.OK(c, gin.H{"url": url})
	})
}

func saveLinkIPs(a *core.App, linkID uint, ips []string) {
	a.DB.Where("link_id = ?", linkID).Delete(&model.TopoLinkIP{})
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		a.DB.Create(&model.TopoLinkIP{LinkID: linkID, IP: ip})
	}
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("topology", RegisterProtected)
}
