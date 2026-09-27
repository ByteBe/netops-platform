# NetOps 网络运维监控平台

> 仓库：[Gitee](https://gitee.com/ergn/netops-platform) | [GitHub](https://github.com/ByteBe/netops-platform)

前后端一体、单二进制部署的网络运维监控平台。Windows / Linux 直接运行二进制即可。
路由与 API 全自动化注册，新增功能只需写前端页面和后端模块代码，无需手动配置路由表或菜单。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25+（Gin + GORM），全部静态编译 |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus + Pinia + ECharts + Three.js |
| 加密 | 国密 SM2（会话密钥交换）+ SM4-CBC（请求/响应信封），密码 SM3 摘要存储 |
| 关系库 | MySQL / Oracle / 达梦 DM8 / 人大金仓 KingbaseES / SQLite（内置） |
| 时序库 | TDengine / InfluxDB / 内置 TSDB |

## 端口

| 用途 | 端口 |
|------|------|
| Web 对外接口（API + 前端页面 + WebSocket） | **30821** |
| 内部管理接口（健康检查） | **30001** |

---

## 快速开始

### 下载二进制

在 [Releases](https://gitee.com/ergn/netops-platform/releases) 下载最新版本：
- `netops-windows-x64.zip` — Windows
- `netops-linux-x64.tar.gz` — Linux

### Linux 启动

```bash
tar -zxvf netops-linux-x64.tar.gz
cd netops-platform
chmod +x install.sh netops-server
./install.sh install
```

浏览器打开 `http://服务器IP:30821`，自动进入初始化向导。

### Windows 启动

解压后管理员运行：

```cmd
install.bat install
```

浏览器打开 `http://localhost:30821`。

---

## 目录结构

```
netops-platform/
├── backend/
│   ├── main.go
│   ├── internal/
│   │   ├── modreg/            # 模块注册中心（路由自动注册）
│   │   ├── module/            # 业务模块（每个目录一个模块，init()自注册）
│   │   ├── collector/         # 采集器（ping/snmp/db/docker/k8s）
│   │   ├── middleware/        # 中间件（认证/加密/审计/CORS）
│   │   ├── storage/          # 关系数据库驱动
│   │   ├── tsdb/             # 时序数据库驱动
│   │   └── ...
│   └── web/dist/             # 前端构建产物（go:embed 嵌入）
│
├── web/
│   └── src/
│       ├── api/index.ts      # 通用 useApi() 客户端
│       ├── views/            # Vue 页面（自动路由）
│       ├── router/nav.ts     # 菜单元数据（自动扫描）
│       ├── stores/           # Pinia 状态
│       └── utils/            # 请求/WS/加密
│
├── build.ps1                 # 一键构建
├── install.bat / install.sh  # 部署脚本
└── README.md
```

---

## 开发指南

### 新增一个功能模块（全自动注册）

**后端**：在 `backend/internal/module/新模块名/` 新建文件，写 `init()` 注册路由：

```go
package mymodule

import (
    "github.com/gin-gonic/gin"
    "netops/internal/core"
    "netops/internal/modreg"
)

func init() {
    modreg.RegisterProtected("mymodule", func(a *core.App, g *gin.RouterGroup) {
        g.GET("/list", func(c *gin.Context) {
            c.JSON(200, gin.H{"data": []string{}})
        })
    })
}
```

**前端**：在 `web/src/views/mymodule/` 新建 `index.vue`，路由自动生成 `/mymodule`，菜单自动出现。

**调用 API**：在 Vue 组件里用通用客户端：

```ts
import { useApi } from '@/api'
const api = useApi('mymodule')
const list = await api.list()
```

无需修改任何路由表、菜单配置或 API 文件。

### 构建

```powershell
# 前端
cd web
npm install
npm run build

# 后端
cd ..\backend
go build -o netops-server.exe .
```

---

## 功能清单

| 模块 | 说明 |
|------|------|
| 链路检测 | 多地址并行监控、实时曲线（WebSocket）、历史数据 |
| 网络拓扑 | 2D/3D 视图、设备图标、连线、可拖拽 |
| 设备监控 | SNMPv2/v3，支持 Cisco/Huawei/H3C/Windows/Linux |
| 磁盘采集 | HR-Storage 表，磁盘使用率实时监控 |
| Docker/K8s | 容器与集群状态监控 |
| 巡检报告 | HTML 格式，可邮件发送 |
| 数据库监控 | MySQL/Oracle/达梦/人大金仓/TDengine/InfluxDB |
| 脚本生成器 | 华为/华三设备配置生成 |
| IP 地址管理 | 地址分配、ARP 读取 |
| 子网计算器 | IP 计算、子网划分、批量复制 |
| 流量监控 | 上下行速率、带宽设置 |
| 数据大屏 | 实时趋势 + 状态汇总 |
| 系统管理 | 用户、审计日志、在线更新 |

---

## 常用命令

| 命令 | 作用 |
|------|------|
| `./install.sh install` | 安装 systemd 服务并启动 |
| `./install.sh stop` | 停止 |
| `./install.sh restart` | 重启 |
| `./install.sh status` | 查看状态 |
| `./install.sh logs` | 查看日志 |
| `./install.sh uninstall` | 卸载服务 |

Windows 对应 `install.bat`。

---

## 重置初始化

```bash
./install.sh stop
rm -f config.yaml
rm -rf data/
./install.sh start
```
