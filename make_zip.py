import os, zipfile, sys

src_bin = r"E:\IDEA\netops-platform\backend\netops-server-linux-amd64"
web_dir = r"E:\IDEA\netops-platform\backend\web"
install_sh = r"E:\IDEA\netops-platform\backend\install.sh"
out_zip = r"E:\IDEA\netops-platform\dist-release\netops-linux-amd64.zip"

if not os.path.exists(src_bin):
    print("ERROR: build linux bin first"); sys.exit(1)

with zipfile.ZipFile(out_zip, "w", zipfile.ZIP_DEFLATED) as z:
    z.write(src_bin, "netops-server")
    for root, dirs, files in os.walk(web_dir):
        for f in files:
            ap = os.path.join(root, f)
            rel = os.path.relpath(ap, r"E:\IDEA\netops-platform\backend")
            z.write(ap, rel.replace("\\", "/"))
    z.write(install_sh, "install.sh")

print("zipped", out_zip, os.path.getsize(out_zip))
