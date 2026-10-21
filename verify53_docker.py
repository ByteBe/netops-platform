# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=30)
cmd = (
    "echo '== sock =='; ls -la /var/run/docker.sock; "
    "echo '== netops user =='; systemctl show netops -p User -p ExecStart; "
    "echo '== containers =='; curl -s --unix-socket /var/run/docker.sock 'http://localhost/containers/json?all=1'; "
    "echo; echo '== mysql logs =='; curl -s --unix-socket /var/run/docker.sock 'http://localhost/containers/196cd5c3c56f/logs?stdout=1&stderr=1&tail=5' | xxd | head -20"
)
_, out, err = c.exec_command(cmd, timeout=40)
rc = out.channel.recv_exit_status()
so = out.read().decode("utf-8", "ignore")
se = err.read().decode("utf-8", "ignore")
print("rc:", rc)
print(so)
if se.strip():
    print("ERR:", se[:500])
c.close()
