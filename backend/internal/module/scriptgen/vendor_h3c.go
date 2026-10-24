package scriptgen

import (
	"fmt"
	"strings"
)

func buildH3c(code string, m map[string]string) []string {
	switch code {
	case "router_acl", "sw_acl":
		act := p(m, "action", "permit")
		num := p(m, "num", "3000")
		acltype := p(m, "acltype", "三层")
		src := p(m, "src", "0.0.0.0/0")
		srcAddr := strings.Split(src, "/")[0]
		wild := wildOf(src)
		if acltype == "二层" {
			srcMac := p(m, "srcmac", "0000-0000-0000")
			return []string{fmt.Sprintf("acl mac %s", num), fmt.Sprintf(" rule 0 %s source-mac %s ffff-ffff-ffff", act, srcMac)}
		}
		if acltype == "基础" {
			return []string{fmt.Sprintf("acl basic %s", num), fmt.Sprintf(" rule 0 %s source %s %s", act, srcAddr, wild)}
		}
		rule := fmt.Sprintf(" rule 0 %s %s source %s %s", act, p(m, "proto", "ip"), srcAddr, wild)
		if dst := p(m, "dst", ""); dst != "" {
			rule += fmt.Sprintf(" destination %s %s", strings.Split(dst, "/")[0], wildOf(dst))
		} else {
			rule += " destination any"
		}
		return []string{fmt.Sprintf("acl advanced %s", num), rule}
	case "router_nat":
		return []string{
			fmt.Sprintf("interface %s", p(m, "outif", "GigabitEthernet0/0/1")),
			fmt.Sprintf(" nat outbound %s", p(m, "acl", "2000")),
		}
	case "router_ospf":
		net := p(m, "net", "0.0.0.0")
		mask := p(m, "netmask", "255.255.255.0")
		vpnLine := ""
		if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
			vpnLine = fmt.Sprintf(" vpn-instance %s", p(m, "vpn", ""))
		}
		return []string{
			fmt.Sprintf("ospf%s %s router-id %s", vpnLine, p(m, "process", "1"), p(m, "rid", "1.1.1.1")),
			fmt.Sprintf(" area 0.0.0.%s", p(m, "area", "0")),
			fmt.Sprintf("  network %s %s", net, wildcardOf(mask)),
		}
	case "router_static", "sw_static":
		vpnPre := ""
		if vpn := p(m, "vpn", ""); vpn != "" {
			vpnPre = fmt.Sprintf(" vpn-instance %s", vpn)
		}
		return []string{fmt.Sprintf("ip route-static%s %s %s %s", vpnPre, p(m, "dest", "0.0.0.0"), p(m, "mask", "0"), p(m, "nh", ""))}
	case "com_ssh":
		user, pass, vty := p(m, "user", "admin"), p(m, "pass", ""), p(m, "vty", "0 63")
		return []string{
			"public-key local create rsa",
			"ssh server enable",
			fmt.Sprintf("local-user %s class manage", user),
			fmt.Sprintf(" password simple %s", pass),
			" service-type ssh",
			" authorization-attribute user-role network-admin",
			fmt.Sprintf("line vty %s", vty),
			" authentication-mode scheme",
			" protocol inbound ssh",
		}
	case "com_user":
		user, pass := p(m, "user", "admin"), p(m, "pass", "")
		return []string{
			fmt.Sprintf("local-user %s class manage", user),
			fmt.Sprintf(" password simple %s", pass),
			" service-type ssh",
			" authorization-attribute user-role network-admin",
		}
	case "com_log":
		return []string{"info-center enable", fmt.Sprintf("info-center loghost %s", p(m, "logip", ""))}
	case "com_snmp":
		return buildHuaweiSnmp(m)
	case "sw_vlan":
		// 华三 vlan 命令不支持逗号列表：逐条 vlan X、连续范围 vlan X to Y
		lines := []string{}
		for _, item := range splitList(p(m, "vlan", "")) {
			if rg := strings.Split(item, "-"); len(rg) == 2 && rg[0] != "" && rg[1] != "" {
				lines = append(lines, fmt.Sprintf("vlan %s to %s", strings.TrimSpace(rg[0]), strings.TrimSpace(rg[1])))
			} else {
				lines = append(lines, "vlan "+strings.TrimSpace(item))
			}
		}
		if vif := p(m, "vlanif", ""); vif != "" && p(m, "ip", "") != "" {
			lines = append(lines, fmt.Sprintf("interface Vlan-interface%s", vif), fmt.Sprintf(" ip address %s", p(m, "ip", "")))
		}
		return lines
	case "sw_access":
		lines := []string{}
		for _, ifa := range splitList(p(m, "iface", "")) {
			lines = append(lines, fmt.Sprintf("interface %s", ifa), " port link-type access", fmt.Sprintf(" port access vlan %s", p(m, "vlan", "1")))
		}
		return lines
	case "sw_trunk":
		lines := []string{}
		for _, ifa := range splitList(p(m, "iface", "")) {
			lines = append(lines, fmt.Sprintf("interface %s", ifa), " port link-type trunk", fmt.Sprintf(" port trunk permit vlan %s", p(m, "vlans", "")))
		}
		return lines
	case "sw_dhcp":
		// 华三 Comware DHCP 服务器视图：dhcp server ip-pool（与华为 ip pool 不同）
		addr, mask := cidrMask(p(m, "net", ""))
		pool := p(m, "pool", "pool1")
		lines := []string{"dhcp enable", fmt.Sprintf("dhcp server ip-pool %s", pool), fmt.Sprintf(" network %s mask %s", addr, mask), fmt.Sprintf(" gateway-list %s", p(m, "gw", ""))}
		if dns := p(m, "dns", ""); dns != "" {
			lines = append(lines, " dns-list "+strings.ReplaceAll(dns, ",", " "))
		}
		return lines
	case "sw_stp":
		return buildHuaweiStp(m)
	case "router_bgp":
		return buildHuaweiBgp(m)
	case "router_isis":
		return buildHuawei(code, m)
	case "mpls_vpn":
		return buildHuawei(code, m)
	case "route_policy":
		name := p(m, "name", "RP1")
		seq := p(m, "seq", "10")
		lines := []string{}
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf("ip prefix-list %s index %s %s %s %s", p(m, "prefix", ""), p(m, "pfx_index", "10"), p(m, "pfx_action", "permit"), p(m, "pfx_net", ""), p(m, "pfx_masklen", "24")))
		}
		lines = append(lines, fmt.Sprintf("route-policy %s permit node %s", name, seq))
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf(" if-match ip-prefix %s", p(m, "prefix", "")))
		}
		return lines
	case "router_policy":
		// 华三 Comware 行为视图重定向：apply next-hop（华为为 redirect ip-nexthop）
		return []string{
			"traffic classifier c1 operator or",
			fmt.Sprintf(" if-match acl %s", p(m, "acl", "3000")),
			"traffic behavior b1",
			fmt.Sprintf(" apply next-hop %s", p(m, "nh", "")),
			"traffic policy p1",
			" classifier c1 behavior b1",
			fmt.Sprintf("interface %s", p(m, "iface", "")),
			" traffic-policy p1 inbound",
		}
	}
	return nil
}
