# -*- coding: utf-8 -*-
"""巡检 prompt 构造：把 Go 快照 + MySQL 统计 + TDengine 趋势组织成结构化 prompt。"""
from typing import Any, Dict, List, Optional

from config import level_zh


def build_prompt(snapshot: Dict[str, Any], extra: Dict[str, Any], trends: Optional[Dict[str, Any]]) -> str:
    """构造增强版巡检 prompt。"""
    lines: List[str] = ["## 系统巡检数据\n"]
    devs = snapshot.get("devices") or []
    lines.append(f"- 设备总数：{snapshot.get('device_total', 0)}，在线：{snapshot.get('device_up', 0)}，离线：{snapshot.get('device_down', 0)}，未知：{snapshot.get('device_unknown', 0)}")
    lines.append(f"- 活跃告警：{snapshot.get('alert_active', 0)}，今日告警：{snapshot.get('alert_today', 0)}")
    lines.append(f"- 24h 严重日志：{snapshot.get('log_error_24h', 0)} 条，警告日志：{snapshot.get('log_warn_24h', 0)} 条")

    if extra.get("ok"):
        ab = extra.get("alert_by_level") or []
        if ab:
            lines.append("\n### 活跃告警级别分布：")
            for r in ab:
                lines.append(f"- {level_zh(r['level'])}：{r['count']} 条")
        at = extra.get("alert_top_targets") or []
        if at:
            lines.append("\n### 告警集中设备 Top：")
            for r in at:
                lines.append(f"- {r['target']}：{r['count']} 条")
        lv = extra.get("log_24h_by_level") or []
        if lv:
            lines.append("\n### 24h 日志分布：")
            for r in lv:
                lines.append(f"- {level_zh(r['level'])}：{r['count']} 条")
        tf = extra.get("task_fail_24h") or []
        if tf:
            lines.append("\n### 24h 采集失败任务：")
            for r in tf:
                lines.append(f"- {r['device']}（{r['type']}）：失败 {r['count']} 次")

    if trends and trends.get("ok"):
        items = trends.get("cpu_items") or []
        if items:
            lines.append("\n### 设备 CPU 趋势（近24h，TDengine）：")
            for it in items:
                avg = it["avg_24h"]
                if avg < 1:  # 数据可能为 0-1 归一化
                    avg = round(avg * 100, 1)
                lines.append(f"- {it['table']}：均值 {avg}%，最近1h 趋势{it['trend']}")

    lines.append("\n### 设备资源明细：")
    if not devs:
        lines.append("- 暂无设备数据")
    for d in devs:
        who = "本服务器" if d.get("is_local") else "远程设备"
        status = "在线" if d.get("up") else "离线"
        lines.append(f"- [{who}] {d.get('name', '-')}({d.get('ip', '-')})：{status}，CPU {d.get('cpu', 0)}%，内存 {d.get('mem', 0)}%，磁盘 {d.get('disk', 0)}%")
        if d.get("message"):
            lines.append("，" + d["message"])

    top = snapshot.get("top_devices") or []
    if top:
        lines.append("\n### 离线设备：")
        for t in top:
            lines.append(f"- {t}")

    lines.append("\n请根据以上数据生成一份专业的中文巡检报告，必须包含以下结构：")
    lines.append("1. **总体评价**：对系统整体运行状态给出结论（正常/存在风险/严重）。")
    lines.append("2. **关键指标概览**：用 Markdown 表格列出设备在线率、活跃告警、日志量、资源使用率等关键数据。")
    lines.append("3. **发现的问题**：逐条列出离线设备、高资源使用、告警集中、采集失败等问题，并给出严重程度。")
    lines.append("4. **风险点**：结合趋势数据指出资源上升趋势、持续告警等潜在风险。")
    lines.append("5. **建议措施**：针对每个问题给出可执行的处置建议。")
    return "\n".join(lines)
