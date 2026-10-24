# -*- coding: utf-8 -*-
"""MySQL 统计增强：活跃告警分布、今日告警、24h 日志分布、设备状态、采集失败任务。
数据口径与 Go 主程序一致（活跃告警 = acked=0 未确认；今日告警用本地日期）。
连不上时返回 error 不阻塞报告。"""
import datetime
from typing import Any, Dict, List

import pymysql

from config import MYSQL


def _conn():
    return pymysql.connect(
        host=MYSQL["host"], port=MYSQL["port"], user=MYSQL["user"],
        password=MYSQL["password"], database=MYSQL["database"],
        charset="utf8mb4", connect_timeout=5, read_timeout=20,
    )


def _q(conn, sql: str) -> List[tuple]:
    with conn.cursor() as cur:
        cur.execute(sql)
        return cur.fetchall()


def mysql_stats() -> Dict[str, Any]:
    """汇总各维度统计，任一失败整体降级（ok=false + error）。"""
    out: Dict[str, Any] = {"ok": False}
    try:
        conn = _conn()
        try:
            rows = _q(conn, "SELECT level, COUNT(*) c FROM alert_events WHERE acked=0 GROUP BY level ORDER BY c DESC")
            out["alert_by_level"] = [{"level": r[0] or "-", "count": r[1]} for r in rows]

            rows = _q(conn, f"SELECT COUNT(*) FROM alert_events WHERE DATE(created_at) = '{datetime.datetime.now().strftime('%Y-%m-%d')}'")
            out["alert_today"] = rows[0][0] if rows else 0

            rows = _q(conn, "SELECT level, COUNT(*) c FROM device_logs WHERE created_at >= DATE_SUB(NOW(), INTERVAL 24 HOUR) GROUP BY level ORDER BY c DESC")
            out["log_24h_by_level"] = [{"level": r[0] or "-", "count": r[1]} for r in rows]

            rows = _q(conn, "SELECT target, COUNT(*) c FROM alert_events WHERE acked=0 GROUP BY target ORDER BY c DESC LIMIT 8")
            out["alert_top_targets"] = [{"target": r[0] or "-", "count": r[1]} for r in rows]

            rows = _q(conn, "SELECT status, COUNT(*) c FROM monitor_devices GROUP BY status")
            out["device_by_status"] = [{"status": r[0] or "-", "count": r[1]} for r in rows]

            rows = _q(conn, "SELECT device_name, type, COUNT(*) c FROM task_logs WHERE success=0 AND created_at >= DATE_SUB(NOW(), INTERVAL 24 HOUR) GROUP BY device_name, type ORDER BY c DESC LIMIT 8")
            out["task_fail_24h"] = [{"device": r[0] or "-", "type": r[1] or "-", "count": r[2]} for r in rows]

            out["ok"] = True
        finally:
            conn.close()
    except Exception as e:
        out["error"] = str(e)
    return out
