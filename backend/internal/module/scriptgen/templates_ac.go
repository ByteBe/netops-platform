package scriptgen

import (
	"fmt"
	"strings"
)

// ---- AC 控制器通用（复用交换机能力） ----
func regACCommon() {
	Register(Template{Code: "ac_vlan", Vendor: "huawei", DeviceType: "ac", Category: "vlan",
		Name: "AC VLAN", Description: "VLAN创建", Fields: []Field{field("vlans", "VLAN列表（如 10,20,30-40）", "text", true)}, Render: renderVLAN})
	Register(Template{Code: "ac_vlanif", Vendor: "huawei", DeviceType: "ac", Category: "vlanif",
		Name: "AC VLANIF", Description: "VLANIF网关配置", Fields: []Field{
			field("vlan", "VLAN号", "number", true), field("ip", "网关IP（如 192.168.1.1/24）", "network", true),
		}, Render: renderVLANIF})
	Register(Template{Code: "ac_acl", Vendor: "huawei", DeviceType: "ac", Category: "acl",
		Name: "AC ACL", Description: "ACL配置", Fields: []Field{
			field("acl_num", "ACL编号", "number", true), field("rules", "规则列表（每行一条）", "list", true),
		}, Render: renderACL})
	Register(Template{Code: "ac_nat", Vendor: "huawei", DeviceType: "ac", Category: "nat",
		Name: "AC NAT", Description: "NAT配置", Fields: []Field{
			field("out_if", "出接口", "text", true), field("acl_num", "内网ACL编号", "number", true),
		}, Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor), "interface " + p(params, "out_if", ""), " nat outbound " + p(params, "acl_num", "2000"), "quit"}
			return lines
		}})
	Register(Template{Code: "ac_ospf", Vendor: "huawei", DeviceType: "ac", Category: "ospf",
		Name: "AC OSPF", Description: "OSPF配置", Fields: regOspfFields(), Render: renderOSPF})
	Register(Template{Code: "ac_bgp", Vendor: "huawei", DeviceType: "ac", Category: "bgp",
		Name: "AC BGP", Description: "BGP配置", Fields: regBgpFields(), Render: renderBGP})
	Register(Template{Code: "ac_isis", Vendor: "huawei", DeviceType: "ac", Category: "isis",
		Name: "AC IS-IS", Description: "IS-IS配置", Fields: regIsisFields(), Render: renderISIS})
	Register(Template{Code: "ac_mpls", Vendor: "huawei", DeviceType: "ac", Category: "mpls",
		Name: "AC MPLS", Description: "MPLS配置", Fields: regMplsFields(), Render: renderMPLS})
	Register(Template{Code: "ac_mpls_vpn", Vendor: "huawei", DeviceType: "ac", Category: "mpls_vpn",
		Name: "AC MPLS VPN", Description: "MPLS L3VPN配置", Fields: regMplsVpnFields(), Render: renderMPLSVPN})
	Register(Template{Code: "ac_static", Vendor: "huawei", DeviceType: "ac", Category: "static_route",
		Name: "AC 静态路由", Description: "静态路由配置", Fields: []Field{fieldWithPlaceholder("routes", "路由列表", "list", true, "每行一条：目的网段 下一跳 [优先级]\n示例：\n10.10.0.0/16 192.168.1.1 60")}, Render: renderStatic})
	Register(Template{Code: "ac_policy_route", Vendor: "huawei", DeviceType: "ac", Category: "policy_route",
		Name: "AC 策略路由", Description: "策略路由配置", Fields: regPolicyRouteFields(), Render: renderPolicyRoute})
	Register(Template{Code: "ac_routing_policy", Vendor: "huawei", DeviceType: "ac", Category: "routing_policy",
		Name: "AC 路由策略", Description: "Route-Policy配置", Fields: regRoutingPolicyFields(), Render: renderRoutingPolicy})
	Register(Template{Code: "ac_interface_access", Vendor: "huawei", DeviceType: "ac", Category: "interface",
		Name: "AC 接口配置（Access）", Description: "Access口配置", Fields: []Field{
			field("interface", "接口", "text", true), field("vlan", "PVID VLAN", "number", true),
		}, Render: func(params map[string]string, vendor string) []string {
			return []string{sysView(vendor), "interface " + p(params, "interface", ""), " port link-type access", " port default vlan " + p(params, "vlan", "1"), "quit"}
		}})
	Register(Template{Code: "ac_interface_trunk", Vendor: "huawei", DeviceType: "ac", Category: "interface",
		Name: "AC 接口配置（Trunk）", Description: "Trunk口配置", Fields: []Field{
			field("interface", "接口", "text", true), field("vlans", "允许VLAN", "text", true),
		}, Render: func(params map[string]string, vendor string) []string {
			return []string{sysView(vendor), "interface " + p(params, "interface", ""), " port link-type trunk", " port trunk allow-pass vlan " + vlanFormat(p(params, "vlans", "")), "quit"}
		}})
	Register(Template{Code: "ac_auth_local", Vendor: "huawei", DeviceType: "ac", Category: "auth",
		Name: "AC 本地认证", Description: "本地用户认证", Fields: []Field{
			field("username", "用户名", "text", true), field("password", "密码", "password", true),
		}, Render: func(params map[string]string, vendor string) []string {
			return []string{sysView(vendor),
				fmt.Sprintf("local-user %s password cipher %s", p(params, "username", ""), p(params, "password", "")),
				fmt.Sprintf("local-user %s privilege level 15", p(params, "username", "")),
				fmt.Sprintf("local-user %s service-type telnet ssh", p(params, "username", "")),
			}
		}})
	Register(Template{Code: "ac_auth_remote", Vendor: "huawei", DeviceType: "ac", Category: "auth",
		Name: "AC 远程认证", Description: "RADIUS远程认证", Fields: []Field{
			field("server_ip", "服务器IP", "ip", true), field("shared_key", "共享密钥", "password", true),
		}, Render: func(params map[string]string, vendor string) []string {
			return []string{sysView(vendor),
				"radius-server template radius1",
				" radius-server authentication " + p(params, "server_ip", "") + " 1812",
				" radius-server shared-key cipher " + p(params, "shared_key", ""),
				"quit", "aaa", " authentication-scheme default", " radius-server group radius1", "quit"}
		}})
	Register(Template{Code: "ac_vrrp", Vendor: "huawei", DeviceType: "ac", Category: "vrrp",
		Name: "AC VRRP", Description: "VRRP配置", Fields: []Field{
			field("vlan", "VLAN号", "number", true), field("vrid", "VRID", "number", true),
			field("virtual_ip", "虚拟IP", "ip", true),
		}, Render: func(params map[string]string, vendor string) []string {
			return []string{sysView(vendor), "interface Vlanif" + p(params, "vlan", ""),
				fmt.Sprintf(" vrrp vrid %s virtual-ip %s", p(params, "vrid", "1"), p(params, "virtual_ip", "")), "quit"}
		}})
	Register(Template{Code: "ac_dhcp", Vendor: "huawei", DeviceType: "ac", Category: "dhcp",
		Name: "AC DHCP", Description: "DHCP地址池配置", Fields: []Field{
			field("pool", "地址池名", "text", true), field("network", "网段", "text", true),
			field("gateway", "网关", "ip", true), field("dns", "DNS", "text", false),
		}, Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor), "dhcp enable", "ip pool " + p(params, "pool", "pool1"),
				" network " + p(params, "network", ""), " gateway-list " + p(params, "gateway", "")}
			if d := p(params, "dns", ""); d != "" {
				lines = append(lines, " dns-list "+d)
			}
			lines = append(lines, "quit")
			return lines
		}})
}

