# -*- coding: utf-8 -*-
"""报告 HTML 渲染：原生模板替换（零第三方依赖），样式与 Go 版 report.go 保持一致。"""
import re

# ---- 图标（内联 SVG，viewBox 24，stroke/fill 白色，section 标题用深蓝） ----

def _icon(name, color="#ffffff", size=16, margin=True):
    """返回内联 SVG 图标；color 控制图形颜色，margin 控制是否带右侧间距。"""
    paths = {
        "gauge": '<path d="M12 2a10 10 0 1 0 10 10" fill="none" stroke-width="2.2" stroke-linecap="round"/>'
                 '<path d="M12 12l5-5" fill="none" stroke-width="2.2" stroke-linecap="round"/>'
                 '<circle cx="12" cy="12" r="1.6"/>',
        "ok": '<path d="M4.5 12.5l5 5L19.5 6.5" fill="none" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>',
        "down": '<path d="M12 4v9" fill="none" stroke-width="2.4" stroke-linecap="round"/>'
                '<circle cx="12" cy="18.5" r="1.7"/>',
        "warn": '<path d="M12 2.5l10 18H2z" fill="none" stroke-width="2" stroke-linejoin="round"/>'
                '<path d="M12 10v5" stroke-width="2" stroke-linecap="round"/>'
                '<circle cx="12" cy="18" r="1.3"/>',
        "link": '<path d="M3 12h4l3-7 4 14 3-7h4" fill="none" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>',
        "server": '<rect x="3.5" y="3.5" width="17" height="17" rx="2.5" fill="none" stroke-width="2"/>'
                  '<path d="M8 8.5h8M8 12h8M8 15.5h8" stroke-width="1.8" stroke-linecap="round"/>',
        "db": '<ellipse cx="12" cy="5" rx="8" ry="3" fill="none" stroke-width="2"/>'
              '<path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5" fill="none" stroke-width="2"/>',
        "box": '<path d="M12 2.5l9 4.5v10l-9 4.5-9-4.5V7z" fill="none" stroke-width="2" stroke-linejoin="round"/>'
               '<path d="M3 7l9 4.5L21 7M12 11.5V21" fill="none" stroke-width="2"/>',
        "node": '<circle cx="12" cy="4.5" r="2.2"/><circle cx="4.5" cy="19" r="2.2"/><circle cx="19.5" cy="19" r="2.2"/>'
                '<path d="M12 6.7v3.8M6.4 17.6l4.6-6.6M17.6 17.6l-4.6-6.6" fill="none" stroke-width="1.8"/>',
        "nginx": '<path d="M4 12a8 8 0 0 1 14-5M20 12a8 8 0 0 1-14 5" fill="none" stroke-width="2.2" stroke-linecap="round"/>'
                 '<path d="M18 3v4h-4M6 21v-4h4" fill="none" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>',
    }
    body = paths.get(name, paths["gauge"])
    mr = "margin-right:6px;" if margin else ""
    return ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="%d" height="%d" '
            'style="vertical-align:-3px;%s" fill="%s" stroke="%s">%s</svg>'
            % (size, size, mr, color, color, body))


def _section_head(text, icon):
    return ('<div class="section"><h3>%s<span style="margin-right:6px"></span>%s</h3>'
            % (_icon(icon, "#0d3b66", 17), text))


# ---- 各 section 的表格行 HTML 生成（与 Go renderXxxSection 输出一致） ----

def _badge(status, color):
    return ("<span style='display:inline-block;padding:2px 10px;border-radius:10px;"
            "font-size:12px;font-weight:500;color:#fff;background:%s'>%s</span>" % (color, status))


