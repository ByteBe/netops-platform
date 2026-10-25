# -*- coding: utf-8 -*-
"""部署 Python 智能外围到双云并重启 netops-python 服务。
上传 pyservice/scriptgen + pyservice/report 全量 + aiinspect/app.py（挂载两路由），
验证 healthz、脚本生成接口与报告渲染接口。"""
import os
import paramiko

SERVERS = [
    ("53总部", "82.156.100.53", "root", "TSsbs@123.."),
    ("158分部", "36.213.106.158", "root", "Qaz3.141"),
]
LOCAL_PYSERVICE = r"E:\IDEA\netops-platform\pyservice"
REMOTE_PYSERVICE = "/opt/netops-platform/pyservice"

# 需要上传的文件（相对 pyservice 根）
FILES = [
    "scriptgen/__init__.py",
    "scriptgen/utils.py",
    "scriptgen/categories.py",
    "scriptgen/validate.py",
    "scriptgen/router.py",
    "scriptgen/vendor_huawei.py",
    "scriptgen/vendor_h3c.py",
    "scriptgen/vendor_cisco.py",
    "report/__init__.py",
    "report/render.py",
    "report/router.py",
    "configaudit/__init__.py",
    "configaudit/router.py",
    "tsarchive/__init__.py",
    "tsarchive/archive.py",
    "tsarchive/router.py",
    "aiinspect/app.py",
]

for name, h, u, p in SERVERS:
    print("=" * 50)
    print("[" + name + "]")
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(h, username=u, password=p, timeout=20)
    sftp = c.open_sftp()

    for rel in FILES:
        local = os.path.join(LOCAL_PYSERVICE, rel)
        remote_dir = os.path.dirname(os.path.join(REMOTE_PYSERVICE, rel).replace("\\", "/"))
        try:
            sftp.stat(remote_dir)
        except IOError:
            sftp.mkdir(remote_dir)
        remote = os.path.join(REMOTE_PYSERVICE, rel).replace("\\", "/")
        sftp.put(local, remote)
        print("  uploaded", rel)
    sftp.close()

    cmds = [
        "systemctl restart netops-python",
        "sleep 2",
        "curl -s http://127.0.0.1:8000/healthz",
        "curl -s -X POST http://127.0.0.1:8000/internal/scriptgen/generate -H 'Content-Type: application/json' -d '{\"code\":\"router_ospf\",\"vendor\":\"huawei\",\"params\":{\"process\":\"1\",\"rid\":\"1.1.1.1\",\"net\":\"10.16.194.0\",\"netmask\":\"255.255.255.0\",\"area\":\"0\"}}'",
        "curl -s -X POST http://127.0.0.1:8000/internal/report/render -H 'Content-Type: application/json' -d '{\"title\":\"冒烟测试报告\",\"start\":\"2026-10-14 20:00\",\"end\":\"2026-10-21 20:00\",\"now\":\"2026-10-21 20:00:05\",\"stats\":{\"total\":1,\"up\":1,\"down\":0,\"warn\":0},\"sections\":[\"link\"],\"link\":[{\"name\":\"测试链路\",\"target\":\"10.16.194.1\",\"status\":\"正常\",\"color\":\"#52c41a\",\"avg_rt\":12.3,\"downs\":0,\"avail\":100.0}],\"dev\":[],\"db\":[],\"nginx\":[],\"container\":[],\"node\":[]}' | head -c 120",
    ]
    for cmd in cmds:
        _, out, err = c.exec_command(cmd, timeout=60)
        o = out.read().decode("utf-8", "ignore").strip()
        e = err.read().decode("utf-8", "ignore").strip()
        if o:
            print("  " + o[:400])
        if e:
            print("  ERR:", e[:300])
    c.close()
print("done")
