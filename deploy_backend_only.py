# -*- coding: utf-8 -*-
"""仅部署后端二进制到双云并重启服务（不碰 web 前端目录）。
与 deploy_all.py 同路径策略：上传 zip -> unzip 仅提取 netops-server -> 重启。"""
import paramiko

SERVERS = [
    ("53总部", "82.156.100.53", "root", "TSsbs@123.."),
    ("158分部", "36.213.106.158", "root", "Qaz3.141"),
]
local_zip = r"E:\IDEA\dist\netops-server-only.zip"
remote_zip = "/opt/netops-server-only.zip"

for name, h, u, p in SERVERS:
    print("=" * 50)
    print("[" + name + "]")
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(h, username=u, password=p, timeout=20)
    sftp = c.open_sftp()
    sftp.put(local_zip, remote_zip)
    sftp.close()
    print("uploaded")
    cmds = [
        "systemctl stop netops",
        "sleep 1",
        "cd /opt && unzip -o netops-server-only.zip -d /opt/netops-platform",
        "chmod +x /opt/netops-platform/netops-server",
        "systemctl reset-failed netops",
        "systemctl start netops",
        "sleep 3",
        "systemctl is-active netops",
    ]
    for cmd in cmds:
        _, out, err = c.exec_command(cmd, timeout=60)
        o = out.read().decode("utf-8", "ignore").strip()
        e = err.read().decode("utf-8", "ignore").strip()
        if o:
            print("  " + o)
        if e:
            print("  ERR:", e[:300])
    c.close()
print("done")
