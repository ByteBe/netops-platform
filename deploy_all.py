# -*- coding: utf-8 -*-
"""一键融合部署（Go 主程序 + Python 智能外围 + Web 前端）到双云 53/158。

流程：
  1) 交叉编译 Go Linux 二进制并打包 netops-server-only.zip（仅 netops-server）
  2) 上传 zip -> unzip 提取替换二进制 -> 重启 netops（保留 config.yaml/data）
  3) 上传 pyservice 文件 -> 重启 netops-python -> healthz/scriptgen/report 冒烟
  4) 上传 web/dist -> /opt/netops-platform/web（不重启，保留登录态）

用法：python deploy_all.py            # 编译 + 全量部署
      python deploy_all.py --no-build # 跳过编译（使用 E:\\IDEA\\dist\\netops-server-only.zip）
"""
import os
import shutil
import subprocess
import sys
import zipfile
import paramiko

BACKEND = r"E:\IDEA\netops-platform\backend"
DIST = r"E:\IDEA\dist"
LOCAL_ZIP = os.path.join(DIST, "netops-server-only.zip")
LOCAL_PYSERVICE = r"E:\IDEA\netops-platform\pyservice"
LOCAL_WEB = r"E:\IDEA\netops-platform\web\dist"

SERVERS = [
    ("53总部", "82.156.100.53", "root", "TSsbs@123.."),
    ("158分部", "36.213.106.158", "root", "Qaz3.141"),
]

PY_FILES = [
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
    "aiinspect/app.py",
]


def build_backend():
    print("== 1) 交叉编译 Go Linux 二进制 ==")
    os.makedirs(DIST, exist_ok=True)
    env = dict(os.environ)
    env["GOOS"] = "linux"
    env["GOARCH"] = "amd64"
    env["CGO_ENABLED"] = "0"
    out = os.path.join(DIST, "netops-linux-amd64")
    r = subprocess.run(
        ["go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, "."],
        cwd=BACKEND, env=env, capture_output=True, text=True,
    )
    if r.returncode != 0:
        print("编译失败:", r.stderr[-2000:])
        sys.exit(1)
    with zipfile.ZipFile(LOCAL_ZIP, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(out, "netops-server")
    print("  二进制 + zip 完成:", LOCAL_ZIP)


def upload_tree(sftp, local, remote):
    for root, dirs, files in os.walk(local):
        rel = os.path.relpath(root, local)
        target = remote if rel == "." else remote + "/" + rel.replace("\\", "/")
        try:
            sftp.stat(target)
        except IOError:
            sftp.mkdir(target)
        for f in files:
            lp = os.path.join(root, f)
            rp = target + "/" + f
            sftp.put(lp, rp)
            print("  up:", rp)


def deploy_one(name, host, user, pwd):
    print("=" * 52)
    print(f"[{name}] {host}")
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(host, username=user, password=pwd, timeout=25)
    sftp = c.open_sftp()

    # 2) 后端二进制
    print("-- 后端二进制 --")
    sftp.put(LOCAL_ZIP, "/opt/netops-server-only.zip")
    cmds = [
        "systemctl stop netops",
        "sleep 1",
        "cd /opt && unzip -o netops-server-only.zip -d /opt/netops-platform",
        "chmod +x /opt/netops-platform/netops-server",
        "systemctl reset-failed netops",
        "systemctl start netops",
        "sleep 3",
        "systemctl is-active netops",
    ]
    for cmd in cmds:
        _, out, err = c.exec_command(cmd, timeout=60)
        o = out.read().decode("utf-8", "ignore").strip()
        e = err.read().decode("utf-8", "ignore").strip()
        if o:
            print("  " + o)
        if e:
            print("  ERR:", e[:300])

    # 3) Python 智能外围
    print("-- Python 智能外围 --")
    for rel in PY_FILES:
        local = os.path.join(LOCAL_PYSERVICE, rel)
        rdir = os.path.dirname(os.path.join("/opt/netops-platform/pyservice", rel).replace("\\", "/"))
        try:
            sftp.stat(rdir)
        except IOError:
            sftp.mkdir(rdir)
        rp = os.path.join("/opt/netops-platform/pyservice", rel).replace("\\", "/")
        sftp.put(local, rp)
    cmds = [
        "systemctl restart netops-python",
        "sleep 2",
        "curl -s http://127.0.0.1:8000/healthz",
        "curl -s -X POST http://127.0.0.1:8000/internal/scriptgen/generate -H 'Content-Type: application/json' -d '{\"code\":\"router_ospf\",\"vendor\":\"huawei\",\"params\":{\"process\":\"1\",\"rid\":\"1.1.1.1\",\"net\":\"10.16.194.0\",\"netmask\":\"255.255.255.0\",\"area\":\"0\"}}' | head -c 80",
        "curl -s -X POST http://127.0.0.1:8000/internal/report/render -H 'Content-Type: application/json' -d '{\"title\":\"融合冒烟\",\"start\":\"2026-10-14 20:00\",\"end\":\"2026-10-21 20:00\",\"now\":\"2026-10-21 20:00:05\",\"stats\":{\"total\":1,\"up\":1,\"down\":0,\"warn\":0},\"sections\":[\"link\"],\"link\":[{\"name\":\"测试链路\",\"target\":\"10.16.194.1\",\"status\":\"正常\",\"color\":\"#52c41a\",\"avg_rt\":12.3,\"downs\":0,\"avail\":100.0}],\"dev\":[],\"db\":[],\"nginx\":[],\"container\":[],\"node\":[]}' | head -c 80",
    ]
    for cmd in cmds:
        _, out, err = c.exec_command(cmd, timeout=60)
        o = out.read().decode("utf-8", "ignore").strip()
        e = err.read().decode("utf-8", "ignore").strip()
        if o:
            print("  " + o[:400])
        if e:
            print("  ERR:", e[:300])

    # 4) Web 前端
    print("-- Web 前端 --")
    upload_tree(sftp, LOCAL_WEB, "/opt/netops-platform/web")
    sftp.close()
    c.close()
    print(f"[{name}] done")


def main():
    skip_build = "--no-build" in sys.argv
    if not skip_build:
        build_backend()
    for srv in SERVERS:
        deploy_one(*srv)
    print("\n全部完成。验证：http://82.156.100.53:40000/ 与 http://36.213.106.158:40000/")


if __name__ == "__main__":
    main()
