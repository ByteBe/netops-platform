import paramiko
ssh=paramiko.SSHClient(); ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('82.156.100.53',username='root',password='TSsbs@123..',timeout=20)
cmd="grep -o 'roam:!1' /opt/netops-platform/web/assets/*.js | head; echo ---; ls -lt /opt/netops-platform/web/assets/*.js | head"
_,o,e=ssh.exec_command(cmd,timeout=60)
print(o.read().decode(errors='ignore')); print(e.read().decode(errors='ignore'))
ssh.close()
