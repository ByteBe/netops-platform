# NetOps 网络运维监控平台

> 仓库：[Gitee](https://gitee.com/ergn/netops-platform) | [GitHub](https://github.com/ByteBe/netops-platform)

前后端一体、单二进制部署的网络运维监控平台。Windows / Linux 直接运行二进制即可。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25+（Gin + GORM），全部静态编译 |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus + Pinia + ECharts + Three.js，样式全部 SCSS |
| 加密 | 国密 SM2（会话密钥交换）+ SM4-CBC（请求/响应信封），密码 SM3 摘要存储 |
| 关系库 | MySQL / Oracle / 达梦 DM8 / 人大金仓 KingbaseES / SQLite（内置） |
| 时序库 | TDengine / InfluxDB / 内置 TSDB（不使用 Redis） |

## 端口

| 用途 | 端口 |
|------|------|
| Web 对外接口（API + 前端页面 + WebSocket） | **30821** |
| 内部管理接口（健康检查） | **30001** |

---

## 目录结构

```
netops-platform/
├── backend/                    # 纯后端（Go）
│   ├── main.go
│   ├── go.mod / go.sum
│   ├── internal/
│   │   ├── appinit/           # 数据库迁移 + 种子数据
│   │   ├── bootstrap/          # 应用启动入口
│   │   ├── collector/         # 采集器（ping/snmp/db/docker/k8s）
│   │   ├── common/             # 国密/日志/密码/响应
│   │   ├── config/            # 配置加载
│   │   ├── core/               # App 核心结构
│   │   ├── middleware/         # 中间件（认证/加密/审计/CORS）
│   │   ├── model/              # 数据模型（GORM）
│   │   ├── module/             # 业务模块
│   │   ├── router/             # 路由自动注册
│   │   ├── service/            # 公共服务（AI/邮箱/MCP/SSH）
│   │   ├── storage/             # 关系数据库驱动
│   │   └── tsdb/               # 时序数据库驱动
│   └── web/dist/              # 前端构建产物（go:embed 嵌入）
│
├── web/                        # 纯前端（Vue3/TS）
│   └── src/
│       ├── api/                # API 接口 + 类型
│       ├── views/              # Vue 页面
│       ├── router/              # 路由
│       ├── stores/              # Pinia 状态
│       ├── styles/              # 全局 SCSS
│       └── utils/               # 请求/WS/加密工具
│
├── build.ps1                   # Windows 一键构建（输出 dist-release/）
├── install.bat                 # Windows 部署脚本
├── install.sh                  # Linux 部署脚本
├── LICENSE
└── README.md
```

---

## 构建

### 前置要求
- Go 1.25+
- Node.js 18+

### Windows 一键构建

```powershell
.\build.ps1
```

输出在 `dist-release/` 目录，包含：
- `netops-server.exe` — 后端二进制（含前端嵌入）
- `web/dist/` — 前端静态文件
- `install.bat` / `install.sh` — 部署脚本

### 交叉编译 Linux 版本

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'
.\build.ps1
```

> **注意**：每次修改前端后必须重新完整构建，因为 `go:embed` 在编译时把前端嵌入二进制。

---

## 部署

### Linux（systemd 后台运行 + 开机自启）

```bash
# 1. 上传 dist-release 整个目录到服务器
scp -r dist-release/* root@<服务器IP>:/opt/netops-platform/

# 2. 进入目录
cd /opt/netops-platform
chmod +x install.sh netops-server

# 3. 安装服务（自动创建 systemd、开机自启、启动）
./install.sh install
```

| 命令 | 作用 |
|------|------|
| `./install.sh install` | 安装 systemd 服务 + 开机自启 + 启动 |
| `./install.sh start` | nohup 后台启动 |
| `./install.sh stop` | 停止 |
| `./install.sh restart` | 重启 |
| `./install.sh status` | 查看状态 |
| `./install.sh uninstall` | 卸载服务 |
| `./install.sh logs` | 查看日志 |

### Windows（任务计划后台运行 + 开机自启）

```cmd
:: 管理员身份运行
install.bat install
```

| 命令 | 作用 |
|------|------|
| `install.bat install` | 安装任务计划 + 开机自启 + 启动 |
| `install.bat start` | 启动 |
| `install.bat stop` | 停止 |
| `install.bat restart` | 重启 |
| `install.bat status` | 查看状态 |
| `install.bat uninstall` | 卸载服务 |
| `install.bat logs` | 打开日志目录 |

---

## 首次初始化

浏览器打开 `http://<服务器IP>:30821`，系统自动进入初始化向导：

1. **选择存储数据库**：MySQL / Oracle / 达梦 DM8 / 人大金仓 / SQLite（内置，无需安装）
2. **选择时序数据库**：内置 TSDB（无需安装）/ TDengine / InfluxDB
3. **创建管理员账号**：用户名 + 密码（>12位，大写+小写+数字+符号）

每步可点"测试连接"验证数据库连通性。完成后自动建表、创建管理员、跳转登录页。

> 管理员创建新用户时，新用户初始密码统一为 `123456`。

---

## 功能清单

| 模块 | 说明 |
|------|------|
| 链路检测 | 多地址并行监控、实时曲线（WebSocket）、历史数据查询、拖拽缩放 |
| 网络拓扑 | 2D/3D 视图、设备图标、连线多 IP、设备可拖拽、支持上传背景图 |
| 设备监控 | SNMPv2/v3 采集服务器/交换机/路由器，CPU/内存/接口流量 |
| Docker 监控 | 独立配置 Docker 主机，容器状态/CPU/内存 |
| K8s 监控 | 独立配置 K8s 集群，Pod/节点/Deployment 状态 |
| 巡检报告 | 链路通断 + 设备 + 数据库状态，HTML 格式，可邮件发送 |
| 数据库监控 | MySQL/Oracle/达梦/人大金仓，可用性/容量/事务/错误 |
| 脚本生成器 | 华为/华三，路由器/交换机/AC，ACL/NAT/OSPF/BGP/VRRP/VLAN 等 |
| IP 地址管理 | 已用/未用地址、ARP 读取、使用人登记、SSH 下发绑定 |
| 资源管理 | 设备分组分类、自动归集、组内设备实时状态 |
| 流量监控 | 上下行独立速率、专线带宽设置 |
| 数据大屏 | 右上角入口，链路实时趋势 + 状态汇总 |
| 系统管理 | 用户管理、AI 接入、MCP 配置、邮箱、审计日志、在线更新 |

---

## 健康检查

```bash
curl http://127.0.0.1:30001/internal/health
```

---

## 重置初始化

```bash
# Linux
./install.sh stop
rm -f config.yaml
rm -rf data/
./install.sh start
```

```cmd
:: Windows
install.bat stop
del config.yaml
rmdir /s /q data
install.bat start
```
