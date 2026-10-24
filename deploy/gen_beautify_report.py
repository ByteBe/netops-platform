# -*- coding: utf-8 -*-
"""用原报告数据 + 美化模板重新生成 HTML，并调用 wkhtmltopdf 转 PDF。"""
import os
import sys
import subprocess

sys.path.insert(0, r"E:\IDEA\netops-platform\pyservice")
from report.render import render_html

payload = {
    "title": "运维巡检报告",
    "start": "2026-10-24 00:22",
    "end": "2026-10-25 00:22",
    "now": "2026-10-25 00:22:06",
    "stats": {"total": 9, "up": 9, "down": 0, "warn": 0},
    "link": [
        {"name": "大唐", "target": "220.171.133.97", "status": "正常", "color": "#34a853",
         "avg_rt": 47.2, "downs": 0, "avail": 100.0},
        {"name": "DNS1", "target": "61.128.114.133", "status": "正常", "color": "#34a853",
         "avg_rt": 48.2, "downs": 0, "avail": 100.0},
    ],
    "dev": [],
    "db": [
        {"name": "时序数据库", "type": "tdengine", "status": "正常", "color": "#34a853",
         "conns": 5, "errors": 0},
        {"name": "存储数据库", "type": "mysql", "status": "正常", "color": "#34a853",
         "conns": 5, "errors": 0},
    ],
    "container": [
        {"name": "本机", "addr": "本机", "kind": "docker", "status": "在线",
         "color": "#34a853", "count": 2, "abnormal": 0},
    ],
    "node": [],
    "nginx": [
        {"name": "本机", "status_url": "http://127.0.0.1/nginx_status", "up": True,
         "active_conns": 1, "requests": 13162, "qps": 0.0},
    ],
}

html = render_html(payload)
out_dir = r"E:\IDEA\netops-platform\dist-release"
os.makedirs(out_dir, exist_ok=True)
html_path = os.path.join(out_dir, "运维巡检报告_v2.4.0.html")
pdf_path = os.path.join(out_dir, "运维巡检报告_v2.4.0.pdf")
with open(html_path, "w", encoding="utf-8") as f:
    f.write(html)
print("html:", os.path.getsize(html_path), "bytes")

wk = r"E:\IDEA\netops-platform\wkhtmltopdf\bin\wkhtmltopdf.exe"
cmd = [wk, "--enable-local-file-access", "--encoding", "utf-8",
       "--margin-top", "0", "--margin-bottom", "0", "--margin-left", "0", "--margin-right", "0",
       "--page-size", "A4", html_path, pdf_path]
r = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
print("wk exit:", r.returncode)
if r.returncode != 0:
    print(r.stderr[-1500:])
print("pdf:", os.path.getsize(pdf_path) if os.path.exists(pdf_path) else "MISSING", "bytes")
