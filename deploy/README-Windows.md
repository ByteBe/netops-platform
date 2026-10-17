# NetOps 网络运维监控平台 v2.2.0（Windows）

## 支持系统

- Windows 10 / 11（x86_64）
- Windows Server 2016+（x86_64）

## 部署步骤

```powershell
# 1. 解压 netops-windows-amd64.zip 到部署目录，如 D:\netops
# 2. 直接运行
.\netops-server.exe

# 或注册为 Windows 服务（可选，使用 NSSM 等工具）
```

## 首次访问

浏览器打开 `http://127.0.0.1:40000`，进入系统初始化向导：

1. 选择部署模式（单机/分布式）
2. 选择访问端口（默认 40000，可改填 0 回退为 API 单端口）
3. 配置业务数据库（SQLite/MySQL/PostgreSQL/人大金仓等）
4. 配置时序数据库（内置/TDengine/InfluxDB）
5. 创建管理员账号

初始化完成后，API 端口默认 30821。

## 目录结构

```
netops-server.exe     可执行文件
web/                  前端静态资源
data/                 运行数据目录（自动创建）
logs/                 日志目录（自动创建）
```
