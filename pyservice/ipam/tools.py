# -*- coding: utf-8 -*-
"""IP 地址管理工具：MAC 规范化、绑定/释放命令生成、命令输出校验、ARP 表解析。

复刻原 Go ipam 实现口径（命令生成与校验行为完全一致）：
- normalizeMAC：大写、去 -:./、12 位、华为/华三 XXXX-XXXX-XXXX 格式；
- 绑定命令：system-view + arp static <IP> <MAC> [vid <vlan>] + quit [+ save force]；
- 释放命令：system-view + undo arp [static] <IP> [vid <vlan>] + quit + save force；
- 输出校验：unrecognized / wrong parameter / incomplete / ambiguous / error: / failed / invalid / wrong number；
- ARP 解析：匹配 IP 与 4-4-4 MAC，跳过全零 MAC，提取 VLAN。
"""
import ipaddress
import re

_ARP_IP_RE = re.compile(r"\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b")
_ARP_MAC_RE = re.compile(r"\b([0-9a-fA-F]{4}[-.][0-9a-fA-F]{4}[-.][0-9a-fA-F]{4})\b")
_BIND_ERR_KEYWORDS = ("unrecognized", "wrong parameter", "incomplete",
                      "ambiguous", "error:", "failed", "invalid", "wrong number")


def normalize_mac(mac: str) -> str:
    """复刻 Go normalizeMAC；非法返回空串。"""
    if not mac:
        return ""
    s = mac.replace("-", "").replace(":", "").replace(".", "").upper()
    if len(s) != 12 or not re.fullmatch(r"[0-9A-F]{12}", s):
        return ""
    return "%s-%s-%s" % (s[0:4], s[4:8], s[8:12])


def valid_ip(ip: str) -> bool:
    try:
        ipaddress.ip_address(ip)
        return True
    except ValueError:
        return False


def build_bind_cmds(ip: str, mac: str, vlan: int, vendor: str):
    """生成华为/华三静态 ARP 绑定命令。返回 (commands, error)。"""
    if not valid_ip(ip):
        return [], "IP地址无效"
    m = normalize_mac(mac)
    if not m:
        return [], "MAC地址无效"
    arp_cmd = "arp static %s %s" % (ip, m)
    if vlan and vlan > 0:
        arp_cmd += " vid %d" % vlan
    cmds = ["system-view", arp_cmd, "quit"]
    if vendor in ("h3c", "huawei"):
        cmds.append("save force")
    return cmds, ""


def build_undo_cmds(ip: str, vlan: int, vendor: str):
    """生成释放 IP 的 undo 命令（华为带 vid，华三/其他仅 undo arp）。"""
    if not valid_ip(ip):
        return [], "IP地址无效"
    if vendor == "huawei":
        undo_cmd = "undo arp static %s" % ip
        if vlan and vlan > 0:
            undo_cmd += " vid %d" % vlan
    else:
        undo_cmd = "undo arp %s" % ip
    return ["system-view", undo_cmd, "quit", "save force"], ""


def has_bind_error(out: str) -> bool:
    """复刻 Go hasBindError：输出含任一错误关键词视为失败。"""
    text = (out or "").lower()
    return any(kw in text for kw in _BIND_ERR_KEYWORDS)


def parse_arp(out: str):
    """复刻 Go parseARP：解析 display arp 输出，返回 [{ip, mac, vlan}]（跳过全零 MAC）。"""
    items = []
    for line in (out or "").splitlines():
        ip_m = _ARP_IP_RE.search(line)
        mac_m = _ARP_MAC_RE.search(line)
        if not ip_m or not mac_m:
            continue
        mac = normalize_mac(mac_m.group(1))
        if not mac or mac == "0000-0000-0000":
            continue
        vlan = 0
        tokens = line.split()
        for i, t in enumerate(tokens):
            if t.upper() == mac and i + 1 < len(tokens) and tokens[i + 1].isdigit():
                vlan = int(tokens[i + 1])
                break
        items.append({"ip": ip_m.group(1), "mac": mac, "vlan": vlan})
    return items
