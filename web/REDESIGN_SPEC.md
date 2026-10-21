# NetOps Platform UI 重构规范（2026-10-20）

> 本次重构为**推倒重来的全站 UI 换血**：摒弃玻璃拟态（半透明 + backdrop blur + 光斑 + 粒子 + 渐变），
> 全面落地新设计语言 **「精密仪表 Precision Instrument」**。
> 本文件是所有页面组改造的**唯一硬约束**；以本文件 + 种子文件（见下）为准，两者冲突时以种子文件为准。

## 0. 必须阅读的种子文件（绝对路径，先读再动手）

- 设计令牌：`E:\IDEA\netops-platform\web\src\styles\variables.scss`（新 token 唯一来源）
- 全局组件层：`E:\IDEA\netops-platform\web\src\styles\index.scss`（已重写的 np-card / np-kpi / np-page-head / el-* 覆盖）
- 布局外壳示例：`E:\IDEA\netops-platform\web\src\layout\index.vue`
- 登录页示例：`E:\IDEA\netops-platform\web\src\views\login\index.vue` + `web\src\styles\login.scss`
- 旗舰页示例：`E:\IDEA\netops-platform\web\src\views\dashboard\index.vue`（含图表取色与主题联动重绘示范）

## 1. 设计方向

**精密仪表**：石墨墨色冷中性表面 + 单一信号橙强调色；发丝级 1px 描边；表格数字对齐（tabular-nums）；
微标签大写 + 字距拉开；菱形信号标（◆）作为标题装饰；暗色下深度靠表面亮度分层（无阴影）。

**硬性 DON'T（违反即返工）**：
- 任何 `backdrop-filter` / `blur()` / 半透明玻璃层 / 光斑 / 粒子（构建产物 grep 必须为 0）
- 任何 `linear-gradient` / `radial-gradient` 背景或文字渐变
- 纯黑 `#000` / 纯白 `#fff`
- 平铺 label:value 指标格；同尺寸等距图标卡片墙
- 指标数值本身着色（KPI 数值必须用 `--np-text-1` 墨色，颜色只出现在图标/信号标线/小点上）
- 硬编码颜色：页面内一律禁止 `#hex` / `rgba()`（只能出现在本规范与 variables.scss 内）

## 2. 令牌速查（全部在 variables.scss 定义，亮/暗双主题自动切换）

| 语义 | 变量 |
|---|---|
| 页面底色 | `--np-bg` |
| 卡片/侧栏/顶栏表面 | `--np-card-bg` / `--np-sidebar-bg` / `--np-header-bg` |
| 次级表面（表格行/凹陷） | `--np-surface-1`、`--np-surface-2`、`--np-table-bg`、`--np-hover` |
| 主/次/弱文字 | `--np-text-1` / `--np-text-2` / `--np-text-3` |
| 描边 | `--np-border`（常规）、`--np-border-strong`（悬浮/强调） |
| 强调色（信号橙） | `--np-primary`、`--np-primary-strong`（hover）、`--np-primary-weak`（弱底）、`--np-primary-fg`（橙上文字） |
| 语义色 | `--np-success` / `--np-warning` / `--np-danger` / `--np-info`，及 `-weak` 弱底 |
| 阴影 | `--np-shadow-sm`（卡片）、`--np-shadow-md`（浮层） |
| 数据大屏 | `--np-monitor-bg` / `--np-monitor-text`（已并入主题） |
| 图表轴/网格线 | `--np-chart-axis` / `--np-chart-split` / `--np-chart-axisline` |

## 3. 旧色 → 新变量映射（页面内硬编码色一律按此替换）

| 旧值 | 新变量 | 用途 |
|---|---|---|
| `#0ea5a4` `#2dd4bf` `#14b8a6`（青绿系强调） | `var(--np-primary)` | 强调/主色语境 |
| `#22c55e` `#10b981` | `var(--np-success)` | 成功/在线 |
| `#f59e0b` `#f97316` | `var(--np-warning)` | 警告 |
| `#ef4444` `#e11d48` | `var(--np-danger)` | 危险/离线 |
| `#8b5cf6` `#06b6d4` `#6366f1` `#3b82f6` `#0ea5e9`（数据系列/信息） | `var(--np-info)` 或图表系列色（见 §5） | 数据/信息 |
| `#64748b` `#5d7673` | `var(--np-text-2)` | 次级文字 |
| `#94a3b8` `#9db8b4` | `var(--np-text-3)` | 弱化文字/标签 |
| `#e2e8f0` `#dceceb`（暗色浅字） | `var(--np-text-1)` | 主文字 |
| `#2a4946` `#24403d` `#3a5a57` `#2c4f4c`（暗色网格） | `var(--np-chart-split)` / `var(--np-chart-axisline)` | 图表轴线 |
| `#0b1817` `#0c1a19` `#0d1f1e`（暗色底） | `var(--np-bg)` | 背景 |
| `#172d2b` `#1e3936` `#23413e`（暗色卡片） | `var(--np-bg-elevated)` / `var(--np-surface-1)` | 表面 |
| `rgba(45,212,191,x)` 等玻璃边 | `var(--np-border)` | 描边 |
| `rgba(12,26,25,x)` 等玻璃卡片底 | `var(--np-card-bg)` | 卡片底 |

