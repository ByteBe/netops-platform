import paramiko
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('82.156.100.53', username='root', password='TSsbs@123..')
for cmd in [
    'echo "<h1>test</h1>" > /tmp/t.html',
    '/usr/bin/wkhtmltopdf --quiet /tmp/t.html /tmp/t.pdf; echo rc=$?',
    'ls -la /tmp/t.pdf',
    'journalctl -u netops -n 15 --no-pager',
]:
    print('>>>', cmd)
    stdin, stdout, stderr = c.exec_command(cmd, timeout=30)
    print('OUT:', stdout.read().decode())
    err = stderr.read().decode()
    if err.strip(): print('ERR:', err[:500])
c.close()
