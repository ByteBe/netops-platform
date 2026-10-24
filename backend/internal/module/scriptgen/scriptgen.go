// Package scriptgen 脚本生成器（全新设计：纯代码生成，无数据库模板）
// 生成逻辑已迁移 Python 智能外围（pyservice/scriptgen，FastAPI），Go 保留 API/校验框架并转发调用，失败自动回退本地 Go 实现。
package scriptgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/bits"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	g.POST("/generate", func(c *gin.Context) {
		var req genReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, 0, "参数错误")
			return
		}
		if label, msg := validateReq(req.Code, req.Params); msg != "" {
			response.Fail(c, http.StatusBadRequest, 0, label+": "+msg)
			return
		}
		// 优先调用 Python 智能外围生成；失败自动回退本地 Go 实现（用户无感知）
		lines, err := callPythonGenerate(a, req.Code, req.Vendor, req.Params)
		if err != nil {
			fmt.Printf("[scriptgen] python 服务调用失败，回退内置生成: %v\n", err)
			lines = build(req.Code, req.Params, req.Vendor)
		}
		filtered := make([]string, 0, len(lines))
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if t == "quit" || t == "exit" || t == "end" {
				continue
			}
			filtered = append(filtered, l)
		}
		response.OK(c, gin.H{"script": strings.Join(decorate(req.Code, filtered), "\n")})
	})
}

