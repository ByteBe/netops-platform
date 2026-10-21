# -*- coding: utf-8 -*-
import paramiko

for name, h, u, p in [("53总部", "82.156.100.53", "root", "TSsbs@123.."),
                      ("158分部", "36.213.106.158", "root", "Qaz3.141")]:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(h, username=u, password=p, timeout=20)
    _, out, err = c.exec_command(
        "curl -s -o /dev/null -D - http://127.0.0.1:40000/ | grep -iE 'HTTP/|cache-control'; "
        "echo ---; systemctl is-active netops"
    )
    rc = out.channel.recv_exit_status()
    so = out.read().decode("utf-8", "ignore").strip()
    se = err.read().decode("utf-8", "ignore").strip()
    print("[" + name + "]")
    print(so)
    if se:
        print("ERR:", se[:300])
    c.close()
