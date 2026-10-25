# -*- coding: utf-8 -*-
"""创建 Gitee Release v2.4.0 并上传 5 个部署包附件。"""
import json
import os
import sys
import urllib.request
import urllib.parse

OWNER = "ergn"
REPO = "netops-platform"
TOKEN = "4084d1f02e681fcea5d812f1a163fb33"
TAG = "v2.4.0"
NOTES = open(r"E:\IDEA\netops-platform\RELEASE_NOTES_v2.4.0.md", encoding="utf-8").read()
ZIPS = [
    r"E:\IDEA\netops-platform\dist-release\netops-windows-amd64-v2.4.0.zip",
    r"E:\IDEA\netops-platform\dist-release\netops-kylin-amd64-v2.4.0.zip",
    r"E:\IDEA\netops-platform\dist-release\netops-uos-amd64-v2.4.0.zip",
    r"E:\IDEA\netops-platform\dist-release\netops-kylin-arm64-v2.4.0.zip",
    r"E:\IDEA\netops-platform\dist-release\netops-uos-arm64-v2.4.0.zip",
]
BASE = "https://gitee.com/api/v5/repos/%s/%s" % (OWNER, REPO)


def req(url, data=None, headers=None, method=None):
    r = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
    try:
        with urllib.request.urlopen(r, timeout=120) as resp:
            return resp.status, resp.read().decode("utf-8", "ignore")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "ignore")


def multipart(field, path):
    """构造 multipart/form-data（文件较大，流式分段拼接）。"""
    boundary = "----netops" + os.urandom(8).hex()
    body = []
    body.append(("--" + boundary + "\r\n").encode())
    body.append(('Content-Disposition: form-data; name="%s"; filename="%s"\r\n' % (field, os.path.basename(path))).encode())
    body.append(b"Content-Type: application/octet-stream\r\n\r\n")
    with open(path, "rb") as f:
        body.append(f.read())
    body.append(("\r\n--" + boundary + "--\r\n").encode())
    data = b"".join(body)
    return boundary, data


def main():
    # 1. 检查 tag 是否已有 Release
    code, txt = req(BASE + "/releases/tags/" + TAG + "?access_token=" + TOKEN)
    if code == 200:
        release_id = json.loads(txt)["id"]
        print("existing release id:", release_id)
    else:
        payload = json.dumps({
            "tag_name": TAG,
            "name": "NetOps v2.4.0",
            "body": NOTES,
            "target_commitish": "main",
            "prerelease": False,
        }).encode()
        code, txt = req(BASE + "/releases?access_token=" + TOKEN, data=payload,
                        headers={"Content-Type": "application/json;charset=UTF-8"})
        if code not in (201, 200):
            print("create release FAILED:", code, txt)
            return
        release_id = json.loads(txt)["id"]
        print("created release id:", release_id)

    # 2. 上传附件（逐个）
    for zp in ZIPS:
        boundary, data = multipart("file", zp)
        url = BASE + "/releases/%s/attach_files?access_token=%s" % (release_id, TOKEN)
        code, txt = req(url, data=data,
                        headers={"Content-Type": "multipart/form-data; boundary=" + boundary},
                        method="POST")
        ok = code in (201, 200)
        print(("OK  " if ok else "FAIL") + (" " + os.path.basename(zp) if ok else (" %s %s" % (code, txt[:120]))))
        if not ok:
            print("abort on failure")
            return
    print("DONE")


if __name__ == "__main__":
    main()
