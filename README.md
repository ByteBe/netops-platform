# NetOps 网络运维监控平台

> 仓库：[Gitee](https://gitee.com/ergn/netops-platform) | [GitHub](https://github.com/ByteBe/netops-platform)

一体化网络运维监控平台，Go 后端 + Vue3 前端，支持单机与分布式级联部署。

## 功能模块

| 分组 | 功能 |
|---|---|
| 网络监控 | 链路检测（ICMP/TCP）、设备监控（SNMP）、网络拓扑（2D/3D）、流量分析 |
| 告警中心 | 阈值告警（CPU/内存/磁盘/链路/节点离线）、分级、事件确认 |
| 资源与资产 | 资源管理、IP地址管理（IPAM）、子网计算、配置备份/对比/回滚 |
| 基础设施 | Docker 监控、K8s 监控、数据库监控 |
| 运维工具 | 巡检报告（日报/周报/月报）、脚本生成器、MCP Agent 接入 |
| 系统管理 | 用户管理、角色权限、AI 接入、MCP 配置、通知渠道、安全配置、审计日志、系统更新 |

## 技术栈

- 后端：Go 1.23 + Gin + GORM
- 前端：Vue3 + TypeScript + Element Plus + ECharts
- 数据库：SQLite / MySQL / PostgreSQL / 人大金仓（国产化）
- 时序库：内置 / TDengine / InfluxDB
- 分布式：MQTT 级联上报

## 快速开始

```bash
# Windows
解压 netops-platform-windows-amd64.zip → 双击 start.bat

# Linux
unzip netops-platform-linux-amd64.zip -d /opt/netops-platform
cd /opt/netops-platform && chmod +x install.sh && ./install.sh install
```

访问 http://服务器IP:30821，首次进入初始化向导。

## 文档

- [分布式架构](docs/分布式架构.md)
- [单机部署](docs/单机部署.md)

## 版本

v1.5.0
