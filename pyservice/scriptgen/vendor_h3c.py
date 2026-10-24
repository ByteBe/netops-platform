# -*- coding: utf-8 -*-
"""华三（Comware）脚本生成。部分功能复用华为生成器。"""
from typing import Dict, List

from scriptgen.utils import p, split_list, cidr_mask, wild_of, wildcard_of
from scriptgen.vendor_huawei import build_huawei_snmp, build_huawei_stp, build_huawei_bgp, build_huawei


def build_h3c(code: str, m: Dict[str, str]) -> List[str]:
    if code in ("router_acl", "sw_acl"):
        act = p(m, "action", "permit")
        num = p(m, "num", "3000")
        acltype = p(m, "acltype", "三层")
        src = p(m, "src", "0.0.0.0/0")
        src_addr = src.split("/")[0]
        wild = wild_of(src)
        if acltype == "二层":
            src_mac = p(m, "srcmac", "0000-0000-0000")
            return [f"acl mac {num}", f" rule 0 {act} source-mac {src_mac} ffff-ffff-ffff"]
        if acltype == "基础":
            return [f"acl basic {num}", f" rule 0 {act} source {src_addr} {wild}"]
        rule = f" rule 0 {act} {p(m, 'proto', 'ip')} source {src_addr} {wild}"
        dst = p(m, "dst", "")
        if dst:
            rule += f" destination {dst.split('/')[0]} {wild_of(dst)}"
        else:
            rule += " destination any"
        return [f"acl advanced {num}", rule]
    if code == "router_nat":
        return [
            f"interface {p(m, 'outif', 'GigabitEthernet0/0/1')}",
            f" nat outbound {p(m, 'acl', '2000')}",
        ]
    if code == "router_ospf":
        net = p(m, "net", "0.0.0.0")
        mask = p(m, "netmask", "255.255.255.0")
        vpn_line = ""
        if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
            vpn_line = f" vpn-instance {p(m, 'vpn', '')}"
        return [
            f"ospf{vpn_line} {p(m, 'process', '1')} router-id {p(m, 'rid', '1.1.1.1')}",
            f" area 0.0.0.{p(m, 'area', '0')}",
            f"  network {net} {wildcard_of(mask)}",
        ]
    if code in ("router_static", "sw_static"):
        vpn_pre = ""
        if p(m, "vpn", "") != "":
            vpn_pre = f" vpn-instance {p(m, 'vpn', '')}"
        return [f"ip route-static{vpn_pre} {p(m, 'dest', '0.0.0.0')} {p(m, 'mask', '0')} {p(m, 'nh', '')}"]
    if code == "com_ssh":
        user, passwd, vty = p(m, "user", "admin"), p(m, "pass", ""), p(m, "vty", "0 63")
        return [
            "public-key local create rsa",
            "ssh server enable",
            f"local-user {user} class manage",
            f" password simple {passwd}",
            " service-type ssh",
            " authorization-attribute user-role network-admin",
            f"line vty {vty}",
            " authentication-mode scheme",
            " protocol inbound ssh",
        ]
    if code == "com_user":
        user, passwd = p(m, "user", "admin"), p(m, "pass", "")
        return [
            f"local-user {user} class manage",
            f" password simple {passwd}",
            " service-type ssh",
            " authorization-attribute user-role network-admin",
        ]
    if code == "com_log":
        return ["info-center enable", f"info-center loghost {p(m, 'logip', '')}"]
    if code == "com_snmp":
        return build_huawei_snmp(m)
    if code == "sw_vlan":
        lines: List[str] = []
        for item in split_list(p(m, "vlan", "")):
            rg = item.split("-")
            if len(rg) == 2 and rg[0].strip() and rg[1].strip():
                lines.append(f"vlan {rg[0].strip()} to {rg[1].strip()}")
            else:
                lines.append("vlan " + item.strip())
        vif = p(m, "vlanif", "")
        ip = p(m, "ip", "")
        if vif and ip:
            lines += [f"interface Vlan-interface{vif}", f" ip address {ip}"]
        return lines
    if code == "sw_access":
        lines: List[str] = []
        for ifa in split_list(p(m, "iface", "")):
            lines += [f"interface {ifa}", " port link-type access", f" port access vlan {p(m, 'vlan', '1')}"]
        return lines
    if code == "sw_trunk":
        lines: List[str] = []
        for ifa in split_list(p(m, "iface", "")):
            lines += [f"interface {ifa}", " port link-type trunk", f" port trunk permit vlan {p(m, 'vlans', '')}"]
        return lines
    if code == "sw_dhcp":
        addr, mask = cidr_mask(p(m, "net", ""))
        pool = p(m, "pool", "pool1")
        lines = ["dhcp enable", f"dhcp server ip-pool {pool}", f" network {addr} mask {mask}", f" gateway-list {p(m, 'gw', '')}"]
        dns = p(m, "dns", "")
        if dns:
            lines.append(" dns-list " + dns.replace(",", " "))
        return lines
    if code == "sw_stp":
        return build_huawei_stp(m)
    if code == "router_bgp":
        return build_huawei_bgp(m)
    if code in ("router_isis", "mpls_vpn"):
        return build_huawei(code, m)
    if code == "route_policy":
        name = p(m, "name", "RP1")
        seq = p(m, "seq", "10")
        lines: List[str] = []
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f"ip prefix-list {p(m, 'prefix', '')} index {p(m, 'pfx_index', '10')} {p(m, 'pfx_action', 'permit')} {p(m, 'pfx_net', '')} {p(m, 'pfx_masklen', '24')}")
        lines.append(f"route-policy {name} permit node {seq}")
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f" if-match ip-prefix {p(m, 'prefix', '')}")
        return lines
    if code == "router_policy":
        return [
            "traffic classifier c1 operator or",
            f" if-match acl {p(m, 'acl', '3000')}",
            "traffic behavior b1",
            f" apply next-hop {p(m, 'nh', '')}",
            "traffic policy p1",
            " classifier c1 behavior b1",
            f"interface {p(m, 'iface', '')}",
            " traffic-policy p1 inbound",
        ]
    return []
