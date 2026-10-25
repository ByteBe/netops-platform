# -*- coding: utf-8 -*-
"""数据清洗服务路由：POST /internal/clean/run —— Go 转发触发（默认 dry_run 只统计）。"""
from fastapi import APIRouter
from pydantic import BaseModel

from cleaner.cleaner import run_clean

cleaner_router = APIRouter(prefix="/internal/clean", tags=["clean"])


class CleanReq(BaseModel):
    mode: str = "all"        # mysql | tdengine | all
    dry_run: bool = True     # 默认只统计不修改
    metric: str = "dev_cpu"  # tdengine 模式指定指标
    days: int = 7            # tdengine 时间窗口


@cleaner_router.post("/run")
def do_clean(req: CleanReq):
    return run_clean(req.mode, req.dry_run, req.metric, req.days)
