# NetOps Go 主程序与 Python 智能外围融合架构

> 版本：v2.4.0 ｜ 更新：2026-10-25
> 一句话：**Go 负责采集与稳定服务，Python 负责智能外围，前端统一走 Go API，部分功能的实现由 Python 提供，使用者无感知。**

## 1. 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│  使用者（Web 前端 / MCP Agent / 定时任务）                     │
│  统一入口：Go API 网关（netops-server，端口 40000 / 30001）    │
├─────────────────────────────────────────────────────────────┤
│  Go 主程序（netops-server）                                  │
│   · 设备采集：SNMP / Ping / Docker / K8s / Nginx / 数据库探测  │
│   · 数据存储：MySQL（业务）+ TDengine（时序）                 │
│   · 稳定服务：认证、告警、拓扑、证书、审计、用户、权限……        │
│   · 融合转发层：pysvc.Client（统一 HTTP 调用 Python）          │
└──────────────────────────┬──────────────────────────────────┘
                           │ http://127.0.0.1:8000（python_service_url）
┌──────────────────────────▼──────────────────────────────────┐
│  Python 智能外围（netops-python，FastAPI 常驻）               │
│   · aiinspect：AI 巡检（LLM 报告 / pandas 统计 / TDengine 趋势）│
│   · scriptgen ：脚本生成器（多厂商配置命令生成 + 校验）          │
│   · report    ：报告渲染（运维巡检报告 HTML 模板 / 周报样式）     │
└─────────────────────────────────────────────────────────────┘
```

## 2. 功能归属表（哪些功能"使用者调用时走 Python"）

| 前端功能 | Go API 入口 | Python 内部路由 | Python 不可用时 |
| --- | --- | --- | --- |
| AI 巡检 | `POST /api/aiinspect/run` | `POST /internal/aiinspect/run` | 明确失败提示（502） |
| 脚本生成器 | `POST /api/scriptgen/generate` | `POST /internal/scriptgen/generate` | 明确失败提示（502） |
| 运维巡检报告 / 周报 | `POST /api/report/*` | `POST /internal/report/render` | 明确失败提示（502） |

降级原则（v2.4.0 起）：**Python 是唯一实现，Go 不保留内置兜底**——AI 巡检 / 脚本生成 / 报告渲染均已删除 Go 本地实现（内置 AI 分支、厂商命令生成器与校验族、本地 HTML 模板与图标）。Python 不可用时接口直接返回 502「请检查 Python 智能外围（netops-python）服务状态」，避免双实现造成的代码臃肿与行为不一致。Go 侧保留的职责：数据采集（设备/链路/数据库/Nginx/容器）、报告数据收集（collect*Rows → payload 序列化）、周报调度与邮件/通知渠道发送、PDF 下载（wkhtmltopdf 转本地渲染 HTML）。

## 3. 融合转发层（Go 侧）

- 公共客户端：`backend/internal/pysvc/client.go`
  - `New(baseURL)` / `Enabled()` / `PostJSON(path, req, out, timeout)` / `Ping()`
  - 统一超时（默认 30s，AI 巡检 120s）、统一错误包装、统一 JSON 编解码
- 全局注入：`core.App.PySvc`（启动时按 `python_service_url` 创建）
- 使用模块：`aiinspect.go` / `scriptgen.go` / `report.go` 三个模块的转发函数内部统一走 `a.PySvc.PostJSON(...)`，消除了原先三份重复的 HTTP 客户端代码。

## 4. 服务状态可观测

- `core.App.Stats()` 新增 `python_svc` 字段：`online` / `offline` / `disabled`
- 通过内部端口 `GET :30001/internal/health` 暴露（含 ready/tsdb 等运行时统计）
- 判定方式：`python_service_url` 为空 → `disabled`；`GET /healthz` 失败 → `offline`；成功 → `online`

## 5. Python 服务（netops-python）

- 入口：`pyservice/aiinspect/app.py`（FastAPI，端口 8000）
- 子路由挂载：`/internal/aiinspect/run`、`/internal/scriptgen/generate`、`/internal/report/render`
- 系统服务：`systemctl status netops-python`（与 netops 同机部署，仅本机回环）
- 依赖：MySQL（统计）/ TDengine（趋势）/ LLM 配置（可选，缺失自动降级统计模式）

## 6. 部署（融合一键部署）

```bash
python deploy_all.py            # 编译 Go + 部署 Go/Python/Web 到双云
python deploy_all.py --no-build # 跳过编译，用现有 zip
```

等价于依次执行：`deploy_backend_only.py` → `deploy_python_only.py` → `deploy_web_only.py`。
铁律不变：**只替换 netops-server 二进制、pyservice 文件与 web 目录，保留 config.yaml / data，不重置初始化状态。**

## 7. 扩展约定（新增"Python 化功能"时的步骤）

1. 在 `pyservice` 下建子包（如 `pyservice/xxx/`），提供 `router.py` 并挂载到 `aiinspect/app.py`
2. Go 侧新增模块转发：使用 `a.PySvc.PostJSON("/internal/xxx/action", req, &out, timeout)`，Python 失败时返回明确错误（如 `response.Fail(c, 502, 50200, "…服务不可用…")`），**不再实现本地兜底**
3. 同步检查：MySQL 表结构 / TDengine 建表与 tags 一致性 / 前端 i18n 双语
4. `deploy_python_only.py` 或 `deploy_all.py` 的 `PY_FILES` 增加新文件
