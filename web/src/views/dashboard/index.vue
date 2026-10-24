<script setup lang="ts">
// 数据大屏 v3：链路检测(折线图) / 专线流量 / 服务器使用率 / 告警 四大模块
// hover 零消失硬保障（五重）：
//  1. emphasis.disabled —— hover 不触发任何图形状态变化，画布零重绘
//  2. tooltip.axisPointer.type='none' —— 不画画布轴指针
//  3. animation:false —— 数据更新不播放动画
//  4. hoverPaused —— 鼠标在图表内时暂停 setOption 重绘（数据照常拉取）
//  5. 签名守卫 —— 数据无变化不 setOption
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import * as echarts from 'echarts'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAppStore } from '@/stores/app'
import { fmtTime, cssVar, chartAxisColors } from '@/utils'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

// ---------- 数据类型 ----------
interface Kpi {
  link_total: number; link_up: number; link_down: number
  device_total: number; device_online: number; device_offline: number
  db_total: number; db_up: number; db_down: number
  user_total: number; ip_used: number
}
interface LinkSnap { name: string; up: boolean; rt_ms?: number; loss_pct?: number }
interface DeviceSnap { name: string; up: boolean; cpu: number; mem_used: number }
interface TrafficItem { name: string; color?: string; in_bps: number; out_bps: number; line_rate: number }
interface AlertItem {
  id: number; level: string; target: string; message: string
  count: number; acked: boolean; created_at: string
}

const kpi = ref<Kpi | null>(null)
const links = ref<LinkSnap[]>([])
const devices = ref<DeviceSnap[]>([])
const trafficData = ref<TrafficItem[]>([])
const alerts = ref<AlertItem[]>([])
const alertTotal = ref(0)
const now = ref(Date.now())
// 链路 chips 按名称稳定排序（接口顺序每次可能不同，避免跳动）
const sortedLinks = computed(() => [...links.value].sort((a, b) => a.name.localeCompare(b.name, 'zh')))

// ---------- 图表引用 ----------
const trendRef = ref<HTMLDivElement>()
const trafficRef = ref<HTMLDivElement>()
const srvRef = ref<HTMLDivElement>()
let trendChart: echarts.ECharts | null = null
let trafficChart: echarts.ECharts | null = null
let srvChart: echarts.ECharts | null = null
let ws: WSClient | null = null
let timer: number | null = null
let wsLoadTimer: number | null = null
let wsTrafficTimer: number | null = null

// hover 保护：鼠标进入任一图表容器时暂停全部 setOption 重绘
let hoverPaused = false
function bindHoverPause(el: HTMLDivElement | undefined) {
  if (!el) return
  el.addEventListener('mouseenter', () => { hoverPaused = true })
  el.addEventListener('mouseleave', () => { hoverPaused = false })
}

// 链路趋势缓冲（近 10 分钟）
interface TrendSeries { name: string; color: string; points: Array<[number, number | null]> }
const trendBuffer: Record<number, TrendSeries> = {}

// ---------- 数据加载 ----------
async function loadOverview() {
  const o = await getEnc<{ kpi: Kpi; links: LinkSnap[]; devices: DeviceSnap[] }>('/dashboard/overview')
  kpi.value = o.kpi
  links.value = o.links
  devices.value = o.devices
  now.value = Date.now()
}
async function loadTraffic() {
  const list = await getEnc<TrafficItem[]>('/dashboard/traffic')
  trafficData.value = list
}
async function loadAlerts() {
  const r = await getEnc<{ total: number; list: AlertItem[] }>('/alert/events?page=1&size=8&acked=false')
  alertTotal.value = r.total
  alerts.value = r.list
  now.value = Date.now()
}
async function seedTrends() {
  try {
    const o = await getEnc<{ series: Array<{ name: string; color: string; points: number[][] }> }>('/dashboard/link-trends')
    const taskList = await getEnc<Array<{ id: number; name: string; color: string }>>('/linkdetect/tasks')
    const idByName: Record<string, number> = {}
    for (const task of taskList) idByName[task.name] = task.id
    for (const s of o.series) {
      const tid = idByName[s.name] ?? 0
      trendBuffer[tid] = {
        name: s.name, color: s.color,
        points: s.points.map((p) => [p[0], p[1] > 0 ? p[1] : null] as [number, number | null])
      }
    }
  } catch (e) { console.error('seedTrends', e) }
}
function pushTrendPoint(taskId: number, name: string, color: string, rtMs: number | null) {
  const nowMs = Date.now()
  const cutoff = nowMs - 10 * 60 * 1000
  const buf = trendBuffer[taskId] ?? { name, color, points: [] as Array<[number, number | null]> }
  buf.name = name || buf.name
  buf.color = color || buf.color
  const hist = buf.points.filter((p) => p[0] > cutoff)
  const last = hist[hist.length - 1]
  if (!last || nowMs - last[0] > 2000) hist.push([nowMs, rtMs])
  else last[1] = rtMs
  buf.points = hist
  trendBuffer[taskId] = buf
}

