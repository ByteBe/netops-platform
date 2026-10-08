// Package scriptgen 脚本生成器（全新设计：纯代码生成，无数据库模板）
package scriptgen

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/modreg"
)

func init() {
	modreg.RegisterProtected("scriptgen", RegisterProtected)
}

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/categories", listCategories)
	g.POST("/generate", generate)
}

// Field 表单字段
type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder"`
	Options     []string `json:"options"`
	ShowIf      *ShowIf  `json:"showIf,omitempty"`
}

type ShowIf struct {
	Key   string `json:"key"`
	Eq    string `json:"eq"`
	NotEq string   `json:"notEq,omitempty"`
	EqAny []string `json:"eqAny,omitempty"`
}

type Category struct {
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Icon   string  `json:"icon"`
	Desc   string  `json:"desc"`
	Fields []Field `json:"fields"`
}

type DeviceGroup struct {
	Type  string     `json:"type"`
	Label string     `json:"label"`
	Items []Category `json:"items"`
}

var groups = map[string][]Category{}

func init() {
	groups["router"] = []Category{
		{Code: "router_acl", Name: "ACL访问控制", Icon: "🔒", Desc: "配置允许/拒绝规则", Fields: []Field{
			{Key: "acltype", Label: "ACL类型", Type: "select", Required: true, Options: []string{"基础", "二层", "三层", "自定义"}},
			{Key: "num", Label: "ACL编号", Type: "number", Required: true, Placeholder: "如3000"},
			{Key: "action", Label: "动作", Type: "select", Required: true, Options: []string{"permit", "deny"}},
			{Key: "proto", Label: "协议", Type: "select", Required: true, Options: []string{"ip", "tcp", "udp", "icmp"}, ShowIf: &ShowIf{Key: "acltype", EqAny: []string{"三层", "自定义"}}},
			{Key: "src", Label: "源地址", Type: "network", Required: true, Placeholder: "如192.168.1.0/24", ShowIf: &ShowIf{Key: "acltype", NotEq: "二层"}},
			{Key: "dst", Label: "目的地址", Type: "network", Required: false, Placeholder: "如any", ShowIf: &ShowIf{Key: "acltype", EqAny: []string{"三层", "自定义"}}},
			{Key: "dport", Label: "目的端口", Type: "number", Required: false, Placeholder: "如80", ShowIf: &ShowIf{Key: "acltype", EqAny: []string{"三层", "自定义"}}},
			{Key: "srcmac", Label: "源MAC地址", Type: "text", Required: true, Placeholder: "如0000-1111-2222", ShowIf: &ShowIf{Key: "acltype", Eq: "二层"}},
		}},
		{Code: "router_nat", Name: "NAT地址转换", Icon: "🌐", Desc: "内网共享上网", Fields: []Field{
			{Key: "outif", Label: "出接口", Type: "text", Required: true, Placeholder: "如GigabitEthernet0/0/1"},
			{Key: "acl", Label: "ACL编号", Type: "number", Required: true, Placeholder: "如2000"},
		}},
		{Code: "router_ospf", Name: "OSPF动态路由", Icon: "🔄", Desc: "内部网关协议", Fields: []Field{
			{Key: "process", Label: "进程号", Type: "number", Required: true, Placeholder: "如1"},
			{Key: "rid", Label: "Router ID", Type: "ip", Required: true, Placeholder: "如1.1.1.1"},
			{Key: "net", Label: "宣告网段", Type: "text", Required: true, Placeholder: "如10.16.194.0"},
			{Key: "netmask", Label: "掩码", Type: "text", Required: true, Placeholder: "如255.255.255.0"},
			{Key: "area", Label: "区域号", Type: "text", Required: true, Placeholder: "如0"},
			{Key: "enable_vpn", Label: "启用MPLS VPN", Type: "switch", Required: false},
			{Key: "vpn", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna", ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
		}},
		{Code: "router_static", Name: "静态路由", Icon: "➡️", Desc: "手动指定路由", Fields: []Field{
			{Key: "dest", Label: "目标网段", Type: "text", Required: true, Placeholder: "如0.0.0.0"},
			{Key: "mask", Label: "掩码", Type: "text", Required: true, Placeholder: "如0"},
			{Key: "nh", Label: "下一跳", Type: "ip", Required: true, Placeholder: "如192.168.1.1"},
			{Key: "enable_vpn", Label: "启用MPLS VPN", Type: "switch", Required: false},
			{Key: "vpn", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna", ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
		}},
		{Code: "router_bgp", Name: "BGP路由", Icon: "🌍", Desc: "跨域路由", Fields: []Field{
			{Key: "asn", Label: "本地AS号", Type: "number", Required: true, Placeholder: "如65001"},
			{Key: "rid", Label: "Router ID", Type: "ip", Required: true, Placeholder: "如1.1.1.1"},
			{Key: "peer_as", Label: "邻居AS号", Type: "text", Required: true, Placeholder: "如65002（多个用逗号分隔）"},
			{Key: "peer_ip", Label: "邻居IP(多个用逗号分隔)", Type: "text", Required: true, Placeholder: "如1.1.1.1,2.2.2.2"},
			{Key: "conn_int", Label: "更新源接口", Type: "text", Required: false, Placeholder: "如LoopBack0"},
			{Key: "enable_net", Label: "启用发布路由", Type: "switch", Required: false},
			{Key: "network", Label: "发布网段(IPv4)", Type: "text", Required: true, Placeholder: "如10.0.0.0", ShowIf: &ShowIf{Key: "enable_net", Eq: "on"}},
			{Key: "netmask", Label: "掩码", Type: "text", Required: true, Placeholder: "如255.255.255.0", ShowIf: &ShowIf{Key: "enable_net", Eq: "on"}},
			{Key: "vpn4", Label: "启用VPNv4地址族", Type: "switch", Required: false},
			{Key: "vpn4_enable_peer", Label: "VPNv4中启用邻居(逗号分隔)", Type: "text", Required: false, Placeholder: "如1.1.1.1,8.8.8.8", ShowIf: &ShowIf{Key: "vpn4", Eq: "on"}},
			{Key: "vpn4_adv_comm", Label: "VPNv4发送团体属性邻居(逗号分隔)", Type: "text", Required: false, Placeholder: "如1.1.1.1", ShowIf: &ShowIf{Key: "vpn4", Eq: "on"}},
			{Key: "vpn4_next_hop_inv", Label: "VPNv4下一跳不变(逗号分隔邻居)", Type: "text", Required: false, Placeholder: "如8.8.8.8", ShowIf: &ShowIf{Key: "vpn4", Eq: "on"}},
			{Key: "enable_vpn", Label: "绑定MPLS VPN实例", Type: "switch", Required: false},
			{Key: "vpn", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna", ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
			{Key: "enable_import", Label: "启用引入路由", Type: "switch", Required: false, ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
			{Key: "vpn_import", Label: "引入路由协议(每行一条,如ospf 2)", Type: "text", Required: false, Placeholder: "ospf 2\ndirect", ShowIf: &ShowIf{Key: "enable_import", Eq: "on"}},
			{Key: "enable_export", Label: "启用发布路由", Type: "switch", Required: false, ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
			{Key: "vpn_export", Label: "发布路由到(每行一条,如ospf 2)", Type: "text", Required: false, Placeholder: "ospf 2\ndirect", ShowIf: &ShowIf{Key: "enable_export", Eq: "on"}},
		}},
		{Code: "router_isis", Name: "IS-IS路由", Icon: "🛰️", Desc: "链路状态协议", Fields: []Field{
			{Key: "net", Label: "NET地址", Type: "text", Required: true, Placeholder: "如49.0001.0000.0000.0001.00"},
			{Key: "pid", Label: "进程号", Type: "number", Required: false, Placeholder: "如1（默认1）"},
			{Key: "rid", Label: "Router ID", Type: "text", Required: false, Placeholder: "如1.1.1.1"},
			{Key: "level", Label: "级别", Type: "select", Required: true, Options: []string{"level-1", "level-2", "level-1-2"}},
			{Key: "enable_vpn", Label: "启用MPLS VPN", Type: "switch", Required: false},
			{Key: "vpn", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna", ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
		}},
		{Code: "router_policy", Name: "策略路由", Icon: "🎯", Desc: "灵活路由", Fields: []Field{
			{Key: "acl", Label: "ACL编号", Type: "number", Required: true, Placeholder: "如3000"},
			{Key: "cname", Label: "流分类名称", Type: "text", Required: false, Placeholder: "如c1"},
			{Key: "bname", Label: "流行为名称", Type: "text", Required: false, Placeholder: "如b1"},
			{Key: "pname", Label: "流策略名称", Type: "text", Required: false, Placeholder: "如p1"},
			{Key: "iface", Label: "应用接口", Type: "text", Required: true, Placeholder: "如GigabitEthernet0/0/0"},
			{Key: "nh", Label: "指定下一跳", Type: "ip", Required: true},
		}},
		{Code: "route_policy", Name: "路由策略", Icon: "📜", Desc: "路由过滤/属性修改", Fields: []Field{
			{Key: "name", Label: "路由策略名", Type: "text", Required: true, Placeholder: "如RP1"},
			{Key: "seq", Label: "节点序号", Type: "number", Required: false, Placeholder: "如10"},
			{Key: "enable_prefix", Label: "启用前缀列表", Type: "switch", Required: false},
			{Key: "prefix", Label: "前缀列表名", Type: "text", Required: true, Placeholder: "如TO-beijing", ShowIf: &ShowIf{Key: "enable_prefix", Eq: "on"}},
			{Key: "pfx_index", Label: "规则序号", Type: "number", Required: false, Placeholder: "如10", ShowIf: &ShowIf{Key: "enable_prefix", Eq: "on"}},
			{Key: "pfx_action", Label: "动作", Type: "select", Required: false, Options: []string{"permit", "deny"}, ShowIf: &ShowIf{Key: "enable_prefix", Eq: "on"}},
			{Key: "pfx_net", Label: "匹配网段", Type: "text", Required: false, Placeholder: "如10.16.192.0", ShowIf: &ShowIf{Key: "enable_prefix", Eq: "on"}},
			{Key: "pfx_masklen", Label: "掩码长度", Type: "number", Required: false, Placeholder: "如24", ShowIf: &ShowIf{Key: "enable_prefix", Eq: "on"}},
		}},
		{Code: "mpls_vpn", Name: "MPLS VPN", Icon: "🏷️", Desc: "MPLS/MPLS-VPN", Fields: []Field{
			{Key: "rd", Label: "RD", Type: "text", Required: true, Placeholder: "如100:1"},
			{Key: "rt_in", Label: "RT导入", Type: "text", Required: true, Placeholder: "如100:1"},
			{Key: "rt_out", Label: "RT导出", Type: "text", Required: true, Placeholder: "如100:1"},
			{Key: "vpn_name", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna"},
			{Key: "iface", Label: "绑定接口", Type: "text", Required: true, Placeholder: "如GigabitEthernet0/0/0"},
			{Key: "ip", Label: "接口IP", Type: "text", Required: true, Placeholder: "如10.0.0.1 255.255.255.0"},
		}},
	}
	groups["switch"] = []Category{
		{Code: "sw_vlan", Name: "VLAN配置", Icon: "🏷️", Desc: "创建VLAN并配置网关", Fields: []Field{
			{Key: "vlan", Label: "VLAN号", Type: "text", Required: true, Placeholder: "如10,20,30-40"},
			{Key: "vlanif", Label: "网关VLAN号", Type: "number", Required: false, Placeholder: "如10"},
			{Key: "ip", Label: "网关IP/掩码", Type: "network", Required: false, Placeholder: "如192.168.1.1/24"},
		}},
		{Code: "sw_access", Name: "接入口Access", Icon: "🔌", Desc: "终端接入端口", Fields: []Field{
			{Key: "iface", Label: "接口", Type: "text", Required: true, Placeholder: "如GE0/0/1,GE0/0/2"},
			{Key: "vlan", Label: "PVID", Type: "number", Required: true, Placeholder: "如10"},
		}},
		{Code: "sw_trunk", Name: "Trunk口", Icon: "🔀", Desc: "级联端口", Fields: []Field{
			{Key: "iface", Label: "接口", Type: "text", Required: true, Placeholder: "如GE0/0/1,GE0/0/2"},
			{Key: "vlans", Label: "允许VLAN", Type: "text", Required: true, Placeholder: "如10,20"},
		}},
		{Code: "sw_dhcp", Name: "DHCP地址池", Icon: "📦", Desc: "自动分配IP", Fields: []Field{
			{Key: "pool", Label: "地址池名", Type: "text", Required: true, Placeholder: "如pool1"},
			{Key: "net", Label: "网段", Type: "network", Required: true, Placeholder: "如192.168.1.0/24"},
			{Key: "gw", Label: "网关", Type: "ip", Required: true},
			{Key: "dns", Label: "DNS", Type: "text", Required: false, Placeholder: "如114.114.114.114"},
		}},
		{Code: "sw_stp", Name: "STP生成树", Icon: "🌳", Desc: "防止环路", Fields: []Field{
			{Key: "mode", Label: "模式", Type: "select", Required: true, Options: []string{"mstp", "stp", "rstp"}},
			{Key: "instance", Label: "MSTI实例号", Type: "number", Required: false, Placeholder: "如1", ShowIf: &ShowIf{Key: "mode", Eq: "mstp"}},
			{Key: "vlan", Label: "VLAN映射", Type: "text", Required: false, Placeholder: "如10,20", ShowIf: &ShowIf{Key: "mode", Eq: "mstp"}},
			{Key: "root", Label: "根桥角色", Type: "select", Required: true, Options: []string{"none", "primary", "secondary"}},
			{Key: "pri", Label: "优先级", Type: "number", Required: false, Placeholder: "如4096"},
		}},
	}
	groups["common"] = []Category{
		{Code: "com_ssh", Name: "SSH管理", Icon: "🔑", Desc: "远程登录配置", Fields: []Field{
			{Key: "user", Label: "用户名", Type: "text", Required: true, Placeholder: "如admin"},
			{Key: "pass", Label: "密码", Type: "password", Required: true},
			{Key: "pri", Label: "权限级别", Type: "select", Required: true, Options: []string{"15", "3", "1"}},
			{Key: "rsabits", Label: "RSA密钥长度", Type: "select", Required: false, Options: []string{"1024", "2048", "4096"}},
			{Key: "vty", Label: "VTY线路范围", Type: "text", Required: false, Placeholder: "如0 4"},
		}},
		{Code: "com_user", Name: "本地用户", Icon: "👤", Desc: "新建管理账号", Fields: []Field{
			{Key: "user", Label: "用户名", Type: "text", Required: true, Placeholder: "如admin"},
			{Key: "pass", Label: "密码", Type: "password", Required: true},
			{Key: "pri", Label: "权限级别", Type: "select", Required: true, Options: []string{"15", "3", "1"}},
		}},
		{Code: "com_log", Name: "日志服务器", Icon: "📋", Desc: "Syslog上报", Fields: []Field{
			{Key: "logip", Label: "日志服务器IP", Type: "ip", Required: true},
		}},
		{Code: "com_snmp", Name: "SNMP配置", Icon: "📡", Desc: "网络管理(v2c/v3)", Fields: []Field{
			{Key: "ver", Label: "SNMP版本", Type: "select", Required: true, Options: []string{"v1", "v2c", "v3"}},
			{Key: "community", Label: "团体字(v1/v2c)", Type: "text", Required: false, Placeholder: "如public"},
			{Key: "community", Label: "团体字(v1/v2c)", Type: "text", Required: false, Placeholder: "如public", ShowIf: &ShowIf{Key: "ver", NotEq: "v3"}},
			{Key: "secname", Label: "用户名(v3)", Type: "text", Required: false, Placeholder: "如monitor", ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
			{Key: "authpass", Label: "认证密码(v3)", Type: "password", Required: false, ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
			{Key: "privpass", Label: "加密密码(v3)", Type: "password", Required: false, ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
		}},
	}
}

func listCategories(c *gin.Context) {
	var all []Category
	all = append(all, groups["common"]...)
	all = append(all, groups["router"]...)
	all = append(all, groups["switch"]...)
	response.OK(c, []DeviceGroup{{Type: "all", Label: "", Items: all}})
}

type genReq struct {
	Code   string            `json:"code"`
	Vendor string            `json:"vendor"`
	Params map[string]string `json:"params"`
}

func generate(c *gin.Context) {
	var req genReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, 0, "参数错误")
		return
	}
	lines := build(req.Code, req.Params, req.Vendor)
	response.OK(c, gin.H{"script": strings.Join(lines, "\n")})
}

func sysView(v string) string {
	switch v {
	case "cisco":
		return "configure terminal"
	default:
		return "system-view"
	}
}

func p(m map[string]string, k, def string) string {
	if v, ok := m[k]; ok && v != "" { return v }
	return def
}

func vlanIfName(vendor, vlan string) string {
	switch vendor {
	case "cisco":
		return "Vlan" + vlan
	case "h3c":
		return "Vlan-interface" + vlan
	default:
		return "Vlanif" + vlan
	}
}

func cidrMask(cidr string) (addr, mask string) {
	parts := strings.Split(strings.TrimSpace(cidr), "/")
	addr = parts[0]
	prefix := 24
	if len(parts) == 2 { fmt.Sscanf(parts[1], "%d", &prefix) }
	mp := []string{}
	for i := 0; i < 4; i++ {
		if prefix >= 8 { mp = append(mp, "255"); prefix -= 8 } else { mp = append(mp, fmt.Sprintf("%d", 256-(1<<(8-prefix)))); prefix = 0 }
	}
	mask = strings.Join(mp, ".")
	return
}

func wildOf(cidr string) string {
	parts := strings.Split(strings.TrimSpace(cidr), "/")
	prefix := 24
	if len(parts) == 2 { fmt.Sscanf(parts[1], "%d", &prefix) }
	w := uint32(0xFFFFFFFF >> prefix)
	return fmt.Sprintf("%d.%d.%d.%d", w>>24, (w>>16)&0xFF, (w>>8)&0xFF, w&0xFF)
}

func wildcardOf(mask string) string {
	var a, b, c, d uint32
	fmt.Sscanf(mask, "%d.%d.%d.%d", &a, &b, &c, &d)
	w := ^(a<<24 | b<<16 | c<<8 | d)
	return fmt.Sprintf("%d.%d.%d.%d", w>>24, (w>>16)&0xFF, (w>>8)&0xFF, w&0xFF)
}

func build(code string, m map[string]string, vendor string) []string {
	sys := sysView(vendor)
	exit := "quit"
	if vendor == "cisco" { exit = "end" }
	_, _ = vendor, exit

	switch code {
	case "router_acl", "sw_acl":
		act := p(m, "action", "permit")
		proto := p(m, "proto", "ip")
		num := p(m, "num", "3000")
		acltype := p(m, "acltype", "三层")
		src := p(m, "src", "0.0.0.0/0")
		srcAddr := strings.Split(src, "/")[0]
		srcWild := wildOf(src)
		if vendor == "cisco" {
			if acltype == "基础" {
				return []string{sys + " //全局配置", fmt.Sprintf("access-list %s permit %s %s //标准ACL", num, srcAddr, srcWild), "exit"}
			}
			if acltype == "二层" {
				srcMac := p(m, "srcmac", "0000.0000.0000")
				return []string{sys + " //全局配置", fmt.Sprintf("mac access-list extended %s //二层ACL", num), fmt.Sprintf(" permit host %s any", srcMac), "exit"}
			}
			rule := fmt.Sprintf("%s %s %s %s", act, proto, srcAddr, srcWild)
			if dst := p(m, "dst", ""); dst != "" {
				rule += fmt.Sprintf(" %s %s", strings.Split(dst, "/")[0], wildOf(dst))
			} else { rule += " any" }
			if port := p(m, "dport", ""); port != "" && (proto == "tcp" || proto == "udp") {
				rule += fmt.Sprintf(" eq %s", port)
			}
			return []string{sys + " //全局配置", fmt.Sprintf("ip access-list extended %s //创建扩展ACL", num), " " + rule, "exit"}
		}
		// 华为/华三
		if acltype == "二层" {
			srcMac := p(m, "srcmac", "0000-0000-0000")
			return []string{sys + " //系统视图", fmt.Sprintf("acl number 4%s //二层ACL", num), fmt.Sprintf(" rule 5 %s source-mac %s ffff-ffff-ffff", act, srcMac), "quit"}
		}
		rule := fmt.Sprintf(" rule 5 %s %s source %s %s", act, proto, srcAddr, srcWild)
		if dst := p(m, "dst", ""); dst != "" {
			rule += fmt.Sprintf(" destination %s %s", strings.Split(dst, "/")[0], wildOf(dst))
		}
		if port := p(m, "dport", ""); port != "" && (proto == "tcp" || proto == "udp") {
			rule += fmt.Sprintf(" destination-port eq %s", port)
		}
		return []string{sys + " //系统视图", fmt.Sprintf("acl number %s //创建%sACL", num, map[string]string{"三层":"三层","自定义":"自定义"}[acltype]), rule, "quit"}
	case "router_nat":
		outif := p(m, "outif", "GigabitEthernet0/0/1")
		acl := p(m, "acl", "2000")
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("access-list 1 permit %s //定义内网", "192.168.1.0 0.0.0.255"),
				fmt.Sprintf("interface %s //进入出接口", outif),
				fmt.Sprintf(" ip nat inside //NAT内侧", outif),
				fmt.Sprintf(" ip nat inside source list 1 interface %s overload //PAT", outif),
				"exit",
			}
		}
		return []string{
			fmt.Sprintf("interface %s //进入出接口", outif),
			fmt.Sprintf(" nat outbound %s //Easy-IP", acl),
			
		}

	case "router_ospf":
		net := p(m, "net", "0.0.0.0")
		mask := p(m, "netmask", "255.255.255.0")
		vpn := p(m, "vpn", "")
		if vendor == "cisco" {
			wild := wildcardOf(mask)
			vpnLine := ""
			if vpn != "" { vpnLine = fmt.Sprintf(" address-family ipv4 vrf %s", vpn) }
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("router ospf %s //创建OSPF", p(m, "process", "1")),
				fmt.Sprintf(" router-id %s //Router ID", p(m, "rid", "1.1.1.1")),
				vpnLine,
				fmt.Sprintf(" network %s %s area %s //宣告网段", net, wild, p(m, "area", "0")),
			}
		}
		vpnLine := ""
		if vpn != "" { vpnLine = fmt.Sprintf(" vpn-instance %s", vpn) }
		if vendor == "h3c" {
			return []string{
				fmt.Sprintf("ospf%s %s router-id %s //创建OSPF", vpnLine, p(m, "process", "1"), p(m, "rid", "1.1.1.1")),
				fmt.Sprintf(" area 0.0.0.%s //进入区域", p(m, "area", "0")),
				fmt.Sprintf("  network %s %s //宣告网段", net, mask),
			}
		}
		return []string{
			fmt.Sprintf("ospf%s %s router-id %s //创建OSPF", vpnLine, p(m, "process", "1"), p(m, "rid", "1.1.1.1")),
			fmt.Sprintf(" area %s //进入区域", p(m, "area", "0")),
			fmt.Sprintf("  network %s %s //宣告网段", net, mask),
		}

	case "router_static", "sw_static":
		dest := p(m, "dest", "0.0.0.0")
		mask := p(m, "mask", "0")
		vpn := p(m, "vpn", "")
		vpnPre := ""
		if vpn != "" { vpnPre = fmt.Sprintf(" vpn-instance %s", vpn) }
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("ip route%s %s %s %s //静态路由", vpnPre, dest, mask, p(m, "nh", "")),
			}
		}
		return []string{
			fmt.Sprintf("ip route-static%s %s %s %s //静态路由", vpnPre, dest, mask, p(m, "nh", "")),
		}

	case "router_vrrp":
		if vendor == "cisco" {
			iface := p(m, "iface", "Vlan10")
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("interface %s //进入接口", iface),
				fmt.Sprintf(" vrrp %s ip %s //VRRP", p(m, "vrid", "1"), p(m, "vip", "")),
				"exit",
			}
		}
		return []string{
			fmt.Sprintf("interface %s //进入接口", p(m, "iface", "Vlanif10")),
			fmt.Sprintf(" vrrp vrid %s virtual-ip %s //VRRP", p(m, "vrid", "1"), p(m, "vip", "")),
			
		}

	case "com_ssh":
		user := p(m, "user", "admin")
		pass := p(m, "pass", "")
		pri := p(m, "pri", "15")
		bits := p(m, "rsabits", "2048")
		vtyRange := p(m, "vty", "0 4")
		lines := []string{sys}
		if vendor == "cisco" {
			lines = append(lines, fmt.Sprintf("username %s privilege %s secret %s //创建用户", user, pri, pass))
			lines = append(lines, "ip domain-name local //域名")
			lines = append(lines, fmt.Sprintf("crypto key generate rsa modulus %s //生成RSA密钥", bits))
			lines = append(lines, fmt.Sprintf("line vty %s //VTY线路", vtyRange))
			lines = append(lines, " transport input ssh //仅允许SSH")
			lines = append(lines, " login local //本地认证")
		} else if vendor == "h3c" {
			lines = append(lines, fmt.Sprintf("public-key local create rsa modulus %s //生成RSA密钥", bits))
			lines = append(lines, "ssh server enable //开启SSH服务端")
			lines = append(lines, fmt.Sprintf("local-user %s class manage //创建用户", user))
			lines = append(lines, fmt.Sprintf(" password simple %s //密码", pass))
			lines = append(lines, fmt.Sprintf(" authorization-attribute level %s //权限级别%s", pri, pri))
			lines = append(lines, " service-type ssh //SSH登录")
			
			lines = append(lines, fmt.Sprintf("line vty %s //VTY线路", vtyRange))
			lines = append(lines, " authentication-mode scheme //AAA认证")
			lines = append(lines, " protocol inbound ssh //仅允许SSH")
		} else {
			lines = append(lines, fmt.Sprintf("rsa local-key-pair create modulus %s //生成RSA密钥对", bits))
			lines = append(lines, "stelnet server enable //开启SSH服务端")
			lines = append(lines, "ssh user " + user + " authentication-type password //SSH用户")
			lines = append(lines, fmt.Sprintf("local-user %s password cipher %s //创建用户", user, pass))
			lines = append(lines, fmt.Sprintf("local-user %s privilege level %s //权限级别%s", user, pri, pri))
			lines = append(lines, fmt.Sprintf("local-user %s service-type ssh //允许SSH登录", user))
			lines = append(lines, fmt.Sprintf("user-interface vty %s //VTY线路", vtyRange))
			lines = append(lines, " authentication-mode aaa //AAA认证")
			lines = append(lines, " protocol inbound ssh //仅允许SSH")
		}
		return lines


	case "com_user":
		user := p(m, "user", "admin")
		pass := p(m, "pass", "")
		pri := p(m, "pri", "15")
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("username %s privilege %s secret %s //创建用户", user, pri, pass),
				"line vty 0 4 //VTY线路",
				" login local //本地认证",
			}
		}
		if vendor == "h3c" {
			return []string{
				fmt.Sprintf("local-user %s class manage //创建用户", user),
				fmt.Sprintf(" password simple %s //密码", pass),
				fmt.Sprintf(" authorization-attribute level %s //权限%s", pri, pri),
				" service-type ssh //SSH登录",
				
			}
		}
		return []string{
			fmt.Sprintf("local-user %s password cipher %s //创建用户", user, pass),
			fmt.Sprintf("local-user %s privilege level %s //权限%s", user, pri, pri),
			fmt.Sprintf("local-user %s service-type ssh //SSH登录", user),
		}

	case "router_bgp":
		asn := p(m, "asn", "65001")
		rid := p(m, "rid", "1.1.1.1")
		peerAs := p(m, "peer_as", "65002")
		peerIps := strings.Split(p(m, "peer_ip", ""), ",")
		maxHop := p(m, "ebgp_max_hop", "")
		connInt := p(m, "conn_int", "")
		net := p(m, "network", "")
		mask := p(m, "netmask", "255.255.255.0")
		net6 := p(m, "network6", "")
		enableNet := p(m, "enable_net", "") == "on"
		vpn4 := p(m, "vpn4", "") == "on"
		vpn6 := p(m, "vpn6", "") == "on"
		vpn := p(m, "vpn", "")
		vpn4Peers := strings.Split(p(m, "vpn4_enable_peer", ""), ",")
		vpn4Adv := strings.Split(p(m, "vpn4_adv_comm", ""), ",")
		vpn4Nh := strings.Split(p(m, "vpn4_next_hop_inv", ""), ",")
		vpn6Peers := strings.Split(p(m, "vpn6_enable_peer", ""), ",")
		vpn6Adv := strings.Split(p(m, "vpn6_adv_comm", ""), ",")
		if vendor == "cisco" {
			lines := []string{
				sys + " //进入全局配置",
				fmt.Sprintf("router bgp %s //创建BGP", asn),
				fmt.Sprintf(" bgp router-id %s //Router ID", rid),
			}
			for _, pip := range peerIps {
				pip = strings.TrimSpace(pip)
				if pip != "" {
					lines = append(lines, fmt.Sprintf(" neighbor %s remote-as %s //指定邻居", pip, peerAs))
					if maxHop != "" { lines = append(lines, fmt.Sprintf(" neighbor %s ebgp-multihop %s //EBGP多跳", pip, maxHop)) }
					if connInt != "" { lines = append(lines, fmt.Sprintf(" update-source %s //更新源", connInt)) }
				}
			}
			if vpn != "" {
				lines = append(lines, fmt.Sprintf(" address-family ipv4 vrf %s //绑定VPN实例", vpn))
				if enableNet && net != "" { lines = append(lines, fmt.Sprintf("  network %s mask %s //宣告网段", net, mask)) }
				if imp := p(m, "vpn_import", ""); imp != "" { lines = append(lines, fmt.Sprintf("  redistribute %s //引入路由", imp)) }
			} else if enableNet && net != "" {
				lines = append(lines, " address-family ipv4 //IPv4单播")
				lines = append(lines, fmt.Sprintf("  network %s mask %s //宣告网段", net, mask))
			}
			if vpn4 {
				lines = append(lines, " address-family vpnv4 //VPNv4地址族")
				for _, p := range vpn4Peers {
					p = strings.TrimSpace(p)
					if p != "" { lines = append(lines, fmt.Sprintf("  neighbor %s activate //启用邻居", p)) }
				}
				for _, p := range vpn4Adv {
					p = strings.TrimSpace(p)
					if p != "" { lines = append(lines, fmt.Sprintf("  neighbor %s send-community both //发送团体属性", p)) }
				}
				for _, p := range vpn4Nh {
					p = strings.TrimSpace(p)
					if p != "" { lines = append(lines, fmt.Sprintf("  neighbor %s next-hop-self //下一跳不变", p)) }
				}
			}
			if vpn6 {
				lines = append(lines, " address-family vpnv6 //VPNv6地址族")
				for _, p := range vpn6Peers {
					p = strings.TrimSpace(p)
					if p != "" { lines = append(lines, fmt.Sprintf("  neighbor %s activate //启用邻居", p)) }
				}
			}
			return lines
		}
		// 华为/华三
		isH3c := vendor == "h3c"
		afV4 := " ipv4-family unicast"
		afVpn4 := " ipv4-family vpnv4"
		afV6 := " ipv6-family unicast"
		afVpn6 := " ipv6-family vpnv6"
		afInst := " ipv4-family vpn-instance "
		if isH3c {
			afV4 = " address-family ipv4 unicast"
			afVpn4 = " address-family vpnv4"
			afV6 = " address-family ipv6 unicast"
			afVpn6 = " address-family vpnv6"
			afInst = " address-family ipv4 vpn-instance "
		}
		lines := []string{
			fmt.Sprintf("bgp %s //创建BGP", asn),
			fmt.Sprintf(" router-id %s //Router ID", rid),
		}
		for _, pip := range peerIps {
			pip = strings.TrimSpace(pip)
			if pip != "" {
				lines = append(lines, fmt.Sprintf(" peer %s as-number %s //指定邻居", pip, peerAs))
				if maxHop != "" { lines = append(lines, fmt.Sprintf(" peer %s ebgp-max-hop %s //EBGP多跳", pip, maxHop)) }
				if connInt != "" { lines = append(lines, fmt.Sprintf(" peer %s connect-interface %s //更新源", pip, connInt)) }
			}
		}
		if vpn != "" && !isH3c {
			lines = append(lines, fmt.Sprintf("%s%s //绑定VPN实例", afInst, vpn))
			if enableNet && net != "" { lines = append(lines, fmt.Sprintf("  network %s %s //宣告网段", net, mask)) }
			if imp := p(m, "vpn_import", ""); imp != "" {
				for _, line := range strings.Split(imp, "\n") {
					line = strings.TrimSpace(line)
					if line != "" { lines = append(lines, fmt.Sprintf("  import-route %s //引入路由", line)) }
				}
			}
			if exp := p(m, "vpn_export", ""); exp != "" {
				for _, line := range strings.Split(exp, "\n") {
					line = strings.TrimSpace(line)
					if line != "" { lines = append(lines, fmt.Sprintf("  export-route %s //发布路由", line)) }
				}
			}
		} else if enableNet && net != "" {
			lines = append(lines, afV4+" //IPv4单播")
			lines = append(lines, fmt.Sprintf("  network %s %s //宣告网段", net, mask))
		}
		if enableNet && net6 != "" {
			lines = append(lines, afV6+" //IPv6单播")
			lines = append(lines, fmt.Sprintf("  network %s //宣告IPv6网段", net6))
		}
		if vpn4 {
			lines = append(lines, afVpn4+" //VPNv4地址族")
			for _, p := range vpn4Peers {
				p = strings.TrimSpace(p)
				if p != "" { lines = append(lines, fmt.Sprintf("  peer %s enable //启用邻居", p)) }
			}
			for _, p := range vpn4Adv {
				p = strings.TrimSpace(p)
				if p != "" { lines = append(lines, fmt.Sprintf("  peer %s advertise-community //发送团体属性", p)) }
			}
			for _, p := range vpn4Nh {
				p = strings.TrimSpace(p)
				if p != "" { lines = append(lines, fmt.Sprintf("  peer %s next-hop-invariable //下一跳不变", p)) }
			}
		}
		if vpn6 {
			lines = append(lines, afVpn6+" //VPNv6地址族")
			for _, p := range vpn6Peers {
				p = strings.TrimSpace(p)
				if p != "" { lines = append(lines, fmt.Sprintf("  peer %s enable //启用邻居", p)) }
			}
			for _, p := range vpn6Adv {
				p = strings.TrimSpace(p)
				if p != "" { lines = append(lines, fmt.Sprintf("  peer %s advertise-community //发送团体属性", p)) }
			}
		}
		if vpn != "" && isH3c {
			lines = append(lines, fmt.Sprintf("ip vpn-instance %s //进入VPN实例", vpn))
			lines = append(lines, " address-family ipv4 //IPv4单播")
			if imp := p(m, "vpn_import", ""); imp != "" {
				for _, line := range strings.Split(imp, "\n") {
					line = strings.TrimSpace(line)
					if line != "" { lines = append(lines, fmt.Sprintf("  import-route %s //引入路由", line)) }
				}
			}
			if exp := p(m, "vpn_export", ""); exp != "" {
				for _, line := range strings.Split(exp, "\n") {
					line = strings.TrimSpace(line)
					if line != "" { lines = append(lines, fmt.Sprintf("  export-route %s //发布路由", line)) }
				}
			}
		}
		return lines

	case "router_isis":
		net := p(m, "net", "49.0001.0000.0000.0001.00")
		level := p(m, "level", "level-2")
		pid := p(m, "pid", "1")
		rid := p(m, "router_id", "1.1.1.1")
		vpn := p(m, "vpn", "")
		vpnLine := ""
		if vpn != "" { vpnLine = fmt.Sprintf(" vpn-instance %s", vpn) }
		return []string{
			fmt.Sprintf("isis%s %s //创建IS-IS进程", vpnLine, pid),
			fmt.Sprintf(" network-entity %s //NET地址", net),
			fmt.Sprintf(" router-id %s //Router ID", rid),
			fmt.Sprintf(" is-level %s //级别", level),
		}

	case "mpls_vpn":
		rd := p(m, "rd", "100:1")
		rtIn := p(m, "rt_in", "100:1")
		rtOut := p(m, "rt_out", "100:1")
		vpn := p(m, "vpn_name", "vpna")
		iface := p(m, "iface", "GigabitEthernet0/0/0")
		ip := p(m, "ip", "10.0.0.1 255.255.255.0")
		return []string{
			"mpls lsr-id 1.1.1.1 //配置LSR ID",
			"mpls //全局使能MPLS",
			"mpls ldp //使能LDP",
			fmt.Sprintf("ip vpn-instance %s //创建VPN实例", vpn),
			fmt.Sprintf(" route-distinguisher %s //配置RD", rd),
			fmt.Sprintf(" vpn-target %s export-extcommunity //RT导出", rtOut),
			fmt.Sprintf(" vpn-target %s import-extcommunity //RT导入", rtIn),
			fmt.Sprintf("interface %s //进入接口", iface),
			fmt.Sprintf(" ip binding vpn-instance %s //绑定VPN实例", vpn),
			fmt.Sprintf(" ip address %s //配置接口IP", ip),
			" mpls //接口使能MPLS",
			" mpls ldp //接口使能LDP",
		}

	case "route_policy":
		name := p(m, "name", "RP1")
		seq := p(m, "seq", "10")
		prefix := p(m, "prefix", "")
		pfxIndex := p(m, "pfx_index", "10")
		pfxAction := p(m, "pfx_action", "permit")
		pfxNet := p(m, "pfx_net", "")
		pfxMask := p(m, "pfx_masklen", "")
		enablePrefix := p(m, "enable_prefix", "") == "on"
		multiRules := strings.Split(p(m, "pfx_rules_multi", ""), "\n")
		if vendor == "cisco" {
			l := []string{sys + " //全局配置"}
			if enablePrefix && prefix != "" {
				for _, r := range multiRules {
					r = strings.TrimSpace(r)
					if r == "" { continue }
					parts := strings.SplitN(r, " ", 2)
					if len(parts) == 2 { l = append(l, fmt.Sprintf("ip prefix-list %s seq %s %s", prefix, parts[0], parts[1])) }
				}
				if pfxNet != "" {
					l = append(l, fmt.Sprintf("ip prefix-list %s seq %s %s %s %s", prefix, pfxIndex, pfxAction, pfxNet, pfxMask))
				}
			}
			l = append(l, fmt.Sprintf("route-map %s permit %s //路由策略", name, seq))
			if enablePrefix && prefix != "" { l = append(l, fmt.Sprintf(" match ip address prefix-list %s //匹配前缀列表", prefix)) }
			return l
		}
		l := []string{}
		if enablePrefix && prefix != "" {
			for _, r := range multiRules {
				r = strings.TrimSpace(r)
				if r == "" { continue }
				l = append(l, fmt.Sprintf("ip prefix-list %s index %s //前缀列表", prefix, r))
			}
			if pfxNet != "" {
				l = append(l, fmt.Sprintf("ip prefix-list %s index %s %s %s %s //前缀列表", prefix, pfxIndex, pfxAction, pfxNet, pfxMask))
			}
		}
		l = append(l, fmt.Sprintf("route-policy %s permit node %s //路由策略", name, seq))
		if enablePrefix && prefix != "" { l = append(l, fmt.Sprintf(" if-match ip address prefix-list %s //匹配前缀列表", prefix)) }
		return l

	case "router_policy":
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				"route-map P1 permit 10 //路由策略",
				fmt.Sprintf(" match ip address %s //匹配ACL", p(m, "acl", "3000")),
				fmt.Sprintf(" set ip next-hop %s //指定下一跳", p(m, "nh", "")),
				" exit",
				fmt.Sprintf("interface %s //应用接口", p(m, "iface", "")),
				" ip policy route-map P1 //应用策略",
				" exit",
			}
		}
		return []string{
			fmt.Sprintf("traffic classifier c1 operator and //流分类"),
			fmt.Sprintf(" if-match acl %s //匹配ACL", p(m, "acl", "3000")),
			
			"traffic behavior b1 //流行为",
			fmt.Sprintf(" redirect ip-nexthop %s //指定下一跳", p(m, "nh", "")),
			
			"traffic policy p1 //策略",
			" classifier c1 behavior b1",
			
			fmt.Sprintf("interface %s //应用接口", p(m, "iface", "")),
			" traffic-policy p1 inbound",
			
		}

	case "com_log":
		logip := p(m, "logip", "")
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				"logging on //开启日志",
				fmt.Sprintf("logging host %s //日志服务器", logip),
			}
		}
		return []string{
			"info-center enable //开启日志",
			fmt.Sprintf("info-center loghost %s //日志服务器", logip),
		}

	case "com_snmp":
		ver := p(m, "ver", "v2c")
		host := p(m, "host", "")
		lines := []string{sys}
		if vendor == "cisco" {
			if ver == "v3" {
				sec := p(m, "secname", "monitor")
				lines = append(lines,
					"snmp-server engineID local //引擎ID",
					"snmp-server group g1 v3 priv //安全组",
					fmt.Sprintf("snmp-server user %s g1 v3 auth sha %s priv aes 128 %s //用户", sec, p(m, "authpass", ""), p(m, "privpass", "")),
					fmt.Sprintf("snmp-server host %s version 3 priv %s //网管站", host, sec))
			} else {
				lines = append(lines,
					fmt.Sprintf("snmp-server community %s RO //团体字", p(m, "community", "public")),
					fmt.Sprintf("snmp-server host %s version %s %s //网管站", host, ver, p(m, "community", "public")))
			}
			return lines
		}
		if ver == "v3" {
			sec := p(m, "secname", "monitor")
			lines = append(lines,
				"snmp-agent //开启SNMP",
				"snmp-agent sys-info version v3 //v3",
				"snmp-agent group v3 g1 privacy //安全组",
				fmt.Sprintf("snmp-agent usm-user v3 %s g1 //用户", sec),
				fmt.Sprintf("snmp-agent usm-user v3 %s authentication-mode sha %s //认证", sec, p(m, "authpass", "")),
				fmt.Sprintf("snmp-agent usm-user v3 %s privacy-mode aes128 %s //加密", sec, p(m, "privpass", "")),
				fmt.Sprintf("snmp-agent target-host trap address udp-domain %s params securityname %s v3 //网管站", host, sec))
		} else {
			lines = append(lines,
				"snmp-agent //开启SNMP",
				fmt.Sprintf("snmp-agent sys-info version %s //版本%s", ver, ver),
				fmt.Sprintf("snmp-agent community read %s //团体字", p(m, "community", "public")),
				fmt.Sprintf("snmp-agent target-host trap address udp-domain %s params securityname %s //网管站", host, p(m, "community", "public")))
		}
		return lines

		case "sw_stp":
			mode := p(m, "mode", "mstp")
			inst := p(m, "instance", "1")
			root := p(m, "root", "none")
			pri := p(m, "pri", "4096")
			svlan := p(m, "vlan", "1")
			lines := []string{sys}
			if vendor == "cisco" {
				lines = append(lines, "spanning-tree mode rapid-pvst //STP模式")
				if root == "primary" { lines = append(lines, fmt.Sprintf("spanning-tree vlan %s root primary //根桥主", svlan)) }
				if root == "secondary" { lines = append(lines, fmt.Sprintf("spanning-tree vlan %s root secondary //根桥备", svlan)) }
				if root == "none" && pri != "" { lines = append(lines, fmt.Sprintf("spanning-tree vlan %s priority %s //优先级", svlan, pri)) }
				return lines
			}
			lines = append(lines, fmt.Sprintf("stp mode %s //STP模式", mode))
			if mode == "mstp" && p(m, "vlan", "") != "" {
				lines = append(lines, "stp region-configuration //MST域")
				lines = append(lines, fmt.Sprintf(" instance %s vlan %s //实例映射", inst, p(m, "vlan", "")))
				lines = append(lines, " active region-configuration //激活")
				
			}
			if root == "primary" {
				if mode == "mstp" { lines = append(lines, fmt.Sprintf("stp instance %s root primary //根桥主", inst)) } else { lines = append(lines, "stp root primary //根桥主") }
			}
			if root == "secondary" {
				if mode == "mstp" { lines = append(lines, fmt.Sprintf("stp instance %s root secondary //根桥备", inst)) } else { lines = append(lines, "stp root secondary //根桥备") }
			}
			if root == "none" && pri != "" {
				if mode == "mstp" { lines = append(lines, fmt.Sprintf("stp instance %s priority %s //优先级", inst, pri)) } else { lines = append(lines, fmt.Sprintf("stp priority %s //优先级", pri)) }
			}
			lines = append(lines, "stp enable //开启STP")
			return lines
	case "sw_mirror":
		dir := p(m, "dir", "both")
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				"monitor session 1 local //镜像",
				fmt.Sprintf("source interface %s %s //源", p(m, "src", ""), dir),
				fmt.Sprintf("destination interface %s //目的", p(m, "dest", "")),
			}
		}
		return []string{
			fmt.Sprintf("observe-port 1 interface %s //镜像目的", p(m, "dest", "")),
			fmt.Sprintf("interface %s //镜像源", p(m, "src", "")),
			fmt.Sprintf(" port-mirroring to observe-port 1 %s //流量%s", dir, dir),
			
		}

	case "sw_vrrp":
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				fmt.Sprintf("interface Vlan%s //进入SVI", p(m, "vlan", "10")),
				fmt.Sprintf(" vrrp %s ip %s //VRRP", p(m, "vrid", "1"), p(m, "vip", "")),
				" exit",
			}
		}
		return []string{
			fmt.Sprintf("interface %s //进入接口", vlanIfName(vendor, p(m, "vlan", "10"))),
			fmt.Sprintf(" vrrp vrid %s virtual-ip %s //VRRP", p(m, "vrid", "1"), p(m, "vip", "")),
			
		}

	case "sw_vlan":
		vlans := p(m, "vlan", "")
		lines := []string{sys}
		if vendor == "cisco" {
			for _, v := range strings.Split(vlans, ",") {
				v = strings.TrimSpace(v)
				if v == "" { continue }
				lines = append(lines, fmt.Sprintf("vlan %s //创建VLAN", v))
			}
		} else {
			if strings.Contains(vlans, ",") {
					lines = append(lines, fmt.Sprintf("vlan batch %s //批量创建VLAN", vlans))
				} else {
					lines = append(lines, fmt.Sprintf("vlan %s //创建VLAN", vlans))
					
				}
		}
		if vif := p(m, "vlanif", ""); vif != "" && p(m, "ip", "") != "" {
			ip := p(m, "ip", "")
			if vendor == "cisco" {
				parts := strings.Split(ip, "/")
				addr := parts[0]
				mask := "255.255.255.0"
				if len(parts) == 2 { _, mask = cidrMask(ip) }
				lines = append(lines, fmt.Sprintf("interface Vlan%s //SVI", vif))
				lines = append(lines, fmt.Sprintf(" ip address %s %s //网关", addr, mask))
				lines = append(lines, " no shutdown")
			} else {
				lines = append(lines, fmt.Sprintf("interface %s //VLANIF", vlanIfName(vendor, vif)))
				lines = append(lines, fmt.Sprintf(" ip address %s //网关", ip))
			}
		}
		return lines

	case "sw_access":
		ifaces := strings.Split(p(m, "iface", ""), ",")
		vlan := p(m, "vlan", "1")
		lines := []string{sys}
		if vendor == "cisco" {
			for _, ifa := range ifaces {
				if strings.TrimSpace(ifa) == "" { continue }
				lines = append(lines, fmt.Sprintf("interface %s //进入接口", strings.TrimSpace(ifa)))
				lines = append(lines, " switchport mode access //Access模式")
				lines = append(lines, fmt.Sprintf(" switchport access vlan %s //加入VLAN", vlan))
				lines = append(lines, " no shutdown //开启")
				lines = append(lines, "exit")
			}
			return lines
		}
		if len(ifaces) > 1 {
			lines = append(lines, fmt.Sprintf("interface range %s to %s //批量接口", strings.TrimSpace(ifaces[0]), strings.TrimSpace(ifaces[len(ifaces)-1])))
		} else {
			lines = append(lines, fmt.Sprintf("interface %s //进入接口", strings.TrimSpace(ifaces[0])))
		}
		lines = append(lines, " port link-type access //Access口")
		lines = append(lines, fmt.Sprintf(" port default vlan %s //加入VLAN", vlan))
		
		return lines
	case "sw_trunk":
		ifaces := strings.Split(p(m, "iface", ""), ",")
		vlans := p(m, "vlans", "")
		lines := []string{sys}
		if vendor == "cisco" {
			for _, ifa := range ifaces {
				if strings.TrimSpace(ifa) == "" { continue }
				lines = append(lines, fmt.Sprintf("interface %s //进入接口", strings.TrimSpace(ifa)))
				lines = append(lines, " switchport mode trunk //Trunk模式")
				lines = append(lines, fmt.Sprintf(" switchport trunk allowed vlan %s //允许VLAN", vlans))
				lines = append(lines, "exit")
			}
			return lines
		}
		if len(ifaces) > 1 {
			lines = append(lines, fmt.Sprintf("interface range %s to %s //批量接口", strings.TrimSpace(ifaces[0]), strings.TrimSpace(ifaces[len(ifaces)-1])))
		} else {
			lines = append(lines, fmt.Sprintf("interface %s //进入接口", strings.TrimSpace(ifaces[0])))
		}
		lines = append(lines, " port link-type trunk //Trunk口")
		lines = append(lines, fmt.Sprintf(" port trunk allow-pass vlan %s //允许VLAN", vlans))
		
		return lines
	case "sw_dhcp":
		addr, mask := cidrMask(p(m, "net", ""))
		pool := p(m, "pool", "pool1")
		gw := p(m, "gw", "")
		if vendor == "cisco" {
			return []string{
				sys + " //进入全局配置",
				"service dhcp //开启DHCP",
				fmt.Sprintf("ip dhcp pool %s //地址池", pool),
				fmt.Sprintf(" network %s %s //网段", addr, mask),
				fmt.Sprintf(" default-router %s //网关", gw),
				" exit",
			}
		}
		lines := []string{
			"dhcp enable //开启DHCP",
			fmt.Sprintf("ip pool %s //地址池", pool),
			fmt.Sprintf(" network %s %s //网段", addr, mask),
			fmt.Sprintf(" gateway-list %s //网关", gw),
		}
		if dns := p(m, "dns", ""); dns != "" { lines = append(lines, " dns-list "+dns) }
		return lines

	}
	return []string{sys}
}
























