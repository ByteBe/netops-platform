# -*- coding: utf-8 -*-
import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("82.156.100.53", username="root", password="TSsbs@123..", timeout=30)
cmd = (
    "find /opt/netops-platform -maxdepth 2 -name '*.yaml' -o -name '*.yml' | head; "
    "echo '=== tsdb cfg ==='; "
    "grep -A8 -i 'tsdb\\|tdengine\\|influx' /opt/netops-platform/config.yaml 2>/dev/null | head -20; "
    "echo '=== env/other ==='; "
    "grep -rn 'tsdb\\|tdengine' /opt/netops-platform/*.yaml /opt/netops-platform/*.yml /opt/netops-platform/data/*.yaml 2>/dev/null | head -10"
)
_, out, err = c.exec_command(cmd, timeout=40)
rc = out.channel.recv_exit_status()
print("rc:", rc)
print(out.read().decode("utf-8", "ignore"))
if err.read():
    print("ERR")
c.close()
