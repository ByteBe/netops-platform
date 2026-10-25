# -*- coding: utf-8 -*-
"""时序数据每日归档核心逻辑：查 TDengine -> 校验 -> 聚合 -> 写 MySQL -> 清理原始数据。

复刻原 Go 实现口径（功能等价）：
- 表命名 t_<metric> 及 t_<metric>_<taghash>，经 information_schema.ins_tables 枚举；
- 无 tags 序列的 tags_key = sha1("__table=<表名>;")，保证与历史归档记录一致；
- 校验：非空 / NaN·Inf / 时间戳在归档范围 / 名称与标签无乱码；
- 聚合：avg / max / min / last / count；
- 幂等：ts_archives 唯一索引 (metric, field, tags_key, day) 兜底（INSERT ... ON DUPLICATE KEY）；
- 清理：校验通过并转存的序列按表+时间范围 DELETE；
- 失败登记 ts_archive_retries（pending），重试达 5 次标记 discard，原始数据保留待人工核查。
"""
import datetime
import hashlib
import json
import math
import re

import pymysql
import requests

from config import MYSQL, TDENGINE

_SANITIZE_RE = re.compile(r"[^a-zA-Z0-9_]")


def _sanitize(s: str) -> str:
    """复刻 Go tsdb.Sanitize：非字母数字下划线一律替换为 _。"""
    return _SANITIZE_RE.sub("_", s)

# 全部监控指标（与各采集器写入 TDengine 的 metric 保持一致）
METRICS = [
    # SNMP 设备
    "dev_up", "dev_cpu", "dev_mem", "if_in", "if_out",
    # 链路
    "link_rt", "link_loss", "link_status",
    # 数据库
    "db_avail", "db_conn", "db_size", "db_tx", "db_err", "db_cpu", "db_mem",
    # 容器 / K8s
    "docker_cpu", "docker_mem", "k8s_cpu", "k8s_mem",
    # Nginx
    "nginx_avail", "nginx_conn", "nginx_accepts", "nginx_handled", "nginx_req",
    "nginx_qps", "nginx_reading", "nginx_writing", "nginx_waiting",
    # BGP
    "bgp_peer_state",
]

RETRY_MAX = 5          # 单条记录最大自动重试次数
RETRY_WINDOW_DAY = 7   # 只补传最近 N 天内失败的归档
TABLES_PREFIX = "t_"   # TDengine 表前缀

# 物理范围校验分类（转化链路清洗规则，见 _clean_points）
CPU_PCT_METRICS = {"dev_cpu", "docker_cpu", "k8s_cpu", "db_cpu"}  # 百分比，0~100
TRAFFIC_METRICS = {"if_in", "if_out"}                            # 流量，不可为负


def _mysql_conn():
    return pymysql.connect(
        host=MYSQL["host"], port=MYSQL["port"], user=MYSQL["user"],
        password=MYSQL["password"], database=MYSQL["database"],
        charset="utf8mb4", connect_timeout=5, read_timeout=60,
    )


def _td_query(sql: str):
    """TDengine REST 查询，返回行 dict 列表；失败抛异常。"""
    url = "http://%s:%s/rest/sql" % (TDENGINE["host"], TDENGINE["port"])
    resp = requests.post(url, data=sql.encode(), auth=(TDENGINE["user"], TDENGINE["password"]), timeout=60)
    if resp.status_code != 200:
        raise RuntimeError("TDengine REST %s: %s" % (resp.status_code, resp.text[:200]))
    j = resp.json()
    if j.get("code") not in (0, None):
        raise RuntimeError("TDengine error: %s" % j.get("desc"))
    cols = j.get("column_meta") or []
    names = [c[0] for c in cols]
    return [dict(zip(names, row)) for row in j.get("data") or []]


def _parse_ts_ms(v):
    """复刻 Go parseTsMs：REST 返回的 ts 可能是 int/float 毫秒或多种时间字符串。"""
    if v is None:
        return 0
    if isinstance(v, bool):
        return int(v)
    if isinstance(v, int):
        return v
    if isinstance(v, float):
        return int(v)
    s = str(v).strip()
    if not s:
        return 0
    if s.isdigit():
        i = int(s)
        return i if i > 1e12 else i * 1000
    for fmt in ("%Y-%m-%d %H:%M:%S.%f", "%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S.%f%z",
                "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%S"):
        try:
            return int(datetime.datetime.strptime(s, fmt).timestamp() * 1000)
        except ValueError:
            continue
    return 0


def _is_clean_text(s: str) -> bool:
    """非乱码性校验：无可替换符、无控制字符（对标 Go isCleanText）。"""
    if "\ufffd" in s:
        return False
    for ch in s:
        if ord(ch) < 0x20 and ch != "\t":
            return False
    return True


def setting_bool(key: str, default: bool) -> bool:
    try:
        conn = _mysql_conn()
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT value FROM system_settings WHERE `key`=%s", (key,))
                row = cur.fetchone()
            return row[0] == "true" if row else default
        finally:
            conn.close()
    except Exception:
        return default