// callPythonGenerate 调用 Python 智能外围服务（FastAPI /internal/scriptgen/generate）
// 成功返回脚本行；失败返回 error（由调用方回退本地 Go 实现）
func callPythonGenerate(a *core.App, code, vendor string, params map[string]string) ([]string, error) {
	base := strings.TrimRight(a.Cfg.Server.PythonSvcURL, "/")
	if base == "" {
		return nil, fmt.Errorf("python svc url empty")
	}
	url := base + "/internal/scriptgen/generate"
	body, err := json.Marshal(map[string]any{
		"code":   code,
		"vendor": vendor,
		"params": params,
	})
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python svc status %d", resp.StatusCode)
	}
	var out struct {
		Script string `json:"script"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Script) == "" {
		return nil, fmt.Errorf("python svc empty script")
	}
	return strings.Split(out.Script, "\n"), nil
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
	Key   string   `json:"key"`
	Eq    string   `json:"eq"`
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
			{Key: "net", Label: "宣告网段", Type: "ip", Required: true, Placeholder: "如10.16.194.0"},
			{Key: "netmask", Label: "掩码", Type: "mask", Required: true, Placeholder: "如255.255.255.0"},
			{Key: "area", Label: "区域号", Type: "text", Required: true, Placeholder: "如0"},
			{Key: "enable_vpn", Label: "启用MPLS VPN", Type: "switch", Required: false},
			{Key: "vpn", Label: "VPN实例名", Type: "text", Required: true, Placeholder: "如vpna", ShowIf: &ShowIf{Key: "enable_vpn", Eq: "on"}},
		}},
		{Code: "router_static", Name: "静态路由", Icon: "➡️", Desc: "手动指定路由", Fields: []Field{
			{Key: "dest", Label: "目标网段", Type: "ip", Required: true, Placeholder: "如0.0.0.0"},
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
			{Key: "network", Label: "发布网段(IPv4)", Type: "ip", Required: true, Placeholder: "如10.0.0.0", ShowIf: &ShowIf{Key: "enable_net", Eq: "on"}},
			{Key: "netmask", Label: "掩码", Type: "mask", Required: true, Placeholder: "如255.255.255.0", ShowIf: &ShowIf{Key: "enable_net", Eq: "on"}},
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
			{Key: "host", Label: "平台/网管站IP", Type: "ip", Required: true, Placeholder: "如10.16.1.10"},
			{Key: "community", Label: "团体字(v1/v2c)", Type: "text", Required: false, Placeholder: "如public", ShowIf: &ShowIf{Key: "ver", NotEq: "v3"}},
			{Key: "secname", Label: "用户名(v3)", Type: "text", Required: false, Placeholder: "如monitor", ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
			{Key: "authproto", Label: "认证协议(v3)", Type: "select", Required: false, Options: []string{"sha", "md5"}, ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
			{Key: "authpass", Label: "认证密码(v3)", Type: "password", Required: false, ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
			{Key: "privproto", Label: "加密协议(v3)", Type: "select", Required: false, Options: []string{"aes128", "des", "aes256"}, ShowIf: &ShowIf{Key: "ver", Eq: "v3"}},
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

// ---------- 输入校验（生成前检查，前后端同一套规则） ----------

var macRe = regexp.MustCompile(`^(?:[0-9A-Fa-f]{4}\.){2}[0-9A-Fa-f]{4}$|^(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)

// fieldVisible 按 ShowIf 判定字段当前是否可见
func fieldVisible(f Field, m map[string]string) bool {
	if f.ShowIf == nil {
		return true
	}
	v := m[f.ShowIf.Key]
	if f.ShowIf.NotEq != "" {
		return v != f.ShowIf.NotEq
	}
	if len(f.ShowIf.EqAny) > 0 {
		for _, e := range f.ShowIf.EqAny {
			if v == e {
				return true
			}
		}
		return false
	}
	return v == f.ShowIf.Eq
}

func validIP(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	return ip != nil && strings.Count(strings.TrimSpace(s), ".") == 3
}

func validMask(s string) bool {
	s = strings.TrimSpace(s)
	var a, b, c, d uint32
	if _, err := fmt.Sscanf(s, "%d.%d.%d.%d", &a, &b, &c, &d); err != nil {
		return false
	}
	if a > 255 || b > 255 || c > 255 || d > 255 {
		return false
	}
	m := a<<24 | b<<16 | c<<8 | d
	if m == 0 {
		return s == "0.0.0.0"
	}
	ones := bits.OnesCount32(m)
	// 注意：IPMask.String() 返回十六进制，须经 net.IP 转点分十进制比较
	return net.IP(net.CIDRMask(ones, 32)).String() == s
}

// checkValue 按字段类型与关键 key 校验，返回错误信息（空串=通过）
func checkValue(f Field, code, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		if f.Required {
			return "不能为空"
		}
		return ""
	}
	switch f.Type {
	case "ip":
		if !validIP(v) {
			return "IP 地址格式不正确"
		}
	case "network":
		parts := strings.Split(v, "/")
		if len(parts) != 2 || !validIP(parts[0]) {
			return "网段格式不正确（如 192.168.1.0/24）"
		}
		p, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || p < 0 || p > 32 {
			return "掩码长度应为 0-32"
		}
	case "number":
		if _, err := strconv.Atoi(v); err != nil {
			return "请输入数字"
		}
	case "mask":
		if !validMask(v) {
			return "掩码格式不正确（需连续，如 255.255.255.0）"
		}
	case "password":
		if len(v) < 6 {
			return "密码长度不能少于 6 位"
		}
	}
	// 按 key 特判（text 类但语义明确）
	switch f.Key {
	case "peer_ip", "vpn4_enable_peer", "vpn4_adv_comm", "vpn4_next_hop_inv", "dns":
		for _, item := range strings.Split(v, ",") {
			if !validIP(item) {
				return "IP 列表格式不正确（逗号分隔，如 1.1.1.1,2.2.2.2）"
			}
		}
	case "peer_as":
		for _, item := range strings.Split(v, ",") {
			if _, err := strconv.Atoi(strings.TrimSpace(item)); err != nil {
				return "AS 号列表格式不正确（数字，逗号分隔）"
			}
		}
	case "vlan", "vlans":
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			rg := strings.Split(item, "-")
			for _, n := range rg {
				if _, err := strconv.Atoi(strings.TrimSpace(n)); err != nil {
					return "VLAN 格式不正确（如 10,20,30-40）"
				}
			}
		}
	case "vty":
		parts := strings.Fields(v)
		if len(parts) != 2 {
			return "VTY 范围格式不正确（如 0 4）"
		}
		for _, p := range parts {
			if _, err := strconv.Atoi(p); err != nil {
				return "VTY 范围格式不正确（如 0 4）"
			}
		}
	case "srcmac":
		if !macRe.MatchString(v) {
			return "MAC 地址格式不正确（如 0000-1111-2222）"
		}
	case "ip":
		// mpls_vpn 的接口 IP 为 "地址 掩码" 双 token
		if code == "mpls_vpn" {
			parts := strings.Fields(v)
			if len(parts) != 2 || !validIP(parts[0]) || !validMask(parts[1]) {
				return "接口IP格式不正确（如 10.0.0.1 255.255.255.0）"
			}
		}
	case "pfx_net":
		if !validIP(v) {
			return "匹配网段格式不正确（如 10.16.192.0）"
		}
	}
	return ""
}

