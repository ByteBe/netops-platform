package scriptgen

import (
	"strings"
	"testing"
)

func findLine(lines []string, substr string) bool {
	for _, l := range lines {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// 华为：BGP VPN 发布 export-route 不存在 → import-route
func TestHuaweiBgpNoExportRoute(t *testing.T) {
	m := map[string]string{"enable_vpn": "on", "vpn": "vpna", "enable_export": "on", "vpn_export": "ospf 2\nstatic"}
	lines := buildHuaweiBgp(m)
	if findLine(lines, "export-route") {
		t.Error("华为不应输出 export-route")
	}
	if !findLine(lines, "  import-route ospf 2") || !findLine(lines, "  import-route static") {
		t.Errorf("应输出 import-route，实际: %v", lines)
	}
}

// 华为：route_policy 前缀掩码长度默认 24
func TestHuaweiRoutePolicyMaskLen(t *testing.T) {
	m := map[string]string{"enable_prefix": "on", "prefix": "P1", "pfx_net": "10.16.192.0"}
	lines := buildHuawei("route_policy", m)
	if !findLine(lines, "ip ip-prefix P1 index 10 permit 10.16.192.0 24") {
		t.Errorf("ip-prefix 应含掩码长度 24，实际: %v", lines)
	}
}

// 华为/华三：非 MSTP 模式 root 用全局 stp root
func TestStpNonMstpRoot(t *testing.T) {
	m := map[string]string{"mode": "rstp", "root": "primary"}
	lines := buildHuaweiStp(m)
	if findLine(lines, "stp instance") {
		t.Errorf("rstp 模式不应输出 stp instance，实际: %v", lines)
	}
	if !findLine(lines, "stp root primary") {
		t.Errorf("rstp 模式应输出 stp root primary，实际: %v", lines)
	}
	m2 := map[string]string{"mode": "mstp", "root": "primary", "vlan": "10"}
	lines2 := buildHuaweiStp(m2)
	if !findLine(lines2, "stp instance 1 root primary") {
		t.Errorf("mstp 模式应输出 stp instance，实际: %v", lines2)
	}
}

// 华为/华三：DHCP dns-list 逗号转空格
func TestDhcpDnsList(t *testing.T) {
	m := map[string]string{"net": "10.0.0.0/24", "dns": "114.114.114.114,8.8.8.8"}
	lines := buildHuaweiDhcp(m)
	if !findLine(lines, "dns-list 114.114.114.114 8.8.8.8") {
		t.Errorf("dns-list 应空格分隔，实际: %v", lines)
	}
}

// 华三：vlan 逗号/范围展开
func TestH3cVlanExpand(t *testing.T) {
	m := map[string]string{"vlan": "10,20,30-32"}
	lines := buildH3c("sw_vlan", m)
	for _, want := range []string{"vlan 10", "vlan 20", "vlan 30 to 32"} {
		if !findLine(lines, want) {
			t.Errorf("缺少 %q，实际: %v", want, lines)
		}
	}
	if findLine(lines, "vlan 10,20") {
		t.Error("华三不应输出逗号列表")
	}
}

// 华三：DHCP ip-pool → dhcp server ip-pool
func TestH3cDhcpPool(t *testing.T) {
	m := map[string]string{"net": "10.0.0.0/24", "pool": "p1"}
	lines := buildH3c("sw_dhcp", m)
	if !findLine(lines, "dhcp server ip-pool p1") || !findLine(lines, "network 10.0.0.0 mask 255.255.255.0") {
		t.Errorf("华三 DHCP 语法错误，实际: %v", lines)
	}
}

// 华三：route_policy if-match ip-prefix
func TestH3cIfMatchPrefix(t *testing.T) {
	m := map[string]string{"enable_prefix": "on", "prefix": "P1"}
	lines := buildH3c("route_policy", m)
	if !findLine(lines, " if-match ip-prefix P1") {
		t.Errorf("华三 if-match 应为 ip-prefix，实际: %v", lines)
	}
}

// 华三：策略路由 apply next-hop
func TestH3cPolicyNextHop(t *testing.T) {
	m := map[string]string{"nh": "192.168.1.254"}
	lines := buildH3c("router_policy", m)
	if !findLine(lines, " apply next-hop 192.168.1.254") {
		t.Errorf("华三策略路由应为 apply next-hop，实际: %v", lines)
	}
}

// 思科：vlan 范围展开为逐条
func TestCiscoVlanRange(t *testing.T) {
	m := map[string]string{"vlan": "30-32,40"}
	lines := buildCisco("sw_vlan", m)
	for _, want := range []string{"vlan 30", "vlan 31", "vlan 32", "vlan 40"} {
		if !findLine(lines, want) {
			t.Errorf("缺少 %q，实际: %v", want, lines)
		}
	}
}

// 思科：MAC ACL 命名式
func TestCiscoMacAclNamed(t *testing.T) {
	m := map[string]string{"acltype": "二层", "num": "100", "srcmac": "0000.1111.2222"}
	lines := buildCisco("router_acl", m)
	if !findLine(lines, "mac access-list extended MAC-ACL-100") {
		t.Errorf("思科 MAC ACL 应为命名式，实际: %v", lines)
	}
}

// 思科：MPLS VPN 提示不适用
func TestCiscoMplsVpnNotSupported(t *testing.T) {
	lines := buildCisco("mpls_vpn", nil)
	if len(lines) == 0 || !findLine(lines, "不适用") {
		t.Errorf("思科 mpls_vpn 应输出提示，实际: %v", lines)
	}
}

// 全 case 冒烟：三厂商所有功能用示例参数跑一遍，确保无 panic、输出非空
func TestAllCasesSmoke(t *testing.T) {
	m := map[string]string{
		"acltype": "三层", "action": "permit", "proto": "tcp", "num": "3000",
		"src": "10.16.192.0/24", "dst": "172.16.0.0/16", "dport": "443",
		"outif": "GigabitEthernet0/0/1", "acl": "2000",
		"process": "1", "rid": "1.1.1.1", "net": "10.16.194.0", "netmask": "255.255.255.0", "area": "0",
		"dest": "0.0.0.0", "mask": "0", "nh": "192.168.1.1",
		"user": "admin", "pass": "admin123", "vty": "0 4", "priv": "15",
		"logip": "10.16.0.10", "host": "10.16.0.11", "community": "netops", "ver": "v2c",
		"vlan": "10,20,30-32", "vlanif": "10", "ip": "192.168.1.1 255.255.255.0",
		"iface": "GigabitEthernet0/0/1,GigabitEthernet0/0/2", "vlans": "10,20",
		"pool": "pool1", "gw": "192.168.1.254", "dns": "114.114.114.114,8.8.8.8",
		"mode": "mstp", "instance": "1", "root": "primary", "pri": "4096",
		"asn": "65001", "peer_as": "65002", "peer_ip": "1.1.1.1,2.2.2.2", "conn_int": "LoopBack0",
		"enable_net": "on", "network": "10.16.192.0",
		"vpn4": "on", "vpn4_enable_peer": "1.1.1.1", "vpn4_adv_comm": "2.2.2.2", "vpn4_next_hop_inv": "3.3.3.3",
		"enable_vpn": "on", "vpn": "vpna", "enable_import": "on", "vpn_import": "ospf 2", "enable_export": "on", "vpn_export": "static",
		"pid": "1", "level": "level-2", "router_id": "1.1.1.1",
		"vpn_name": "vpna", "rd": "100:1", "rt_in": "100:1", "rt_out": "100:1",
		"name": "RP1", "seq": "10", "enable_prefix": "on", "prefix": "TO-BJ", "pfx_index": "10", "pfx_action": "permit", "pfx_net": "10.16.192.0", "pfx_masklen": "24",
	}
	codes := []string{"router_acl", "sw_acl", "router_nat", "router_ospf", "router_static", "sw_static", "com_ssh", "com_user", "com_log", "com_snmp", "sw_vlan", "sw_access", "sw_trunk", "sw_dhcp", "sw_stp", "router_bgp", "router_isis", "mpls_vpn", "route_policy", "router_policy"}
	builders := map[string]func(string, map[string]string) []string{
		"华为": buildHuawei, "华三": buildH3c, "思科": buildCisco,
	}
	for vname, b := range builders {
		for _, code := range codes {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("[%s] %s panic: %v", vname, code, r)
					}
				}()
				lines := b(code, m)
				if vname == "思科" && (code == "mpls_vpn") {
					if len(lines) == 0 {
						t.Errorf("[思科] %s 应输出提示", code)
					}
					return
				}
				if len(lines) == 0 {
					t.Errorf("[%s] %s 输出为空", vname, code)
				}
			}()
		}
	}
}
