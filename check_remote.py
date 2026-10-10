import paramiko
ssh=paramiko.SSHClient(); ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('82.156.100.53', username='root', password='TSsbs@123..', timeout=20)
for c in ['systemctl status netops --no-pager -l', 'journalctl -u netops -n 15 --no-pager']:
 print('>>>',c)
 _,out,err=ssh.exec_command(c)
 print(out.read().decode(errors='ignore'))
 e=err.read().decode(errors='ignore')
 if e: print('ERR',e)
ssh.close()
