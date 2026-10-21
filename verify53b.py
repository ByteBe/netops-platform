# -*- coding: utf-8 -*-
import sys
import traceback
import paramiko

try:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=25)
    print("connected", flush=True)
    q = "docker exec mysql mysql -uroot -ptssbs123 netops -e 'SELECT source, COUNT(*) n FROM device_logs GROUP BY source ORDER BY n DESC;' 2>/dev/null"
    stdin, out, err = c.exec_command(q, timeout=40)
    rc = out.channel.recv_exit_status()
    print("rc=", rc, flush=True)
    data = out.read().decode("utf-8", "ignore")
    print(data, flush=True)
    e = err.read().decode("utf-8", "ignore")
    if e.strip():
        print("ERR:", e[:300], flush=True)
    c.close()
except Exception as ex:
    traceback.print_exc()
    sys.exit(1)
