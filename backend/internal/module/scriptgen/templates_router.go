package scriptgen

import (
	"fmt"
	"strings"
)

// ---- ACL ----
func regRouterACL() {
	Register(Template{
		Code: "router_acl", Vendor: "huawei", DeviceType: "router", Category: "acl",
		Name: "路由器 ACL（二层/高级/自定义）", Description: "配置基础/高级ACL及规则（二层ACL适用于交换机；路由器默认使用高级ACL）",
		Fields: []Field{
			field("acl_num", "ACL编号", "number", true),
			field("acl_type", "ACL类型", "select", true), // 占位，实际由编号决定
			field("rules", "规则列表（每行一条，如 permit ip source 192.168.1.0 0.0.0.255）", "list", true),
			field("description", "ACL描述", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			num := p(params, "acl_num", "3000")
			lines = append(lines, fmt.Sprintf("acl number %s", num))
			if d := p(params, "description", ""); d != "" {
				lines = append(lines, " description "+d)
			}
			for _, r := range nonEmptyLines(params["rules"]) {
				lines = append(lines, " rule "+r)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- NAT ----
func regRouterNAT() {
	Register(Template{
		Code: "router_nat", Vendor: "huawei", DeviceType: "router", Category: "nat",
		Name:        "路由器 NAT（Easy IP/静态映射/服务器映射）",
		Description: "NAT地址转换：Easy IP 复用出接口、静态NAT、内网服务器映射",
		Fields: []Field{
			field("out_if", "出接口（如 GigabitEthernet0/0/1）", "text", true),
			{Key: "nat_type", Label: "NAT类型", Type: "select", Required: true, Options: []string{"easy-ip", "static", "server"}},
			field("acl_num", "内网ACL编号（easy-ip用，如2000）", "number", false),
			field("inside_net", "内网网段（static用，如192.168.1.0/24）", "network", false),
			field("public_ip", "公网地址（static/server用）", "ip", false),
			field("global_if", "绑定出接口（static/server用）", "text", false),
			field("server_proto", "协议（server用）", "select", false),
			field("server_port", "外部端口（server用）", "number", false),
			field("server_in_ip", "内网服务器IP（server用）", "ip", false),
			field("server_in_port", "内网服务器端口（server用）", "number", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			outIf := p(params, "out_if", "GigabitEthernet0/0/1")
			switch p(params, "nat_type", "easy-ip") {
			case "static":
				lines = append(lines, fmt.Sprintf("nat static global %s inside %s", p(params, "public_ip", ""), p(params, "inside_net", "")))
				lines = append(lines, fmt.Sprintf("interface %s", outIf))
				lines = append(lines, " nat static enable")
				lines = append(lines, "quit")
			case "server":
				lines = append(lines, fmt.Sprintf("interface %s", outIf))
				lines = append(lines, fmt.Sprintf(" nat server protocol %s global %s %s inside %s %s",
					p(params, "server_proto", "tcp"), p(params, "public_ip", ""), p(params, "server_port", "80"),
					p(params, "server_in_ip", ""), p(params, "server_in_port", "80")))
				lines = append(lines, "quit")
			default: // easy-ip
				if acl := p(params, "acl_num", ""); acl != "" {
					lines = append(lines, fmt.Sprintf("interface %s", outIf))
					lines = append(lines, fmt.Sprintf(" nat outbound %s", acl))
					lines = append(lines, "quit")
				}
			}
			return lines
		},
	})
}

// ---- OSPF ----
func regRouterOSPF() {
	Register(Template{
		Code: "router_ospf", Vendor: "huawei", DeviceType: "router", Category: "ospf",
		Name: "路由器 OSPF", Description: "OSPF动态路由协议配置（进程/区域/网段宣告）",
		Fields: []Field{
			field("process", "进程号", "number", true),
			field("router_id", "Router ID", "ip", true),
			field("area", "区域号", "number", true),
			field("networks", "宣告网段列表（每行一条 CIDR）", "list", true),
			field("auth", "认证模式", "select", false),
			field("auth_key", "认证密钥", "password", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("ospf %s router-id %s", p(params, "process", "1"), p(params, "router_id", "1.1.1.1")))
			lines = append(lines, fmt.Sprintf(" area %s", p(params, "area", "0")))
			for _, n := range nonEmptyLines(params["networks"]) {
				lines = append(lines, "  network "+n)
			}
			if p(params, "auth", "") != "" {
				lines = append(lines, fmt.Sprintf("  authentication-mode %s", p(params, "auth", "md5")))
				lines = append(lines, fmt.Sprintf("  authentication-key %s", p(params, "auth_key", "")))
			}
			lines = append(lines, "quit", "quit")
			return lines
		},
	})
}

// ---- BGP ----
func regRouterBGP() {
	Register(Template{
		Code: "router_bgp", Vendor: "huawei", DeviceType: "router", Category: "bgp",
		Name: "路由器 BGP", Description: "BGP 路由协议配置（AS/对等体/宣告网段）",
		Fields: []Field{
			field("as_num", "本地AS号", "number", true),
			field("router_id", "Router ID", "ip", true),
			field("peers", "对等体列表（每行：IP 或 IP AS，如 10.0.0.2 65002）", "list", false),
			field("networks", "宣告网段列表（每行一条 CIDR）", "list", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("bgp %s", p(params, "as_num", "65001")))
			lines = append(lines, fmt.Sprintf(" router-id %s", p(params, "router_id", "1.1.1.1")))
			for _, n := range nonEmptyLines(params["peers"]) {
				parts := strings.Fields(n)
				if len(parts) >= 1 {
					lines = append(lines, fmt.Sprintf(" peer %s as-number %s", parts[0], peerAS(parts)))
				}
			}
			for _, n := range nonEmptyLines(params["networks"]) {
				lines = append(lines, " network "+n)
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

func peerAS(parts []string) string {
	if len(parts) >= 2 {
		return parts[1]
	}
	return "65002"
}

// ---- ISIS ----
func regRouterISIS() {
	Register(Template{
		Code: "router_isis", Vendor: "huawei", DeviceType: "router", Category: "isis",
		Name: "路由器 IS-IS", Description: "IS-IS 配置（进程/System ID/区域地址）",
		Fields: []Field{
			field("process", "进程号", "number", true),
			field("system_id", "System ID（如 0000.0000.0001）", "text", true),
			field("net_entity", "NET地址（如 49.0001.0000.0000.0001.00）", "text", true),
			field("level", "级别", "select", true),
			field("interfaces", "接口列表（每行一个接口名）", "list", true),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("isis %s", p(params, "process", "1")))
			lines = append(lines, fmt.Sprintf(" network-entity %s", p(params, "net_entity", "")))
			lines = append(lines, fmt.Sprintf(" is-name %s", p(params, "system_id", "")))
			lines = append(lines, "quit")
			for _, itf := range nonEmptyLines(params["interfaces"]) {
				lines = append(lines, fmt.Sprintf("interface %s", itf))
				lines = append(lines, fmt.Sprintf(" isis enable %s", p(params, "process", "1")))
				if lv := p(params, "level", "level-2"); lv != "" {
					lines = append(lines, fmt.Sprintf(" isis circuit-level %s", lv))
				}
				lines = append(lines, "quit")
			}
			return lines
		},
	})
}

// ---- MPLS ----
func regRouterMPLS() {
	Register(Template{
		Code: "router_mpls", Vendor: "huawei", DeviceType: "router", Category: "mpls",
		Name: "路由器 MPLS", Description: "MPLS 基础配置（LSR-ID、MPLS启用）",
		Fields: []Field{
			field("lsr_id", "LSR ID", "ip", true),
			field("interfaces", "启用MPLS的接口列表（每行一个）", "list", true),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "mpls lsr-id "+p(params, "lsr_id", ""))
			lines = append(lines, "mpls")
			lines = append(lines, "quit")
			for _, itf := range nonEmptyLines(params["interfaces"]) {
				lines = append(lines, fmt.Sprintf("interface %s", itf))
				lines = append(lines, " mpls")
				lines = append(lines, "quit")
			}
			return lines
		},
	})
}

// ---- MPLS VPN ----
func regRouterMPLSVPN() {
	Register(Template{
		Code: "router_mpls_vpn", Vendor: "huawei", DeviceType: "router", Category: "mpls_vpn",
		Name: "路由器 MPLS VPN", Description: "MPLS L3VPN 配置（VPN实例/RD/RT/绑定接口）",
		Fields: []Field{
			field("vpn_name", "VPN实例名", "text", true),
			field("rd", "RD（如 100:1）", "text", true),
			field("vpn_target", "VPN Target（如 100:1）", "text", true),
			field("interfaces", "绑定接口列表（每行一个）", "list", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			name := p(params, "vpn_name", "vpn1")
			lines = append(lines, fmt.Sprintf("ip vpn-instance %s", name))
			lines = append(lines, " ipv4-family")
			lines = append(lines, fmt.Sprintf("  route-distinguisher %s", p(params, "rd", "100:1")))
			lines = append(lines, fmt.Sprintf("  vpn-target %s both", p(params, "vpn_target", "100:1")))
			lines = append(lines, "quit")
			for _, itf := range nonEmptyLines(params["interfaces"]) {
				lines = append(lines, fmt.Sprintf("interface %s", itf))
				lines = append(lines, fmt.Sprintf(" ip binding vpn-instance %s", name))
				lines = append(lines, "quit")
			}
			return lines
		},
	})
}

// ---- 静态路由 ----
func regRouterStatic() {
	Register(Template{
		Code: "router_static", Vendor: "huawei", DeviceType: "router", Category: "static_route",
		Name: "路由器 静态路由", Description: "静态路由配置",
		Fields: []Field{
			fieldWithPlaceholder("dest", "目标网段", "network", true, "如 10.10.0.0"),
			field("mask", "掩码", "text", true),
			fieldWithPlaceholder("nexthop", "下一跳地址", "network", true, "如 192.168.1.1"),
			fieldOpt("preference", "优先级（可选）", "number", false, ""),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			dest := strings.TrimSpace(params["dest"])
			mask := strings.TrimSpace(params["mask"])
			nh := strings.TrimSpace(params["nexthop"])
			if dest == "" || nh == "" {
				return lines
			}
			line := fmt.Sprintf("ip route-static %s %s %s", dest, mask, nh)
			if p := strings.TrimSpace(params["preference"]); p != "" && p != "0" {
				line += " preference " + p
			}
			lines = append(lines, line)
			return lines
		},
	})
}

// ---- 策略路由 ----
func regRouterPolicyRoute() {
	Register(Template{
		Code: "router_policy_route", Vendor: "huawei", DeviceType: "router", Category: "policy_route",
		Name: "路由器 策略路由", Description: "基于策略的路由（流量分类器+行为+应用接口）",
		Fields: []Field{
			field("name", "策略名", "text", true),
			field("match_acl", "匹配ACL编号", "number", true),
			field("action", "动作（下一跳/出接口）", "select", true),
			field("next_hop", "下一跳IP", "ip", false),
			field("out_if", "出接口", "text", false),
			field("apply_if", "应用接口", "text", true),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			name := p(params, "name", "pbr1")
			lines = append(lines, fmt.Sprintf("traffic classifier %s", name))
			lines = append(lines, fmt.Sprintf(" if-match acl %s", p(params, "match_acl", "")))
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf("traffic behavior %s", name))
			if p(params, "action", "nh") == "nh" {
				lines = append(lines, fmt.Sprintf(" redirect ip-nexthop %s", p(params, "next_hop", "")))
			} else {
				lines = append(lines, fmt.Sprintf(" redirect ip-nexthop %s", p(params, "next_hop", "")))
			}
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf("traffic policy %s", name))
			lines = append(lines, fmt.Sprintf(" classifier %s behavior %s", name, name))
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf("interface %s", p(params, "apply_if", "")))
			lines = append(lines, fmt.Sprintf(" traffic-policy %s inbound", name))
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 路由策略 ----
func regRouterRoutingPolicy() {
	Register(Template{
		Code: "router_routing_policy", Vendor: "huawei", DeviceType: "router", Category: "routing_policy",
		Name: "路由器 路由策略", Description: "Route-Policy 配置（if-match / apply）",
		Fields: []Field{
			field("name", "策略名", "text", true),
			field("node", "节点号", "number", true),
			field("mode", "节点模式", "select", true),
			field("match_acl", "匹配ACL编号", "number", false),
			field("match_prefix", "匹配前缀列表名", "text", false),
			field("apply_metric", "设置度量值", "number", false),
			field("apply_tag", "设置Tag", "number", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("route-policy %s %s node %s",
				p(params, "name", "rp1"), p(params, "mode", "permit"), p(params, "node", "10")))
			if acl := p(params, "match_acl", ""); acl != "" {
				lines = append(lines, fmt.Sprintf(" if-match acl %s", acl))
			}
			if pre := p(params, "match_prefix", ""); pre != "" {
				lines = append(lines, fmt.Sprintf(" if-match ip-prefix %s", pre))
			}
			if m := p(params, "apply_metric", ""); m != "" {
				lines = append(lines, fmt.Sprintf(" apply cost %s", m))
			}
			if t := p(params, "apply_tag", ""); t != "" {
				lines = append(lines, fmt.Sprintf(" apply tag %s", t))
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

// ---- 认证（本地/远程） ----
func regRouterAuth() {
	Register(Template{
		Code: "router_auth_local", Vendor: "huawei", DeviceType: "router", Category: "auth",
		Name: "路由器 本地认证", Description: "本地用户认证配置",
		Fields: []Field{
			field("username", "用户名", "text", true),
			field("password", "密码", "password", true),
			field("privilege", "权限级别", "number", false),
			field("service", "服务类型", "select", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("local-user %s password cipher %s", p(params, "username", ""), p(params, "password", "")))
			lines = append(lines, fmt.Sprintf("local-user %s privilege level %s", p(params, "username", ""), p(params, "privilege", "15")))
			lines = append(lines, fmt.Sprintf("local-user %s service-type %s", p(params, "username", ""), p(params, "service", "telnet ssh")))
			return lines
		},
	})
	Register(Template{
		Code: "router_auth_remote", Vendor: "huawei", DeviceType: "router", Category: "auth",
		Name: "路由器 远程认证（RADIUS/TACACS）", Description: "远程认证服务器接入配置",
		Fields: []Field{
			field("server_type", "认证类型", "select", true),
			field("server_ip", "服务器IP", "ip", true),
			field("server_port", "端口", "number", true),
			field("shared_key", "共享密钥", "password", true),
			field("scheme", "方案名", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("radius-server template %s", p(params, "scheme", "radius1")))
			lines = append(lines, fmt.Sprintf(" radius-server authentication %s %s", p(params, "server_ip", ""), p(params, "server_port", "1812")))
			lines = append(lines, fmt.Sprintf(" radius-server shared-key cipher %s", p(params, "shared_key", "")))
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf("aaa"), fmt.Sprintf(" authentication-scheme default"), fmt.Sprintf(" radius-server group %s", p(params, "scheme", "radius1")), "quit")
			return lines
		},
	})
}

// ---- VRRP ----
func regRouterVRRP() {
	Register(Template{
		Code: "router_vrrp", Vendor: "huawei", DeviceType: "router", Category: "vrrp",
		Name: "路由器 VRRP", Description: "VRRP 虚拟路由冗余配置",
		Fields: []Field{
			field("interface", "接口", "text", true),
			field("vrid", "VRID", "number", true),
			field("virtual_ip", "虚拟IP", "ip", true),
			field("priority", "优先级", "number", false),
			field("preempt", "是否抢占", "select", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, fmt.Sprintf("interface %s", p(params, "interface", "")))
			lines = append(lines, fmt.Sprintf(" vrrp vrid %s virtual-ip %s", p(params, "vrid", "1"), p(params, "virtual_ip", "")))
			if pr := p(params, "priority", ""); pr != "" {
				lines = append(lines, fmt.Sprintf(" vrrp vrid %s priority %s", p(params, "vrid", "1"), pr))
			}
			if p(params, "preempt", "true") == "false" {
				lines = append(lines, fmt.Sprintf(" vrrp vrid %s preempt-mode disable", p(params, "vrid", "1")))
			}
			lines = append(lines, "quit")
			return lines
		},
	})
}

