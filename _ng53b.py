# -*- coding: utf-8 -*-
import sys
import paramiko

SH = (
    "grep -E 'include' /www/server/nginx/conf/nginx.conf | head -6; echo ---;\n"
    "ls /www/server/panel/vhost/nginx/ 2>/dev/null | head -5; echo ---;\n"
)

try:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=15)
    sftp = c.open_sftp()
    with sftp.open("/tmp/ng3.sh", "w") as f:
        f.write(SH)
    sftp.close()
    cmd = "bash /tmp/ng3.sh > /tmp/ng3.out 2>&1; cat /tmp/ng3.out; rm -f /tmp/ng3.sh /tmp/ng3.out"
    _, out, _ = c.exec_command(cmd, timeout=60)
    o = out.read().decode("utf-8", "ignore")
    sys.stdout.write("53>>>\n" + o[:800] + "\n")
    sys.stdout.flush()
    c.close()
except Exception as ex:
    sys.stdout.write("53 EXC>>>" + repr(ex) + "\n")
    sys.stdout.flush()
