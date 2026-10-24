package scriptgen

import "testing"

func TestValidMask(t *testing.T) {
	cases := []struct{ in string; ok bool }{
		{"255.255.255.0", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"255.255.254.0", true},
		{"255.0.0.0", true},
		{"255.0.255.0", false},   // 不连续
		{"255.255.0.255", false}, // 不连续
		{"256.255.255.0", false}, // 段超界
		{"0.0.0.255", false},     // 反掩码不是正掩码
		{"abc", false},
		{"255.255.255", false},
	}
	for _, c := range cases {
		if got := validMask(c.in); got != c.ok {
			t.Errorf("validMask(%q)=%v, want %v", c.in, got, c.ok)
		}
	}
}

func TestValidIP(t *testing.T) {
	cases := []struct{ in string; ok bool }{
		{"1.1.1.1", true},
		{"10.16.194.0", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.1.1.1", false},
		{"1.2.3", false},
		{"1.2.3.4.5", false},
		{"a.b.c.d", false},
		{"fe80::1", false}, // 只允许 IPv4
	}
	for _, c := range cases {
		if got := validIP(c.in); got != c.ok {
			t.Errorf("validIP(%q)=%v, want %v", c.in, got, c.ok)
		}
	}
}

func TestCheckValue(t *testing.T) {
	cases := []struct {
		f    Field
		code string
		v    string
		ok   bool
	}{
		{Field{Key: "rid", Type: "ip"}, "router_ospf", "1.1.1.1", true},
		{Field{Key: "rid", Type: "ip"}, "router_ospf", "1.1.1.256", false},
		{Field{Key: "netmask", Type: "mask"}, "router_ospf", "255.255.255.0", true},
		{Field{Key: "netmask", Type: "mask"}, "router_ospf", "255.0.255.0", false},
		{Field{Key: "process", Type: "number"}, "router_ospf", "1", true},
		{Field{Key: "process", Type: "number"}, "router_ospf", "a", false},
		{Field{Key: "src", Type: "network"}, "router_acl", "192.168.1.0/24", true},
		{Field{Key: "src", Type: "network"}, "router_acl", "192.168.1.0/33", false},
		{Field{Key: "peer_ip", Type: "text"}, "router_bgp", "1.1.1.1,2.2.2.2", true},
		{Field{Key: "peer_ip", Type: "text"}, "router_bgp", "1.1.1.1,x", false},
		{Field{Key: "vlan", Type: "text"}, "sw_vlan", "10,20,30-40", true},
		{Field{Key: "vlan", Type: "text"}, "sw_vlan", "10,20,a", false},
		{Field{Key: "srcmac", Type: "text"}, "router_acl", "0000.1111.2222", true},
		{Field{Key: "srcmac", Type: "text"}, "router_acl", "00-11-22-33-44-55", true},
		{Field{Key: "srcmac", Type: "text"}, "router_acl", "abcd", false},
		{Field{Key: "ip", Type: "text"}, "mpls_vpn", "10.0.0.1 255.255.255.0", true},
		{Field{Key: "ip", Type: "text"}, "mpls_vpn", "10.0.0.1", false},
		{Field{Key: "pass", Type: "password"}, "com_ssh", "123456", true},
		{Field{Key: "pass", Type: "password"}, "com_ssh", "123", false},
		{Field{Key: "vty", Type: "text"}, "com_ssh", "0 4", true},
		{Field{Key: "vty", Type: "text"}, "com_ssh", "0", false},
		{Field{Key: "net", Type: "ip"}, "router_ospf", "", true}, // 非必填为空不报错
	}
	for _, c := range cases {
		if got := checkValue(c.f, c.code, c.v); (got == "") != c.ok {
			t.Errorf("checkValue(%+v, %s, %q)=%q, want ok=%v", c.f, c.code, c.v, got, c.ok)
		}
	}
}

func TestCheckValueRequired(t *testing.T) {
	f := Field{Key: "rid", Type: "ip", Required: true}
	if msg := checkValue(f, "router_ospf", ""); msg == "" {
		t.Error("required 字段为空应报错")
	}
	f2 := Field{Key: "rid", Type: "ip", Required: false}
	if msg := checkValue(f2, "router_ospf", ""); msg != "" {
		t.Errorf("非必填为空不应报错: %q", msg)
	}
}
