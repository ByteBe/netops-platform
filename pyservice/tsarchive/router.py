# -*- coding: utf-8 -*-
"""时序归档路由：POST /internal/tsarchive/run —— Go 调度器触发（daily/retry）。"""
import datetime

from fastapi import APIRouter
from pydantic import BaseModel

from tsarchive.archive import run_archive

tsarchive_router = APIRouter(prefix="/internal/tsarchive", tags=["tsarchive"])


class RunReq(BaseModel):
    mode: str = "daily"  # daily | retry
    now: str = ""


@tsarchive_router.post("/run")
def do_run(req: RunReq):
    now = datetime.datetime.now()
    if req.now:
        try:
            now = datetime.datetime.strptime(req.now, "%Y-%m-%d %H:%M:%S")
        except ValueError:
            pass
    return run_archive(req.mode or "daily", now)
