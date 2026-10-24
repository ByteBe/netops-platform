# -*- coding: utf-8 -*-
"""
NetOps Python 智能外围 - AI 巡检报告服务（FastAPI 常驻服务）
由 Go 主程序 /api/aiinspect/run 转发调用。
职责编排：接收 Go 采集快照 -> MySQL 统计 -> TDengine 趋势 -> LLM 生成专业巡检报告。
"""
import datetime
import os
import sys
from typing import Any, Dict

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

# 兼容任意启动目录：pyservice 根（scriptgen 等子包）与 aiinspect 自身目录均加入 sys.path
_HERE = os.path.dirname(os.path.abspath(__file__))
_ROOT = os.path.dirname(_HERE)
for _p in (_ROOT, _HERE):
    if _p not in sys.path:
        sys.path.insert(0, _p)

import mysql_stats
import tdengine_stats
import prompt as prompt_builder
import llm_config
import llm
import fallback_report
from config import PY_PORT

app = FastAPI(title="netops-python", version="1.1.0")

# 挂载脚本生成器子路由（同进程复用 8000 端口，Go 转发调用）
from scriptgen.router import router as scriptgen_router

app.include_router(scriptgen_router)


class RunRequest(BaseModel):
    snapshot: Dict[str, Any] = {}
    time: str = ""


class RunResponse(BaseModel):
    report: str
    snapshot: Dict[str, Any]
    stats: Dict[str, Any]
    time: str
    source: str  # llm | fallback-stats


@app.get("/healthz")
def healthz():
    return JSONResponse({"ok": True, "service": "aiinspect"})


@app.post("/internal/aiinspect/run", response_model=RunResponse)
def run(req: RunRequest):
    snapshot = req.snapshot or {}
    now_str = req.time or datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    # 1) MySQL 统计增强
    extra = mysql_stats.mysql_stats()

    # 2) TDengine 趋势（尽力而为，失败不影响报告）
    trends = tdengine_stats.tdengine_trends()

    stats_data: Dict[str, Any] = {"extra": extra, "trends": trends}

    # 3) 构造 prompt 并调用 LLM；失败降级为统计报告
    prompt_text = prompt_builder.build_prompt(snapshot, extra, trends)
    cfg = llm_config.load_config()
    source = "fallback-stats"
    report = fallback_report.fallback_report(snapshot, extra, trends)
    if cfg is not None:
        ai_report = llm.chat(cfg, prompt_text)
        if ai_report:
            report = ai_report
            source = "llm"

    return RunResponse(
        report=report,
        snapshot=snapshot,
        stats=stats_data,
        time=now_str,
        source=source,
    )


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=PY_PORT)
