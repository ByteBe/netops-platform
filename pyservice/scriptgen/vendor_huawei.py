# -*- coding: utf-8 -*-
"""华为（VRP）脚本生成。"""
from typing import Dict, List

from scriptgen.utils import p, split_list, cidr_mask, wild_of, wildcard_of


def build_huawei_snmp(m: Dict[str, str]) -> List[str]:
    ver = p(m, "ver", "v2c")
    host = p(m, "host", "")
    lines = ["snmp-agent"]
    if ver == "v3":
        sec = p(m, "secname", "monitor")
        lines += [
            "snmp-agent sys-info version v3",
            "snmp-agent group v3 g1 privacy",
            f"snmp-agent usm-user v3 {sec} g1",
            f"snmp-agent usm-user v3 {sec} authentication-mode {p(m, 'authproto', 'sha')} {p(m, 'authpass', '')}",
            f"snmp-agent usm-user v3 {sec} privacy-mode {p(m, 'privproto', 'aes128')} {p(m, 'privpass', '')}",
        ]
        if host:
            lines.append(f"snmp-agent target-host trap address udp-domain {host} udp-port 162 params securityname {sec} v3")
    else:
        lines += [
            f"snmp-agent sys-info version {ver}",
            f"snmp-agent community read {p(m, 'community', 'public')}",
        ]
        if host:
            lines.append(f"snmp-agent target-host trap address udp-domain {host} udp-port 162 params securityname {p(m, 'community', 'public')}")
    return lines


def build_huawei_vlan(m: Dict[str, str]) -> List[str]:
    vlans = p(m, "vlan", "")
    lines: List[str] = []
    if vlans:
        lines.append("vlan batch " + vlans)
    vif = p(m, "vlanif", "")
    ip = p(m, "ip", "")
    if vif and ip:
        lines += [f"interface Vlanif{vif}", f" ip address {ip}"]
    return lines


def build_huawei_access(m: Dict[str, str]) -> List[str]:
    vlan = p(m, "vlan", "1")
    lines: List[str] = []
    for ifa in split_list(p(m, "iface", "")):
        lines += [f"interface {ifa}", " port link-type access", f" port default vlan {vlan}"]
    return lines


def build_huawei_trunk(m: Dict[str, str]) -> List[str]:
    vlans = p(m, "vlans", "")
    lines: List[str] = []
    for ifa in split_list(p(m, "iface", "")):
        lines += [f"interface {ifa}", " port link-type trunk", f" port trunk allow-pass vlan {vlans}"]
    return lines


def build_huawei_dhcp(m: Dict[str, str]) -> List[str]:
    addr, mask = cidr_mask(p(m, "net", ""))
    pool = p(m, "pool", "pool1")
    lines = ["dhcp enable", f"ip pool {pool}", f" network {addr} {mask}", f" gateway-list {p(m, 'gw', '')}"]
    dns = p(m, "dns", "")
    if dns:
        lines.append(" dns-list " + dns.replace(",", " "))
    return lines


def build_huawei_stp(m: Dict[str, str]) -> List[str]:
    mode = p(m, "mode", "mstp")
    inst = p(m, "instance", "1")
    vlan = p(m, "vlan", "")
    lines = [f"stp mode {mode}"]
    if mode == "mstp" and vlan:
        lines += ["stp region-configuration", f" instance {inst} vlan {vlan}", " active region-configuration"]
    root = p(m, "root", "none")
    pri = p(m, "pri", "")
    if mode == "mstp":
        if root == "primary":
            lines.append(f"stp instance {inst} root primary")
        if root == "secondary":
            lines.append(f"stp instance {inst} root secondary")
        if root == "none" and pri:
            lines.append(f"stp instance {inst} priority {pri}")
    else:
        if root == "primary":
            lines.append("stp root primary")
        if root == "secondary":
            lines.append("stp root secondary")
        if root == "none" and pri:
            lines.append(f"stp priority {pri}")
    return lines + ["stp enable"]


def build_huawei_bgp(m: Dict[str, str]) -> List[str]:
    asn = p(m, "asn", "65001")
    rid = p(m, "rid", "1.1.1.1")
    peer_as = p(m, "peer_as", "65002")
    peer_ips = split_list(p(m, "peer_ip", ""))
    lines = [f"bgp {asn}", f" router-id {rid}"]
    conn_int = p(m, "conn_int", "")
    for pip in peer_ips:
        lines.append(f" peer {pip} as-number {peer_as}")
        if conn_int:
            lines.append(f" peer {pip} connect-interface {conn_int}")
    if p(m, "enable_net", "") == "on":
        lines += [" ipv4-family unicast", f"  network {p(m, 'network', '')} {p(m, 'netmask', '255.255.255.0')}"]
    if p(m, "vpn4", "") == "on":
        lines.append(" ipv4-family vpnv4")
        for pip in split_list(p(m, "vpn4_enable_peer", "")):
            lines.append(f"  peer {pip} enable")
        for pip in split_list(p(m, "vpn4_adv_comm", "")):
            lines.append(f"  peer {pip} advertise-community")
        for pip in split_list(p(m, "vpn4_next_hop_inv", "")):
            lines.append(f"  peer {pip} next-hop-invariable")
    if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
        lines.append(f" ipv4-family vpn-instance {p(m, 'vpn', '')}")
        if p(m, "enable_import", "") == "on":
            for line in p(m, "vpn_import", "").split("\n"):
                line = line.strip()
                if line:
                    lines.append("  import-route " + line)
        if p(m, "enable_export", "") == "on":
            for line in p(m, "vpn_export", "").split("\n"):
                line = line.strip()
                if line:
                    lines.append("  import-route " + line)
    return lines


