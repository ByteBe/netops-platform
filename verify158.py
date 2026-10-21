# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("36.213.106.158", username="root", password="Qaz3.141", timeout=25)
_, out, _ = c.exec_command("mysql -uroot -ptssbs123 netops -e 'SELECT source, COUNT(*) n FROM device_logs GROUP BY source ORDER BY n DESC;' 2>/dev/null; ls /var/run/docker.sock 2>/dev/null || echo NO_DOCKER_SOCKET", timeout=40)
rc = out.channel.recv_exit_status()
print("rc=", rc)
print(out.read().decode("utf-8", "ignore"))
c.close()
