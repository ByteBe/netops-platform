# -*- coding: utf-8 -*-
"""配置只读审计差异对比路由：POST /internal/configaudit/diff。

职责：对 Go 侧 SSH 读取的新旧运行配置做结构化行级 diff（difflib），
返回兼容原 Go 实现的 diff 文本（新增行最多 20 行 / "no change"）
外加 added/removed 结构化字段，供后续前端增强展示。
审计仅读配置对比，不下发任何配置。
"""
from fastapi import APIRouter
from pydantic import BaseModel

configaudit_router = APIRouter(prefix="/internal/configaudit", tags=["configaudit"])

MAX_SHOW = 20


class DiffReq(BaseModel):
    old_content: str = ""
    new_content: str = ""


def _clean_lines(content: str) -> list:
    out = []
    for line in content.splitlines():
        line = line.strip()
        if line:
            out.append(line)
    return out


@configaudit_router.post("/diff")
def do_diff(req: DiffReq):
    old_lines = _clean_lines(req.old_content or "")
    new_lines = _clean_lines(req.new_content or "")
    old_set = set(old_lines)
    new_set = set(new_lines)

    added = [line for line in new_lines if line not in old_set]
    removed = [line for line in old_lines if line not in new_set]

    if not added and not removed:
        return {"changed": False, "diff": "no change", "added": [], "removed": []}

    text = "\n".join(added[:MAX_SHOW])
    return {"changed": True, "diff": text, "added": added, "removed": removed}
