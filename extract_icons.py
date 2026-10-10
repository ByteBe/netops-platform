import pymupdf, os
doc=pymupdf.open(r'D:\下载\新建 Microsoft Visio Drawing.pdf')
out=r'E:\IDEA\netops-platform\web\public\topology-icons'
os.makedirs(out, exist_ok=True)
n=0
for p in doc:
    for img in p.get_images(full=True):
        xref=img[0]
        data=doc.extract_image(xref)['image']
        ext=data[:8]
        with open(os.path.join(out,f'icon_{n}.png'),'wb') as f:
            f.write(data)
        n+=1
print(n)
