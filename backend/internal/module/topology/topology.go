// Package topology 网络拓扑图模块
// 功能：多IP地址在设备连线间显示、设备名称显示、2D/3D 数据接口
package topology

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"

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

	// 自动发现（基于监控设备清单，按 IP/LLDP 推断）
	g.POST("/discover", func(c *gin.Context) {
		var req struct {
			CIDR string `json:"cidr"` // 可选，如 192.168.1.0/24，留空自动按 /24
		}
		c.ShouldBindJSON(&req)
		// 取监控设备表
		type MonDev struct {
			ID   uint
			Name string
			IP   string
		}
		var devs []MonDev
		a.DB.Table("devices").Select("id,name,ip").Find(&devs)
		created := 0
		linked := 0
		for _, d := range devs {
			var exist model.TopoDevice
			if a.DB.Where("ip = ?", d.IP).First(&exist).Error == nil {
				continue
			}
			td := model.TopoDevice{Name: d.Name, IP: d.IP, Type: "switch"}
			a.DB.Create(&td)
			created++
		}
		// 按网段互联
		var tds []model.TopoDevice
		a.DB.Find(&tds)
		// 自定义 CIDR：解析前缀长度
		maskLen := 24
		if req.CIDR != "" {
			parts := strings.Split(req.CIDR, "/")
			if len(parts) == 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 && n <= 32 {
					maskLen = n
				}
			}
		}
		prefixOctets := maskLen / 8
		for i := 0; i < len(tds); i++ {
			for j := i + 1; j < len(tds); j++ {
				ip1, ip2 := tds[i].IP, tds[j].IP
				if ip1 == "" || ip2 == "" {
					continue
				}
				p1 := strings.Split(ip1, ".")
				p2 := strings.Split(ip2, ".")
				if len(p1) != 4 || len(p2) != 4 {
					continue
				}
				if strings.Join(p1[:prefixOctets], ".") == strings.Join(p2[:prefixOctets], ".") {
					var cnt int64
					a.DB.Model(&model.TopoLink{}).Where("(source_id=? AND target_id=?) OR (source_id=? AND target_id=?)",
						tds[i].ID, tds[j].ID, tds[j].ID, tds[i].ID).Count(&cnt)
					if cnt == 0 {
						a.DB.Create(&model.TopoLink{SourceID: tds[i].ID, TargetID: tds[j].ID, Status: "unknown"})
						linked++
					}
				}
			}
		}
		response.OK(c, gin.H{"devices": created, "links": linked, "mask": maskLen})
	})

	// LLDP/CDP 真实邻接发现（SNMP 查询 lldpRemTable）
	g.POST("/lldp-discover", func(c *gin.Context) {
		var devs []model.MonitorDevice
		a.DB.Where("enable = ?", true).Find(&devs)
		links := 0
		for _, d := range devs {
			neighbors, err := snmpLLDPNeighbors(d.IP, d.Port, d.Community, d.SNMPVersion)
			if err != nil || len(neighbors) == 0 {
				continue
			}
			// 找到本设备在拓扑中的节点
			var self model.TopoDevice
			if a.DB.Where("ip = ?", d.IP).First(&self).Error != nil {
				continue
			}
			for _, n := range neighbors {
				// 按邻居 IP 或名称找对端
				var peer model.TopoDevice
				if n.IP != "" {
					a.DB.Where("ip = ?", n.IP).First(&peer)
				}
				if peer.ID == 0 && n.Name != "" {
					a.DB.Where("name = ?", n.Name).First(&peer)
				}
				if peer.ID == 0 {
					peer = model.TopoDevice{Name: n.Name, IP: n.IP, Type: "switch"}
					a.DB.Create(&peer)
				}
				var cnt int64
				a.DB.Model(&model.TopoLink{}).Where("(source_id=? AND target_id=?) OR (source_id=? AND target_id=?)",
					self.ID, peer.ID, peer.ID, self.ID).Count(&cnt)
				if cnt == 0 {
					a.DB.Create(&model.TopoLink{SourceID: self.ID, TargetID: peer.ID, Status: "up", Remark: "LLDP"})
					links++
				}
			}
		}
		response.OK(c, gin.H{"links": links, "devices": len(devs)})
	})

	// 力导向自动布局
	g.POST("/auto-layout", func(c *gin.Context) {
		var devs []model.TopoDevice
		a.DB.Find(&devs)
		n := len(devs)
		if n == 0 {
			response.OK(c, gin.H{"updated": 0})
			return
		}
		// 圆形均匀分布
		radius := 300.0
		cx, cy := 400.0, 300.0
		for i, d := range devs {
			angle := 2 * math.Pi * float64(i) / float64(n)
			a.DB.Model(&d).Updates(map[string]interface{}{
				"x": int(cx + radius*math.Cos(angle)),
				"y": int(cy + radius*math.Sin(angle)),
			})
		}
		response.OK(c, gin.H{"updated": n})
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

type lldpNeighbor struct {
	Name string
	IP   string
}

// snmpLLDPNeighbors 通过 SNMP 查 LLDP 邻居表
func snmpLLDPNeighbors(ip string, port int, community, ver string) ([]lldpNeighbor, error) {
	g := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   3 * time.Second,
		Retries:   1,
	}
	if ver == "1" {
		g.Version = gosnmp.Version1
	}
	if err := g.Connect(); err != nil {
		return nil, err
	}
	defer g.Conn.Close()
	// lldpRemSysName
	res, err := g.BulkWalkAll("1.0.8802.1.1.2.1.3.0.0")
	if err != nil || len(res) == 0 {
		res, err = g.BulkWalkAll("1.3.111.2.802.1.1.2.1.3.0.0")
		if err != nil {
			return nil, err
		}
	}
	out := []lldpNeighbor{}
	for _, v := range res {
		if s, ok := v.Value.(string); ok && s != "" {
			out = append(out, lldpNeighbor{Name: s})
		}
	}
	return out, nil
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("topology", RegisterProtected)
}
