# -*- coding: utf-8 -*-
import paramiko

for name, host, user, pwd in [("53", "82.156.100.53", "root", "TSsbs@123.."), ("158", "36.213.106.158", "root", "Qaz3.141")]:
    try:
        c = paramiko.SSHClient()
        c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        c.connect(host, username=user, password=pwd, timeout=25)
        _, out, _ = c.exec_command("systemctl is-active netops; ls -la --time-style=+%H:%M /opt/netops-platform/netops-server; ls /opt/netops-platform/web/assets/ | grep -E 'index-W4pEyvMT|index-t56yxCIg' | head -3", timeout=30)
        print(f"===== {name} =====")
        print(out.read().decode("utf-8", "ignore"))
        c.close()
    except Exception as e:
        print(f"FAILED {name}: {e}")
