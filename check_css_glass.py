# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=20)
_, out, err = c.exec_command(
    "grep -o np-glass /opt/netops-platform/web/assets/index-CN4K7GH5.css | wc -l; "
    "grep -o np-particles /opt/netops-platform/web/assets/index-CN4K7GH5.css | wc -l; "
    "grep -o backdrop-filter /opt/netops-platform/web/assets/index-CN4K7GH5.css | wc -l"
)
rc = out.channel.recv_exit_status()
so = out.read().decode("utf-8", "ignore")
se = err.read().decode("utf-8", "ignore")
print("rc:", rc)
print(so)
if se.strip():
    print("ERR:", se[:300])
c.close()
