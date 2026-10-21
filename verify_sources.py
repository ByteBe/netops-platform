# -*- coding: utf-8 -*-
import paramiko
import json
import time

for name, h, u, p in [("53总部", "82.156.100.53", "root", "TSsbs@123.."),
                      ("158分部", "36.213.106.158", "root", "Qaz3.141")]:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(h, username=u, password=p, timeout=25)
    ts = str(int(time.time()))
    nonce = ts + "000"
    _, out, _ = c.exec_command(
        "curl -s -H 'X-Timestamp: " + ts + "' -H 'X-Nonce: " + nonce + "' http://127.0.0.1:30821/api/v1/devicelog/sources",
        timeout=30,
    )
    rc = out.channel.recv_exit_status()
    raw = out.read().decode("utf-8", "ignore").strip()
    print("[" + name + "] rc=" + str(rc))
    try:
        data = json.loads(raw)
        items = data.get("data") or data
        vals = [i.get("value") for i in items]
        print("  来源列表:", vals)
    except Exception:
        print("  raw:", raw[:400])
    c.close()
