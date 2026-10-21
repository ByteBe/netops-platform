# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=30)
cmd = (
    "echo '=== schema ==='; "
    "docker exec mysql mysql -uroot -ptssbs123 netops -e 'DESC db_instances;' 2>&1 | head -25; "
    "echo '=== rows ==='; "
    "docker exec mysql mysql -uroot -ptssbs123 netops -e 'SELECT id, name, type, host, port, db_name, enabled FROM db_instances;' 2>&1 | head -30; "
    "echo '=== LAST 24269c5d ==='; "
    "docker exec tdengine taos -s 'SELECT LAST(ts) FROM data.t_db_conn_24269c5d;' 2>&1; "
    "echo '=== LAST 7c2a9d2b ==='; "
    "docker exec tdengine taos -s 'SELECT LAST(ts) FROM data.t_db_conn_7c2a9d2b;' 2>&1; "
    "echo '=== LAST 7c2a9d4a ==='; "
    "docker exec tdengine taos -s 'SELECT LAST(ts) FROM data.t_db_conn_7c2a9d4a;' 2>&1; "
    "echo '=== tsdb log ==='; "
    "grep -i 'tsdb\\|tdengine' /opt/netops-platform/logs/app-20261021.log | tail -8"
)
_, out, err = c.exec_command(cmd, timeout=90)
rc = out.channel.recv_exit_status()
print("rc:", rc)
print(out.read().decode("utf-8", "ignore"))
c.close()
