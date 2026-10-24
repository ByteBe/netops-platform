# -*- coding: utf-8 -*-
"""脚本生成器共用工具：参数取值、掩码/反掩码/网段换算、列表拆分。"""
from typing import Dict, List, Tuple


def p(m: Dict[str, str], key: str, default: str = "") -> str:
    """取参数，空串回退默认值（与 Go p() 一致）。"""
    v = m.get(key)
    if v is not None and v != "":
        return v
    return default


def split_list(s: str) -> List[str]:
    """逗号拆分并去除空白与空项。"""
    out = []
    for x in s.split(","):
        x = x.strip()
        if x:
            out.append(x)
    return out


def cidr_mask(cidr: str) -> Tuple[str, str]:
    """'192.168.1.0/24' -> ('192.168.1.0', '255.255.255.0')，无 / 时默认 /24。"""
    parts = cidr.strip().split("/")
    addr = parts[0].strip()
    prefix = 24
    if len(parts) == 2:
        try:
            prefix = int(parts[1].strip())
        except ValueError:
            prefix = 24
    mp = []
    for _ in range(4):
        if prefix >= 8:
            mp.append("255")
            prefix -= 8
        else:
            mp.append(str(256 - (1 << (8 - prefix))))
            prefix = 0
    return addr, ".".join(mp)


def wild_of(cidr: str) -> str:
    """'192.168.1.0/24' -> 反掩码 '0.0.0.255'。"""
    parts = cidr.strip().split("/")
    prefix = 24
    if len(parts) == 2:
        try:
            prefix = int(parts[1].strip())
        except ValueError:
            prefix = 24
    w = 0xFFFFFFFF >> prefix
    return f"{(w >> 24) & 0xFF}.{(w >> 16) & 0xFF}.{(w >> 8) & 0xFF}.{w & 0xFF}"


def wildcard_of(mask: str) -> str:
    """点分十进制掩码 -> 反掩码（如 255.255.255.0 -> 0.0.0.255）。"""
    a, b, c, d = (int(x) for x in mask.strip().split("."))
    w = ~((a << 24) | (b << 16) | (c << 8) | d) & 0xFFFFFFFF
    return f"{(w >> 24) & 0xFF}.{(w >> 16) & 0xFF}.{(w >> 8) & 0xFF}.{w & 0xFF}"
