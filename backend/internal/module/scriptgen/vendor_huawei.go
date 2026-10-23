package scriptgen

import (
	"fmt"
	"strings"
)

func buildHuawei(code string, m map[string]string) []string {
	switch code {
	case "router_acl", "sw_acl":
		act := p(m, "action", "permit")
		proto := p(m, "proto", "ip")
		num := p(m, "num", "3000")
		acltype := p(m, "acltype", "三层")
		src := p(m, "src", "0.0.0.0/0")
		srcAddr := strings.Split(src, "/")[0]
		srcWild := wildOf(src)
		entry := "advanced"
		switch acltype {
		case "基础":
			entry = "basic"
		case "二层":
			entry = "mac"
		case "自定义":
			entry = "user-defined"
		}
		if acltype == "二层" {
			srcMac := p(m, "srcmac", "0000-0000-0000")
			return []string{fmt.Sprintf("acl mac %s", num), fmt.Sprintf(" rule 5 %s source-mac %s ffff-ffff-ffff", act, srcMac)}
		}
		if acltype == "基础" {
			return []string{
				fmt.Sprintf("acl basic %s", num),
				fmt.Sprintf(" rule 5 %s source %s %s", act, srcAddr, srcWild),
			}
		}
		rule := fmt.Sprintf(" rule 5 %s %s source %s %s", act, proto, srcAddr, srcWild)
		if dst := p(m, "dst", ""); dst != "" {
			rule += fmt.Sprintf(" destination %s %s", strings.Split(dst, "/")[0], wildOf(dst))
		} else {
			rule += " destination any"
		}
		if port := p(m, "dport", ""); port != "" && (proto == "tcp" || proto == "udp") {
			rule += fmt.Sprintf(" destination-port eq %s", port)
		}
		return []string{fmt.Sprintf("acl %s %s", entry, num), rule}
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
			fmt.Sprintf(" area %s", p(m, "area", "0")),
			fmt.Sprintf("  network %s %s", net, mask),
		}
	case "router_static", "sw_static":
		vpnPre := ""
		if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
			vpnPre = fmt.Sprintf(" vpn-instance %s", p(m, "vpn", ""))
		}
		return []string{fmt.Sprintf("ip route-static%s %s %s %s", vpnPre, p(m, "dest", "0.0.0.0"), p(m, "mask", "0"), p(m, "nh", ""))}
	case "com_ssh":
		user, pass, bits, vty := p(m, "user", "admin"), p(m, "pass", ""), p(m, "rsabits", "2048"), p(m, "vty", "0 4")
		return []string{
			fmt.Sprintf("rsa local-key-pair create modulus %s", bits),
			"stelnet server enable",
			fmt.Sprintf("local-user %s password cipher %s", user, pass),
			fmt.Sprintf("local-user %s privilege level %s", user, p(m, "pri", "15")),
			fmt.Sprintf("local-user %s service-type ssh", user),
			fmt.Sprintf("user-interface vty %s", vty),
			" authentication-mode aaa",
			" protocol inbound ssh",
		}
	case "com_user":
		user, pass, pri := p(m, "user", "admin"), p(m, "pass", ""), p(m, "pri", "15")
		return []string{
			fmt.Sprintf("local-user %s password cipher %s", user, pass),
			fmt.Sprintf("local-user %s privilege level %s", user, pri),
			fmt.Sprintf("local-user %s service-type ssh", user),
		}
	case "com_log":
		return []string{"info-center enable", fmt.Sprintf("info-center loghost %s", p(m, "logip", ""))}
	case "com_snmp":
		return buildHuaweiSnmp(m)
	case "sw_vlan":
		return buildHuaweiVlan(m)
	case "sw_access":
		return buildHuaweiAccess(m)
	case "sw_trunk":
		return buildHuaweiTrunk(m)
	case "sw_dhcp":
		return buildHuaweiDhcp(m)
	case "sw_stp":
		return buildHuaweiStp(m)
	case "router_bgp":
		return buildHuaweiBgp(m)
	case "router_isis":
		vpnLine := ""
		if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
			vpnLine = fmt.Sprintf(" vpn-instance %s", p(m, "vpn", ""))
		}
		return []string{
			fmt.Sprintf("isis%s %s", vpnLine, p(m, "pid", "1")),
			fmt.Sprintf(" network-entity %s", p(m, "net", "49.0001.0000.0000.0001.00")),
			fmt.Sprintf(" router-id %s", p(m, "router_id", p(m, "rid", "1.1.1.1"))),
			fmt.Sprintf(" is-level %s", p(m, "level", "level-2")),
		}
	case "mpls_vpn":
		return []string{
			"mpls lsr-id 1.1.1.1",
			"mpls",
			"mpls ldp",
			fmt.Sprintf("ip vpn-instance %s", p(m, "vpn_name", "vpna")),
			fmt.Sprintf(" route-distinguisher %s", p(m, "rd", "100:1")),
			fmt.Sprintf(" vpn-target %s export-extcommunity", p(m, "rt_out", "100:1")),
			fmt.Sprintf(" vpn-target %s import-extcommunity", p(m, "rt_in", "100:1")),
			fmt.Sprintf("interface %s", p(m, "iface", "GigabitEthernet0/0/0")),
			fmt.Sprintf(" ip binding vpn-instance %s", p(m, "vpn_name", "vpna")),
			fmt.Sprintf(" ip address %s", p(m, "ip", "10.0.0.1 255.255.255.0")),
			" mpls",
			" mpls ldp",
		}
	case "route_policy":
		name := p(m, "name", "RP1")
		seq := p(m, "seq", "10")
		lines := []string{}
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf("ip ip-prefix %s index %s %s %s %s", p(m, "prefix", ""), p(m, "pfx_index", "10"), p(m, "pfx_action", "permit"), p(m, "pfx_net", ""), p(m, "pfx_masklen", "")))
		}
		lines = append(lines, fmt.Sprintf("route-policy %s permit node %s", name, seq))
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf(" if-match ip-prefix %s", p(m, "prefix", "")))
		}
		return lines
	case "router_policy":
		return []string{
			"traffic classifier c1 operator or",
			fmt.Sprintf(" if-match acl %s", p(m, "acl", "3000")),
			"traffic behavior b1",
			fmt.Sprintf(" redirect ip-nexthop %s", p(m, "nh", "")),
			"traffic policy p1",
			" classifier c1 behavior b1",
			fmt.Sprintf("interface %s", p(m, "iface", "")),
			" traffic-policy p1 inbound",
		}
	}
	return nil
}
