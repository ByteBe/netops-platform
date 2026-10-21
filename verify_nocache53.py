# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=30)
_, out, err = c.exec_command(
    "curl -s -o /dev/null -D - http://127.0.0.1:40000/ | grep -iE 'HTTP/|cache-control'; "
    "echo ---; systemctl is-active netops"
)
rc = out.channel.recv_exit_status()
so = out.read().decode("utf-8", "ignore").strip()
se = err.read().decode("utf-8", "ignore").strip()
print("rc:", rc)
print(so)
if se:
    print("ERR:", se[:500])
c.close()
