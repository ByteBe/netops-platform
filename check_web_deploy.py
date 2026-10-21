# -*- coding: utf-8 -*-
"""对比双云部署的 index.html 资源 hash 与本地 dist 是否一致。"""
import paramiko

def run(host, user, pwd, cmd):
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(host, username=user, password=pwd, timeout=20)
    _, out, err = c.exec_command(cmd, timeout=30)
    rc = out.channel.recv_exit_status()
    so = out.read().decode('utf-8', 'ignore')
    c.close()
    return so

for name, h, u, p in [("53总部", "82.156.100.53", "root", "TSsbs@123.."),
                      ("158分部", "36.213.106.158", "root", "Qaz3.141")]:
    print("=" * 50)
    print("[%s]" % name)
    print(run(h, u, p, "grep -oE 'assets/index-[A-Za-z0-9_-]+\\.(js|css)' /opt/netops-platform/web/index.html; echo ---; ls -la /opt/netops-platform/web/index.html; echo ---; grep -c 'np-glass' /opt/netops-platform/web/assets/index-*.css 2>/dev/null | head -2; echo ---; date"))
