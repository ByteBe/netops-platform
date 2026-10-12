package scriptgen

import (
	"fmt"
	"strings"
)

func buildHuaweiSnmp(m map[string]string) []string {
	ver := p(m, "ver", "v2c")
	host := p(m, "host", "")
	lines := []string{"snmp-agent"}
	if ver == "v3" {
		sec := p(m, "secname", "monitor")
		lines = append(lines,
			"snmp-agent sys-info version v3",
			"snmp-agent group v3 g1 privacy",
			fmt.Sprintf("snmp-agent usm-user v3 %s g1", sec),
			fmt.Sprintf("snmp-agent usm-user v3 %s authentication-mode %s %s", sec, p(m, "authproto", "sha"), p(m, "authpass", "")),
			fmt.Sprintf("snmp-agent usm-user v3 %s privacy-mode %s %s", sec, p(m, "privproto", "aes128"), p(m, "privpass", "")))
		if host != "" {
			lines = append(lines, fmt.Sprintf("snmp-agent target-host trap address udp-domain %s udp-port 162 params securityname %s v3", host, sec))
		}
	} else {
		lines = append(lines,
			fmt.Sprintf("snmp-agent sys-info version %s", ver),
			fmt.Sprintf("snmp-agent community read %s", p(m, "community", "public")))
		if host != "" {
			lines = append(lines, fmt.Sprintf("snmp-agent target-host trap address udp-domain %s udp-port 162 params securityname %s", host, p(m, "community", "public")))
		}
	}
	return lines
}

func buildHuaweiVlan(m map[string]string) []string {
	vlans := p(m, "vlan", "")
	lines := []string{}
	if vlans != "" {
		lines = append(lines, "vlan batch "+vlans)
	}
	if vif := p(m, "vlanif", ""); vif != "" && p(m, "ip", "") != "" {
		lines = append(lines, fmt.Sprintf("interface Vlanif%s", vif), fmt.Sprintf(" ip address %s", p(m, "ip", "")))
	}
	return lines
}

func buildHuaweiAccess(m map[string]string) []string {
	ifaces := splitList(p(m, "iface", ""))
	vlan := p(m, "vlan", "1")
	lines := []string{}
	for _, ifa := range ifaces {
		lines = append(lines, fmt.Sprintf("interface %s", ifa), " port link-type access", fmt.Sprintf(" port default vlan %s", vlan))
	}
	return lines
}

func buildHuaweiTrunk(m map[string]string) []string {
	ifaces := splitList(p(m, "iface", ""))
	vlans := p(m, "vlans", "")
	lines := []string{}
	for _, ifa := range ifaces {
		lines = append(lines, fmt.Sprintf("interface %s", ifa), " port link-type trunk", fmt.Sprintf(" port trunk allow-pass vlan %s", vlans))
	}
	return lines
}

func buildHuaweiDhcp(m map[string]string) []string {
	addr, mask := cidrMask(p(m, "net", ""))
	pool := p(m, "pool", "pool1")
	lines := []string{"dhcp enable", fmt.Sprintf("ip pool %s", pool), fmt.Sprintf(" network %s %s", addr, mask), fmt.Sprintf(" gateway-list %s", p(m, "gw", ""))}
	if dns := p(m, "dns", ""); dns != "" {
		lines = append(lines, " dns-list "+dns)
	}
	return lines
}

func buildHuaweiStp(m map[string]string) []string {
	mode := p(m, "mode", "mstp")
	inst := p(m, "instance", "1")
	lines := []string{fmt.Sprintf("stp mode %s", mode)}
	if mode == "mstp" && p(m, "vlan", "") != "" {
		lines = append(lines, "stp region-configuration", fmt.Sprintf(" instance %s vlan %s", inst, p(m, "vlan", "")), " active region-configuration")
	}
	root := p(m, "root", "none")
	if root == "primary" {
		lines = append(lines, fmt.Sprintf("stp instance %s root primary", inst))
	}
	if root == "secondary" {
		lines = append(lines, fmt.Sprintf("stp instance %s root secondary", inst))
	}
	if root == "none" && p(m, "pri", "") != "" {
		lines = append(lines, fmt.Sprintf("stp instance %s priority %s", inst, p(m, "pri", "")))
	}
	return append(lines, "stp enable")
}

func buildHuaweiBgp(m map[string]string) []string {
	asn := p(m, "asn", "65001")
	rid := p(m, "rid", "1.1.1.1")
	peerAs := p(m, "peer_as", "65002")
	peerIps := splitList(p(m, "peer_ip", ""))
	lines := []string{fmt.Sprintf("bgp %s", asn), fmt.Sprintf(" router-id %s", rid)}
	for _, pip := range peerIps {
		lines = append(lines, fmt.Sprintf(" peer %s as-number %s", pip, peerAs))
		if ci := p(m, "conn_int", ""); ci != "" {
			lines = append(lines, fmt.Sprintf(" peer %s connect-interface %s", pip, ci))
		}
	}
	if p(m, "enable_net", "") == "on" {
		lines = append(lines, " ipv4-family unicast", fmt.Sprintf("  network %s %s", p(m, "network", ""), p(m, "netmask", "255.255.255.0")))
	}
	if p(m, "vpn4", "") == "on" {
		lines = append(lines, " ipv4-family vpnv4")
		for _, pip := range splitList(p(m, "vpn4_enable_peer", "")) {
			lines = append(lines, fmt.Sprintf("  peer %s enable", pip))
		}
		for _, pip := range splitList(p(m, "vpn4_adv_comm", "")) {
			lines = append(lines, fmt.Sprintf("  peer %s advertise-community", pip))
		}
		for _, pip := range splitList(p(m, "vpn4_next_hop_inv", "")) {
			lines = append(lines, fmt.Sprintf("  peer %s next-hop-invariable", pip))
		}
	}
	if p(m, "enable_vpn", "") == "on" && p(m, "vpn", "") != "" {
		lines = append(lines, fmt.Sprintf(" ipv4-family vpn-instance %s", p(m, "vpn", "")))
		for _, line := range strings.Split(p(m, "vpn_import", ""), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && p(m, "enable_import", "") == "on" {
				lines = append(lines, "  import-route "+line)
			}
		}
		for _, line := range strings.Split(p(m, "vpn_export", ""), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && p(m, "enable_export", "") == "on" {
				lines = append(lines, "  export-route "+line)
			}
		}
	}
	return lines
}

func splitList(s string) []string {
	out := []string{}
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
