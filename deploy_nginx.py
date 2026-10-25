# -*- coding: utf-8 -*-
"""一键部署 NetOps 前端 Nginx 代理配置到双云 53/158。

模板：deploy/nginx/netops.conf（Nginx 托管前端静态 + /api /ws /mcp 反代 Go API）
落位：
  - 53（宝塔 BT）：/www/server/panel/vhost/nginx/netops.conf
  - 158（Ubuntu 标准）：/etc/nginx/sites-available/netops -> 软链 sites-enabled/netops
流程：备份旧配置（首次） -> 写入模板 -> nginx -t -> 平滑 reload（未运行则 start）

用法：python deploy_nginx.py          # 双云全量
      python deploy_nginx.py 53      # 仅 53
      python deploy_nginx.py 158     # 仅 158
"""
import os
import sys
import paramiko

TEMPLATE = r"E:\IDEA\netops-platform\deploy\nginx\netops.conf"

SERVERS = {
    "53": ("53总部", "82.156.100.53", "root", "TSsbs@123..", "/www/server/panel/vhost/nginx/netops.conf", None),
    "158": ("158分部", "36.213.106.158", "root", "Qaz3.141", "/etc/nginx/sites-available/netops", "/etc/nginx/sites-enabled/netops"),
}


def deploy_one(name, host, user, pw, conf_path, link_path):
    with open(TEMPLATE, "r", encoding="utf-8") as f:
        conf = f.read()
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(host, username=user, password=pw, timeout=15)
    sftp = c.open_sftp()
    tmp = "/tmp/netops_nginx.conf"
    with sftp.open(tmp, "w") as f:
        f.write(conf)
    sftp.close()

    cmds = [
        "test -f %s || cp %s %s.bak" % (conf_path, conf_path, conf_path),  # 首次备份
        "cp %s %s" % (tmp, conf_path),
    ]
    if link_path:
        cmds.append("ln -sf %s %s" % (conf_path, link_path))
    cmds.append("nginx -t > /tmp/ng_t.txt 2>&1; echo NT_RC=$?")
    # reload 优先（对运行中实例平滑生效）；未运行则 start
    # 注意：宝塔 LSB 服务会把"已运行再 start"报错并标记 failed，故不能依赖 is-active 判断
    cmds.append("(nginx -s reload 2>/dev/null || systemctl start nginx); echo NG_RC=$?")
    cmds.append("sleep 1")
    cmds.append("ss -tlnp 2>/dev/null | grep ':40000' | head -2")
    cmds.append("curl -s -o /dev/null -w 'static:%{http_code} %{content_type}' http://127.0.0.1:40000/ ; echo")
    cmds.append("curl -s -o /dev/null -w 'api:%{http_code}' http://127.0.0.1:40000/api/v1/health; echo")
    cmds.append("cat /tmp/ng_t.txt; rm -f %s /tmp/ng_t.txt" % tmp)
    script = "\n".join(cmds)

    _, out, err = c.exec_command("bash -s", timeout=120)
    out.channel.send(script + "\n")
    out.channel.shutdown_write()
    o = out.read().decode("utf-8", "ignore")
    e = err.read().decode("utf-8", "ignore").strip()
    c.close()

    print("== %s (%s) ==" % (name, host))
    print(o[:1400] if o else "(no output)")
    if e:
        print("ERR>>>", e[:300])
    sys.stdout.flush()


def main():
    targets = sys.argv[1:] if len(sys.argv) > 1 else list(SERVERS.keys())
    for t in targets:
        if t not in SERVERS:
            print("未知目标: %s（可选 53/158）" % t)
            continue
        deploy_one(*SERVERS[t])


if __name__ == "__main__":
    main()
