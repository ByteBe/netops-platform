# -*- coding: utf-8 -*-
"""脚本生成器输入校验（与 Go 端同一套规则，生成前检查）。"""
import ipaddress
import re
from typing import Dict, List, Tuple

from scriptgen.categories import CATEGORIES

MAC_RE = re.compile(r"^(?:[0-9A-Fa-f]{4}\.){2}[0-9A-Fa-f]{4}$|^(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$")


def field_visible(f: Dict, m: Dict[str, str]) -> bool:
    """按 ShowIf 判定字段当前是否可见。"""
    show_if = f.get("show_if")
    if not show_if:
        return True
    v = m.get(show_if["key"], "")
    if show_if.get("not_eq"):
        return v != show_if["not_eq"]
    eq_any = show_if.get("eq_any") or []
    if eq_any:
        return v in eq_any
    return v == show_if.get("eq", "")


def valid_ip(s: str) -> bool:
    s = s.strip()
    if s.count(".") != 3:
        return False
    try:
        ipaddress.IPv4Address(s)
        return True
    except ValueError:
        return False


def valid_mask(s: str) -> bool:
    s = s.strip()
    try:
        a, b, c, d = (int(x) for x in s.split("."))
    except ValueError:
        return False
    if a > 255 or b > 255 or c > 255 or d > 255:
        return False
    m = (a << 24) | (b << 16) | (c << 8) | d
    if m == 0:
        return s == "0.0.0.0"
    ones = bin(m).count("1")
    # 掩码必须是前 ones 位连续为 1（等价 Go: net.CIDRMask(ones,32) == m）
    mask_from_ones = (0xFFFFFFFF << (32 - ones)) & 0xFFFFFFFF if ones else 0
    return m == mask_from_ones


def check_value(f: Dict, code: str, v: str) -> str:
    """按字段类型与关键 key 校验，返回错误信息（空串=通过）。"""
    v = v.strip()
    if v == "":
        return "不能为空" if f.get("required") else ""
    ftype = f.get("type", "")
    if ftype == "ip":
        if not valid_ip(v):
            return "IP 地址格式不正确"
    elif ftype == "network":
        parts = v.split("/")
        if len(parts) != 2 or not valid_ip(parts[0]):
            return "网段格式不正确（如 192.168.1.0/24）"
        try:
            pfx = int(parts[1].strip())
        except ValueError:
            return "掩码长度应为 0-32"
        if pfx < 0 or pfx > 32:
            return "掩码长度应为 0-32"
    elif ftype == "number":
        try:
            int(v)
        except ValueError:
            return "请输入数字"
    elif ftype == "mask":
        if not valid_mask(v):
            return "掩码格式不正确（需连续，如 255.255.255.0）"
    elif ftype == "password":
        if len(v) < 6:
            return "密码长度不能少于 6 位"

    key = f.get("key", "")
    if key in ("peer_ip", "vpn4_enable_peer", "vpn4_adv_comm", "vpn4_next_hop_inv", "dns"):
        for item in v.split(","):
            if not valid_ip(item.strip()):
                return "IP 列表格式不正确（逗号分隔，如 1.1.1.1,2.2.2.2）"
    elif key == "peer_as":
        for item in v.split(","):
            try:
                int(item.strip())
            except ValueError:
                return "AS 号列表格式不正确（数字，逗号分隔）"
    elif key in ("vlan", "vlans"):
        for item in v.split(","):
            item = item.strip()
            for n in item.split("-"):
                try:
                    int(n.strip())
                except ValueError:
                    return "VLAN 格式不正确（如 10,20,30-40）"
    elif key == "vty":
        parts = v.split()
        if len(parts) != 2:
            return "VTY 范围格式不正确（如 0 4）"
        for p_ in parts:
            try:
                int(p_)
            except ValueError:
                return "VTY 范围格式不正确（如 0 4）"
    elif key == "srcmac":
        if not MAC_RE.match(v):
            return "MAC 地址格式不正确（如 0000-1111-2222）"
    elif key == "ip":
        if code == "mpls_vpn":
            parts = v.split()
            if len(parts) != 2 or not valid_ip(parts[0]) or not valid_mask(parts[1]):
                return "接口IP格式不正确（如 10.0.0.1 255.255.255.0）"
    elif key == "pfx_net":
        if not valid_ip(v):
            return "匹配网段格式不正确（如 10.16.192.0）"
    return ""


def validate_req(code: str, m: Dict[str, str]) -> Tuple[str, str]:
    """校验所有可见字段，返回第一个错误字段 label 与错误信息（无错时 ('','')）。"""
    for cat in CATEGORIES:
        if cat["code"] != code:
            continue
        for f in cat["fields"]:
            if not field_visible(f, m):
                continue
            msg = check_value(f, code, m.get(f["key"], ""))
            if msg:
                return f["label"], msg
        return "", ""
    return "", ""
