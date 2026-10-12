package topology

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/graph", func(c *gin.Context) {
		var devices []model.TopoDevice
		a.DB.Order("id asc").Find(&devices)
		for i := range devices {
			if devices[i].X == 0 && devices[i].Y == 0 {
				angle := 2 * math.Pi * float64(devices[i].ID%12) / 12
				devices[i].X = 180 * math.Cos(angle)
				devices[i].Y = 110 * math.Sin(angle)
				a.DB.Model(&devices[i]).Updates(map[string]any{"x": devices[i].X, "y": devices[i].Y})
			}
		}
		var links []model.TopoLink
		a.DB.Order("id asc").Find(&links)
		nodes := make([]gin.H, 0, len(devices))
		for _, d := range devices {
			nodes = append(nodes, gin.H{"id": d.ID, "name": d.Name, "ip": d.IP, "vendor": d.Vendor, "type": d.Type, "layer": d.Layer, "status": d.Status, "x": d.X, "y": d.Y, "z": d.Z, "icon": d.Icon, "remark": d.Remark})
		}
		edges := make([]gin.H, 0, len(links))
		for _, l := range links {
			var ips []string
			a.DB.Model(&model.TopoLinkIP{}).Where("link_id=?", l.ID).Pluck("ip", &ips)
			edges = append(edges, gin.H{"id": l.ID, "name": l.Name, "source": l.SourceID, "target": l.TargetID, "color": l.Color, "bandwidth": l.Bandwidth, "utilization": l.Utilization, "status": l.Status, "ips": ips})
		}
		response.OK(c, gin.H{"nodes": nodes, "edges": edges})
	})

	g.GET("/devices", func(c *gin.Context) {
		var list []model.TopoDevice
		a.DB.Order("id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/devices", func(c *gin.Context) {
		var d model.TopoDevice
		if err := c.ShouldBindJSON(&d); err != nil || d.Name == "" {
			response.Bad(c, "参数错误")
			return
		}
		if d.Vendor == "" {
			d.Vendor = "h3c"
		}
		if d.Layer == "" {
			d.Layer = "access"
		}
		if d.Status == "" {
			d.Status = "up"
		}
		if d.X == 0 && d.Y == 0 {
			var cnt int64
			a.DB.Model(&model.TopoDevice{}).Count(&cnt)
			angle := 2 * math.Pi * float64(cnt) / 12
			d.X = 180 * math.Cos(angle)
			d.Y = 110 * math.Sin(angle)
		}
		a.DB.Create(&d)
		response.OK(c, d)
	})
	g.PUT("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.TopoDevice
		if a.DB.First(&d, id).Error != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		var req model.TopoDevice
		c.ShouldBindJSON(&req)
		a.DB.Model(&d).Updates(map[string]any{"name": req.Name, "ip": req.IP, "vendor": req.Vendor, "type": req.Type, "layer": req.Layer, "status": req.Status, "icon": req.Icon, "x": req.X, "y": req.Y, "z": req.Z, "remark": req.Remark})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var lids []uint
		a.DB.Model(&model.TopoLink{}).Where("source_id=? OR target_id=?", id, id).Pluck("id", &lids)
		if len(lids) > 0 {
			a.DB.Where("link_id IN ?", lids).Delete(&model.TopoLinkIP{})
			a.DB.Delete(&model.TopoLink{}, lids)
		}
		a.DB.Delete(&model.TopoDevice{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	g.POST("/links", func(c *gin.Context) {
		var req struct {
			Name        string   `json:"name"`
			SourceID    uint     `json:"source_id"`
			TargetID    uint     `json:"target_id"`
			Color       string   `json:"color"`
			Bandwidth   string   `json:"bandwidth"`
			Utilization float64  `json:"utilization"`
			Status      string   `json:"status"`
			IPs         []string `json:"ips"`
		}
		if c.ShouldBindJSON(&req) != nil || req.SourceID == 0 || req.TargetID == 0 {
			response.Bad(c, "连线两端必填")
			return
		}
		l := model.TopoLink{Name: req.Name, SourceID: req.SourceID, TargetID: req.TargetID, Color: req.Color, Bandwidth: req.Bandwidth, Utilization: req.Utilization, Status: req.Status}
		if l.Color == "" {
			l.Color = "#22c55e"
		}
		if l.Status == "" {
			l.Status = "up"
		}
		a.DB.Create(&l)
		saveIPs(a, l.ID, req.IPs)
		response.OK(c, l)
	})
	g.PUT("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var l model.TopoLink
		if a.DB.First(&l, id).Error != nil {
			response.NotFound(c, "连线不存在")
			return
		}
		var req struct {
			Name        string   `json:"name"`
			SourceID    uint     `json:"source_id"`
			TargetID    uint     `json:"target_id"`
			Color       string   `json:"color"`
			Bandwidth   string   `json:"bandwidth"`
			Utilization float64  `json:"utilization"`
			Status      string   `json:"status"`
			IPs         []string `json:"ips"`
		}
		c.ShouldBindJSON(&req)
		a.DB.Model(&l).Updates(map[string]any{"name": req.Name, "source_id": req.SourceID, "target_id": req.TargetID, "color": req.Color, "bandwidth": req.Bandwidth, "utilization": req.Utilization, "status": req.Status})
		saveIPs(a, l.ID, req.IPs)
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/links/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Where("link_id=?", id).Delete(&model.TopoLinkIP{})
		a.DB.Delete(&model.TopoLink{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	g.POST("/auto-layout", func(c *gin.Context) {
		var devs []model.TopoDevice
		a.DB.Find(&devs)
		n := len(devs)
		if n == 0 {
			response.OK(c, gin.H{"updated": 0})
			return
		}
		for i, d := range devs {
			angle := 2 * math.Pi * float64(i) / float64(n)
			a.DB.Model(&d).Updates(map[string]any{"x": 160 * math.Cos(angle), "y": 100 * math.Sin(angle)})
		}
		response.OK(c, gin.H{"updated": n})
	})

	g.POST("/upload", func(c *gin.Context) {
		f, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "未选择文件")
			return
		}
		ext := filepath.Ext(f.Filename)
		dir := filepath.Join("data", "uploads")
		os.MkdirAll(dir, 0755)
		name := fmt.Sprintf("topo_%d%s", time.Now().UnixNano(), ext)
		c.SaveUploadedFile(f, filepath.Join(dir, name))
		response.OK(c, gin.H{"url": "/uploads/" + name})
	})
}

func saveIPs(a *core.App, linkID uint, ips []string) {
	a.DB.Where("link_id=?", linkID).Delete(&model.TopoLinkIP{})
	for _, ip := range ips {
		if ip != "" {
			a.DB.Create(&model.TopoLinkIP{LinkID: linkID, IP: ip})
		}
	}
}

func init() { modreg.RegisterProtected("topology", RegisterProtected) }
