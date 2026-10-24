# -*- coding: utf-8 -*-
"""脚本生成器 Python 冒烟测试（对照 Go vendor_test.go 用例）。"""
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from scriptgen.vendor_huawei import build_huawei, build_huawei_bgp, build_huawei_dhcp, build_huawei_stp
from scriptgen.vendor_h3c import build_h3c
from scriptgen.vendor_cisco import build_cisco
from scriptgen.validate import validate_req, valid_mask, valid_ip

FAILED = []


def find_line(lines, substr):
    return any(substr in l for l in lines)


def check(name, cond, detail=""):
    if not cond:
        FAILED.append(f"{name} {detail}")
        print(f"FAIL {name} {detail}")
    else:
        print(f"ok   {name}")


# 华为：BGP VPN 发布用 import-route 而非 export-route
m = {"enable_vpn": "on", "vpn": "vpna", "enable_export": "on", "vpn_export": "ospf 2\nstatic"}
lines = build_huawei_bgp(m)
check("华为BGP无export-route", not find_line(lines, "export-route"), str(lines))
check("华为BGP import-route", find_line(lines, "  import-route ospf 2") and find_line(lines, "  import-route static"), str(lines))

# 华为：route_policy 前缀掩码长度默认 24
m = {"enable_prefix": "on", "prefix": "P1", "pfx_net": "10.16.192.0"}
lines = build_huawei("route_policy", m)
check("华为route_policy掩码24", find_line(lines, "ip ip-prefix P1 index 10 permit 10.16.192.0 24"), str(lines))

# 华为/华三：非 MSTP root 用全局命令
lines = build_huawei_stp({"mode": "rstp", "root": "primary"})
check("rstp无stp instance", not find_line(lines, "stp instance"), str(lines))
check("rstp全局root", find_line(lines, "stp root primary"), str(lines))
lines2 = build_huawei_stp({"mode": "mstp", "root": "primary", "vlan": "10"})
check("mstp instance root", find_line(lines2, "stp instance 1 root primary"), str(lines2))

# 华为/华三：DHCP dns-list 逗号转空格
lines = build_huawei_dhcp({"net": "10.0.0.0/24", "dns": "114.114.114.114,8.8.8.8"})
check("DHCP dns空格", find_line(lines, "dns-list 114.114.114.114 8.8.8.8"), str(lines))

# 华三：vlan 逗号/范围展开
lines = build_h3c("sw_vlan", {"vlan": "10,20,30-32"})
for want in ("vlan 10", "vlan 20", "vlan 30 to 32"):
    check(f"华三vlan {want}", find_line(lines, want), str(lines))
check("华三无逗号列表", not find_line(lines, "vlan 10,20"), str(lines))

# 华三：DHCP ip-pool → dhcp server ip-pool
lines = build_h3c("sw_dhcp", {"net": "10.0.0.0/24", "pool": "p1"})
check("华三DHCP server ip-pool", find_line(lines, "dhcp server ip-pool p1") and find_line(lines, "network 10.0.0.0 mask 255.255.255.0"), str(lines))

# 华三：route_policy if-match ip-prefix
lines = build_h3c("route_policy", {"enable_prefix": "on", "prefix": "P1"})
check("华三if-match ip-prefix", find_line(lines, " if-match ip-prefix P1"), str(lines))

# 华三：策略路由 apply next-hop
lines = build_h3c("router_policy", {"nh": "192.168.1.254"})
check("华三apply next-hop", find_line(lines, " apply next-hop 192.168.1.254"), str(lines))

# 思科：vlan 范围展开为逐条
lines = build_cisco("sw_vlan", {"vlan": "30-32,40"})
for want in ("vlan 30", "vlan 31", "vlan 32", "vlan 40"):
    check(f"思科vlan {want}", find_line(lines, want), str(lines))

# 思科：MAC ACL 命名式
lines = build_cisco("router_acl", {"acltype": "二层", "num": "100", "srcmac": "0000.1111.2222"})
check("思科MAC ACL命名", find_line(lines, "mac access-list extended MAC-ACL-100"), str(lines))

# 思科：MPLS VPN 提示不适用
lines = build_cisco("mpls_vpn", {})
check("思科MPLS不适用", len(lines) > 0 and find_line(lines, "不适用"), str(lines))

# 全 case 冒烟：三厂商所有功能跑一遍
m_all = {
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
codes = ["router_acl", "sw_acl", "router_nat", "router_ospf", "router_static", "sw_static",
         "com_ssh", "com_user", "com_log", "com_snmp", "sw_vlan", "sw_access", "sw_trunk",
         "sw_dhcp", "sw_stp", "router_bgp", "router_isis", "mpls_vpn", "route_policy", "router_policy"]
builders = {"huawei": build_huawei, "h3c": build_h3c, "cisco": build_cisco}
for vname, b in builders.items():
    for code in codes:
        try:
            lines = b(code, m_all)
        except Exception as e:
            FAILED.append(f"[{vname}] {code} panic: {e}")
            print(f"FAIL [{vname}] {code} panic: {e}")
            continue
        if vname == "cisco" and code == "mpls_vpn":
            if len(lines) == 0:
                FAILED.append(f"[cisco] {code} 应输出提示")
                print(f"FAIL [cisco] {code} 应输出提示")
            continue
        if len(lines) == 0:
            FAILED.append(f"[{vname}] {code} 输出为空")
            print(f"FAIL [{vname}] {code} 输出为空")

# 校验规则
check("validMask 255.255.255.0", valid_mask("255.255.255.0"))
check("validMask 非连续", not valid_mask("255.0.255.0"))
check("validMask 0.0.0.0", valid_mask("0.0.0.0"))
check("validIP 1.1.1.1", valid_ip("1.1.1.1"))
check("validIP 非法", not valid_ip("1.2.3.999") and not valid_ip("1.2.3"))
label, msg = validate_req("router_ospf", {"process": "1", "rid": "1.1.1.1", "net": "10.16.194.0", "netmask": "255.255.255.0", "area": "0"})
check("router_ospf 校验通过", label == "" and msg == "", f"{label}:{msg}")
label, msg = validate_req("router_ospf", {"process": "1", "rid": "bad", "net": "10.16.194.0", "netmask": "255.255.255.0", "area": "0"})
check("router_ospf 校验失败", msg != "", f"{label}:{msg}")

print()
print("TOTAL FAIL:", len(FAILED))
sys.exit(1 if FAILED else 0)
