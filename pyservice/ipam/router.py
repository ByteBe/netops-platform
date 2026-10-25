# -*- coding: utf-8 -*-
"""IP 地址管理工具路由：Go ipam 模块 SSH 执行后调用（命令生成/输出校验/ARP 解析）。"""
from fastapi import APIRouter
from pydantic import BaseModel

from ipam.tools import build_bind_cmds, build_undo_cmds, has_bind_error, parse_arp

ipam_router = APIRouter(prefix="/internal/ipam", tags=["ipam"])


class BindReq(BaseModel):
    ip: str = ""
    mac: str = ""
    vlan: int = 0
    vendor: str = ""


@ipam_router.post("/bind-cmd")
def bind_cmd(req: BindReq):
    cmds, err = build_bind_cmds(req.ip, req.mac, req.vlan, req.vendor)
    if err:
        return {"ok": False, "error": err}
    return {"ok": True, "commands": cmds}


class UndoReq(BaseModel):
    ip: str = ""
    vlan: int = 0
    vendor: str = ""


@ipam_router.post("/undo-cmd")
def undo_cmd(req: UndoReq):
    cmds, err = build_undo_cmds(req.ip, req.vlan, req.vendor)
    if err:
        return {"ok": False, "error": err}
    return {"ok": True, "commands": cmds}


class OutReq(BaseModel):
    out: str = ""


@ipam_router.post("/check-output")
def check_output(req: OutReq):
    return {"ok": True, "has_error": has_bind_error(req.out)}


@ipam_router.post("/parse-arp")
def parse_arp_api(req: OutReq):
    items = parse_arp(req.out)
    return {"ok": True, "count": len(items), "items": items}
