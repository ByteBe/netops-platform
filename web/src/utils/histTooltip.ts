// 历史图表公共 tooltip：时间规范格式化、值格式化（null→'--'、小数位/单位/自定义）、十字轴指针、防裁剪
// 接入所有带历史数据弹窗的页面，避免各页 formatter 行为分叉
import { cssVar } from '@/utils'

export interface HistTooltipOpts {
  digits?: number // 值小数位，默认 2
  suffix?: string // 值单位后缀，如 ' ms' / '%'
  fmt?: (v: number) => string // 自定义值格式化（优先级最高）
  position?: any // tooltip 位置：不传=固定右上角['right','top']（宽图防遮挡）；传 null=跟随鼠标（窄卡片图避免压住柱子/图例）
}

function fmtTs(axisValue: unknown, axisValueLabel?: string): string {
  if (axisValueLabel) return axisValueLabel
  if (axisValue == null) return ''
  const d = new Date(Number(axisValue))
  if (Number.isNaN(d.getTime())) return String(axisValue)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function histTooltip(opts: HistTooltipOpts = {}) {
  const digits = opts.digits ?? 2
  const suffix = opts.suffix ?? ''
  return {
    trigger: 'axis',
    confine: true,
    // 默认固定显示在图表右上角，不跟随鼠标：无论 hover 任何位置，曲线区域零遮挡
    // 传入 null 时跟随鼠标（窄卡片图右上角贴近柱子/图例的场景使用）
    position: opts.position === undefined ? ['right', 'top'] : opts.position,
    // 禁用 canvas 轴指针：hover 仅显示 DOM tooltip，不触发画布重绘（避免部分环境重绘导致曲线消失）
    axisPointer: { type: 'none' },
    // 半透明背景：即使 tooltip 覆盖到曲线区域，曲线仍透过可见，避免"曲线消失"
    backgroundColor: cssVar('--np-bg-raise', 'rgba(255,255,255,0.72)'),
    borderColor: cssVar('--np-border', '#d0d4da'),
    borderWidth: 1,
    padding: [8, 12],
    textStyle: { color: cssVar('--np-text', '#1a1d23'), fontSize: 12 },
    formatter: (params: any) => {
      const arr = Array.isArray(params) ? params : [params]
      if (!arr.length) return ''
      const time = fmtTs(arr[0].axisValue, arr[0].axisValueLabel)
      let html = `<div style="font-size:12px;line-height:1.6;margin-bottom:2px;color:${cssVar('--np-text-2', '#5a6070')}">${time}</div>`
      for (const p of arr) {
        const raw = Array.isArray(p.value) ? p.value[1] : p.value
        let v: string
        if (raw == null || !Number.isFinite(Number(raw))) {
          v = '--'
        } else if (opts.fmt) {
          v = opts.fmt(Number(raw))
        } else {
          v = Number(raw).toFixed(digits) + suffix
        }
        const color = typeof p.color === 'string' ? p.color : '#999'
        html += `<div style="display:flex;align-items:center;gap:8px;font-size:12px;line-height:1.7">
          <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color}"></span>
          <span>${p.seriesName}</span>
          <span style="margin-left:auto;padding-left:12px;font-variant-numeric:tabular-nums;font-weight:600">${v}</span>
        </div>`
      }
      return html
    }
  }
}
