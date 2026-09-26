package scriptgen

import (
	"fmt"
	"strings"
)

// ---- 交换机 VLAN ----
func regSwitchVLAN() {
	Register(Template{
		Code: "switch_vlan", Vendor: "huawei", DeviceType: "switch", Category: "vlan",
		Name: "交换机 VLAN", Description: "VLAN 创建（支持单个/批量/连续）",
		Fields: []Field{
			field("vlans", "VLAN列表（如 10 或 10-20 或 10,12,14-16）", "text", true),
			field("names", "VLAN名称（可选，格式：vlan号=名称，每行一条）", "list", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "vlan batch "+vlanFormat(p(params, "vlans", "")))
			for _, n := range nonEmptyLines(params["names"]) {
				kv := strings.SplitN(n, "=", 2)
				if len(kv) == 2 {
					lines = append(lines, fmt.Sprintf("vlan %s", strings.TrimSpace(kv[0])))
					lines = append(lines, fmt.Sprintf(" name %s", strings.TrimSpace(kv[1])))
					lines = append(lines, "quit")
				}
			}
			return lines
		},
	})
}

// ---- 交换机 VLANIF ----
func regSwitchVLANIF() {
	Register(Template{
		Code: "switch_vlanif", Vendor: "huawei", DeviceType: "switch", Category: "vlanif",
		Name: "交换机 VLANIF（三层网关）", Description: "VLANIF 接口地址配置",
		Fields: []Field{
			field("vlan", "VLAN号", "number", true),
			field("ip", "网关IP（如 192.168.1.1/24）", "network", true),
			field("description", "描述", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("interface Vlanif%s", p(params, "vlan", "")))
			lines = append(lines, " ip address "+p(params, "ip", ""))
			if d := p(params, "description", ""); d != "" {
				lines = append(lines, " description "+d)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 交换机 ACL（同路由器逻辑，独立模板便于按设备类型筛选） ----
func regSwitchACL() {
	Register(Template{
		Code: "switch_acl", Vendor: "huawei", DeviceType: "switch", Category: "acl",
		Name: "交换机 ACL", Description: "二层/高级 ACL 规则配置",
		Fields: []Field{
			field("acl_num", "ACL编号", "number", true),
			field("rules", "规则列表（每行一条）", "list", true),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("acl number %s", p(params, "acl_num", "3000")))
			for _, r := range nonEmptyLines(params["rules"]) {
				lines = append(lines, " rule "+r)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 交换机 NAT ----
func regSwitchNAT() {
	Register(Template{
		Code: "switch_nat", Vendor: "huawei", DeviceType: "switch", Category: "nat",
		Name: "交换机 NAT", Description: "交换机NAT配置（Easy IP/静态）",
		Fields: []Field{
			field("out_if", "出接口", "text", true),
			{Key: "nat_type", Label: "NAT类型", Type: "select", Required: true, Options: []string{"easy-ip", "static"}},
			field("acl_num", "内网ACL编号", "number", false),
			field("inside_net", "内网网段", "network", false),
			field("public_ip", "公网地址", "ip", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("interface %s", p(params, "out_if", "")))
			if p(params, "nat_type", "easy-ip") == "static" {
				lines = append(lines, fmt.Sprintf(" nat static global %s inside %s", p(params, "public_ip", ""), p(params, "inside_net", "")))
			} else if acl := p(params, "acl_num", ""); acl != "" {
				lines = append(lines, fmt.Sprintf(" nat outbound %s", acl))
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 交换机 OSPF/BGP/ISIS/MPLS/MPLSVPN/静态/策略路由/路由策略（与路由器一致的渲染逻辑） ----
func regSwitchOSPF() {
	Register(Template{Code: "switch_ospf", Vendor: "huawei", DeviceType: "switch", Category: "ospf",
		Name: "交换机 OSPF", Description: "OSPF配置", Fields: regOspfFields(), Render: renderOSPF})
}
func regSwitchBGP() {
	Register(Template{Code: "switch_bgp", Vendor: "huawei", DeviceType: "switch", Category: "bgp",
		Name: "交换机 BGP", Description: "BGP配置", Fields: regBgpFields(), Render: renderBGP})
}
func regSwitchISIS() {
	Register(Template{Code: "switch_isis", Vendor: "huawei", DeviceType: "switch", Category: "isis",
		Name: "交换机 IS-IS", Description: "IS-IS配置", Fields: regIsisFields(), Render: renderISIS})
}
func regSwitchMPLS() {
	Register(Template{Code: "switch_mpls", Vendor: "huawei", DeviceType: "switch", Category: "mpls",
		Name: "交换机 MPLS", Description: "MPLS基础配置", Fields: regMplsFields(), Render: renderMPLS})
}
func regSwitchMPLSVPN() {
	Register(Template{Code: "switch_mpls_vpn", Vendor: "huawei", DeviceType: "switch", Category: "mpls_vpn",
		Name: "交换机 MPLS VPN", Description: "MPLS L3VPN配置", Fields: regMplsVpnFields(), Render: renderMPLSVPN})
}
func regSwitchStatic() {
	Register(Template{Code: "switch_static", Vendor: "huawei", DeviceType: "switch", Category: "static_route",
		Name: "交换机 静态路由", Description: "静态路由配置", Fields: []Field{fieldWithPlaceholder("routes", "路由列表", "list", true, "每行一条：目的网段 下一跳 [优先级]\n示例：\n10.10.0.0/16 192.168.1.1 60\n172.16.0.0/12 10.0.0.1")}, Render: renderStatic})
}
func regSwitchPolicyRoute() {
	Register(Template{Code: "switch_policy_route", Vendor: "huawei", DeviceType: "switch", Category: "policy_route",
		Name: "交换机 策略路由", Description: "策略路由配置", Fields: regPolicyRouteFields(), Render: renderPolicyRoute})
}
func regSwitchRoutingPolicy() {
	Register(Template{Code: "switch_routing_policy", Vendor: "huawei", DeviceType: "switch", Category: "routing_policy",
		Name: "交换机 路由策略", Description: "Route-Policy配置", Fields: regRoutingPolicyFields(), Render: renderRoutingPolicy})
}

// ---- 交换机 接口配置 ----
func regSwitchInterface() {
	Register(Template{
		Code: "switch_interface_access", Vendor: "huawei", DeviceType: "switch", Category: "interface",
		Name: "交换机 接口配置（Access）", Description: "Access 口配置",
		Fields: []Field{
			field("interface", "接口（如 GigabitEthernet0/0/1）", "text", true),
			field("vlan", "PVID VLAN", "number", true),
			field("description", "描述", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "interface "+p(params, "interface", ""))
			lines = append(lines, " port link-type access")
			lines = append(lines, fmt.Sprintf(" port default vlan %s", p(params, "vlan", "1")))
			if d := p(params, "description", ""); d != "" {
				lines = append(lines, " description "+d)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
	Register(Template{
		Code: "switch_interface_trunk", Vendor: "huawei", DeviceType: "switch", Category: "interface",
		Name: "交换机 接口配置（Trunk）", Description: "Trunk 口配置",
		Fields: []Field{
			field("interface", "接口", "text", true),
			field("vlans", "允许通过的VLAN（如 10,20,30-40）", "text", true),
			field("native_vlan", "Native VLAN", "number", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "interface "+p(params, "interface", ""))
			lines = append(lines, " port link-type trunk")
			lines = append(lines, fmt.Sprintf(" port trunk allow-pass vlan %s", vlanFormat(p(params, "vlans", ""))))
			if nv := p(params, "native_vlan", ""); nv != "" {
				lines = append(lines, fmt.Sprintf(" port trunk pvid vlan %s", nv))
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 交换机 认证/VRRP/DHCP ----
func regSwitchAuth() {
	Register(Template{Code: "switch_auth_local", Vendor: "huawei", DeviceType: "switch", Category: "auth",
		Name: "交换机 本地认证", Description: "本地用户认证", Fields: []Field{
			field("username", "用户名", "text", true), field("password", "密码", "password", true),
			field("service", "服务类型", "select", false),
		}, Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("local-user %s password cipher %s", p(params, "username", ""), p(params, "password", "")))
			lines = append(lines, fmt.Sprintf("local-user %s privilege level 15", p(params, "username", "")))
			lines = append(lines, fmt.Sprintf("local-user %s service-type %s", p(params, "username", ""), p(params, "service", "telnet ssh")))
			return lines
		}})
	Register(Template{Code: "switch_auth_remote", Vendor: "huawei", DeviceType: "switch", Category: "auth",
		Name: "交换机 远程认证", Description: "RADIUS远程认证", Fields: []Field{
			field("server_ip", "服务器IP", "ip", true), field("server_port", "端口", "number", true),
			field("shared_key", "共享密钥", "password", true),
		}, Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("radius-server template radius1"))
			lines = append(lines, fmt.Sprintf(" radius-server authentication %s %s", p(params, "server_ip", ""), p(params, "server_port", "1812")))
			lines = append(lines, fmt.Sprintf(" radius-server shared-key cipher %s", p(params, "shared_key", "")))
			lines = append(lines, "quit", "aaa", " authentication-scheme default", " radius-server group radius1", "quit")
			return lines
		}})
}
func regSwitchVRRP() {
	Register(Template{Code: "switch_vrrp", Vendor: "huawei", DeviceType: "switch", Category: "vrrp",
		Name: "交换机 VRRP", Description: "VRRP配置", Fields: []Field{
			field("vlan", "VLAN号", "number", true), field("vrid", "VRID", "number", true),
			field("virtual_ip", "虚拟IP", "ip", true), field("priority", "优先级", "number", false),
		}, Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("interface Vlanif%s", p(params, "vlan", "")))
			lines = append(lines, fmt.Sprintf(" vrrp vrid %s virtual-ip %s", p(params, "vrid", "1"), p(params, "virtual_ip", "")))
			if pr := p(params, "priority", ""); pr != "" {
				lines = append(lines, fmt.Sprintf(" vrrp vrid %s priority %s", p(params, "vrid", "1"), pr))
			}
			lines = append(lines, "quit")
			return lines
		}})
}
func regSwitchDHCP() {
	Register(Template{
		Code: "switch_dhcp", Vendor: "huawei", DeviceType: "switch", Category: "dhcp",
		Name: "交换机 DHCP", Description: "DHCP 地址池配置",
		Fields: []Field{
			field("pool", "地址池名", "text", true),
			field("network", "网段（如 192.168.10.0 255.255.255.0）", "text", true),
			field("gateway", "网关", "ip", true),
			field("dns", "DNS（可多个逗号分隔）", "text", false),
			field("lease_days", "租期（天）", "number", false),
			field("excluded", "排除地址（每行一个IP）", "list", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "dhcp enable")
			lines = append(lines, fmt.Sprintf("ip pool %s", p(params, "pool", "pool1")))
			lines = append(lines, " network "+p(params, "network", ""))
			lines = append(lines, " gateway-list "+p(params, "gateway", ""))
			if d := p(params, "dns", ""); d != "" {
				lines = append(lines, " dns-list "+d)
			}
			if ld := p(params, "lease_days", ""); ld != "" {
				lines = append(lines, fmt.Sprintf(" lease day %s", ld))
			}
			for _, ex := range nonEmptyLines(params["excluded"]) {
				lines = append(lines, " excluded-ip-address "+ex)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

