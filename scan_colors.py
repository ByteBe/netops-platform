# -*- coding: utf-8 -*-
"""扫描 web/src 下所有 vue/scss/ts 的硬编码颜色，按颜色分组列出文件。"""
import re
import os
import collections

root = r"E:\IDEA\netops-platform\web\src"
pat = re.compile(r"#(?:[0-9a-fA-F]{3,8})\b|rgba?\([^)]*\)")
colors = collections.defaultdict(list)
for dp, dn, fn in os.walk(root):
    for f in fn:
        if not (f.endswith(".vue") or f.endswith(".scss") or f.endswith(".ts")):
            continue
        p = os.path.join(dp, f)
        try:
            txt = open(p, encoding="utf-8", errors="ignore").read()
        except Exception:
            continue
        for m in pat.finditer(txt):
            colors[m.group(0)].append(os.path.relpath(p, root).replace("\\", "/"))

print("total colors:", len(colors))
for c, files in sorted(colors.items(), key=lambda x: -len(x[1])):
    uniq = sorted(set(files))
    print("%s  x%d  %s" % (c, len(files), ", ".join(uniq[:6])))
