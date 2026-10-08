// Package ipam IP 地址管理器
// 功能：显示已使用/未使用地址、登记使用人员(姓名/MAC/办公室)、
//
//	通过 SSH 下发绑定命令到指定交换机（可选开关）
package ipam

import (
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
	"netops/internal/service/sshsvc"
)

var arpIPRe = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
var arpMACRe = regexp.MustCompile(`\b([0-9a-fA-F]{4}[-.][0-9a-fA-F]{4}[-.][0-9a-fA-F]{4})\b`)

func regexpMustCompile(s string) *regexp.Regexp { return regexp.MustCompile(s) }

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// ===== 网段 =====
	g.GET("/subnets", func(c *gin.Context) {
		var list []model.Subnet
		a.DB.Order("id asc").Find(&list)
		// 附加使用统计
		result := make([]gin.H, 0, len(list))
		for _, s := range list {
			used, total := subnetStats(a, s)
			result = append(result, gin.H{
				"id": s.ID, "name": s.Name, "cidr": s.CIDR, "gateway": s.Gateway,
				"vlan": s.VLAN, "group_id": s.GroupID, "binding_enabled": s.BindingEnabled,
				"description": s.Description, "used": used, "total": total,
				"created_at": s.CreatedAt,
			})
		}
		response.OK(c, result)
	})

	g.POST("/subnets", func(c *gin.Context) {
		var s model.Subnet
		if err := c.ShouldBindJSON(&s); err != nil {
			response.Bad(c, "参数错误: "+err.Error())
			return
		}
		if s.CIDR == "" {
			response.Bad(c, "CIDR必填")
			return
		}
		if _, _, err := net.ParseCIDR(s.CIDR); err != nil {
			response.Bad(c, "CIDR格式错误")
			return
		}
		var cnt int64
		a.DB.Model(&model.Subnet{}).Where("cidr = ?", s.CIDR).Count(&cnt)
		if cnt > 0 {
			response.Fail(c, 409, response.CodeConflict, "该网段已存在")
			return
		}
		if err := a.DB.Create(&s).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, s)
	})

	g.PUT("/subnets/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var s model.Subnet
		if err := a.DB.First(&s, id).Error; err != nil {
			response.NotFound(c, "网段不存在")
			return
		}
		var req model.Subnet
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&s).Updates(map[string]any{
			"name": req.Name, "gateway": req.Gateway, "vlan": req.VLAN,
			"group_id": req.GroupID, "binding_enabled": req.BindingEnabled, "description": req.Description,
		})
		response.OK(c, gin.H{"ok": true})
	})

	g.DELETE("/subnets/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Where("subnet_id = ?", id).Delete(&model.IPRecord{})
		a.DB.Delete(&model.Subnet{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// ===== IP 列表（显示已使用/未使用，分页） =====
	g.GET("/subnets/:id/ips", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var s model.Subnet
		if err := a.DB.First(&s, id).Error; err != nil {
			response.NotFound(c, "网段不存在")
			return
		}
		status := c.Query("status") // used/unused/all
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "100"))
		kw := c.Query("keyword")

		// 已使用记录
		q := a.DB.Model(&model.IPRecord{}).Where("subnet_id = ?", id)
		if kw != "" {
			q = q.Where("ip LIKE ? OR owner_name LIKE ? OR mac LIKE ? OR office LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		var records []model.IPRecord
		q.Order("ip asc").Find(&records)

		// 网段全部可用IP
		allIPs := usableIPs(s.CIDR)
		usedSet := map[string]bool{}
		recByIP := map[string]model.IPRecord{}
		for _, r := range records {
			usedSet[r.IP] = true
			recByIP[r.IP] = r
		}

		// 过滤
		var items []gin.H
		for _, ip := range allIPs {
			isUsed := usedSet[ip]
			if status == "used" && !isUsed {
				continue
			}
			if status == "unused" && isUsed {
				continue
			}
			if kw != "" && !strings.Contains(ip, kw) && !isUsed {
				continue
			}
			item := gin.H{"ip": ip, "status": "unused", "owner_name": "", "mac": "", "office": "", "record_id": 0}
			if isUsed {
				r := recByIP[ip]
				item = gin.H{
					"ip": ip, "status": "used", "owner_name": r.OwnerName, "mac": r.MAC,
					"office": r.Office, "record_id": r.ID, "bind_device_id": r.BindDeviceID,
					"bind_status": r.BindStatus, "bind_log": r.BindLog, "remark": r.Remark,
				}
			}
			items = append(items, item)
		}
		total := len(items)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}
		response.OK(c, gin.H{"total": total, "list": items[start:end], "subnet": s, "page": page, "size": size})
	})

	// ===== 地址登记 =====
	g.POST("/records", func(c *gin.Context) {
		var req struct {
			SubnetID  uint   `json:"subnet_id"`
			IP        string `json:"ip"`
			OwnerName string `json:"owner_name"`
			MAC       string `json:"mac"`
			Office    string `json:"office"`
			Remark    string `json:"remark"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if req.IP == "" || req.OwnerName == "" {
			response.Bad(c, "IP与使用人员必填")
			return
		}
		var s model.Subnet
		if err := a.DB.First(&s, req.SubnetID).Error; err != nil {
			response.NotFound(c, "网段不存在")
			return
		}
		var cnt int64
		a.DB.Model(&model.IPRecord{}).Where("subnet_id = ? AND ip = ?", req.SubnetID, req.IP).Count(&cnt)
		if cnt > 0 {
			response.Fail(c, 409, response.CodeConflict, "该IP已登记")
			return
		}
		rec := model.IPRecord{SubnetID: req.SubnetID, IP: req.IP, OwnerName: req.OwnerName,
			MAC: strings.ToUpper(req.MAC), Office: req.Office, Remark: req.Remark}
		if err := a.DB.Create(&rec).Error; err != nil {
			response.Err(c, err)
			return
		}
		// 若开启绑定功能且网段启用绑定，自动下发
		if s.BindingEnabled && setting(a, "ipam_binding_enable") == "true" && rec.MAC != "" {
			doBind(a, &rec, &s)
		}
		response.OK(c, rec)
	})

	// 更新登记
	g.PUT("/records/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.IPRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			response.NotFound(c, "登记记录不存在")
			return
		}
		var req struct {
			OwnerName string `json:"owner_name"`
			MAC       string `json:"mac"`
			Office    string `json:"office"`
			Remark    string `json:"remark"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&rec).Updates(map[string]any{
			"owner_name": req.OwnerName, "mac": strings.ToUpper(req.MAC),
			"office": req.Office, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})

	// 删除登记（释放IP）
	g.DELETE("/records/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.IPRecord{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// ===== 下发绑定命令（指定设备） =====
	g.POST("/records/:id/bind", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var rec model.IPRecord
		if err := a.DB.First(&rec, id).Error; err != nil {
			response.NotFound(c, "登记记录不存在")
			return
		}
		var req struct {
			BindDeviceID uint `json:"bind_device_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		var s model.Subnet
		a.DB.First(&s, rec.SubnetID)
		rec.BindDeviceID = req.BindDeviceID
		doBind(a, &rec, &s)
		response.OK(c, gin.H{"ok": true, "bind_status": rec.BindStatus, "bind_log": rec.BindLog})
	})

	// ===== 绑定设备管理（SSH 登录，区别于 SNMP 监控） =====
	g.GET("/bind-devices", func(c *gin.Context) {
		var list []model.BindDevice
		a.DB.Order("id asc").Find(&list)
		response.OK(c, list)
	})
	g.POST("/bind-devices", func(c *gin.Context) {
		var d model.BindDevice
		if err := c.ShouldBindJSON(&d); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if d.Name == "" || d.IP == "" {
			response.Bad(c, "名称与IP必填")
			return
		}
		if err := a.DB.Create(&d).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, d)
	})
	g.PUT("/bind-devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.BindDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		var req model.BindDevice
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.DB.Model(&d).Updates(map[string]any{
			"name": req.Name, "ip": req.IP, "ssh_user": req.SSHUser, "ssh_port": req.SSHPort,
			"auth_type": req.AuthType, "credential": req.Credential, "vendor": req.Vendor,
			"enable": req.Enable, "remark": req.Remark,
		})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/bind-devices/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.BindDevice{}, id)
		response.OK(c, gin.H{"ok": true})
	})
	// 测试SSH连接
	g.POST("/bind-devices/:id/test", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var d model.BindDevice
		if err := a.DB.First(&d, id).Error; err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		out, err := sshsvc.Test(sshsvc.Target{Host: d.IP, Port: d.SSHPort, User: d.SSHUser,
			AuthType: d.AuthType, Credential: d.Credential})
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "SSH测试失败: "+err.Error()+" | "+out)
			return
		}
		response.OK(c, gin.H{"ok": true, "output": out})
	})

	// ===== 从交换机读取ARP表并导入IP记录 =====
	g.POST("/subnets/:id/import-arp", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var s model.Subnet
		if err := a.DB.First(&s, id).Error; err != nil {
			response.NotFound(c, "网段不存在")
			return
		}
		var req struct {
			BindDeviceID uint `json:"bind_device_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.BindDeviceID == 0 {
			response.Bad(c, "请选择交换机")
			return
		}
		var d model.BindDevice
		if err := a.DB.First(&d, req.BindDeviceID).Error; err != nil {
			response.NotFound(c, "交换机不存在")
			return
		}
		cmd := "display arp"
		if d.Vendor == "h3c" {
			cmd = "display arp all"
		}
		out, err := sshsvc.Run(sshsvc.Target{Host: d.IP, Port: d.SSHPort, User: d.SSHUser,
			AuthType: d.AuthType, Credential: d.Credential}, cmd)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "读取ARP失败: "+err.Error())
			return
		}
		arpMap := parseARP(out)
		// 网段可用IP
		allIPs := usableIPs(s.CIDR)
		ipSet := map[string]bool{}
		for _, ip := range allIPs {
			ipSet[ip] = true
		}
		// 批量取出已有记录，避免逐行查询
		existing := map[string]string{}
		var oldRecs []model.IPRecord
		a.DB.Where("subnet_id = ?", id).Find(&oldRecs)
		for _, r := range oldRecs {
			existing[r.IP] = r.MAC
		}
		imported, skipped := 0, 0
		var toCreate []model.IPRecord
		for ip, mac := range arpMap {
			if !ipSet[ip] {
				continue
			}
			if _, ok := existing[ip]; ok {
				a.DB.Model(&model.IPRecord{}).Where("subnet_id = ? AND ip = ?", id, ip).
					Update("mac", mac)
				skipped++
				continue
			}
			toCreate = append(toCreate, model.IPRecord{SubnetID: uint(id), IP: ip, OwnerName: "ARP自动发现",
				MAC: mac, Office: "", Remark: "从交换机" + d.Name + "ARP表自动读取"})
		}
		if len(toCreate) > 0 {
			a.DB.CreateInBatches(toCreate, 100)
			imported = len(toCreate)
		}
		response.OK(c, gin.H{"ok": true, "imported": imported, "updated": skipped, "arp_count": len(arpMap), "raw": out})
	})

	// 全局绑定开关
	g.GET("/settings", func(c *gin.Context) {
		response.OK(c, gin.H{"ipam_binding_enable": setting(a, "ipam_binding_enable") == "true"})
	})
	g.PUT("/settings", func(c *gin.Context) {
		var req struct {
			BindingEnable bool `json:"ipam_binding_enable"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		set(a, "ipam_binding_enable", strconv.FormatBool(req.BindingEnable))
		response.OK(c, gin.H{"ok": true})
	})
}

// doBind 下发地址绑定命令（华为/华三：arp static + 静态MAC绑定）
func doBind(a *core.App, rec *model.IPRecord, s *model.Subnet) {
	var dev model.BindDevice
	if err := a.DB.First(&dev, rec.BindDeviceID).Error; err != nil {
		rec.BindStatus = "fail"
		rec.BindLog = "绑定设备不存在或未配置"
		a.DB.Model(rec).Updates(map[string]any{"bind_status": rec.BindStatus, "bind_log": rec.BindLog})
		return
	}
	if !dev.Enable {
		rec.BindStatus = "fail"
		rec.BindLog = "绑定设备未启用"
		a.DB.Model(rec).Updates(map[string]any{"bind_status": rec.BindStatus, "bind_log": rec.BindLog})
		return
	}
	mac := normalizeMAC(rec.MAC)
	if mac == "" {
		rec.BindStatus = "fail"
		rec.BindLog = "MAC地址无效"
		a.DB.Model(rec).Updates(map[string]any{"bind_status": rec.BindStatus, "bind_log": rec.BindLog})
		return
	}
	// 生成绑定命令（华为/华三兼容）
	commands := []string{
		"system-view",
		fmt.Sprintf("arp static %s %s", rec.IP, mac),
		"return",
	}
	out, err := sshsvc.Exec(sshsvc.Target{Host: dev.IP, Port: dev.SSHPort, User: dev.SSHUser,
		AuthType: dev.AuthType, Credential: dev.Credential}, commands)
	if err != nil {
		rec.BindStatus = "fail"
		rec.BindLog = err.Error()
	} else {
		rec.BindStatus = "ok"
		rec.BindLog = out
	}
	a.DB.Model(rec).Updates(map[string]any{"bind_status": rec.BindStatus, "bind_log": rec.BindLog})
}

func normalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.ReplaceAll(mac, "-", ""))
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, ".", "")
	if len(mac) != 12 {
		return ""
	}
	// 华为/华三格式：XXXX-XXXX-XXXX
	return mac[0:4] + "-" + mac[4:8] + "-" + mac[8:12]
}

// parseARP 解析华为/华三 display arp 输出，返回 map[ip]mac
func parseARP(out string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		ip := arpIPRe.FindString(line)
		mac := arpMACRe.FindString(line)
		if ip == "" || mac == "" {
			continue
		}
		if strings.Contains(line, "0000-0000-0000") {
			continue
		}
		result[ip] = normalizeMAC(mac)
	}
	return result
}

func usableIPs(cidr string) []string {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	ones, bits := ipnet.Mask.Size()
	if bits-ones < 2 {
		return nil // /31 无可用主机
	}
	if bits-ones > 20 {
		// 超大网段（>100万）不做全量枚举，按前1024个展示
		limit := 1024
		start := ip.Mask(ipnet.Mask)
		start = nextIP(start)
		var out []string
		for i := 0; i < limit; i++ {
			if !ipnet.Contains(start) {
				break
			}
			out = append(out, start.String())
			start = nextIP(start)
		}
		return out
	}
	var out []string
	start := ip.Mask(ipnet.Mask)
	start = nextIP(start)
	end := lastIP(ipnet)
	for ipnet.Contains(start) && !start.Equal(end) {
		out = append(out, start.String())
		start = nextIP(start)
	}
	sort.Strings(out)
	return out
}

func nextIP(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	for i := len(out) - 1; i >= 0; i-- {
		out[i]++
		if out[i] != 0 {
			break
		}
	}
	return out
}

func lastIP(ipnet *net.IPNet) net.IP {
	mask := ipnet.Mask
	last := make(net.IP, len(ipnet.IP))
	copy(last, ipnet.IP)
	for i := 0; i < len(mask); i++ {
		last[i] |= ^mask[i]
	}
	return last
}

func subnetStats(a *core.App, s model.Subnet) (used, total int) {
	var cnt int64
	a.DB.Model(&model.IPRecord{}).Where("subnet_id = ?", s.ID).Count(&cnt)
	_, ipnet, err := net.ParseCIDR(s.CIDR)
	if err != nil {
		return int(cnt), 0
	}
	ones, bits := ipnet.Mask.Size()
	hosts := 1 << (bits - ones)
	if bits-ones <= 1 {
		hosts = 0
	} else {
		hosts -= 2
	}
	return int(cnt), hosts
}

func setting(a *core.App, k string) string {
	var s model.SystemSetting
	if err := a.DB.Where("`key` = ?", k).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func set(a *core.App, k, v string) {
	var s model.SystemSetting
	if err := a.DB.Where("`key` = ?", k).First(&s).Error; err != nil {
		a.DB.Create(&model.SystemSetting{Key: k, Value: v})
	} else {
		a.DB.Model(&s).Update("value", v)
	}
}

var _ = binary.BigEndian

// init 自动注册路由
func init() {
	modreg.RegisterProtected("ipam", RegisterProtected)
}