## 4. 页面改造要求

1. **清理 scoped style**：删除页面内对 `np-card` / `np-kpi` / `np-page-head` / `np-toolbar` / `np-empty-box` / `np-dot` 等全局类的重复定义（全局已重写，页面内旧玻璃样式一律移除）；只保留页面特有布局类，并全部改为新语言。
2. **硬编码颜色清零**：模板内联 `style` 与 scoped style 中的 `#hex` / `rgba()` 全部替换为 CSS 变量；JS 内用于 DOM 的 `:style` 颜色改用 `cssVar()`（见 §5）。
3. **模板结构**：允许为适配新视觉微调类名/结构（如 KPI 卡改为 `--accent` 变量驱动的信号标线），但**禁止**增删功能、改接口、改路由、改 i18n 键、改组件交互逻辑。
4. **暗色可读性优先**：所有颜色必须走变量；不得出现亮色下看得见、暗色下看不见的文字（验收重点）；暗色下禁用阴影依赖（全局已处理，页面内不要再加亮色硬阴影）。
5. **动效**：只用 `transform` / `opacity`，时长 ≤ 250ms，缓动 `ease-out` 系；不新增粒子/光斑/滚动动画。
6. **响应式**：页面已有的响应式栅格（`repeat(auto-fill, minmax(...))` 等）保留；小屏不破版。
7. **i18n**：不改、不新增任何 i18n 文案（zh-CN / en-US 双份禁止触碰）。

## 5. 图表取色配方（页面内有 ECharts 的必须照做）

ECharts canvas 不解析 CSS 变量，统一通过工具读取：

```ts
import { cssVar, chartAxisColors } from '@/utils'   // 已实现：读取 --np-chart-* 与语义色
const ca = chartAxisColors()                          // { axis, split, axisLine }
// 语义系列色：
cssVar('--np-success') / cssVar('--np-warning') / cssVar('--np-danger') / cssVar('--np-info') / cssVar('--np-primary')
```

- 图例/坐标轴文字 → `ca.axis`；网格线 → `ca.split`；轴线 → `ca.axisLine`
- 系列区分色：优先保留后端下发的任务色（task.color 等，不动），缺失时用上述语义变量
- **主题联动重绘**：图表配置了颜色后，必须监听主题切换重绘：
  ```ts
  import { useAppStore } from '@/stores/app'
  const appStore = useAppStore()
  watch(() => appStore.theme, () => nextTick(() => renderXxx()))
  ```
  页面已有定时/WS 重绘的，可复用同一 render 函数。
- 页面内已有的 `chartAxisColors()` 调用（如 nginxmon）保持调用即可，新实现已切到新调色板。

## 6. 逐页自检清单（改造完成后逐项核对并在交付报告里写明结果）

对**你负责的每个文件**：
- [ ] 文件中已无 `backdrop-filter` / `blur(` / `np-glass` / `np-particles` / `np-particle` / 渐变字样
- [ ] 无 `#hex` / `rgba(` 残留（用 Grep 复查你的文件）
- [ ] 无纯黑/纯白硬编码
- [ ] 无玻璃时期的阴影/高光线残留（`inset 0 1px 0 ...` 高光线等）
- [ ] KPI 数值为墨色、未着色
- [ ] 页面不再重复定义全局 np-* 类的旧玻璃样式
- [ ] 暗色下文字层次成立（对照 §2 token：背景→text-1/2/3 对比度明确）
- [ ] 图表已走 §5 配方
- [ ] 功能/交互/接口/路由/i18n 未动

## 7. 范围边界

**只允许**：修改分配给你的页面/组件文件中的 `<template>`（仅视觉相关）与 `<style>` 块，以及必要时的最小 `<script>` 改动（仅 chartAxisColors/cssVar/useAppStore 引入与主题 watch）。

**禁止触碰**：
- `web/src/styles/*`（variables.scss / index.scss / login.scss）
- `web/src/layout/*`、`web/src/views/login/*`、`web/src/views/dashboard/*`
- `web/src/utils/index.ts`、`web/src/i18n/**`、`web/src/router/**`、`web/src/api/**`、`web/src/stores/**`
- `backend/**`、部署脚本、`web/dist`

## 8. 交付要求

报告：改动文件清单 + 每个文件的 §6 自检结果（逐项 ✅/❌）+ 无法消除的残留及原因 + 暗色可读性自查结论。
不需要运行构建（由主代理统一构建并回传错误）。
