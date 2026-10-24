# NetOps 网络运维监控平台 v2.4.0 更新说明

> 发布类型：正式发布 ｜ 发布日期：2026-10-25 ｜ 双远端同步：Gitee(ergn) / GitHub(ByteBe)

## 一、架构升级：Python 智能外围

延续「Go 采集不动 + Python 智能外围」架构（AI 巡检已验证），本次将 **脚本生成器（scriptgen）** 与 **巡检报告（report）** 两个模块的生成/渲染逻辑迁移至 Python 服务（pyservice，与 Go 主服务同机部署、127.0.0.1:8000 内部通信）：

- **Go 职责**：API 路由、权限校验、参数校验框架、数据收集（report 各 section 结构化数据）、调度与存储
- **Python 职责**：命令生成（scriptgen）、报告 HTML 渲染（report）
- **高可用兜底**：Python 服务异常时 Go 自动回退原实现，用户无感知；双云（53/158）已部署验证

## 二、脚本生成器（scriptgen）

1. **Python 化改造**：18 类命令生成迁移至 `pyservice/scriptgen`（华为 VRP / 华三 Comware / 思科 IOS），Go 保留校验框架并转发，失败自动回退
2. **掩码校验误报修复**：OSPF 等填写 `255.255.255.0` 不再误报「掩码格式不正确」——前端 `validMask` 由 `clz32` 改为位计数（与 Go/Python 位计数一致）；生成时移除重复的全局提示，仅保留字段下方红字
3. **厂商命令对照检查修复**（Python 与 Go 同步）：
   - 思科静态路由：掩码自动转点分（`0`→`0.0.0.0`、`24`→`255.255.255.0`）
   - 思科 BGP：多邻居不再整串输出，逐条生成 `neighbor x remote-as y`
   - 思科 IS-IS：移除 IOS 视图不存在的 `router-id` 命令
   - 华三 OSPF：`area` 原样输出十进制（支持 >255 的大区域号）
4. 新增 6 项回归用例，Python 冒烟测试对照 Go 全量用例通过

## 三、巡检报告（report）

1. **渲染 Python 化**：Go 收集（链路/服务器/数据库/Nginx/容器/节点 6 个 section 数据）+ Python 渲染 HTML + 失败回退 Go 原渲染
2. **关键修复**：`stripHTML` 原实现未剥离 `<style>` 样式块，导致通知渠道（飞书/钉钉/企微）**文本摘要 1900 字节被 CSS 占满**——周报摘要异常/内容缺失的根因；Python/Go 两侧已同步修复（剥离 style/script 块后再生成纯文本摘要）
3. Python 冒烟测试 18 项全绿（含空数据、摘要剥离、1900 字节截断边界）

## 四、代码质量

simplify + code-review 全面检查并修复：

- 删除 `_fmt` 死代码；`render_node` 局部变量命名修正（避免遮蔽）
- 新增 `_num()` 数值兜底：Go 传入 JSON `null` 时防御性转 0，覆盖全部 6 个渲染 section
- pydantic v1/v2 兼容（`model_dump`/`dict`）
- scriptgen 侧确认无死代码/TODO 遗留

## 五、版本信息

| 组件 | 版本 |
|---|---|
| 主服务（Go） | v2.4.0 |
| Python 外围服务 | 1.2.0 |
| 支持平台 | Windows / Linux（麒麟、统信）amd64 + arm64 |

## 六、部署说明

- 云端部署仅替换 `netops-server` 二进制与 web 前端目录，保留 `config.yaml` 与 `data` 数据，不重置初始化状态
- Python 外围随 `netops-python` systemd 服务运行，部署 `pyservice/` 后重启该服务即可
- 本次提交：`88d5d21` → `0ba94ba` → `9c59b8e` → `eb3c01d` → `5b8036c` → `851304f`
