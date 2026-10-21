// 通用工具
export interface ApiBody<T = unknown> {
  code: number
  message: string
  data: T
}

export const OK = 0

// 调色板：为链路/规则分配区分颜色（精密仪表系列色：低饱和、灰底可读）
export const PALETTE = [
  '#d95f1f', '#3a6fb0', '#4f9e63', '#b5893a', '#c0503d',
  '#7a63c9', '#2f8f83', '#a8648f', '#5f7a99', '#8a9a3a',
  '#c2733a', '#4f6fb8', '#9e5d3c', '#7f8f2f', '#9a6c4d'
]

export function pickColor(index: number): string {
  return PALETTE[((index % PALETTE.length) + PALETTE.length) % PALETTE.length]
}

// 字节 → 可读速率
export function fmtBps(bps: number): string {
  if (!bps || bps <= 0) return '0 bps'
  const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps']
  let v = bps
  let u = 0
  while (v >= 1000 && u < units.length - 1) {
    v /= 1000
    u++
  }
  return `${v.toFixed(u === 0 ? 0 : 2)} ${units[u]}`
}

// 字节 → 可读容量
export function fmtBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = bytes
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v.toFixed(u === 0 ? 0 : 2)} ${units[u]}`
}

// 秒 → 可读时长
export function fmtDuration(sec: number): string {
  if (!sec || sec <= 0) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}天${h}小时`
  if (h > 0) return `${h}小时${m}分`
  return `${m}分`
}

// 时间戳(ms) → 本地时间字符串
export function fmtTime(ts: number | string | null | undefined): string {
  if (ts == null || ts === '') return '-'
  const d = typeof ts === 'number' ? new Date(ts) : new Date(ts)
  if (Number.isNaN(d.getTime())) return String(ts)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// 校验密码强度：长度>12 且含大写/小写/数字/符号
export function validatePasswordStrength(pwd: string): string | null {
  if (pwd.length <= 12) return '密码长度必须大于12位'
  if (!/[a-z]/.test(pwd)) return '密码必须包含小写字母'
  if (!/[A-Z]/.test(pwd)) return '密码必须包含大写字母'
  if (!/[0-9]/.test(pwd)) return '密码必须包含数字'
  if (!/[^A-Za-z0-9]/.test(pwd)) return '密码必须包含符号'
  return null
}

export function downloadText(filename: string, content: string, mime = 'text/plain'): void {
  const blob = new Blob([content], { type: mime + ';charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// 读取 CSS 设计令牌（ECharts canvas 不解析 CSS 变量，需显式取色）
export function cssVar(name: string, fallback = ''): string {
  if (typeof document === 'undefined') return fallback
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

// 图表暗/亮适配色：按主题从设计令牌取轴/网格线颜色
export function chartAxisColors(): { axis: string; split: string; axisLine: string } {
  return {
    axis: cssVar('--np-chart-axis', '#6b7a96'),
    split: cssVar('--np-chart-split', 'rgba(107, 122, 150, 0.25)'),
    axisLine: cssVar('--np-chart-axisline', '#c9d2e0')
  }
}