def build_huawei(code: str, m: Dict[str, str]) -> List[str]:
    if code in ("router_acl", "sw_acl"):
        act = p(m, "action", "permit")
        proto = p(m, "proto", "ip")
        num = p(m, "num", "3000")
        acltype = p(m, "acltype", "三层")
        src = p(m, "src", "0.0.0.0/0")
        src_addr = src.split("/")[0]
        src_wild = wild_of(src)
        entry = "advanced"
        if acltype == "基础":
            entry = "basic"
        elif acltype == "二层":
            entry = "mac"
        elif acltype == "自定义":
            entry = "user-defined"
        if acltype == "二层":
            src_mac = p(m, "srcmac", "0000-0000-0000")
            return [f"acl mac {num}", f" rule 5 {act} source-mac {src_mac} ffff-ffff-ffff"]
        if acltype == "基础":
            return [f"acl basic {num}", f" rule 5 {act} source {src_addr} {src_wild}"]
        rule = f" rule 5 {act} {proto} source {src_addr} {src_wild}"
        dst = p(m, "dst", "")
        if dst:
            rule += f" destination {dst.split('/')[0]} {wild_of(dst)}"
        else:
            rule += " destination any"
        dport = p(m, "dport", "")
        if dport and proto in ("tcp", "udp"):
            rule += f" destination-port eq {dport}"
        return [f"acl {entry} {num}", rule]
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
            f" area {p(m, 'area', '0')}",
            f"  network {net} {wildcard_of(mask)}",
        ]
    if code in ("router_static", "sw_static"):
        vpn_pre = ""
        if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
            vpn_pre = f" vpn-instance {p(m, 'vpn', '')}"
        return [f"ip route-static{vpn_pre} {p(m, 'dest', '0.0.0.0')} {p(m, 'mask', '0')} {p(m, 'nh', '')}"]
    if code == "com_ssh":
        user, passwd, bits, vty = p(m, "user", "admin"), p(m, "pass", ""), p(m, "rsabits", "2048"), p(m, "vty", "0 4")
        return [
            f"rsa local-key-pair create modulus {bits}",
            "stelnet server enable",
            f"local-user {user} password cipher {passwd}",
            f"local-user {user} privilege level {p(m, 'pri', '15')}",
            f"local-user {user} service-type ssh",
            f"user-interface vty {vty}",
            " authentication-mode aaa",
            " protocol inbound ssh",
        ]
    if code == "com_user":
        user, passwd, pri = p(m, "user", "admin"), p(m, "pass", ""), p(m, "pri", "15")
        return [
            f"local-user {user} password cipher {passwd}",
            f"local-user {user} privilege level {pri}",
            f"local-user {user} service-type ssh",
        ]
    if code == "com_log":
        return ["info-center enable", f"info-center loghost {p(m, 'logip', '')}"]
    if code == "com_snmp":
        return build_huawei_snmp(m)
    if code == "sw_vlan":
        return build_huawei_vlan(m)
    if code == "sw_access":
        return build_huawei_access(m)
    if code == "sw_trunk":
        return build_huawei_trunk(m)
    if code == "sw_dhcp":
        return build_huawei_dhcp(m)
    if code == "sw_stp":
        return build_huawei_stp(m)
    if code == "router_bgp":
        return build_huawei_bgp(m)
    if code == "router_isis":
        vpn_line = ""
        if p(m, "enable_vpn", "") == "on" and p(m, "vpn", "") != "":
            vpn_line = f" vpn-instance {p(m, 'vpn', '')}"
        return [
            f"isis{vpn_line} {p(m, 'pid', '1')}",
            f" network-entity {p(m, 'net', '49.0001.0000.0000.0001.00')}",
            f" router-id {p(m, 'router_id', p(m, 'rid', '1.1.1.1'))}",
            f" is-level {p(m, 'level', 'level-2')}",
        ]
    if code == "mpls_vpn":
        return [
            "mpls lsr-id 1.1.1.1",
            "mpls",
            "mpls ldp",
            f"ip vpn-instance {p(m, 'vpn_name', 'vpna')}",
            f" route-distinguisher {p(m, 'rd', '100:1')}",
            f" vpn-target {p(m, 'rt_out', '100:1')} export-extcommunity",
            f" vpn-target {p(m, 'rt_in', '100:1')} import-extcommunity",
            f"interface {p(m, 'iface', 'GigabitEthernet0/0/0')}",
            f" ip binding vpn-instance {p(m, 'vpn_name', 'vpna')}",
            f" ip address {p(m, 'ip', '10.0.0.1 255.255.255.0')}",
            " mpls",
            " mpls ldp",
        ]
    if code == "route_policy":
        name = p(m, "name", "RP1")
        seq = p(m, "seq", "10")
        lines: List[str] = []
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f"ip ip-prefix {p(m, 'prefix', '')} index {p(m, 'pfx_index', '10')} {p(m, 'pfx_action', 'permit')} {p(m, 'pfx_net', '')} {p(m, 'pfx_masklen', '24')}")
        lines.append(f"route-policy {name} permit node {seq}")
        if p(m, "enable_prefix", "") == "on" and p(m, "prefix", "") != "":
            lines.append(f" if-match ip-prefix {p(m, 'prefix', '')}")
        return lines
    if code == "router_policy":
        return [
            "traffic classifier c1 operator or",
            f" if-match acl {p(m, 'acl', '3000')}",
            "traffic behavior b1",
            f" redirect ip-nexthop {p(m, 'nh', '')}",
            "traffic policy p1",
            " classifier c1 behavior b1",
            f"interface {p(m, 'iface', '')}",
            " traffic-policy p1 inbound",
        ]
    return []
