import pymupdf, os
out=r'E:\IDEA\netops-platform\web\public\topology-icons'
os.makedirs(out, exist_ok=True)
files={
 'server':r'D:\下载\服务器.pdf',
 'vuln':r'D:\下载\漏洞扫描.pdf',
 'audit':r'D:\下载\日志审计.pdf',
 'bastion':r'D:\下载\堡垒机.pdf',
 'wan':r'D:\下载\广域网.pdf',
 'ac':r'D:\下载\ac.pdf',
}
for name,path in files.items():
    doc=pymupdf.open(path)
    imgs=sum([p.get_images(full=True) for p in doc],[])
    if imgs:
        data=doc.extract_image(imgs[0][0])['image']
        with open(os.path.join(out,f'{name}.png'),'wb') as f: f.write(data)
        print(name, len(data))
