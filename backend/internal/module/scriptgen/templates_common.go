package scriptgen

import (
	"fmt"
	"strings"
)

// ---- 共享渲染器与字段 ----

func regOspfFields() []Field {
	return []Field{
		field("process", "进程号", "number", true), field("router_id", "Router ID", "ip", true),
		field("area", "区域号", "number", true), field("networks", "宣告网段列表（每行一条 CIDR）", "list", true),
	}
}
func renderOSPF(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor)}
	lines = append(lines, fmt.Sprintf("ospf %s router-id %s", p(params, "process", "1"), p(params, "router_id", "1.1.1.1")))
	lines = append(lines, fmt.Sprintf(" area %s", p(params, "area", "0")))
	for _, n := range nonEmptyLines(params["networks"]) {
		lines = append(lines, "  network "+n)
	}
	lines = append(lines, "quit", "quit")
	return lines
}

func regBgpFields() []Field {
	return []Field{
		field("as_num", "本地AS号", "number", true), field("router_id", "Router ID", "ip", true),
		field("peers", "对等体列表（每行：IP AS）", "list", false),
		field("networks", "宣告网段列表（每行一条 CIDR）", "list", false),
	}
}
func renderBGP(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor)}
	lines = append(lines, "bgp "+p(params, "as_num", "65001"))
	lines = append(lines, " router-id "+p(params, "router_id", "1.1.1.1"))
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
}

func regIsisFields() []Field {
	return []Field{
		field("process", "进程号", "number", true), field("system_id", "System ID", "text", true),
		field("net_entity", "NET地址", "text", true), field("interfaces", "接口列表（每行一个）", "list", true),
	}
}
func renderISIS(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor)}
	lines = append(lines, "isis "+p(params, "process", "1"))
	lines = append(lines, " network-entity "+p(params, "net_entity", ""))
	lines = append(lines, " is-name "+p(params, "system_id", ""))
	lines = append(lines, "quit")
	for _, itf := range nonEmptyLines(params["interfaces"]) {
		lines = append(lines, "interface "+itf)
		lines = append(lines, " isis enable "+p(params, "process", "1"))
		lines = append(lines, "quit")
	}
	return lines
}

func regMplsFields() []Field {
	return []Field{field("lsr_id", "LSR ID", "ip", true), field("interfaces", "启用MPLS的接口列表", "list", true)}
}
func renderMPLS(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor), "mpls lsr-id " + p(params, "lsr_id", ""), "mpls", "quit"}
	for _, itf := range nonEmptyLines(params["interfaces"]) {
		lines = append(lines, "interface "+itf, " mpls", "quit")
	}
	return lines
}

func regMplsVpnFields() []Field {
	return []Field{
		field("vpn_name", "VPN实例名", "text", true), field("rd", "RD（如 100:1）", "text", true),
		field("vpn_target", "VPN Target", "text", true), field("interfaces", "绑定接口列表", "list", false),
	}
}
func renderMPLSVPN(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor)}
	name := p(params, "vpn_name", "vpn1")
	lines = append(lines, "ip vpn-instance "+name)
	lines = append(lines, " ipv4-family")
	lines = append(lines, "  route-distinguisher "+p(params, "rd", "100:1"))
	lines = append(lines, "  vpn-target "+p(params, "vpn_target", "100:1")+" both")
	lines = append(lines, "quit")
	for _, itf := range nonEmptyLines(params["interfaces"]) {
		lines = append(lines, "interface "+itf, " ip binding vpn-instance "+name, "quit")
	}
	return lines
}

func staticRouteFields() []Field {
	return []Field{
		fieldWithPlaceholder("dest", "目标网段", "network", true, "如 10.10.0.0"),
		field("mask", "掩码", "text", true),
		fieldWithPlaceholder("nexthop", "下一跳地址", "network", true, "如 192.168.1.1"),
		fieldOpt("preference", "优先级（可选）", "number", false, ""),
	}
}

func renderStatic(params map[string]string, vendor string) []string {
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
}

func regPolicyRouteFields() []Field {
	return []Field{
		field("name", "策略名", "text", true), field("match_acl", "匹配ACL编号", "number", true),
		field("next_hop", "下一跳IP", "ip", true), field("apply_if", "应用接口", "text", true),
	}
}
func renderPolicyRoute(params map[string]string, vendor string) []string {
	name := p(params, "name", "pbr1")
	lines := []string{sysView(vendor)}
	lines = append(lines, "traffic classifier "+name)
	lines = append(lines, " if-match acl "+p(params, "match_acl", ""))
	lines = append(lines, "quit")
	lines = append(lines, "traffic behavior "+name)
	lines = append(lines, " redirect ip-nexthop "+p(params, "next_hop", ""))
	lines = append(lines, "quit")
	lines = append(lines, "traffic policy "+name)
	lines = append(lines, " classifier "+name+" behavior "+name)
	lines = append(lines, "quit")
	lines = append(lines, "interface "+p(params, "apply_if", ""))
	lines = append(lines, " traffic-policy "+name+" inbound")
	lines = append(lines, "quit")
	return lines
}

func regRoutingPolicyFields() []Field {
	return []Field{
		field("name", "策略名", "text", true), field("node", "节点号", "number", true),
		field("mode", "节点模式", "select", true), field("match_acl", "匹配ACL编号", "number", false),
		field("apply_metric", "设置度量值", "number", false), field("apply_tag", "设置Tag", "number", false),
	}
}
func renderRoutingPolicy(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor)}
	lines = append(lines, fmt.Sprintf("route-policy %s %s node %s",
		p(params, "name", "rp1"), p(params, "mode", "permit"), p(params, "node", "10")))
	if acl := p(params, "match_acl", ""); acl != "" {
		lines = append(lines, " if-match acl "+acl)
	}
	if m := p(params, "apply_metric", ""); m != "" {
		lines = append(lines, " apply cost "+m)
	}
	if t := p(params, "apply_tag", ""); t != "" {
		lines = append(lines, " apply tag "+t)
	}
	lines = append(lines, "quit")
	return lines
}

func renderVLAN(params map[string]string, vendor string) []string {
	return []string{sysView(vendor), "vlan batch " + vlanFormat(p(params, "vlans", ""))}
}

func renderVLANIF(params map[string]string, vendor string) []string {
	return []string{sysView(vendor), "interface Vlanif" + p(params, "vlan", ""), " ip address " + p(params, "ip", ""), "quit"}
}

func renderACL(params map[string]string, vendor string) []string {
	lines := []string{sysView(vendor), "acl number " + p(params, "acl_num", "3000")}
	for _, r := range nonEmptyLines(params["rules"]) {
		lines = append(lines, " rule "+r)
	}
	lines = append(lines, "quit")
	return lines
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}
