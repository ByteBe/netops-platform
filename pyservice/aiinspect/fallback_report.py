# -*- coding: utf-8 -*-
"""LLM 不可用/未配置时的统计兜底报告（仍包含数据概览，不依赖外部 AI）。"""
from typing import Any, Dict, Optional

from config import level_zh


def fallback_report(snapshot: Dict[str, Any], extra: Dict[str, Any], trends: Optional[Dict[str, Any]]) -> str:
    """生成统计模式 Markdown 报告。"""
    devs = snapshot.get("devices") or []
    up = snapshot.get("device_up", 0)
    total = snapshot.get("device_total", 0)
    online_rate = (up / total * 100) if total else 0.0
    lines = [
        "# 自动巡检报告（统计模式）\n",
        "> 注：AI 服务未配置或调用失败，本报告由系统统计自动生成。\n",
        "## 总体评价",
        f"设备总数 {total}，在线 {up}（在线率 {online_rate:.1f}%），离线 {snapshot.get('device_down', 0)}；活跃告警 {snapshot.get('alert_active', 0)} 条，今日新增 {snapshot.get('alert_today', 0)} 条。",
        "",
        "## 关键指标概览",
        "| 指标 | 数值 |",
        "| --- | --- |",
        f"| 设备在线率 | {online_rate:.1f}% |",
        f"| 活跃告警 | {snapshot.get('alert_active', 0)} |",
        f"| 今日告警 | {snapshot.get('alert_today', 0)} |",
        f"| 24h 严重日志 | {snapshot.get('log_error_24h', 0)} |",
        f"| 24h 警告日志 | {snapshot.get('log_warn_24h', 0)} |",
        "",
        "## 设备明细",
        "| 设备 | IP | 状态 | CPU | 内存 | 磁盘 |",
        "| --- | --- | --- | --- | --- | --- |",
    ]
    for d in devs:
        st = "在线" if d.get("up") else "离线"
        lines.append(f"| {d.get('name', '-')} | {d.get('ip', '-')} | {st} | {d.get('cpu', 0)}% | {d.get('mem', 0)}% | {d.get('disk', 0)}% |")

    if extra.get("ok"):
        ab = extra.get("alert_by_level") or []
        if ab:
            lines += ["", "## 活跃告警分布"]
            for r in ab:
                lines.append(f"- {level_zh(r['level'])}：{r['count']} 条")
        tf = extra.get("task_fail_24h") or []
        if tf:
            lines += ["", "## 24h 采集失败任务"]
            for r in tf:
                lines.append(f"- {r['device']}（{r['type']}）：失败 {r['count']} 次")

    top = snapshot.get("top_devices") or []
    if top:
        lines += ["", "## 离线设备清单"]
        for t in top:
            lines.append(f"- {t}")

    lines += ["", "## 建议措施", "1. 优先排查离线设备与持续告警。", "2. 关注资源使用率较高的设备，及时扩容或优化。", "3. 检查采集失败任务对应的连接与凭证配置。"]
    return "\n".join(lines)
