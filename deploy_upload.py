import paramiko, sys, os, time

host = "82.156.100.53"
user = "root"
pwd = "TSsbs@123.."
local_zip = r"E:\IDEA\netops-platform\dist-release\netops-linux-amd64.zip"
remote_zip = "/tmp/netops-linux-amd64.zip"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pwd, timeout=20)

def run(cmd):
    print(f"$ {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd, get_pty=True, timeout=120)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    rc = stdout.channel.recv_exit_status()
    if out.strip(): print(out)
    if err.strip(): print("[stderr]", err)
    print(f"[rc={rc}]")
    return rc

# Upload via SFTP
print("Uploading...")
sftp = ssh.open_sftp()
sftp.put(local_zip, remote_zip)
sftp.close()
print("Upload done")

# Backup current, unzip, restart
run("mkdir -p /opt/netops-platform-backup && cp -r /opt/netops-platform /opt/netops-platform-backup/$(date +%Y%m%d-%H%M) 2>/dev/null; echo backup-ok")
run("systemctl stop netops")
run("cd /opt/netops-platform && rm -f netops-server && rm -rf web/assets web/index.html && unzip -o /tmp/netops-linux-amd64.zip -d /opt/netops-platform > /dev/null && chmod +x /opt/netops-platform/netops-server /opt/netops-platform/install.sh && echo unzip-ok")
run("systemctl start netops && sleep 3 && systemctl is-active netops && journalctl -u netops -n 10 --no-pager")

ssh.close()
print("DONE")