// ---------- 图表通用配置（hover 零消失核心） ----------
function tooltipCfg(fmt?: (v: number) => string) {
  return {
    trigger: 'axis' as const,
    triggerOn: 'mousemove|click' as const,
    confine: true,
    // 核心：不画画布轴指针，hover 只显示 DOM tooltip，canvas 不重绘
    axisPointer: { type: 'none' as const },
    position: ['right', 'top'] as const,
    backgroundColor: cssVar('--np-card-bg', '#ffffff'),
    borderColor: cssVar('--np-border', '#e3e5e8'),
    textStyle: { color: cssVar('--np-text-1', '#39424f'), fontSize: 12 },
    ...(fmt ? { valueFormatter: (v: unknown) => fmt(Number(v)) } : {})
  }
}
// 每个系列统一附加：hover 不产生 emphasis 状态变化（不重绘图形）
function seriesOpt(extra: Record<string, unknown>) {
  return { animation: false, emphasis: { disabled: true }, ...extra }
}

// 速率自适应短格式
function fmtShortBps(v: number): string {
  if (!v || v <= 0) return '0'
  if (v >= 1e6) return (v / 1e6).toFixed(2) + 'M'
  if (v >= 1e3) return (v / 1e3).toFixed(1) + 'K'
  return v.toFixed(0)
}

// ---------- 链路检测折线图 ----------
let lastTrendCount = 0
function renderTrends() {
  if (!trendRef.value) return
  if (hoverPaused) return
  trendChart ??= echarts.init(trendRef.value)
  const ca = chartAxisColors()
  const nowMs = Date.now()
  const startMs = nowMs - 10 * 60 * 1000
  const series = Object.values(trendBuffer).map((s) => seriesOpt({
    name: s.name,
    type: 'line',
    showSymbol: false,
    smooth: true,
    lineStyle: { width: 2, color: s.color || cssVar('--np-primary', '#d95f1f') },
    itemStyle: { color: s.color || cssVar('--np-primary', '#d95f1f') },
    data: s.points
  }))
  const option = {
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => `${v.toFixed(1)} ms`),
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 48, right: 16, top: 34, bottom: 26 },
    xAxis: { type: 'time', min: startMs, max: nowMs, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', name: 'ms', min: 0, splitLine: { lineStyle: { color: ca.split } } },
    series
  }
  if (series.length !== lastTrendCount) {
    trendChart.setOption(option, true)
    lastTrendCount = series.length
  } else {
    trendChart.setOption(option)
  }
}

