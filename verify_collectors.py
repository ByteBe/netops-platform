# -*- coding: utf-8 -*-
import paramiko

for name, h, u, p in [("53总部", "82.156.100.53", "root", "TSsbs@123.."),
                      ("158分部", "36.213.106.158", "root", "Qaz3.141")]:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(h, username=u, password=p, timeout=25)
    q = ("docker exec mysql mysql -uroot -ptssbs123 netops -e \"SELECT source, COUNT(*) n FROM device_logs GROUP BY source ORDER BY n DESC;\" 2>/dev/null"
         if name.startswith("53")
         else "mysql -uroot -ptssbs123 netops -e \"SELECT source, COUNT(*) n FROM device_logs GROUP BY source ORDER BY n DESC;\" 2>/dev/null")
    _, out, err = c.exec_command(q, timeout=40)
    rc = out.channel.recv_exit_status()
    print("[" + name + "] rc=" + str(rc))
    print(out.read().decode("utf-8", "ignore").strip())
    c.close()
