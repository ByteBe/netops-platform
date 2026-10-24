# -*- coding: utf-8 -*-
"""报告 HTML 渲染：原生模板替换（零第三方依赖），样式与 Go 版 report.go 保持一致。"""
import re

# ---- 各 section 的表格行 HTML 生成（与 Go renderXxxSection 输出一致） ----

def _badge(status, color):
    return ("<span style='display:inline-block;padding:2px 10px;border-radius:10px;"
            "font-size:12px;font-weight:500;color:#fff;background:%s'>%s</span>" % (color, status))


def _fmt(v, dot):
    if v is None:
        return "-"
    return ("%%.%df" % dot) % v


def render_link(rows):
    parts = ['<div class="section"><h3>【链路检测汇总】</h3><table><thead><tr>'
             '<th>链路名称</th><th>目标地址</th><th>当前状态</th><th>平均时延</th>'
             '<th>累计中断</th><th>可用率</th></tr></thead><tbody>']
    for r in rows:
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.1f ms</td>"
                     "<td>%d 次</td><td>%.2f%%</td></tr>" % (
                         r.get("name", ""), r.get("target", ""),
                         _badge(r.get("status", "未知"), r.get("color", "#909399")),
                         r.get("avg_rt", 0.0), r.get("downs", 0), r.get("avail", 100.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_dev(rows):
    parts = ['<div class="section"><h3>【服务器资源使用汇总】</h3><table><thead><tr>'
             '<th>设备</th><th>IP</th><th>状态</th><th>CPU(均值)</th><th>内存使用(均值)</th></tr></thead><tbody>']
    for r in rows:
        cpu = r.get("cpu", 0.0)
        cpu_color = "#52c41a"
        if cpu > 80:
            cpu_color = "#f5222d"
        elif cpu > 60:
            cpu_color = "#faad14"
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td>"
                     "<td style='color:%s'>%.1f%%</td><td>%.1f%%</td></tr>" % (
                         r.get("name", ""), r.get("ip", ""),
                         _badge(r.get("status", "离线"), r.get("color", "#f5222d")),
                         cpu_color, cpu, r.get("mem", 0.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_db(rows):
    parts = ['<div class="section"><h3>【数据库状态汇总】</h3><table><thead><tr>'
             '<th>实例</th><th>类型</th><th>状态</th><th>数据库数</th><th>错误数</th></tr></thead><tbody>']
    for r in rows:
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td></tr>" % (
            r.get("name", ""), r.get("type", ""),
            _badge(r.get("status", "异常"), r.get("color", "#f5222d")),
            r.get("conns", 0.0), r.get("errors", 0.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_nginx(rows):
    parts = ['<div class="section"><h3>【Nginx 状态汇总】</h3><table><thead><tr>'
             '<th>实例</th><th>状态页地址</th><th>状态</th><th>活跃连接</th>'
             '<th>累计请求</th><th>QPS(次/秒)</th></tr></thead><tbody>']
    if not rows:
        parts.append("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Nginx 监控实例</td></tr>")
    for r in rows:
        if r.get("up"):
            parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td>"
                         "<td>%.0f</td><td>%.1f</td></tr>" % (
                             r.get("name", ""), r.get("status_url", ""),
                             _badge("正常", "#52c41a"),
                             r.get("active_conns", 0.0), r.get("requests", 0.0), r.get("qps", 0.0)))
        else:
            parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td>"
                         "<td colspan='3' style='text-align:center;color:#999'>未采集</td></tr>" % (
                             r.get("name", ""), r.get("status_url", ""), _badge("异常", "#f5222d")))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_container(rows):
    parts = ['<div class="section"><h3>【容器与 Kubernetes 汇总】</h3><table><thead><tr>'
             '<th>节点</th><th>IP/地址</th><th>类型</th><th>状态</th><th>容器/Pod 数</th>'
             '<th>异常数</th></tr></thead><tbody>']
    if not rows:
        parts.append("<tr><td colspan='6' style='text-align:center;color:#999'>暂无 Docker/K8s 节点监控</td></tr>")
    for r in rows:
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>" % (
            r.get("name", ""), r.get("addr", ""), r.get("kind", "docker"),
            _badge(r.get("status", "离线"), r.get("color", "#f5222d")),
            "-" if r.get("count") is None else str(r.get("count")),
            "-" if r.get("abnormal") is None else str(r.get("abnormal"))))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_node(rows):
    parts = ['<div class="section"><h3>节点健康性</h3><table><thead><tr>'
             '<th>节点名称</th><th>UUID</th><th>层级</th><th>状态</th><th>设备数</th>'
             '<th>容器数</th><th>最后上报</th></tr></thead><tbody>']
    for r in rows:
        color = "#52c41a" if r.get("status") == "online" else "#f5222d"
        badge = "在线" if r.get("status") == "online" else "离线"
        parts.append("<tr><td><b>%s</b></td><td><code>%s</code></td><td>%s</td>"
                     "<td><span style='color:%s'>%s</span></td><td>%d</td><td>%d</td><td>%s</td></tr>" % (
                         r.get("name", ""), r.get("uuid", ""), r.get("level", ""),
                         color, badge, r.get("device_cnt", 0), r.get("container_cnt", 0),
                         r.get("last_seen", "")))
    parts.append("</tbody></table></div>")
    return "".join(parts)


# ---- 整体 HTML 模板（样式与 Go report.go 完全一致） ----

_TPL_SRC = """<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>__TITLE__</title>
<style>
*{box-sizing:border-box}
body{font-family:"Noto Sans CJK SC","WenQuanYi Micro Hei","WenQuanYi Zen Hei","Microsoft YaHei",sans-serif;margin:0;padding:32px;background:linear-gradient(180deg,#e9eef6 0%,#f5f7fa 100%);color:#2c3e50;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.wrap{max-width:920px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 4px 20px rgba(26,115,232,.10);overflow:hidden}
.header{padding:30px 40px 20px;background:linear-gradient(135deg,#1a73e8 0%,#0d47a1 100%);border-radius:12px 12px 0 0}
.header h1{margin:0;font-size:24px;font-weight:700;color:#fff;letter-spacing:1px}
.header .meta{margin-top:8px;font-size:13px;color:#d6e4ff}
.summary{width:calc(100% - 72px);margin:26px 36px 10px;border-collapse:separate;border-spacing:10px 0}
.summary td{width:25%;text-align:center;padding:16px 8px;background:#f6f9ff;border:1px solid #e4ecfb;border-radius:10px}
.summary .num{font-size:28px;font-weight:700;line-height:1.2}
.summary .lbl{font-size:12px;color:#7a8798;margin-top:6px}
.section{padding:22px 40px}
h3{margin:0 0 14px;font-size:16px;color:#1a73e8;padding:7px 12px;border-left:4px solid #1a73e8;background:#eef4ff;border-radius:4px}
table{width:100%;border-collapse:collapse;font-size:13px}
thead th{background:#1a73e8;padding:10px 12px;text-align:left;font-weight:600;color:#fff;border:none}
tbody td{padding:10px 12px;border-bottom:1px solid #eef1f5}
tbody tr:nth-child(even){background:#f7f9fc}
tbody tr:hover{background:#eef4ff}
.footer{padding:18px 40px;text-align:center;color:#8a97a8;font-size:12px;border-top:1px solid #eef1f5;background:#f7f9fc}
</style></head><body><div class="wrap">
<div class="header">
<h1>__TITLE__</h1>
<div class="meta">巡检周期：__START__ 至 __END__ ｜ 生成时间：__NOW__</div>
</div>
<table class="summary"><tr>
<td><div class="num" style="color:#1a73e8">__TOTAL__</div><div class="lbl">监控对象总数</div></td>
<td><div class="num" style="color:#52c41a">__UP__</div><div class="lbl">正常</div></td>
<td><div class="num" style="color:#f5222d">__DOWN__</div><div class="lbl">异常/离线</div></td>
<td><div class="num" style="color:#faad14">__WARN__</div><div class="lbl">告警</div></td>
</tr></table>
__LINK____DEV____DB____CONTAINER____NODE____NGINX__
<div class="footer">本报告由 NetOps 网络运维监控平台自动生成</div>
</div></body></html>"""


def render_html(payload):
    """payload: {title,start,end,now,stats,link,dev,db,nginx,container,node,sections}
    → HTML 字符串。sections 指定要渲染的区块（缺省渲染全部）。"""
    include = payload.get("sections") or ["link", "dev", "db", "nginx", "container", "node"]
    sections = []
    for key, fn in (("link", render_link), ("dev", render_dev), ("db", render_db),
                    ("nginx", render_nginx), ("container", render_container), ("node", render_node)):
        if key in include:
            sections.append(fn(payload.get(key) or []))
        else:
            sections.append("")
    stats = payload.get("stats") or {}
    return (_TPL_SRC
            .replace("__TITLE__", str(payload.get("title", "运维巡检报告")))
            .replace("__START__", str(payload.get("start", "")))
            .replace("__END__", str(payload.get("end", "")))
            .replace("__NOW__", str(payload.get("now", "")))
            .replace("__TOTAL__", str(stats.get("total", 0)))
            .replace("__UP__", str(stats.get("up", 0)))
            .replace("__DOWN__", str(stats.get("down", 0)))
            .replace("__WARN__", str(stats.get("warn", 0)))
            .replace("__LINK__", sections[0]).replace("__DEV__", sections[1])
            .replace("__DB__", sections[2]).replace("__NGINX__", sections[3])
            .replace("__CONTAINER__", sections[4]).replace("__NODE__", sections[5]))


# ---- 纯文本摘要（与 Go channelText 一致：剥离 HTML → 1900 字节截断） ----

def strip_html(html):
    # 先整体剥离 <style>/<script> 块（CSS/JS 不是正文，Go 版同样处理）
    html = re.sub(r"<style[^>]*>.*?</style>", "", html, flags=re.S)
    html = re.sub(r"<script[^>]*>.*?</script>", "", html, flags=re.S)
    html = html.replace("<br", "\n<br").replace("</tr>", "\n").replace("</div>", "\n")
    html = html.replace("</p>", "\n").replace("</h1>", "\n").replace("</h2>", "\n").replace("</h3>", "\n")
    out = []
    in_tag = False
    for ch in html:
        if ch == "<":
            in_tag = True
        elif ch == ">":
            in_tag = False
        elif not in_tag:
            out.append(ch)
    lines = []
    for l in "".join(out).split("\n"):
        l = l.strip().replace("&nbsp;", " ").replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">")
        if l:
            lines.append(l)
    return lines


def channel_text(title, content):
    """标题 + 纯文本摘要，1900 字节截断并回退完整字符边界（企业微信 2048 字节限制）。"""
    s = title + "\n" + "\n".join(strip_html(content))
    b = s.encode("utf-8")
    if len(b) <= 1900:
        return s
    cut = 0
    i = 0
    while i < len(b) and i < 1900:
        size = _char_size(b, i)
        if i + size > 1900:
            break
        i += size
        cut = i
    return b[:cut].decode("utf-8", "ignore") + "\n…（详见完整报告）"


def _char_size(b, i):
    lead = b[i]
    if lead < 0x80:
        return 1
    if lead < 0xE0:
        return 2
    if lead < 0xF0:
        return 3
    return 4