def _list_tables(metric: str):
    """枚举该 metric 的全部子表（t_<metric> 及 t_<metric>_<taghash>）。"""
    db = TDENGINE["database"]
    prefix = TABLES_PREFIX + _sanitize(metric)
    try:
        rows = _td_query(
            "SELECT table_name FROM information_schema.ins_tables "
            "WHERE db_name='%s' AND table_name LIKE '%s%%'" % (db, prefix))
        names = [r.get("table_name") or r.get("tbname") for r in rows]
    except Exception:
        rows = _td_query("SHOW TABLES")
        names = []
        for r in rows:
            name = r.get("table_name") or r.get("tbname") or ""
            if name.startswith(prefix):
                names.append(name)
    names = [n for n in names if n]
    return sorted(names)


def _query_series(metric: str, tbl: str, start_ms: int, end_ms: int):
    """查询单表当天全部点，按 fname 分组返回 {field: [(ts_ms, val), ...]}。"""
    db = TDENGINE["database"]
    rows = _td_query(
        "SELECT ts, fname, val FROM %s.%s WHERE ts >= %d AND ts <= %d" % (db, tbl, start_ms, end_ms))
    groups = {}
    order = []
    for r in rows:
        ts_ms = _parse_ts_ms(r.get("ts"))
        field = str(r.get("fname") or "")
        try:
            val = float(r.get("val"))
        except (TypeError, ValueError):
            continue
        if field not in groups:
            groups[field] = []
            order.append(field)
        groups[field].append((ts_ms, val))
    return groups, order


def _clean_points(metric: str, points, start_ms: int, end_ms: int):
    """转化链路清洗（逐点）：
    1. 剔除时间戳越界的点（落在归档窗口之外）；
    2. 剔除 NaN / Inf；
    3. 按 metric 物理范围剔除不可能值（CPU 百分比 <0 或 >100、流量 <0）；
    4. 同时间戳重复采样去重，保留最后一次采样（采集边界重复兜底）。
    返回 (清洗后按时间升序的点列表, 剔除点数)。
    """
    if metric in CPU_PCT_METRICS:
        def _out_of_range(v):
            return v < 0 or v > 100
    elif metric in TRAFFIC_METRICS:
        def _out_of_range(v):
            return v < 0
    else:
        def _out_of_range(v):
            return False
    dedup = {}
    removed = 0
    for ts_ms, v in points:
        if ts_ms < start_ms or ts_ms > end_ms + 1:
            removed += 1
            continue
        if math.isnan(v) or math.isinf(v):
            removed += 1
            continue
        if _out_of_range(v):
            removed += 1
            continue
        dedup[ts_ms] = v  # 同时间戳保留最后采样
    return sorted(dedup.items()), removed


def _validate(metric: str, field: str, tags: dict):
    """结构性校验（清洗无法修复的问题）：名称/标签非乱码。返回错误信息（None=通过）。"""
    for s in (metric, field):
        if not _is_clean_text(s):
            return "名称含乱码: %r" % s
    for k, v in tags.items():
        if not _is_clean_text(k) or not _is_clean_text(v):
            return "标签含乱码: %r=%r" % (k, v)
    return None


def _agg(points):
    """聚合原始采样点为 avg/max/min/last/count。"""
    total = 0.0
    mn = mx = points[0][1]
    last = points[0][1]
    cnt = 0
    for _, v in points:
        total += v
        if v < mn:
            mn = v
        if v > mx:
            mx = v
        last = v
        cnt += 1
    return total / cnt, mx, mn, last, cnt


def _sha1_hex(s: str) -> str:
    return hashlib.sha1(s.encode("utf-8")).hexdigest()


def _tags_key(tags: dict) -> str:
    """复刻 Go TagsKey：k=v; 逐项拼接（归档场景仅 __table 单键，顺序稳定）。"""
    if not tags:
        return ""
    return "".join("%s=%s;" % (k, v) for k, v in tags.items())


def _upsert_retry(conn, metric: str, day: str, reason: str):
    with conn.cursor() as cur:
        cur.execute("SELECT id FROM ts_archive_retries WHERE metric=%s AND day=%s", (metric, day))
        row = cur.fetchone()
        if row:
            cur.execute("UPDATE ts_archive_retries SET reason=%s, updated_at=%s WHERE id=%s",
                        (reason, datetime.datetime.now(), row[0]))
        else:
            cur.execute(
                "INSERT INTO ts_archive_retries (metric, day, reason, retries, status, created_at, updated_at) "
                "VALUES (%s,%s,%s,0,'pending',%s,%s)",
                (metric, day, reason, datetime.datetime.now(), datetime.datetime.now()))
    conn.commit()


def _clear_retry(conn, metric: str, day: str):
    with conn.cursor() as cur:
        cur.execute("DELETE FROM ts_archive_retries WHERE metric=%s AND day=%s", (metric, day))
    conn.commit()


