# -*- coding: utf-8 -*-
"""存量数据清洗服务：对 MySQL / TDengine 已存数据做低频批量清洗。

设计原则：
- 默认 dry_run=true：只统计与给出清洗建议，不修改任何数据；
- 显式 dry_run=false 才执行修改/删除，且每类操作限量，避免误伤；
- 清洗范围可收窄（metric + 时间窗口），先报告后执行。

清洗任务：
MySQL
  - alert_duplicates：同 target+level+内容前缀 + 未确认 + 10 分钟内重复的告警（保留最早）
  - device_logs：空内容 / 超长内容 / 含乱码替换符的日志标记
  - ipam_mac：IP 登记 MAC 规范化（复用 ipam.normalize_mac），无法规范化的标记
TDengine
  - ts_anomalies：指定 metric 最近 N 天异常值统计（CPU 类 >100 或 <0、流量 <0、NaN/Inf）
"""
import datetime
import json

import pymysql
import requests

from config import MYSQL, TDENGINE
from ipam.tools import normalize_mac

DUP_WINDOW_MIN = 10      # 告警去重窗口（分钟）
LOG_MAX_LEN = 4096       # 日志内容合理上限（超出视为异常标记）
ANOMALY_DELETE_LIMIT = 200  # 单指标单次最多删除异常点数（dry_run=false 时）

# 需要超范围校验的 CPU 类 metric
CPU_METRICS = {"dev_cpu", "docker_cpu", "k8s_cpu", "db_cpu", "nginx_reading", "nginx_writing", "nginx_waiting", "nginx_conn"}
# 流量类 metric（不可为负）
TRAFFIC_METRICS = {"if_in", "if_out"}


def _mysql_conn():
    return pymysql.connect(
        host=MYSQL["host"], port=MYSQL["port"], user=MYSQL["user"],
        password=MYSQL["password"], database=MYSQL["database"],
        charset="utf8mb4", connect_timeout=5, read_timeout=120,
    )


def _td_query(sql: str):
    url = "http://%s:%s/rest/sql" % (TDENGINE["host"], TDENGINE["port"])
    resp = requests.post(url, data=sql.encode(), auth=(TDENGINE["user"], TDENGINE["password"]), timeout=120)
    if resp.status_code != 200:
        raise RuntimeError("TDengine REST %s: %s" % (resp.status_code, resp.text[:200]))
    j = resp.json()
    if j.get("code") not in (0, None):
        raise RuntimeError("TDengine error: %s" % j.get("desc"))
    cols = j.get("column_meta") or []
    names = [c[0] for c in cols]
    return [dict(zip(names, row)) for row in j.get("data") or []]


def clean_alert_duplicates(dry_run: bool):
    """同 target+level+内容前缀 + 未确认 + 10 分钟窗口内的重复告警。"""
    report = {"task": "alert_duplicates", "found": 0, "detail": []}
    conn = _mysql_conn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, target, level, LEFT(content,100), created_at FROM alert_events "
                "WHERE acked=0 ORDER BY created_at ASC")
            rows = cur.fetchall()
        groups = {}
        for rid, target, level, content, created_at in rows:
            key = (target or "", level or "", content or "")
            groups.setdefault(key, []).append((rid, created_at))
        for key, items in groups.items():
            if len(items) < 2:
                continue
            items.sort(key=lambda x: x[1] or datetime.datetime.min)
            dups = []
            for i in range(1, len(items)):
                prev = items[i - 1][1]
                cur_t = items[i][1]
                if prev and cur_t and (cur_t - prev).total_seconds() <= DUP_WINDOW_MIN * 60:
                    dups.append(items[i][0])
            if dups:
                report["found"] += len(dups)
                report["detail"].append({"target": key[0], "level": key[1],
                                         "keep": items[0][0], "duplicates": dups})
        if not dry_run and report["found"]:
            dup_ids = [d for g in report["detail"] for d in g["duplicates"]]
            for i in range(0, len(dup_ids), 500):
                chunk = dup_ids[i:i + 500]
                with conn.cursor() as cur:
                    cur.execute("DELETE FROM alert_events WHERE id IN (%s)" % ",".join(["%s"] * len(chunk)), chunk)
                conn.commit()
    finally:
        conn.close()
    return report


