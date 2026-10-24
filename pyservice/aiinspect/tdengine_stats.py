# -*- coding: utf-8 -*-
"""TDengine 趋势查询：列出设备 CPU 子表，计算近 24h 均值与最近 1h 均值及趋势方向。
尽力而为：TDengine 不可达或表缺失时返回 None，不阻塞报告生成。"""
from typing import Any, Dict, List, Optional

import requests

from config import TDENGINE


def _td_query(sql: str) -> Optional[List[dict]]:
    """TDengine REST 查询（/rest/sql）。"""
    url = f"http://{TDENGINE['host']}:{TDENGINE['port']}/rest/sql"
    resp = requests.post(url, data=sql.encode(), auth=(TDENGINE["user"], TDENGINE["password"]),
                         timeout=15)
    if resp.status_code != 200:
        return None
    j = resp.json()
    if j.get("code") not in (0, None) or not j.get("data"):
        return None
    cols = j.get("column_meta") or []
    names = [c[0] for c in cols]
    return [dict(zip(names, row)) for row in j["data"]]


def _list_cpu_tables() -> List[str]:
    """列出 data 库 t_dev_cpu% 子表（TDengine 3.x information_schema）。"""
    q = (f"SELECT table_name FROM information_schema.ins_tables "
         f"WHERE db_name='{TDENGINE['database']}' AND table_name LIKE 't_dev_cpu%' LIMIT 6")
    rows = _td_query(q)
    if not rows:
        q2 = (f"SELECT tbname FROM information_schema.ins_tables "
              f"WHERE db_name='{TDENGINE['database']}' AND table_name LIKE 't_dev_cpu%'")
        rows = _td_query(q2)
    return [r.get("table_name") or r.get("tbname") or "" for r in (rows or []) if r]


def _direction(recent: Optional[float], avg24: float) -> str:
    """根据最近 1h 与 24h 均值差异判断趋势方向。"""
    if recent is None or avg24 <= 0:
        return "平稳"
    ratio = (recent - avg24) / avg24
    if ratio > 0.08:
        return "上升"
    if ratio < -0.08:
        return "下降"
    return "平稳"


def tdengine_trends() -> Optional[Dict[str, Any]]:
    """近 24h 设备 CPU 均值与最近窗口变化趋势。"""
    try:
        tables = _list_cpu_tables()
        if not tables:
            return None
        db = TDENGINE["database"]
        items = []
        for tb in tables[:5]:
            rows = _td_query(f"SELECT AVG(val) avgv FROM {db}.{tb} WHERE ts >= NOW() - 24h")
            if not rows or not rows[0].get("avgv"):
                continue
            avg24 = float(rows[0]["avgv"])
            r2 = _td_query(f"SELECT AVG(val) avgv FROM {db}.{tb} WHERE ts >= NOW() - 1h")
            recent = float(r2[0]["avgv"]) if r2 and r2[0].get("avgv") else None
            items.append({
                "table": tb,
                "avg_24h": round(avg24, 1),
                "recent_1h": round(recent, 1) if recent is not None else None,
                "trend": _direction(recent, avg24),
            })
        return {"cpu_items": items, "ok": True} if items else None
    except Exception as e:
        return {"ok": False, "error": str(e)}
