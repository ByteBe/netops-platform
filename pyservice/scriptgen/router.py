# -*- coding: utf-8 -*-
"""脚本生成器 FastAPI 路由（由 Go 主程序 /scriptgen/generate 转发调用）。

响应: {"script": "生成脚本(已剔除quit/exit/end)"}
失败: HTTP 400 + {"error": "字段Label: 错误信息"}
"""
from typing import Dict

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from scriptgen import validate
from scriptgen.vendor_huawei import build_huawei
from scriptgen.vendor_h3c import build_h3c
from scriptgen.vendor_cisco import build_cisco

router = APIRouter(prefix="/internal/scriptgen")

# 脚本类型中文注释说明（生成结果区展示，前端"纯净版"自动剥离 // 注释）
COMMENTS = {
    "router_acl": "配置 ACL 访问控制规则（允许/拒绝）",
    "sw_acl": "配置 ACL 访问控制规则（允许/拒绝）",
    "router_nat": "配置 NAT 地址转换（内网共享上网）",
    "router_ospf": "配置 OSPF 动态路由",
    "router_static": "配置静态路由",
    "sw_static": "配置静态路由",
    "router_bgp": "配置 BGP 路由",
    "router_isis": "配置 IS-IS 链路状态路由",
    "router_policy": "配置策略路由（灵活选路）",
    "route_policy": "配置路由策略（过滤/属性修改）",
    "mpls_vpn": "配置 MPLS VPN 实例",
    "com_ssh": "配置 SSH 远程管理",
    "com_user": "新建本地管理账号",
    "com_log": "配置 Syslog 日志服务器上报",
    "com_snmp": "配置 SNMP 网络管理",
    "sw_vlan": "创建 VLAN 并配置网关",
    "sw_access": "配置接入口为 Access 模式",
    "sw_trunk": "配置级联口为 Trunk 模式",
    "sw_dhcp": "配置 DHCP 地址池（自动分配 IP）",
    "sw_stp": "配置生成树协议（防止环路）",
}


class GenRequest(BaseModel):
    code: str = ""
    vendor: str = ""
    params: Dict[str, str] = {}


def build(code: str, m: Dict[str, str], vendor: str):
    if vendor == "h3c":
        return build_h3c(code, m)
    if vendor == "cisco":
        return build_cisco(code, m)
    return build_huawei(code, m)


@router.post("/generate")
def generate(req: GenRequest):
    label, msg = validate.validate_req(req.code, req.params or {})
    if msg:
        raise HTTPException(status_code=400, detail=f"{label}: {msg}")
    lines = build(req.code, req.params or {}, req.vendor)
    filtered = []
    for l in lines:
        t = l.strip()
        if t in ("quit", "exit", "end"):
            continue
        filtered.append(l)
    title = COMMENTS.get(req.code) or req.code
    return {"script": "\n".join(["// " + title] + filtered)}
