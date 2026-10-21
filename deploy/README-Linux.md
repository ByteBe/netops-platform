# NetOps 网络运维监控平台 v2.2.2

## 支持系统

- 麒麟操作系统（银河麒麟 V10，x86_64 / aarch64）
- 统信 UOS（V20，x86_64 / aarch64）
- Ubuntu / Debian / CentOS / Rocky 等主流 Linux（glibc）

> 说明：本包为 Linux 通用二进制（Go 静态编译，无外部依赖）。麒麟与统信系统
> 均基于 glibc，同一架构包可直接运行，无需针对系统单独编译。

## 部署步骤

```bash
# 1. 解压到部署目录
mkdir -p /opt/netops && tar -xzf netops-*.tar.gz -C /opt/netops   # 或 unzip netops-*.zip -d /opt/netops
cd /opt/netops

# 2. 赋予执行权限
chmod +x netops-server

# 3. 启动
./netops-server

# 4. （推荐）注册为 systemd 服务
cat > /etc/systemd/system/netops.service <<'EOF'
[Unit]
Description=NetOps Platform
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/netops
ExecStart=/opt/netops/netops-server
Restart=always

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload && systemctl enable --now netops
```

## 首次访问

浏览器打开 `http://服务器IP:40000`，进入系统初始化向导：

1. 选择部署模式（单机/分布式）
2. 选择访问端口（默认 40000，可改填 0 回退为 API 单端口）
3. 配置业务数据库（SQLite/MySQL/PostgreSQL/人大金仓等国产库）
4. 配置时序数据库（内置/TDengine/InfluxDB）
5. 创建管理员账号

初始化完成后，API 端口默认 30821。

## 功能

网络监控（链路/设备/拓扑/流量/BGP）、告警中心、资源与资产、
基础设施（Docker / K8s / 数据库 / **Nginx 监控**）、运维工具、系统管理。

## 目录结构

```
netops-server         可执行文件（Go 静态编译）
web/                  前端静态资源（Vue3 构建产物）
config.yaml           （可选）自定义配置，缺省自动生成
data/                 运行数据目录（数据库/时序库/上传文件，自动创建）
logs/                 日志目录（自动创建）
```
