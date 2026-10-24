# -*- coding: utf-8 -*-
"""思科（IOS）脚本生成。"""
from typing import Dict, List

from scriptgen.utils import p, split_list, wild_of, wildcard_of


def build_cisco(code: str, m: Dict[str, str]) -> List[str]:
    if code in ("router_acl", "sw_acl"):
        num = p(m, "num", "100")
        act = p(m, "action", "permit")
        proto = p(m, "proto", "ip")
        src = p(m, "src", "0.0.0.0/0")
        src_addr = src.split("/")[0]
        wild = wild_of(src)
        if p(m, "acltype", "三层") == "基础":
            return [f"access-list {num} {act} {src_addr} {wild}"]
        if p(m, "acltype", "三层") == "二层":
            src_mac = p(m, "srcmac", "0000.0000.0000")
            return [f"mac access-list extended MAC-ACL-{num}", f" {act} host {src_mac} any"]
        rule = f"{act} {proto} {src_addr} {wild}"
        dst = p(m, "dst", "")
        if dst:
            rule += f" {dst.split('/')[0]} {wild_of(dst)}"
        else:
            rule += " any"
        return [f"ip access-list extended {num}", " " + rule]
    if code == "router_nat":
        outif = p(m, "outif", "GigabitEthernet0/0")
        return [
            f"interface {outif}",
            " ip nat outside",
            f"ip nat inside source list {p(m, 'acl', 'NAT')} interface {outif} overload",
        ]
    if code == "router_ospf":
        net = p(m, "net", "0.0.0.0")
        vrf = ""
        if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
            vrf = f" vrf {p(m, 'vpn', '')}"
        return [
            f"router ospf {p(m, 'process', '1')}{vrf}",
            f" router-id {p(m, 'rid', '1.1.1.1')}",
            f" network {net} {wildcard_of(p(m, 'netmask', '255.255.255.0'))} area {p(m, 'area', '0')}",
        ]
    if code in ("router_static", "sw_static"):
        vrf = ""
        if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
            vrf = f" vrf {p(m, 'vpn', '')}"
        return [f"ip route{vrf} {p(m, 'dest', '0.0.0.0')} {p(m, 'mask', '0.0.0.0')} {p(m, 'nh', '')}"]
    if code == "com_ssh":
        user, passwd, vty = p(m, "user", "admin"), p(m, "pass", ""), p(m, "vty", "0 15")
        return [
            "crypto key generate rsa modulus 2048",
            "ip domain name local",
            f"username {user} password 0 {passwd}",
            "line vty " + vty,
            " transport input ssh",
            " login local",
        ]
    if code == "com_user":
        return [f"username {p(m, 'user', 'admin')} password 0 {p(m, 'pass', '')}"]
    if code == "com_log":
        return [f"logging host {p(m, 'logip', '')}"]
    if code == "com_snmp":
        host = p(m, "host", "")
        lines = [f"snmp-server community {p(m, 'community', 'public')} RO"]
        if host:
            lines.append(f"snmp-server host {host} version 2c {p(m, 'community', 'public')}")
        return lines
    if code == "sw_vlan":
        lines: List[str] = []
        for v in split_list(p(m, "vlan", "")):
            rg = v.split("-")
            if len(rg) == 2 and rg[0].strip() and rg[1].strip():
                try:
                    start = int(rg[0].strip())
                    end = int(rg[1].strip())
                except ValueError:
                    start, end = 0, 0
                if start > 0 and end >= start and end <= 4094:
                    for i in range(start, end + 1):
                        lines += [f"vlan {i}", f" name VLAN_{i}"]
                    continue
            vv = v.strip()
            lines += [f"vlan {vv}", f" name VLAN_{vv}"]
        vif = p(m, "vlanif", "")
        ip = p(m, "ip", "")
        if vif and ip:
            lines += [f"interface Vlan{vif}", f" ip address {ip}"]
        return lines
    if code == "sw_access":
        lines: List[str] = []
        for ifa in split_list(p(m, "iface", "")):
            lines += [f"interface {ifa}", " switchport mode access", f" switchport access vlan {p(m, 'vlan', '1')}"]
        return lines
    if code == "sw_trunk":
        lines: List[str] = []
        for ifa in split_list(p(m, "iface", "")):
            lines += [f"interface {ifa}", " switchport mode trunk", f" switchport trunk allowed vlan {p(m, 'vlans', '')}"]
        return lines
    if code == "sw_dhcp":
        return [
            "ip dhcp excluded-address 10.0.0.1",
            f"ip dhcp pool {p(m, 'pool', 'POOL1')}",
            f" network {p(m, 'net', '10.0.0.0')} {p(m, 'netmask', '255.255.255.0')}",
            f" default-router {p(m, 'gw', '')}",
        ]
    if code == "sw_stp":
        return [f"spanning-tree mode rapid-pvst", f"spanning-tree vlan {p(m, 'vlan', '1')} root {p(m, 'root', 'primary')}"]
    if code == "router_bgp":
        return [
            f"router bgp {p(m, 'asn', '65001')}",
            f" bgp router-id {p(m, 'rid', '1.1.1.1')}",
            f" neighbor {p(m, 'peer_ip', '')} remote-as {p(m, 'peer_as', '65002')}",
        ]
    if code == "router_isis":
        return [
            f"router isis {p(m, 'pid', '1')}",
            f" net {p(m, 'net', '49.0001.0000.0000.0001.00')}",
            f" router-id {p(m, 'router_id', p(m, 'rid', '1.1.1.1'))}",
            f" is-type {p(m, 'level', 'level-2')}",
        ]
    if code == "route_policy":
        name = p(m, "name", "RP1")
        seq = p(m, "seq", "10")
        lines: List[str] = []
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f"ip prefix-list {p(m, 'prefix', '')} seq {seq} {p(m, 'pfx_action', 'permit')} {p(m, 'pfx_net', '')}/{p(m, 'pfx_masklen', '24')}")
        lines.append(f"route-map {name} permit {seq}")
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f" match ip address prefix-list {p(m, 'prefix', '')}")
        return lines
    if code == "router_policy":
        return [
            f"access-list {p(m, 'acl', '100')} permit ip any any",
            "route-map P permit 10",
            f" match ip address {p(m, 'acl', '100')}",
            f" set ip next-hop {p(m, 'nh', '')}",
            f"interface {p(m, 'iface', '')}",
            " ip policy route-map P",
        ]
    if code == "mpls_vpn":
        return ["// Cisco IOS 不适用 MPLS VPN 模板，请选择华为/华三厂商"]
    return []