def _archive_metric(metric: str, day: str, start_ms: int, end_ms: int, cleanup: bool):
    """归档单个指标：查 -> 清洗（去重/剔除异常值）-> 校验（结构性）-> 聚合写 MySQL -> 清理 TDengine。
    返回 (写入条数, 清洗剔除点数)。"""
    tables = _list_tables(metric)
    if not tables:
        return 0, 0
    conn = _mysql_conn()
    try:
        written = 0
        cleaned = 0
        bad = 0
        failed = False
        for tbl in tables:
            groups, order = _query_series(metric, tbl, start_ms, end_ms)
            for field in order:
                points = groups[field]
                tags = {"__table": tbl}
                err = _validate(metric, field, tags)
                if err:
                    bad += 1
                    _upsert_retry(conn, metric, day, "校验未通过: %s" % err)
                    continue
                points, removed = _clean_points(metric, points, start_ms, end_ms)
                cleaned += removed
                if not points:
                    bad += 1
                    _upsert_retry(conn, metric, day, "清洗后无有效采样点")
                    continue
                avg, mx, mn, last, cnt = _agg(points)
                tags_json = json.dumps(tags, ensure_ascii=False)
                tags_key = _sha1_hex(_tags_key(tags))
                with conn.cursor() as cur:
                    cur.execute(
                        "INSERT INTO ts_archives (metric, field, tags_key, tags, day, avg_val, max_val, min_val, "
                        "last_val, count, created_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s) "
                        "ON DUPLICATE KEY UPDATE id=id",
                        (metric, field, tags_key, tags_json, day, avg, mx, mn, last, cnt, datetime.datetime.now()))
                conn.commit()
                written += 1
                if cleanup:
                    try:
                        _td_query("DELETE FROM %s.%s WHERE ts >= %d AND ts <= %d"
                                  % (TDENGINE["database"], tbl, start_ms, end_ms))
                    except Exception as e:
                        failed = True
                        _upsert_retry(conn, metric, day, "清除时序库失败: %s" % e)
        if bad == 0 and not failed:
            _clear_retry(conn, metric, day)
        return written, cleaned
    finally:
        conn.close()


def retry_failed(now: datetime.datetime):
    """补传 pending 归档：成功删除记录，失败计数，达到上限标记 discard。"""
    cutoff = (now - datetime.timedelta(days=RETRY_WINDOW_DAY)).strftime("%Y-%m-%d")
    conn = _mysql_conn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT metric, day FROM ts_archive_retries WHERE status='pending' AND day >= %s "
                "ORDER BY day ASC LIMIT 50", (cutoff,))
            items = [(r[0], r[1]) for r in cur.fetchall()]
        done = 0
        for metric, day in items:
            dt = datetime.datetime.strptime(day, "%Y-%m-%d").date()
            start = datetime.datetime.combine(dt, datetime.time.min)
            end = start + datetime.timedelta(days=1) - datetime.timedelta(milliseconds=1)
            _archive_metric(metric, day,
                            int(start.timestamp() * 1000), int(end.timestamp() * 1000), False)
            with conn.cursor() as cur:
                cur.execute("SELECT id, retries FROM ts_archive_retries WHERE metric=%s AND day=%s", (metric, day))
                row = cur.fetchone()
            if row:
                retries = row[1] + 1
                if retries >= RETRY_MAX:
                    with conn.cursor() as cur:
                        cur.execute("UPDATE ts_archive_retries SET retries=%s, status='discard', updated_at=%s "
                                    "WHERE id=%s", (retries, datetime.datetime.now(), row[0]))
                else:
                    with conn.cursor() as cur:
                        cur.execute("UPDATE ts_archive_retries SET retries=%s, updated_at=%s WHERE id=%s",
                                    (retries, datetime.datetime.now(), row[0]))
                conn.commit()
            else:
                done += 1
        return done
    finally:
        conn.close()


def run_archive(mode: str, now: datetime.datetime):
    """归档入口：daily=先补传再归档前一天；retry=只补传。"""
    if not setting_bool("ts_archive_enable", True):
        return {"ok": True, "total": 0, "errors": [], "skipped": "disabled"}
    retried = retry_failed(now)
    if mode == "retry":
        return {"ok": True, "total": 0, "retried": retried, "errors": []}
    cleanup = setting_bool("ts_archive_cleanup", True)
    day = now - datetime.timedelta(days=1)
    day_str = day.strftime("%Y-%m-%d")
    start = datetime.datetime.combine(day.date(), datetime.time.min)
    end = start + datetime.timedelta(days=1) - datetime.timedelta(milliseconds=1)
    start_ms = int(start.timestamp() * 1000)
    end_ms = int(end.timestamp() * 1000)
    total = 0
    cleaned = 0
    errors = []
    for m in METRICS:
        try:
            w, c = _archive_metric(m, day_str, start_ms, end_ms, cleanup)
            total += w
            cleaned += c
        except Exception as e:
            errors.append("%s: %s" % (m, e))
    return {"ok": True, "total": total, "cleaned": cleaned, "retried": retried, "day": day_str, "errors": errors}


def main():
    """直接运行（调试）：执行一次 daily 归档。"""
    result = run_archive("daily", datetime.datetime.now())
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
