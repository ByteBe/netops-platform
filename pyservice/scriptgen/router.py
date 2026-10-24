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
    return {"script": "\n".join(filtered)}
