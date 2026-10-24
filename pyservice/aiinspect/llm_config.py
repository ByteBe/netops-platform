# -*- coding: utf-8 -*-
"""LLM 配置读取：从 MySQL ai_configs 表读取启用配置（与 Go 主程序同源）。"""
from typing import Any, Dict, Optional

import pymysql

from config import MYSQL


def load_config() -> Optional[Dict[str, Any]]:
    """读取启用的 AI 配置（priority 最高优先）；失败或未配置返回 None。"""
    try:
        conn = pymysql.connect(
            host=MYSQL["host"], port=MYSQL["port"], user=MYSQL["user"],
            password=MYSQL["password"], database=MYSQL["database"],
            charset="utf8mb4", connect_timeout=5, read_timeout=10,
        )
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT provider, base_url, api_key, model, temperature FROM ai_configs WHERE enable=1 ORDER BY priority DESC LIMIT 1")
                row = cur.fetchone()
                if not row:
                    return None
                return {
                    "provider": row[0], "base_url": (row[1] or "").rstrip("/"),
                    "api_key": row[2] or "", "model": row[3] or "",
                    "temperature": float(row[4] or 0.7),
                }
        finally:
            conn.close()
    except Exception:
        return None