// ---- AC 无线模板 ----
func regACWireless() {
	Register(Template{
		Code: "ac_wireless_ap", Vendor: "huawei", DeviceType: "ac", Category: "wireless_ap",
		Name: "AC 无线AP注册", Description: "无线AP注册与AP组配置",
		Fields: []Field{
			field("source_if", "源接口（如 Vlanif10）", "text", true),
			field("ap_group", "AP组名", "text", true),
			field("aps", "AP列表（每行：MAC [AP名称]，如 00E0-FC00-0001 ap1）", "list", false),
			field("regulatory_domain", "国家码（如 CN）", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor)}
			lines = append(lines, "wlan")
			lines = append(lines, fmt.Sprintf(" ac source interface %s", p(params, "source_if", "Vlanif10")))
			if rd := p(params, "regulatory_domain", "CN"); rd != "" {
				lines = append(lines, fmt.Sprintf(" regulatory-domain-profile name %s", rd))
				lines = append(lines, " country-code "+rd)
				lines = append(lines, "quit")
			}
			lines = append(lines, fmt.Sprintf(" ap-group name %s", p(params, "ap_group", "default")))
			lines = append(lines, "quit")
			for _, ap := range nonEmptyLines(params["aps"]) {
				parts := strings.Fields(ap)
				if len(parts) >= 1 {
					mac := parts[0]
					apName := "ap" + strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")
					if len(parts) >= 2 {
						apName = parts[1]
					}
					lines = append(lines, fmt.Sprintf(" ap-id %s type-id 19 mac %s", apName, mac))
					lines = append(lines, fmt.Sprintf("  ap-name %s", apName))
					lines = append(lines, fmt.Sprintf("  ap-group %s", p(params, "ap_group", "default")))
					lines = append(lines, "quit")
				}
			}
			lines = append(lines, "quit")
			return lines
		},
	})
	Register(Template{
		Code: "ac_wireless_template", Vendor: "huawei", DeviceType: "ac", Category: "wireless_template",
		Name: "AC 无线模板配置", Description: "SSID/安全/VAP 模板绑定",
		Fields: []Field{
			field("ssid", "SSID名称", "text", true),
			field("security", "加密方式", "select", true),
			field("password", "无线密码", "password", false),
			field("vlan", "业务VLAN", "number", true),
			field("max_users", "最大接入用户数", "number", false),
			field("ap_group", "AP组名", "text", false),
			field("vap_name", "VAP模板名", "text", false),
		},
		Render: func(params map[string]string, vendor string) []string {
			lines := []string{sysView(vendor), "wlan"}
			ssidProf := "ssid-" + p(params, "ssid", "wifi")
			secProf := "security-" + p(params, "ssid", "wifi")
			vap := p(params, "vap_name", "vap-"+p(params, "ssid", "wifi"))
			lines = append(lines, fmt.Sprintf(" ssid-profile name %s", ssidProf))
			lines = append(lines, fmt.Sprintf("  ssid %s", p(params, "ssid", "")))
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf(" security-profile name %s", secProf))
			if p(params, "security", "wpa2") == "wpa2" {
				lines = append(lines, "  security wpa2 psk pass-phrase "+p(params, "password", "")+" aes")
			} else if p(params, "security", "wpa2") == "wpa3" {
				lines = append(lines, "  security wpa3 psk pass-phrase "+p(params, "password", "")+" aes")
			} else {
				lines = append(lines, "  security open")
			}
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf(" vap-profile name %s", vap))
			lines = append(lines, fmt.Sprintf("  forward-mode tunnel"))
			lines = append(lines, fmt.Sprintf("  service-vlan vlan-id %s", p(params, "vlan", "1")))
			lines = append(lines, fmt.Sprintf("  ssid-profile %s", ssidProf))
			lines = append(lines, fmt.Sprintf("  security-profile %s", secProf))
			if mu := p(params, "max_users", ""); mu != "" {
				lines = append(lines, fmt.Sprintf("  max-sta-number %s", mu))
			}
			lines = append(lines, "quit")
			lines = append(lines, fmt.Sprintf(" ap-group name %s", p(params, "ap_group", "default")))
			lines = append(lines, fmt.Sprintf("  vap-profile %s wlan 1 radio all", vap))
			lines = append(lines, "quit", "quit")
			return lines
		},
	})
}

