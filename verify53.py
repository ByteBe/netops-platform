# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=30)
cmds = [
    "docker ps --format '{{.Names}}' | head -20",
    "docker exec mysql mysql -uroot -ptssbs123 netops -e 'SELECT source, COUNT(*) n FROM device_logs GROUP BY source ORDER BY n DESC;'",
]
for cmd in cmds:
    print("$", cmd)
    _, out, err = c.exec_command(cmd, timeout=60)
    rc = out.channel.recv_exit_status()
    so = out.read().decode("utf-8", "ignore").strip()
    se = err.read().decode("utf-8", "ignore").strip()
    print("  rc:", rc)
    if so:
        print("  " + so.replace("\n", "\n  "))
    if se:
        print("  ERR: " + se[:400])
c.close()
