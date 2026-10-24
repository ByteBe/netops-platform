# -*- coding: utf-8 -*-
import glob, os, re
os.chdir(r"E:\IDEA\netops-platform\web")
ep = [f for f in glob.glob("dist/assets/element-plus-*.js")][0]
data = open(ep, encoding="utf-8", errors="replace").read()
paths = re.findall(r'd="M[^"]{10,}"', data)
print("element-plus chunk SVG path 数:", len(paths))
print("文件:", os.path.basename(ep), f"{os.path.getsize(ep)//1024}KB")
# layout chunk 引用检查
lay = glob.glob("dist/assets/layout-*.js")
print("layout chunk:", lay)
if lay:
    ldata = open(lay[0], encoding="utf-8", errors="replace").read()
    # 导航菜单相关：np-mi / navMeta 的图标字段编译后
    print("layout 中 icon 相关引用片段数:", len(re.findall(r'"[A-Z][A-Za-z]+"', ldata)))
