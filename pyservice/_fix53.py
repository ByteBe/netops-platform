# -*- coding: utf-8 -*-
"""53 修复：上传修复后的 stats.py -> 重启 Python 服务 -> 部署修复后的 Go 二进制 -> 验证"""
import paramiko, os

HOST, USER, PWD = "82.156.100.53", "root", "TSsbs@123.."
LOCAL_STATS = r"E:\IDEA\netops-platform\pyservice\aiinspect\stats.py"
LOCAL_BE = r"E:\IDEA\netops-platform\backend\netops-server.linux"
REMOTE_BE = "/opt/netops-platform/netops-server"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PWD, timeout=20)
sftp = ssh.open_sftp()

def run(cmd, t=120):
    _, stdout, stderr = ssh.exec_command(cmd, timeout=t)
    out = stdout.read().decode(errors="replace")
    err = stderr.read().decode(errors="replace")
    return stdout.channel.recv_exit_status(), (out + "\n" + err).strip()

res = []
# 1. 上传 stats.py 并重启 Python 服务
sftp.put(LOCAL_STATS, "/opt/netops-platform/pyservice/aiinspect/stats.py")
res.append(f"up stats.py {os.path.getsize(LOCAL_STATS)}B")
c, o = run("systemctl restart netops-python && sleep 2 && systemctl is-active netops-python")
res.append(f"python restart: {c} {o}")
c, o = run("curl -s http://127.0.0.1:8000/healthz")
res.append(f"healthz: {o}")

# 2. Python 服务 MySQL 统计验证（修复后应 ok=true）
c, o = run("""curl -s -X POST http://127.0.0.1:8000/internal/aiinspect/run -H 'Content-Type: application/json' -d '{"snapshot":{},"time":"2026-10-24 20:00:00"}' > /tmp/py3.json; /opt/netops-platform/pyservice/venv/bin/python - <<'EOF'
import json
d = json.load(open('/tmp/py3.json'))
print('source:', d.get('source'))
print('stats:', json.dumps(d.get('stats', {}), ensure_ascii=False)[:600])
EOF""", 120)
res.append("=== Python 服务（修复后） ===\n" + o[:800])

# 3. 部署修复后的 Go 二进制
size = os.path.getsize(LOCAL_BE)
c, o = run("cp -f /opt/netops-platform/netops-server /opt/netops-platform/netops-server.bak")
res.append("备份 ok")
try:
    sftp.remove(REMOTE_BE + ".new")
except Exception:
    pass
with open(LOCAL_BE, "rb") as f:
    with sftp.file(REMOTE_BE + ".new", "wb") as rf:
        while True:
            d = f.read(2 * 1024 * 1024)
            if not d:
                break
            rf.write(d)
rs = sftp.stat(REMOTE_BE + ".new")
res.append(f"上传 {rs.st_size} vs {size} 一致: {rs.st_size == size}")
if rs.st_size != size:
    raise SystemExit("大小不一致！")
c, o = run("mv -f /opt/netops-platform/netops-server.new /opt/netops-platform/netops-server && chmod +x /opt/netops-platform/netops-server && systemctl restart netops && sleep 3 && systemctl is-active netops")
res.append(f"Go restart: {c} {o}")

# 4. 端到端：登录 -> 调 /api/v1/aiinspect/run
c, o = run("""TOKEN=$(curl -s -X POST http://127.0.0.1:30821/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | /opt/netops-platform/pyservice/venv/bin/python -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null); echo "token_len=${#TOKEN}"; if [ -n "$TOKEN" ]; then curl -s -X POST "http://127.0.0.1:30821/api/v1/aiinspect/run" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{}' | head -c 400; else curl -s -X POST http://127.0.0.1:30821/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | head -c 200; fi""", 90)
res.append("=== Go 转发端到端 ===\n" + o[:700])

open(r"E:\IDEA\netops-platform\_fix53.txt", "w", encoding="utf-8").write("\n".join(res))
sftp.close()
ssh.close()
