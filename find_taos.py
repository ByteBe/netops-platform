import paramiko
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('82.156.100.53', username='root', password='TSsbs@123..', timeout=10)
stdin, stdout, stderr = ssh.exec_command('find / -maxdepth 5 -name "taos.cfg" 2>/dev/null; echo ===; find / -maxdepth 5 -name "vnode" -type d 2>/dev/null; echo ===; cat /etc/taos/taos.cfg 2>/dev/null | grep -v "^#" | grep -v "^$"')
print(stdout.read().decode())
ssh.close()
