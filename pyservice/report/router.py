# -*- coding: utf-8 -*-
"""报告渲染 FastAPI 路由：POST /internal/report/render —— Go 收集数据后转发渲染。"""
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from report.render import render_html

report_router = APIRouter(prefix="/internal/report", tags=["report"])


class RenderReq(BaseModel):
    title: str = "运维巡检报告"
    start: str = ""
    end: str = ""
    now: str = ""
    stats: dict = {}
    sections: list = []
    link: list = []
    dev: list = []
    db: list = []
    nginx: list = []
    container: list = []
    node: list = []


@report_router.post("/render")
def do_render(req: RenderReq):
    try:
        html = render_html(req.dict())
        if not html or len(html) < 200:
            raise HTTPException(status_code=400, detail="渲染结果为空")
        return {"html": html}
    except HTTPException:
        raise
    except Exception as e:  # noqa: BLE001
        raise HTTPException(status_code=400, detail="Label: 渲染失败: %s" % e)