def render_link(rows):
    parts = [_section_head("【链路检测汇总】", "link"),
             '<table><thead><tr>'
             '<th>链路名称</th><th>目标地址</th><th>当前状态</th><th>平均时延</th>'
             '<th>累计中断</th><th>可用率</th></tr></thead><tbody>']
    for r in rows:
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.1f ms</td>"
                     "<td>%d 次</td><td>%.2f%%</td></tr>" % (
                         r.get("name", ""), r.get("target", ""),
                         _badge(r.get("status", "未知"), r.get("color", "#909399")),
                         _num(r, "avg_rt", 0.0), int(_num(r, "downs", 0)), _num(r, "avail", 100.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


# ---- 数值兜底：Go 传入 JSON 可能为 null，统一转 float/int（防御 None） ----

def _num(r, key, default):
    v = r.get(key)
    try:
        return float(v) if v is not None else float(default)
    except (TypeError, ValueError):
        return float(default)


def render_dev(rows):
    parts = [_section_head("【服务器资源使用汇总】", "server"),
             '<table><thead><tr>'
             '<th>设备</th><th>IP</th><th>状态</th><th>CPU(均值)</th><th>内存使用(均值)</th></tr></thead><tbody>']
    if not rows:
        parts.append("<tr><td colspan='5' style='text-align:center;color:#999'>暂无服务器监控数据</td></tr>")
    for r in rows:
        cpu = _num(r, "cpu", 0.0)
        cpu_color = "#52c41a"
        if cpu > 80:
            cpu_color = "#f5222d"
        elif cpu > 60:
            cpu_color = "#faad14"
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td>"
                     "<td style='color:%s'>%.1f%%</td><td>%.1f%%</td></tr>" % (
                         r.get("name", ""), r.get("ip", ""),
                         _badge(r.get("status", "离线"), r.get("color", "#f5222d")),
                         cpu_color, cpu, _num(r, "mem", 0.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_db(rows):
    parts = [_section_head("【数据库状态汇总】", "db"),
             '<table><thead><tr>'
             '<th>实例</th><th>类型</th><th>状态</th><th>数据库数</th><th>错误数</th></tr></thead><tbody>']
    for r in rows:
        parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td></tr>" % (
            r.get("name", ""), r.get("type", ""),
            _badge(r.get("status", "异常"), r.get("color", "#f5222d")),
            _num(r, "conns", 0.0), _num(r, "errors", 0.0)))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_nginx(rows):
    parts = [_section_head("【Nginx 状态汇总】", "nginx"),
             '<table><thead><tr>'
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
                             _num(r, "active_conns", 0.0), _num(r, "requests", 0.0), _num(r, "qps", 0.0)))
        else:
            parts.append("<tr><td><b>%s</b></td><td>%s</td><td>%s</td>"
                         "<td colspan='3' style='text-align:center;color:#999'>未采集</td></tr>" % (
                             r.get("name", ""), r.get("status_url", ""), _badge("异常", "#f5222d")))
    parts.append("</tbody></table></div>")
    return "".join(parts)


def render_container(rows):
    parts = [_section_head("【容器与 Kubernetes 汇总】", "box"),
             '<table><thead><tr>'
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
    parts = [_section_head("节点健康性", "node"),
             '<table><thead><tr>'
             '<th>节点名称</th><th>UUID</th><th>层级</th><th>状态</th><th>设备数</th>'
             '<th>容器数</th><th>最后上报</th></tr></thead><tbody>']
    if not rows:
        parts.append("<tr><td colspan='7' style='text-align:center;color:#999'>暂无节点监控数据</td></tr>")
    for r in rows:
        color = "#52c41a" if r.get("status") == "online" else "#f5222d"
        state = "在线" if r.get("status") == "online" else "离线"
        parts.append("<tr><td><b>%s</b></td><td><code>%s</code></td><td>%s</td>"
                     "<td><span style='color:%s'>%s</span></td><td>%d</td><td>%d</td><td>%s</td></tr>" % (
                         r.get("name", ""), r.get("uuid", ""), r.get("level", ""),
                         color, state, int(_num(r, "device_cnt", 0)), int(_num(r, "container_cnt", 0)),
                         r.get("last_seen", "")))
    parts.append("</tbody></table></div>")
    return "".join(parts)


# ---- 整体 HTML 模板（样式与 Go report.go 完全一致） ----

_TPL_SRC = """<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>__TITLE__</title>
<style>
*{box-sizing:border-box}
body{font-family:"Noto Sans CJK SC","WenQuanYi Micro Hei","Microsoft YaHei",sans-serif;margin:0;padding:0;background:#eef1f6;color:#2c3e50;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.wrap{max-width:960px;margin:0 auto;background:#fff;min-height:100vh;box-shadow:0 2px 16px rgba(15,76,129,.10)}
.header{position:relative;padding:34px 44px 26px;background:#0d3b66;background:linear-gradient(135deg,#0d3b66 0%,#1a73e8 55%,#3f9bff 100%);color:#fff;overflow:hidden}
.header:before{content:"";position:absolute;right:-70px;top:-70px;width:230px;height:230px;border-radius:50%;background:rgba(255,255,255,.09)}
.header:after{content:"";position:absolute;right:70px;bottom:-90px;width:170px;height:170px;border-radius:50%;background:rgba(255,255,255,.06)}
.header h1{margin:0;font-size:25px;font-weight:700;letter-spacing:2px;position:relative}
.header .meta{margin-top:10px;font-size:12.5px;color:#d3e3ff;position:relative}
.header .meta span{margin-right:22px}
.header .tag{display:inline-block;margin-top:14px;padding:4px 14px;border-radius:20px;background:rgba(255,255,255,.16);font-size:12px;color:#fff;border:1px solid rgba(255,255,255,.35);position:relative}
.summary{width:calc(100% - 80px);margin:22px 40px 6px;border-collapse:separate;border-spacing:12px 0}
.summary td{width:25%;text-align:center;padding:14px 8px 12px;background:#f8fbff;border:1px solid #e3edfc;border-radius:10px;position:relative}
.summary .ic{width:32px;height:32px;border-radius:50%;margin:0 auto 4px;text-align:center;line-height:34px;box-shadow:0 2px 6px rgba(15,76,129,.15)}
.summary .num{font-size:30px;font-weight:800;line-height:1.15}
.summary .lbl{font-size:12px;color:#7a8798;margin-top:5px}
.section{padding:18px 40px 4px}
h3{margin:0 0 14px;font-size:15px;color:#0d3b66;padding:8px 14px;border-left:4px solid #1a73e8;background:#eef4ff;background:linear-gradient(90deg,#eef4ff 0%,#f8fbff 100%);border-radius:0 6px 6px 0;letter-spacing:1px}
table{width:100%;border-collapse:collapse;font-size:12.5px;box-shadow:0 1px 5px rgba(15,76,129,.07);border-radius:8px;overflow:hidden}
thead th{background:#0d3b66;background:linear-gradient(180deg,#1a73e8 0%,#0d3b66 100%);padding:10px 12px;text-align:left;font-weight:600;color:#fff;border:none}
tbody td{padding:9px 12px;border-bottom:1px solid #eef1f5}
tbody tr:nth-child(even){background:#f7f9fc}
tbody tr:hover{background:#eef4ff}
.footer{padding:16px 40px;text-align:center;color:#9aa7b8;font-size:11px;letter-spacing:1px;border-top:1px solid #eef1f5;background:#fafbfd}
@media print{body{background:#fff}.wrap{box-shadow:none}}
</style></head><body><div class="wrap">
<div class="header">
<h1>__TITLE__</h1>
<div class="meta"><span>巡检周期：__START__ 至 __END__</span><span>生成时间：__NOW__</span></div>
<div class="tag">NetOps 网络运维监控平台 · 自动生成</div>
</div>
<table class="summary"><tr>
<td><div class="ic" style="background:#1a73e8">__ICON_GAUGE__</div><div class="num" style="color:#1a73e8">__TOTAL__</div><div class="lbl">监控对象总数</div></td>
<td><div class="ic" style="background:#34a853">__ICON_OK__</div><div class="num" style="color:#34a853">__UP__</div><div class="lbl">正常</div></td>
<td><div class="ic" style="background:#ea4335">__ICON_DOWN__</div><div class="num" style="color:#ea4335">__DOWN__</div><div class="lbl">异常/离线</div></td>
<td><div class="ic" style="background:#f9ab00">__ICON_WARN__</div><div class="num" style="color:#f9ab00">__WARN__</div><div class="lbl">告警</div></td>
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
            .replace("__ICON_GAUGE__", _icon("gauge", "#ffffff", 18, margin=False))
            .replace("__ICON_OK__", _icon("ok", "#ffffff", 18, margin=False))
            .replace("__ICON_DOWN__", _icon("down", "#ffffff", 18, margin=False))
            .replace("__ICON_WARN__", _icon("warn", "#ffffff", 18, margin=False))
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
