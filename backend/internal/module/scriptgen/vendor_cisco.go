package scriptgen

import (
	"fmt"
	"strings"
)

func buildCisco(code string, m map[string]string) []string {
	switch code {
	case "router_acl", "sw_acl":
		num := p(m, "num", "100")
		act := p(m, "action", "permit")
		proto := p(m, "proto", "ip")
		src := p(m, "src", "0.0.0.0/0")
		srcAddr := strings.Split(src, "/")[0]
		wild := wildOf(src)
		if acltype := p(m, "acltype", "三层"); acltype == "基础" {
			return []string{fmt.Sprintf("access-list %s %s %s %s", num, act, srcAddr, wild)}
		}
		if acltype := p(m, "acltype", "三层"); acltype == "二层" {
			srcMac := p(m, "srcmac", "0000.0000.0000")
			return []string{fmt.Sprintf("mac access-list extended %s", num), fmt.Sprintf(" %s host %s any", act, srcMac)}
		}
		rule := fmt.Sprintf("%s %s %s %s", act, proto, srcAddr, wild)
		if dst := p(m, "dst", ""); dst != "" {
			rule += fmt.Sprintf(" %s %s", strings.Split(dst, "/")[0], wildOf(dst))
		} else {
			rule += " any"
		}
		return []string{fmt.Sprintf("ip access-list extended %s", num), " " + rule}
	case "router_nat":
		return []string{
			fmt.Sprintf("interface %s", p(m, "outif", "GigabitEthernet0/0")),
			" ip nat outside",
			fmt.Sprintf("ip nat inside source list %s interface %s overload", p(m, "acl", "NAT"), p(m, "outif", "GigabitEthernet0/0")),
		}
	case "router_ospf":
		net := p(m, "net", "0.0.0.0")
		vrf := ""
		if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
			vrf = fmt.Sprintf(" vrf %s", p(m, "vpn", ""))
		}
		return []string{
			fmt.Sprintf("router ospf %s%s", p(m, "process", "1"), vrf),
			fmt.Sprintf(" router-id %s", p(m, "rid", "1.1.1.1")),
			fmt.Sprintf(" network %s %s area %s", net, wildcardOf(p(m, "netmask", "255.255.255.0")), p(m, "area", "0")),
		}
	case "router_static", "sw_static":
		vrf := ""
		if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
			vrf = fmt.Sprintf(" vrf %s", p(m, "vpn", ""))
		}
		return []string{fmt.Sprintf("ip route%s %s %s %s", vrf, p(m, "dest", "0.0.0.0"), p(m, "mask", "0.0.0.0"), p(m, "nh", ""))}
	case "com_ssh":
		user, pass, vty := p(m, "user", "admin"), p(m, "pass", ""), p(m, "vty", "0 15")
		return []string{
			"crypto key generate rsa modulus 2048",
			"ip domain name local",
			fmt.Sprintf("username %s password 0 %s", user, pass),
			"line vty " + vty,
			" transport input ssh",
			" login local",
		}
	case "com_user":
		return []string{fmt.Sprintf("username %s password 0 %s", p(m, "user", "admin"), p(m, "pass", ""))}
	case "com_log":
		return []string{fmt.Sprintf("logging host %s", p(m, "logip", ""))}
	case "com_snmp":
		host := p(m, "host", "")
		lines := []string{fmt.Sprintf("snmp-server community %s RO", p(m, "community", "public"))}
		if host != "" {
			lines = append(lines, fmt.Sprintf("snmp-server host %s version 2c %s", host, p(m, "community", "public")))
		}
		return lines
	case "sw_vlan":
		lines := []string{}
		for _, v := range splitList(p(m, "vlan", "")) {
			lines = append(lines, "vlan "+v, " name VLAN_"+v)
		}
		if vif := p(m, "vlanif", ""); vif != "" && p(m, "ip", "") != "" {
			lines = append(lines, fmt.Sprintf("interface Vlan%s", vif), fmt.Sprintf(" ip address %s", p(m, "ip", "")))
		}
		return lines
	case "sw_access":
		lines := []string{}
		for _, ifa := range splitList(p(m, "iface", "")) {
			lines = append(lines, fmt.Sprintf("interface %s", ifa), " switchport mode access", fmt.Sprintf(" switchport access vlan %s", p(m, "vlan", "1")))
		}
		return lines
	case "sw_trunk":
		lines := []string{}
		for _, ifa := range splitList(p(m, "iface", "")) {
			lines = append(lines, fmt.Sprintf("interface %s", ifa), " switchport mode trunk", fmt.Sprintf(" switchport trunk allowed vlan %s", p(m, "vlans", "")))
		}
		return lines
	case "sw_dhcp":
		return []string{
			"ip dhcp excluded-address 10.0.0.1",
			fmt.Sprintf("ip dhcp pool %s", p(m, "pool", "POOL1")),
			fmt.Sprintf(" network %s %s", p(m, "net", "10.0.0.0"), p(m, "netmask", "255.255.255.0")),
			fmt.Sprintf(" default-router %s", p(m, "gw", "")),
		}
	case "sw_stp":
		return []string{"spanning-tree mode rapid-pvst", fmt.Sprintf("spanning-tree vlan %s root %s", p(m, "vlan", "1"), p(m, "root", "primary"))}
	case "router_bgp":
		return []string{
			fmt.Sprintf("router bgp %s", p(m, "asn", "65001")),
			fmt.Sprintf(" bgp router-id %s", p(m, "rid", "1.1.1.1")),
			fmt.Sprintf(" neighbor %s remote-as %s", p(m, "peer_ip", ""), p(m, "peer_as", "65002")),
		}
	case "route_policy":
		name := p(m, "name", "RP1")
		seq := p(m, "seq", "10")
		lines := []string{}
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf("ip prefix-list %s seq %s %s %s/%s", p(m, "prefix", ""), seq, p(m, "pfx_action", "permit"), p(m, "pfx_net", ""), p(m, "pfx_masklen", "24")))
		}
		lines = append(lines, fmt.Sprintf("route-map %s permit %s", name, seq))
		if p(m, "enable_prefix", "") == "on" && p(m, "prefix", "") != "" {
			lines = append(lines, fmt.Sprintf(" match ip address prefix-list %s", p(m, "prefix", "")))
		}
		return lines
	case "router_policy":
		return []string{
			fmt.Sprintf("access-list %s permit ip any any", p(m, "acl", "100")),
			"route-map P permit 10",
			fmt.Sprintf(" match ip address %s", p(m, "acl", "100")),
			fmt.Sprintf(" set ip next-hop %s", p(m, "nh", "")),
			fmt.Sprintf("interface %s", p(m, "iface", "")),
			" ip policy route-map P",
		}
	case "mpls_vpn":
		return buildHuawei(code, m)
	}
	return nil
}
