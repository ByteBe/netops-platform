# -*- coding: utf-8 -*-
"""report 渲染冒烟测试：构造样例数据 → 渲染 HTML 非空且关键字段齐全。"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from report.render import render_html, channel_text, strip_html

FAILED = []


def check(name, cond, detail=""):
    if not cond:
        FAILED.append(f"{name} {detail}")
        print(f"FAIL {name} {detail}")
    else:
        print(f"ok   {name}")


payload = {
    "title": "自动巡检周报 2026-10-21",
    "start": "2026-10-14 20:00",
    "end": "2026-10-21 20:00",
    "now": "2026-10-21 20:00:05",
    "stats": {"total": 12, "up": 10, "down": 2, "warn": 0},
    "link": [
        {"name": "总部-分部", "target": "10.16.194.1", "status": "正常", "color": "#52c41a", "avg_rt": 12.3, "downs": 0, "avail": 100.0},
        {"name": "备线", "target": "10.16.194.2", "status": "中断", "color": "#f5222d", "avg_rt": 0.0, "downs": 5, "avail": 88.2},
    ],
    "dev": [
        {"name": "web-01", "ip": "10.16.194.11", "status": "在线", "color": "#52c41a", "cpu": 32.5, "mem": 55.0},
        {"name": "db-01", "ip": "10.16.194.12", "status": "离线", "color": "#f5222d", "cpu": 0.0, "mem": 0.0},
    ],
    "db": [
        {"name": "MySQL", "type": "mysql", "status": "正常", "color": "#52c41a", "conns": 120, "errors": 0},
        {"name": "TDengine", "type": "tdengine", "status": "异常", "color": "#f5222d", "conns": 0, "errors": 3},
    ],
    "nginx": [
        {"name": "nginx-01", "status_url": "http://10.16.194.21/status", "up": True, "active_conns": 42, "requests": 100000, "qps": 18.5},
        {"name": "nginx-02", "status_url": "http://10.16.194.22/status", "up": False, "active_conns": 0, "requests": 0, "qps": 0.0},
    ],
    "container": [
        {"name": "k8s-01", "addr": "10.16.194.31", "kind": "k8s", "status": "在线", "color": "#52c41a", "count": None, "abnormal": None},
    ],
    "node": [
        {"name": "总部", "uuid": "node-0001", "level": "总部", "status": "online", "device_cnt": 8, "container_cnt": 2, "last_seen": "2026-10-21 20:00:00"},
    ],
}

html = render_html(payload)
check("渲染非空", html and len(html) > 500)
for key in ("链路检测汇总", "服务器资源使用汇总", "数据库状态汇总", "Nginx 状态汇总",
            "容器与 Kubernetes 汇总", "节点健康性", "总部-分部", "web-01", "MySQL",
            "自动巡检周报 2026-10-21", "12.3 ms", "88.20%"):
    check(f"包含 {key}", key in html)

# 空 section 渲染不报错
empty = {"title": "空报告", "start": "", "end": "", "now": "", "stats": {}, "link": [], "dev": [],
         "db": [], "nginx": [], "container": [], "node": []}
html_empty = render_html(empty)
check("空数据渲染", html_empty and "暂无 Nginx 监控实例" in html_empty and "暂无 Docker/K8s 节点监控" in html_empty)

# 纯文本摘要：剥离标签 + 1900 字节截断
text = channel_text("自动巡检周报", html)
lines = strip_html(html)
check("摘要含标题", text.startswith("自动巡检周报"))
check("摘要含数据", "总部-分部" in text or "web-01" in text)
check("HTML 已剥离", "<tr>" not in text and "<div" not in text)
check("长文截断边界", len(text.encode("utf-8")) <= 1900 + 32)

print()
print("TOTAL FAIL:", len(FAILED))
sys.exit(1 if FAILED else 0)