// ---------- 专线流量柱状图 ----------
let lastTrafficSig = ''
function renderTraffic() {
  if (!trafficRef.value) return
  if (hoverPaused) return
  trafficChart ??= echarts.init(trafficRef.value)
  const sig = JSON.stringify([trafficData.value.map((x) => x.name), trafficData.value.map((x) => x.in_bps), trafficData.value.map((x) => x.out_bps)])
  if (sig === lastTrafficSig) return
  lastTrafficSig = sig
  const ca = chartAxisColors()
  trafficChart.setOption({
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => fmtShortBps(v)),
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 64, right: 14, top: 30, bottom: 24 },
    xAxis: { type: 'category', data: trafficData.value.map((x) => x.name), axisLabel: { color: ca.axis, interval: 0, rotate: 0, margin: 8 } },
    yAxis: {
      type: 'value', name: 'bps', min: 0,
      axisLabel: { color: ca.axis, formatter: (v: number) => fmtShortBps(v) },
      splitLine: { lineStyle: { color: ca.split } }
    },
    series: [
      seriesOpt({
        name: t('traffic.in'),
        type: 'bar',
        barWidth: 14,
        label: { show: true, position: 'top', color: ca.axis, fontSize: 10, formatter: (p: { value: number }) => fmtShortBps(p.value) },
        itemStyle: { color: cssVar('--np-info', '#3a6fb0'), borderRadius: [3, 3, 0, 0] },
        data: trafficData.value.map((x) => x.in_bps)
      }),
      seriesOpt({
        name: t('traffic.out'),
        type: 'bar',
        barWidth: 14,
        label: { show: true, position: 'top', color: ca.axis, fontSize: 10, formatter: (p: { value: number }) => fmtShortBps(p.value) },
        itemStyle: { color: cssVar('--np-warning', '#b5893a'), borderRadius: [3, 3, 0, 0] },
        data: trafficData.value.map((x) => x.out_bps)
      })
    ]
  })
}

// ---------- 服务器使用率柱状图 ----------
let lastSrvSig = ''
function renderServer() {
  if (!srvRef.value) return
  if (hoverPaused) return
  srvChart ??= echarts.init(srvRef.value)
  const sig = JSON.stringify([devices.value.map((d) => d.name), devices.value.map((d) => d.cpu), devices.value.map((d) => d.mem_used)])
  if (sig === lastSrvSig) return
  lastSrvSig = sig
  const ca = chartAxisColors()
  srvChart.setOption({
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => `${v.toFixed(1)}%`),
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 48, right: 14, top: 30, bottom: 40 },
    xAxis: { type: 'category', data: devices.value.map((d) => d.name), axisLabel: { color: ca.axis, interval: 0, rotate: 0, margin: 8 } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: ca.axis, formatter: '{value}%' }, splitLine: { lineStyle: { color: ca.split } } },
    series: [
      seriesOpt({
        name: t('dashboard.cpuUsage'),
        type: 'bar',
        barWidth: 14,
        label: { show: true, position: 'top', color: ca.axis, fontSize: 10, formatter: '{c}%' },
        itemStyle: { color: cssVar('--np-primary', '#d95f1f'), borderRadius: [3, 3, 0, 0] },
        data: devices.value.map((d) => d.cpu)
      }),
      seriesOpt({
        name: t('dashboard.memUsage'),
        type: 'bar',
        barWidth: 14,
        label: { show: true, position: 'top', color: ca.axis, fontSize: 10, formatter: '{c}%' },
        itemStyle: { color: cssVar('--np-success', '#3f9e63'), borderRadius: [3, 3, 0, 0] },
        data: devices.value.map((d) => d.mem_used)
      })
    ]
  })
}

