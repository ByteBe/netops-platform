import paramiko, sys
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('82.156.100.53', username='root', password='TSsbs@123..', timeout=20)
stdin, stdout, stderr = ssh.exec_command("docker exec $(docker ps -qf name=mysql) mysql -uroot -ptssbs123 netops -e 'DELETE FROM alert_events WHERE id=1;'", timeout=15)
print(stdout.read().decode())
ssh.close()