// validateReq 校验所有可见字段，返回第一个错误字段 label 与错误信息
func validateReq(code string, m map[string]string) (string, string) {
	for _, cat := range allCategories() {
		if cat.Code != code {
			continue
		}
		for _, f := range cat.Fields {
			if !fieldVisible(f, m) {
				continue
			}
			if msg := checkValue(f, code, m[f.Key]); msg != "" {
				return f.Label, msg
			}
		}
		return "", ""
	}
	return "", ""
}

func allCategories() []Category {
	var all []Category
	for _, g := range groups {
		all = append(all, g...)
	}
	return all
}

// codeComments 各脚本类型的中文注释说明（生成结果区展示，前端"纯净版"自动剥离 // 注释）
var codeComments = map[string]string{
	"router_acl": "配置 ACL 访问控制规则（允许/拒绝）",
	"sw_acl":     "配置 ACL 访问控制规则（允许/拒绝）",
	"router_nat": "配置 NAT 地址转换（内网共享上网）",
	"router_ospf": "配置 OSPF 动态路由",
	"router_static": "配置静态路由",
	"sw_static":    "配置静态路由",
	"router_bgp":   "配置 BGP 路由",
	"router_isis":  "配置 IS-IS 链路状态路由",
	"router_policy": "配置策略路由（灵活选路）",
	"route_policy":  "配置路由策略（过滤/属性修改）",
	"mpls_vpn":      "配置 MPLS VPN 实例",
	"com_ssh":       "配置 SSH 远程管理",
	"com_user":      "新建本地管理账号",
	"com_log":       "配置 Syslog 日志服务器上报",
	"com_snmp":      "配置 SNMP 网络管理",
	"sw_vlan":       "创建 VLAN 并配置网关",
	"sw_access":     "配置接入口为 Access 模式",
	"sw_trunk":      "配置级联口为 Trunk 模式",
	"sw_dhcp":       "配置 DHCP 地址池（自动分配 IP）",
	"sw_stp":        "配置生成树协议（防止环路）",
}

// decorate 为脚本加分段中文注释（// 行），命令保持原样可逐条下发
func decorate(code string, lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	title := codeComments[code]
	if title == "" {
		title = code
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, "// "+title)
	out = append(out, lines...)
	return out
}

func p(m map[string]string, k, def string) string {
	if v, ok := m[k]; ok && v != "" {
		return v
	}
	return def
}

func cidrMask(cidr string) (addr, mask string) {
	parts := strings.Split(strings.TrimSpace(cidr), "/")
	addr = parts[0]
	prefix := 24
	if len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &prefix)
	}
	mp := []string{}
	for i := 0; i < 4; i++ {
		if prefix >= 8 {
			mp = append(mp, "255")
			prefix -= 8
		} else {
			mp = append(mp, fmt.Sprintf("%d", 256-(1<<(8-prefix))))
			prefix = 0
		}
	}
	mask = strings.Join(mp, ".")
	return
}

func wildOf(cidr string) string {
	parts := strings.Split(strings.TrimSpace(cidr), "/")
	prefix := 24
	if len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &prefix)
	}
	w := uint32(0xFFFFFFFF >> prefix)
	return fmt.Sprintf("%d.%d.%d.%d", w>>24, (w>>16)&0xFF, (w>>8)&0xFF, w&0xFF)
}

func wildcardOf(mask string) string {
	var a, b, c, d uint32
	fmt.Sscanf(mask, "%d.%d.%d.%d", &a, &b, &c, &d)
	w := ^(a<<24 | b<<16 | c<<8 | d)
	return fmt.Sprintf("%d.%d.%d.%d", w>>24, (w>>16)&0xFF, (w>>8)&0xFF, w&0xFF)
}

// maskToDotted 把掩码统一为点分十进制（思科 ip route 要求点分掩码）。
// 输入可为掩码长度(0-32)或点分掩码；非法输入原样返回。
func maskToDotted(s string) string {
	s = strings.TrimSpace(s)
	if strings.Count(s, ".") == 3 {
		return s
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 32 {
		_, m := cidrMask(fmt.Sprintf("0.0.0.0/%d", n))
		return m
	}
	return s
}

func build(code string, m map[string]string, vendor string) []string {
	switch vendor {
	case "h3c":
		return buildH3c(code, m)
	case "cisco":
		return buildCisco(code, m)
	default:
		return buildHuawei(code, m)
	}
}
