# -*- coding: utf-8 -*-
"""仅更新 web 前端目录（index.html + assets），不重启 netops 服务，保留登录态"""
import paramiko, os

local_web = r"E:\IDEA\netops-platform\web\dist"
SERVERS = [
    ("53 腾讯云总部", "82.156.100.53", "root", "TSsbs@123.."),
    ("158 移动云分部", "36.213.106.158", "root", "Qaz3.141"),
]

def upload_tree(sftp, local, remote):
    for root, dirs, files in os.walk(local):
        rel = os.path.relpath(root, local)
        target = remote if rel == "." else remote + "/" + rel.replace("\\", "/")
        try:
            sftp.stat(target)
        except FileNotFoundError:
            sftp.mkdir(target)
        for f in files:
            lp = os.path.join(root, f)
            rp = target + "/" + f
            sftp.put(lp, rp)
            print("  up:", rp)

for name, host, user, pwd in SERVERS:
    print(f"===== {name} =====")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(host, username=user, password=pwd, timeout=25)
    sftp = ssh.open_sftp()
    upload_tree(sftp, local_web, "/opt/netops-platform/web")
    sftp.close()
    ssh.close()
print("done")