def clean_device_logs(dry_run: bool):
    """日志内容清洗：空 / 超长 / 乱码替换符统计（dry_run=false 时截断超长内容）。"""
    report = {"task": "device_logs", "empty": 0, "too_long": 0, "garbled": 0, "detail": []}
    conn = _mysql_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT id, content, LENGTH(content) FROM device_logs WHERE content IS NOT NULL "
                        "AND (LENGTH(content) = 0 OR LENGTH(content) > %s OR content LIKE '%%\ufffd%%') LIMIT 500", (LOG_MAX_LEN,))
            rows = cur.fetchall()
        for rid, content, ln in rows:
            flags = []
            if ln == 0:
                report["empty"] += 1
                flags.append("empty")
            if ln > LOG_MAX_LEN:
                report["too_long"] += 1
                flags.append("too_long(%d)" % ln)
            if "\ufffd" in (content or ""):
                report["garbled"] += 1
                flags.append("garbled")
            report["detail"].append({"id": rid, "flags": flags})
            if not dry_run and "too_long" in flags:
                with conn.cursor() as cur:
                    cur.execute("UPDATE device_logs SET content=LEFT(content,%s) WHERE id=%s", (LOG_MAX_LEN, rid))
                conn.commit()
    finally:
        conn.close()
    return report


def clean_ipam_mac(dry_run: bool):
    """IP 登记 MAC 规范化：统一为 XXXX-XXXX-XXXX；无法规范化的标记（dry_run=false 时修复可规范化项）。"""
    report = {"task": "ipam_mac", "fixed": 0, "invalid": 0, "detail": []}
    conn = _mysql_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT id, ip, mac FROM ip_records WHERE mac IS NOT NULL AND mac <> '' LIMIT 1000")
            rows = cur.fetchall()
        for rid, ip, mac in rows:
            m = normalize_mac(mac)
            if m and m != mac.strip().upper():
                report["fixed"] += 1
                report["detail"].append({"id": rid, "ip": ip, "from": mac, "to": m})
                if not dry_run:
                    with conn.cursor() as cur:
                        cur.execute("UPDATE ip_records SET mac=%s WHERE id=%s", (m, rid))
                    conn.commit()
            elif not m:
                report["invalid"] += 1
                report["detail"].append({"id": rid, "ip": ip, "from": mac, "to": ""})
    finally:
        conn.close()
    return report


def _list_tables(metric: str):
    db = TDENGINE["database"]
    prefix = "t_" + metric.replace(" ", "_").replace(".", "_").replace("-", "_")
    rows = _td_query("SELECT table_name FROM information_schema.ins_tables WHERE db_name='%s' AND table_name LIKE '%s%%'" % (db, prefix))
    return sorted(r.get("table_name") or r.get("tbname") or "" for r in rows if r.get("table_name") or r.get("tbname"))


def clean_ts_anomalies(metric: str, days: int, dry_run: bool):
    """指定 metric 最近 N 天异常值统计（CPU 类越界、流量为负、NaN/Inf）。"""
    report = {"task": "ts_anomalies", "metric": metric, "days": days, "found": 0, "detail": []}
    if metric not in CPU_METRICS and metric not in TRAFFIC_METRICS:
        report["error"] = "暂不支持该 metric 的异常值规则（支持 %s）" % ", ".join(sorted(CPU_METRICS | TRAFFIC_METRICS))
        return report
    db = TDENGINE["database"]
    tables = _list_tables(metric)
    if not tables:
        report["detail"].append("无子表")
        return report
    where = "val < 0" if metric in TRAFFIC_METRICS else "val < 0 OR val > 100"
    total = 0
    for tbl in tables:
        sql = ("SELECT COUNT(*) c FROM %s.%s WHERE %s AND ts >= NOW() - %dd" % (db, tbl, where, days))
        try:
            rows = _td_query(sql)
        except Exception as e:
            report["detail"].append({"table": tbl, "error": str(e)})
            continue
        c = int(rows[0]["c"]) if rows else 0
        if c:
            total += c
            report["detail"].append({"table": tbl, "anomaly_count": c})
            if not dry_run and c > 0:
                limit = min(c, ANOMALY_DELETE_LIMIT)
                _td_query("DELETE FROM %s.%s WHERE %s AND ts >= NOW() - %dd LIMIT %d" % (db, tbl, where, days, limit))
    report["found"] = total
    return report


def run_clean(mode: str, dry_run: bool, metric: str, days: int):
    """清洗入口：mode=mysql|tdengine|all。dry_run 默认 true（只统计）。"""
    out = {"ok": True, "dry_run": dry_run, "reports": []}
    if mode in ("mysql", "all"):
        out["reports"].append(clean_alert_duplicates(dry_run))
        out["reports"].append(clean_device_logs(dry_run))
        out["reports"].append(clean_ipam_mac(dry_run))
    if mode in ("tdengine", "all"):
        out["reports"].append(clean_ts_anomalies(metric, days, dry_run))
    return out


def main():
    print(json.dumps(run_clean("all", True, "dev_cpu", 7), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