// ---------- 派生展示值 ----------
function linkRate(): string {
  const k = kpi.value
  if (!k || !k.link_total) return '--'
  return `${Math.round((k.link_up / k.link_total) * 100)}%`
}
function liveIn(): string { return trafficData.value.length ? fmtShortBps(trafficData.value[0].in_bps) : '--' }
function liveOut(): string { return trafficData.value.length ? fmtShortBps(trafficData.value[0].out_bps) : '--' }
function lineRateStr(): string {
  const r = trafficData.value[0]
  return r && r.line_rate ? `${r.line_rate} Mbps` : '--'
}
function alertLevelClass(level: string): string {
  return level === 'critical' ? 'critical' : level === 'warning' ? 'warning' : 'info'
}
function alertLevelText(level: string): string {
  if (level === 'critical') return t('alert.critical')
  if (level === 'warning') return t('alert.warning')
  return t('alert.info')
}
function alertTime(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

// ---------- WS / 定时刷新 ----------
function wsLoadOverview() {
  if (wsLoadTimer) return
  wsLoadTimer = window.setTimeout(() => {
    wsLoadTimer = null
    void loadOverview().then(() => renderServer()).catch(() => {})
  }, 2000)
}
function wsLoadTraffic() {
  if (wsTrafficTimer) return
  wsTrafficTimer = window.setTimeout(() => {
    wsTrafficTimer = null
    void loadTraffic().then(() => renderTraffic()).catch(() => {})
  }, 2000)
}

watch(() => appStore.theme, () => {
  nextTick(() => { renderTrends(); renderTraffic(); renderServer() })
})

onMounted(async () => {
  await nextTick()
  bindHoverPause(trendRef.value)
  bindHoverPause(trafficRef.value)
  bindHoverPause(srvRef.value)
  try { await loadOverview() } catch (e) { console.error('overview', e) }
  try { await loadTraffic() } catch (e) { console.error('traffic', e) }
  try { await loadAlerts() } catch (e) { console.error('alerts', e) }
  if (trafficRef.value) { trafficChart?.dispose(); trafficChart = echarts.init(trafficRef.value) }
  if (srvRef.value) { srvChart?.dispose(); srvChart = echarts.init(srvRef.value) }
  renderTraffic(); renderServer()
  try { await seedTrends() } catch (e) { console.error('seedTrends', e) }
  if (trendRef.value) { trendChart?.dispose(); trendChart = echarts.init(trendRef.value) }
  renderTrends()
  setTimeout(() => { trendChart?.resize(); trafficChart?.resize(); srvChart?.resize() }, 100)
  window.addEventListener('resize', onResize)
  try {
    ws = new WSClient(['dashboard', 'linkdetect', 'monitor', 'traffic'])
    ws.on('dashboard', 'overview', () => wsLoadOverview())
    ws.on('linkdetect', 'result', (msg: any) => {
      const s = msg?.data
      if (!s || !s.task_id) return
      pushTrendPoint(s.task_id, s.name, s.color, s.up ? (s.rt_ms || 0) : null)
      renderTrends()
    })
    ws.on('linkdetect', 'status_change', (msg: any) => {
      const s = msg?.data
      if (!s || !s.task_id) return
      pushTrendPoint(s.task_id, s.name, s.color, s.up ? (s.rt_ms || 0) : null)
      renderTrends()
    })
    ws.on('monitor', 'device_snapshot', () => wsLoadOverview())
    ws.on('traffic', 'if_rate', () => wsLoadTraffic())
    ws.connect()
  } catch (e) { console.error('ws', e) }
  timer = window.setInterval(() => {
    void loadOverview().then(() => renderServer()).catch(() => {})
    void loadTraffic().then(() => renderTraffic()).catch(() => {})
    void loadAlerts().catch(() => {})
  }, 10000)
})

function onResize() { trendChart?.resize(); trafficChart?.resize(); srvChart?.resize() }

onBeforeUnmount(() => {
  ws?.close()
  if (timer) window.clearInterval(timer)
  if (wsLoadTimer) window.clearTimeout(wsLoadTimer)
  if (wsTrafficTimer) window.clearTimeout(wsTrafficTimer)
  window.removeEventListener('resize', onResize)
  trendChart?.dispose()
  trafficChart?.dispose()
  srvChart?.dispose()
})
</script>

<template>
  <div class="np-screen">
    <!-- 顶部 -->
    <div class="np-topbar">
      <div class="np-brand">
        <span class="np-brand-mark"></span>
        <span class="np-brand-title">NETOPS {{ t('dashboard.entry') }}</span>
        <span class="np-brand-sub">{{ t('dashboard.realtimeStatus') }}</span>
      </div>
      <div class="np-top-right">
        <span class="np-clock">{{ fmtTime(now) }}</span>
        <el-button size="small" plain @click="router.push('/linkdetect')">{{ t('dashboard.backToAdmin') }}</el-button>
      </div>
    </div>

    <!-- KPI 带 -->
    <div class="np-kpis">
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-link"></span>{{ t('dashboard.linkDetect') }}</div>        <div class="np-kpi-main">
          <span class="np-kpi-big">{{ linkRate() }}</span>
          <span class="np-kpi-unit">{{ t('dashboard.linkOnlineRate') }}</span>
        </div>
        <div class="np-kpi-sub np-kpi-links">
          <span class="np-lk">{{ t('dashboard.linkUp') }} <b class="np-ok">{{ kpi?.link_up ?? '--' }}</b></span>
          <span class="np-lk">{{ t('dashboard.linkDown') }} <b :class="(kpi?.link_down ?? 0) > 0 ? 'np-bad' : 'np-ok'">{{ kpi?.link_down ?? '--' }}</b></span>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-traffic"></span>{{ t('dashboard.traffic') }}</div>
        <div class="np-kpi-main np-kpi-duo">
          <div class="np-duo"><span class="np-duo-tag up">▼ {{ t('traffic.in') }}</span><span class="np-duo-val">{{ liveIn() }}</span></div>
          <div class="np-duo"><span class="np-duo-tag down">▲ {{ t('traffic.out') }}</span><span class="np-duo-val">{{ liveOut() }}</span></div>
        </div>
        <div class="np-kpi-sub">{{ t('dashboard.lineBandwidth') }} {{ lineRateStr() }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-srv"></span>{{ t('dashboard.serverUsage') }}</div>
        <div class="np-kpi-main">
          <span class="np-kpi-big">{{ kpi?.device_online ?? '--' }}</span>
          <span class="np-kpi-unit">/ {{ kpi?.device_total ?? '--' }}</span>
        </div>
        <div class="np-kpi-sub">{{ t('dashboard.serverOnline') }} <b class="np-ok">{{ kpi?.device_online ?? '--' }}</b></div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-alert"></span>{{ t('dashboard.activeAlerts') }}</div>
        <div class="np-kpi-main">
          <span class="np-kpi-big" :class="{ 'np-bad': alertTotal > 0 }">{{ alertTotal }}</span>
        </div>
        <div class="np-kpi-sub">{{ t('dashboard.recentAlerts') }} · {{ alerts.length }} 条</div>
      </div>
    </div>

    <!-- 主体 -->
    <div class="np-body">
      <!-- 链路检测：折线图 -->
      <div class="np-panel np-panel-link">
        <div class="np-panel-head">
          <span class="np-panel-tick"></span>{{ t('dashboard.linkDetect') }}
          <span class="np-panel-rate">{{ t('dashboard.linkOnlineRate') }} {{ linkRate() }}</span>
          <span class="np-panel-hint">{{ t('dashboard.linkTrends') }} · 10 min</span>
        </div>
        <div class="np-link-chips">
          <div v-for="l in sortedLinks" :key="l.name" class="np-link-chip" :class="l.up ? 'up' : 'down'">
            <span class="np-link-dot"></span>
            <span class="np-link-name">{{ l.name }}</span>
            <span class="np-link-rt">{{ l.rt_ms != null ? `${Number(l.rt_ms).toFixed(1)}ms` : '--' }}</span>
            <span class="np-link-loss" :class="{ bad: Number(l.loss_pct) > 0 }">{{ l.loss_pct != null ? `${Number(l.loss_pct).toFixed(0)}%` : '' }}</span>
          </div>
          <div v-if="!links.length" class="np-empty">{{ t('common.noData') }}</div>
        </div>
        <div ref="trendRef" class="np-chart" style="height: 290px"></div>
      </div>

      <!-- 专线流量 -->
      <div class="np-panel np-panel-traffic">
        <div class="np-panel-head">
          <span class="np-panel-tick"></span>{{ t('dashboard.traffic') }}
          <span class="np-panel-hint">{{ t('dashboard.liveRate') }}</span>
        </div>
        <div ref="trafficRef" class="np-chart" style="height: 240px"></div>
      </div>

      <!-- 服务器使用率 -->
      <div class="np-panel np-panel-srv">
        <div class="np-panel-head">
          <span class="np-panel-tick"></span>{{ t('dashboard.serverUsage') }}
        </div>
        <div ref="srvRef" class="np-chart" style="height: 240px"></div>
      </div>

      <!-- 告警 -->
      <div class="np-panel np-panel-alert">
        <div class="np-panel-head">
          <span class="np-panel-tick"></span>{{ t('dashboard.recentAlerts') }}
          <span class="np-panel-hint" :class="{ 'np-hint-bad': alertTotal > 0 }">{{ t('dashboard.activeAlerts') }} {{ alertTotal }}</span>
        </div>
        <div v-if="alerts.length" class="np-alert-list">
          <div v-for="a in alerts" :key="a.id" class="np-alert-row">
            <span class="np-alert-level" :class="alertLevelClass(a.level)">{{ alertLevelText(a.level) }}</span>
            <span class="np-alert-target">{{ a.target }}</span>
            <span class="np-alert-msg">{{ a.message }}</span>
            <span v-if="a.count > 1" class="np-alert-count">×{{ a.count }}</span>
            <span class="np-alert-time">{{ alertTime(a.created_at) }}</span>
          </div>
        </div>
        <div v-else class="np-empty np-empty-alert">{{ t('dashboard.noAlerts') }}</div>
      </div>
    </div>
  </div>
</template>

<style lang="scss">
// ============ 数据大屏 v3 ============
.np-screen {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: var(--np-bg);
  min-height: 100%;

  .np-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 4px;
    border-bottom: 1px solid var(--np-border);
  }
  .np-brand { display: flex; align-items: center; gap: 10px; }
  .np-brand-mark {
    width: 12px; height: 12px; transform: rotate(45deg); border-radius: 2px;
    background: linear-gradient(135deg, var(--np-primary), color-mix(in oklch, var(--np-primary) 45%, var(--np-info)));
    box-shadow: 0 0 10px color-mix(in oklch, var(--np-primary) 50%, transparent);
  }
  .np-brand-title { font-size: 20px; font-weight: 800; letter-spacing: 0.02em; color: var(--np-text-1); }
  .np-brand-sub { font-size: 12px; color: var(--np-text-3); letter-spacing: 0.1em; }
  .np-top-right { display: flex; align-items: center; gap: 12px; }
  .np-clock { font-size: 14px; font-variant-numeric: tabular-nums; color: var(--np-text-2); }

  // KPI
  .np-kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
  .np-kpi {
    background: var(--np-card-bg);
    border: 1px solid var(--np-border);
    border-radius: $radius-lg;
    padding: 14px 16px 12px;
    box-shadow: var(--np-shadow-sm);
    transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out;
    &:hover { border-color: var(--np-border-strong); }
  }
  .np-kpi-head { display: flex; align-items: center; gap: 7px; font-size: 12px; color: var(--np-text-2); letter-spacing: 0.06em; }
  .np-kpi-ico { width: 16px; height: 16px; border-radius: 4px; background: color-mix(in oklch, var(--np-primary) 12%, transparent); }
  .np-kpi-main { display: flex; align-items: baseline; gap: 6px; margin: 6px 0 2px; }
  .np-kpi-big { font-size: 32px; font-weight: 800; line-height: 1.1; font-variant-numeric: tabular-nums; color: var(--np-text-1); &.np-bad { color: var(--np-danger); } }
  .np-kpi-unit { font-size: 13px; color: var(--np-text-3); }
  .np-kpi-duo { display: flex; gap: 20px; margin: 8px 0 2px; }
  .np-duo { display: flex; flex-direction: column; gap: 2px;
    .np-duo-tag { font-size: 11px; color: var(--np-text-3); &.up { color: var(--np-info); } &.down { color: var(--np-warning); } }
    .np-duo-val { font-size: 26px; font-weight: 800; font-variant-numeric: tabular-nums; color: var(--np-text-1); }
  }
  .np-kpi-sub { font-size: 12px; color: var(--np-text-2); font-variant-numeric: tabular-nums;
    b { color: var(--np-text-1); } b.np-ok { color: var(--np-success); } b.np-bad { color: var(--np-danger); } }
  .np-kpi-links { display: flex; justify-content: space-between; gap: 8px; white-space: nowrap; }
  .np-lk { white-space: nowrap; }
  .np-sep { margin: 0 4px; color: var(--np-border-strong); }

  // 主体网格
  .np-body {
    display: grid;
    grid-template-columns: 1.35fr 1fr;
    gap: 14px;
    .np-panel-link { grid-row: 1 / span 2; }
    .np-panel-alert { grid-row: 3; grid-column: 1 / -1; }
  }
  .np-panel {
    background: var(--np-card-bg);
    border: 1px solid var(--np-border);
    border-radius: $radius-lg;
    padding: 16px;
    box-shadow: var(--np-shadow-sm);
    min-width: 0;
    transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out;
    &:hover { border-color: var(--np-border-strong); }
  }
  .np-panel-head {
    display: flex; align-items: center; gap: 8px;
    font-size: 14px; font-weight: 600; color: var(--np-text-1); margin-bottom: 12px;
    .np-panel-tick { width: 5px; height: 5px; transform: rotate(45deg); border-radius: 1px; background: var(--np-primary); }
    .np-panel-rate { font-size: 12px; font-weight: 600; color: var(--np-success); font-variant-numeric: tabular-nums; }
    .np-panel-hint { margin-left: auto; font-size: 12px; font-weight: 400; color: var(--np-text-3); &.np-hint-bad { color: var(--np-danger); } }
  }
  .np-chart { width: 100%; }

  // 链路 chips
  .np-link-chips { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .np-link-chip {
    display: flex; align-items: center; gap: 7px; padding: 6px 10px;
    border-radius: $radius-md; font-size: 12px;
    border: 1px solid var(--np-border); background: var(--np-surface-1);
    transition: background-color 0.18s ease-out, border-color 0.18s ease-out;
    &:hover { background: var(--np-hover); border-color: var(--np-border-strong); }
    &.up { border-color: color-mix(in oklch, var(--np-success) 35%, var(--np-border)); }
    &.down { border-color: color-mix(in oklch, var(--np-danger) 45%, var(--np-border)); }
    .np-link-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--np-success); }
    &.down .np-link-dot { background: var(--np-danger); }
    .np-link-name { color: var(--np-text-1); font-weight: 600; }
    .np-link-rt { color: var(--np-success); font-variant-numeric: tabular-nums; }
    &.down .np-link-rt { color: var(--np-danger); }
    .np-link-loss { color: var(--np-text-3); font-variant-numeric: tabular-nums; &.bad { color: var(--np-danger); } }
  }

  // 告警
  .np-alert-list { display: flex; flex-direction: column; gap: 6px; max-height: 210px; overflow-y: auto; }
  .np-alert-row {
    display: flex; align-items: center; gap: 10px; padding: 8px 10px;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    font-size: 12px; color: var(--np-text-1);
    transition: background-color 0.18s ease-out, border-color 0.18s ease-out;
    &:hover { background: var(--np-hover); border-color: var(--np-border-strong); }
  }
  .np-alert-level { flex: none; padding: 1px 8px; border-radius: 999px; font-size: 11px; color: #fff;
    &.critical { background: var(--np-danger); } &.warning { background: var(--np-warning); } &.info { background: var(--np-info); } }
  .np-alert-target { flex: none; font-weight: 600; max-width: 180px; @include ellipsis; }
  .np-alert-msg { flex: 1; color: var(--np-text-2); @include ellipsis; }
  .np-alert-count { flex: none; color: var(--np-warning); font-weight: 700; }
  .np-alert-time { flex: none; color: var(--np-text-3); font-variant-numeric: tabular-nums; }
  .np-empty { color: var(--np-text-3); font-size: 13px; padding: 12px 0; text-align: center; }
  .np-empty-alert { padding: 24px 0; }
}
</style>
