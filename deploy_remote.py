import paramiko, os, sys
host='82.156.100.53'
user='root'
pwd='TSsbs@123..'
local=r'E:\IDEA\netops-platform\package\netops-linux-amd64.zip'
remote='/opt/netops-platform-linux-amd64.zip'
ssh=paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pwd, timeout=20)
sftp=ssh.open_sftp()
print('uploading...')
sftp.put(local, remote)
sftp.close()
cmds=[
 'systemctl stop netops || true',
 'rm -rf /opt/netops-platform-bak',
 'mv /opt/netops-platform /opt/netops-platform-bak || true',
 'mkdir -p /opt/netops-platform',
 'unzip -o /opt/netops-platform-linux-amd64.zip -d /opt/netops-platform',
 'chmod +x /opt/netops-platform/netops-server /opt/netops-platform/install.sh',
 '/opt/netops-platform/install.sh',
]
for c in cmds:
 print('>>>', c)
 stdin,stdout,stderr=ssh.exec_command(c, timeout=120)
 out=stdout.read().decode(errors='ignore')
 err=stderr.read().decode(errors='ignore')
 print(out)
 if err: print('ERR:', err)
ssh.close()
print('done')
