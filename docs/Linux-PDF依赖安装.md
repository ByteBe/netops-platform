# NetOps 平台 Linux 服务器依赖安装清单

## 概述

NetOps 巡检报告 PDF 导出功能依赖 `wkhtmltopdf` 将 HTML 报告渲染为 PDF 文件。

全新安装的 Linux 服务器（尤其是国产化系统）默认缺少这些组件，需手动安装。



***

## 一、Ubuntu / Debian 系统



```
# 1. 更新软件源
apt-get update

# 2. 安装 wkhtmltopdf（HTML 转 PDF 引擎）
apt-get install -y wkhtmltopdf

# 3. 安装中文字体（必须，否则 PDF 中文显示为方块）
apt-get install -y fonts-wqy-zenhei fonts-wqy-microhei fonts-noto-cjk

# 4. 安装 wkhtmltopdf 运行依赖（服务器无图形界面时必需）
apt-get install -y xvfb libfontconfig1 fontconfig

# 5. 刷新字体缓存
fc-cache -fv
```

### 验证安装



```
# 检查 wkhtmltopdf
which wkhtmltopdf
wkhtmltopdf --version
# 应输出: wkhtmltopdf 0.12.6

# 检查中文字体
fc-list :lang=zh
# 应列出 Noto Sans CJK / WenQuanYi 等中文字体
```



***

## 二、CentOS / RHEL / 麒麟 V10



```
# 1. 安装 EPEL 源（如未安装）
yum install -y epel-release

# 2. 安装 wkhtmltopdf
yum install -y wkhtmltopdf

# 3. 安装中文字体
yum install -y wqy-zenhei-fonts wqy-microhei-fonts google-noto-sans-cjk-fonts

# 4. 安装运行依赖
yum install -y xorg-x11-server-Xvfb fontconfig

# 5. 刷新字体缓存
fc-cache -fv
```



***

## 三、统信 UOS / 深度 Deepin



```
# 1. 更新软件源
apt-get update

# 2. 安装依赖
apt-get install -y wkhtmltopdf fonts-wqy-zenhei fonts-wqy-microhei fonts-noto-cjk xvfb libfontconfig1

# 3. 刷新字体缓存
fc-cache -fv
```



***

## 四、验证 PDF 生成

安装完成后，可手动测试：



```
# 创建测试 HTML
cat > /tmp/test.html << 'EOF'
<html><head><meta charset="utf-8"></head>
<body>
<h1>测试报告</h1>
<p>中文渲染测试：运维巡检报告 链路检测 服务器资源使用</p>
</body></html>
EOF

# 生成 PDF
wkhtmltopdf --encoding utf-8 /tmp/test.html /tmp/test.pdf

# 查看结果
ls -lh /tmp/test.pdf
```

打开 `/tmp/test.pdf`，中文应正常显示，无方块或乱码。



***

## 五、常见问题



| 问题                                        | 原因                    | 解决                                   |
| ----------------------------------------- | --------------------- | ------------------------------------ |
| PDF 中文显示为方块                               | 未装中文字体                | 安装 fonts-noto-cjk 或 fonts-wqy-zenhei |
| PDF 中 emoji 显示为方框                         | wkhtmltopdf 不支持 emoji | 报告已去除 emoji，使用【】标记替代                 |
| `wkhtmltopdf: cannot connect to X server` | 服务器无图形界面              | 安装 xvfb，或使用 `xvfb-run wkhtmltopdf`   |
| PDF 生成失败 / 超时                             | wkhtmltopdf 版本过低      | 确保版本 >= 0.12.6                       |
| 字体不生效                                     | 字体缓存未刷新               | 执行 `fc-cache -fv`                    |



***

## 六、Docker 部署注意

如果使用 Docker 容器部署，Dockerfile 中需包含：



```
RUN apt-get update && apt-get install -y \
    wkhtmltopdf \
    fonts-wqy-zenhei \
    fonts-wqy-microhei \
    fonts-noto-cjk \
    xvfb \
    libfontconfig1 \
    && rm -rf /var/lib/apt/lists/* \
    && fc-cache -fv
```