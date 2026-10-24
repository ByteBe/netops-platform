<script setup lang="ts">
// 数据大屏：KPI + 链路趋势 + 设备健康 + 专线流量 + 数据库健康（右上角入口）
import { ref, onMounted, onBeforeUnmount, nextTick, computed, watch } from 'vue'
import * as echarts from 'echarts'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { fmtBps, fmtTime, cssVar, chartAxisColors } from '@/utils'
import NodeHealth from './components/NodeHealth.vue'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const appStore = useAppStore()

interface Kpi {
  link_total: number
  link_up: number
  link_down: number
  device_total: number
  device_online: number
  device_offline: number
  db_total: number
  db_up: number
  db_down: number
  user_total: number
  ip_used: number
}
interface Snapshot {
  device_id?: number
  name: string
  ip: string
  up: boolean
  cpu: number
  mem_used: number
  rtt_ms?: number
  rt_ms?: number
  loss_pct?: number
}
interface Overview {
  kpi: Kpi
  links: Array<Record<string, unknown>>
  devices: Snapshot[]
  dbs: Array<Record<string, unknown>>
  containers: { docker: boolean; k8s: boolean }
}

const kpi = ref<Kpi | null>(null)
const nodeHealth = ref<{total:number;online:number;offline:number;device_total:number;container_total:number}|null>(null)
const deployMode = ref<'standalone'|'distributed'>('standalone')
const links = ref<Snapshot[]>([])
const devices = ref<Snapshot[]>([])
const dbs = ref<Snapshot[]>([])
const now = ref(Date.now())

const trendRef = ref<HTMLDivElement>()
const healthRef = ref<HTMLDivElement>()
const trafficRef = ref<HTMLDivElement>()
const dbRef = ref<HTMLDivElement>()
const gaugeRef = ref<HTMLDivElement>()

let trendChart: echarts.ECharts | null = null
let healthChart: echarts.ECharts | null = null
let trafficChart: echarts.ECharts | null = null
let dbChart: echarts.ECharts | null = null
let gaugeChart: echarts.ECharts | null = null
let ws: WSClient | null = null
let timer: number | null = null
// WS 高频事件节流：monitor device_snapshot / traffic if_rate 推送频繁，合并为 2 秒一次
let wsLoadTimer: number | null = null
let wsTrafficTimer: number | null = null
// 悬停暂停：鼠标进入任一图表容器时暂停全部重绘，彻底杜绝 hover 时柱子闪烁/消失
let hoverPaused = false
function bindHoverPause(el: HTMLDivElement | undefined) {
  if (!el) return
  el.addEventListener('mouseenter', () => { hoverPaused = true })
  el.addEventListener('mouseleave', () => { hoverPaused = false })
}
function wsLoadOverview() {
  if (wsLoadTimer) return
  wsLoadTimer = window.setTimeout(() => {
    wsLoadTimer = null
    void loadOverview().then(() => { renderHealth(); renderDb(); renderGauge() }).catch(() => {})
  }, 2000)
}
function wsLoadTraffic() {
  if (wsTrafficTimer) return
  wsTrafficTimer = window.setTimeout(() => {
    wsTrafficTimer = null
    void loadTraffic().then(() => renderTraffic()).catch(() => {})
  }, 2000)
}

// 链路趋势本地缓冲（WebSocket 实时累积，与链路页面一致）
interface TrendSeries { name: string; color: string; points: Array<[number, number | null]> }
const trendBuffer: Record<number, TrendSeries & { taskId: number }> = {}

async function loadOverview() {
  const o = await getEnc<Overview>('/dashboard/overview')
  kpi.value = o.kpi
  links.value = o.links as unknown as Snapshot[]
  devices.value = o.devices
  dbs.value = o.dbs as unknown as Snapshot[]
  try {
    const self = await getEnc('/distributed/self')
    deployMode.value = (self as any).deploy_mode || 'standalone'
    if (deployMode.value === 'distributed') {
      nodeHealth.value = await getEnc('/distributed/health')
    }
  } catch {}
  now.value = Date.now()
}

function renderCharts() {
  renderTrends()
  renderHealth()
  renderTraffic()
  renderDb()
  renderGauge()
}

async function seedTrends() {
  try {
    const o = await getEnc<{
      series: Array<{ name: string; color: string; points: number[][] }>
    }>('/dashboard/link-trends')
    const taskList = await getEnc<Array<{ id: number; name: string; color: string }>>('/linkdetect/tasks')
    const idByName: Record<string, number> = {}
    for (const t of taskList) idByName[t.name] = t.id
    for (const s of o.series) {
      const tid = idByName[s.name] || 0
      trendBuffer[tid] = {
        taskId: tid, name: s.name, color: s.color,
        points: s.points.map((p: number[]) => [p[0], p[1] > 0 ? p[1] : null] as [number, number | null])
      }
    }
  } catch (e) { console.error('seedTrends', e) }
}

function pushTrendPoint(taskId: number, name: string, color: string, rtMs: number | null) {
  const now = Date.now()
  const cutoff = now - 10 * 60 * 1000
  if (!trendBuffer[taskId]) {
    trendBuffer[taskId] = { taskId, name, color, points: [] }
  }
  const buf = trendBuffer[taskId]
  buf.color = color || buf.color
  buf.name = name || buf.name
  const hist = buf.points.filter((p) => p[0] > cutoff)
  const last = hist[hist.length - 1]
  if (!last || now - last[0] > 2000) hist.push([now, rtMs])
  else last[1] = rtMs
  buf.points = hist
}

async function renderTrends() {
  if (!trendRef.value) return
  if (hoverPaused) return // 悬停期间不重绘
  trendChart ??= echarts.init(trendRef.value)
  const ca = chartAxisColors()
  const nowMs = Date.now()
  const startMs = nowMs - 10 * 60 * 1000
  const series = Object.values(trendBuffer).map((s) => ({
    name: s.name,
    type: 'line' as const,
    showSymbol: false,
    smooth: true,
    animation: false, // 实时折线：关闭重绘动画，避免 hover 时曲线重放消失/闪烁
    lineStyle: { width: 2, color: s.color || cssVar('--np-primary', '#d95f1f') },
    itemStyle: { color: s.color || cssVar('--np-primary', '#d95f1f') },
    areaStyle: { opacity: 0.12 },
    connectNulls: false,
    data: s.points
  }))
  const option = {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true, valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + ' ms' },
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 24 },
    xAxis: { type: 'time', min: startMs, max: nowMs, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', name: 'ms', min: 0, splitLine: { lineStyle: { color: ca.split } } },
    series
  }
  // 系列数量变化时全量重建（新增/删除链路）；否则 merge 增量更新，避免重绘打断 tooltip
  if (series.length !== lastTrendSeriesCount) {
    trendChart.setOption(option, true)
    lastTrendSeriesCount = series.length
  } else {
    trendChart.setOption(option)
  }
}
let lastTrendSeriesCount = 0

let lastHealthSig = ''
function renderHealth() {
  if (!healthRef.value) return
  if (hoverPaused) return // 悬停期间不重绘
  healthChart ??= echarts.init(healthRef.value)
  // 数据无变化不重绘（WS/定时器高频触发时避免重绘动画打断 hover）
  const sig = JSON.stringify([devices.value.map((d) => d.name), devices.value.map((d) => d.cpu), devices.value.map((d) => d.mem_used)])
  if (sig === lastHealthSig) return
  lastHealthSig = sig
  const ca = chartAxisColors()
  healthChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true, valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + '%' },
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 40 },
    xAxis: { type: 'category', data: devices.value.map((d) => d.name), axisLabel: { color: ca.axis, rotate: 30 } },
    yAxis: { type: 'value', max: 100, splitLine: { lineStyle: { color: ca.split } } },
    series: [
      {
        name: 'CPU %',
        type: 'bar',
        barWidth: 12,
        animation: false,
        itemStyle: { color: cssVar('--np-primary', '#d95f1f'), borderRadius: [3, 3, 0, 0] },
        data: devices.value.map((d) => d.cpu)
      },
      {
        name: 'MEM %',
        type: 'bar',
        barWidth: 12,
        animation: false,
        itemStyle: { color: cssVar('--np-success', '#3f9e63'), borderRadius: [3, 3, 0, 0] },
        data: devices.value.map((d) => d.mem_used)
      }
    ]
  })
}

let lastTrafficSig = ''
function renderTraffic() {
  if (!trafficRef.value) return
  if (hoverPaused) return // 悬停期间不重绘
  trafficChart ??= echarts.init(trafficRef.value)
  // 数据无变化不重绘（WS if_rate 高频触发时避免重绘动画打断 hover）
  const sig = JSON.stringify([trafficData.value.map((x) => x.name), trafficData.value.map((x) => x.in_bps), trafficData.value.map((x) => x.out_bps)])
  if (sig === lastTrafficSig) return
  lastTrafficSig = sig
  const ca = chartAxisColors()
  trafficChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', confine: true, valueFormatter: (v: number) => v == null ? '--' : (v/1e6).toFixed(2) + ' Mbps' },
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 70, right: 16, top: 32, bottom: 24 },
    xAxis: { type: 'category', data: trafficData.value.map((x) => x.name), axisLabel: { color: ca.axis } },
    yAxis: {
      type: 'value',
      name: 'Mbps',
      // 小流量(<1Mbps)保留两位小数，避免所有刻度取整后显示为 0
      axisLabel: { color: ca.axis, formatter: (v: number) => (v / 1e6 >= 1 ? (v / 1e6).toFixed(0) : (v / 1e6).toFixed(2)) },
      splitLine: { lineStyle: { color: ca.split } }
    },
    series: [
      {
        name: t('traffic.in'),
        type: 'bar',
        barWidth: 10,
        animation: false,
        itemStyle: { color: cssVar('--np-info', '#3a6fb0'), borderRadius: [3,3,0,0] },
        data: trafficData.value.map((x) => x.in_bps)
      },
      {
        name: t('traffic.out'),
        type: 'bar',
        barWidth: 10,
        animation: false,
        itemStyle: { color: cssVar('--np-warning', '#b5893a'), borderRadius: [3,3,0,0] },
        data: trafficData.value.map((x) => x.out_bps)
      }
    ]
  })
}

const trafficData = ref<Array<{ name: string; in_bps: number; out_bps: number }>>([])

async function loadTraffic() {
  const list = await getEnc<Array<{ name: string; in_bps: number; out_bps: number }>>('/dashboard/traffic')
  trafficData.value = list
  if (trafficChart) renderTraffic()
}

let lastDbSig = ''
function renderDb() {
  if (!dbRef.value) return
  if (hoverPaused) return // 悬停期间不重绘
  dbChart ??= echarts.init(dbRef.value)
  // 数据无变化不重绘
  const sig = JSON.stringify([dbs.value.map((d) => d.name), dbs.value.map((d) => Number((d as unknown as { conns?: number }).conns || 0))])
  if (sig === lastDbSig) return
  lastDbSig = sig
  const ca = chartAxisColors()
  dbChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', confine: true },
    legend: { textStyle: { color: ca.axis }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 40 },
    xAxis: { type: 'category', data: dbs.value.map((d) => d.name), axisLabel: { color: ca.axis, rotate: 20 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: ca.split } } },
    series: [
      {
        name: t('dbmon.connections'),
        type: 'bar',
        animation: false,
        itemStyle: { color: cssVar('--np-info', '#3a6fb0'), borderRadius: [3, 3, 0, 0] },
        data: dbs.value.map((d) => Number((d as unknown as { conns?: number }).conns || 0))
      }
    ]
  })
}

// 仪表盘：平均CPU/内存
const avgCpu = computed(() => {
  if (!devices.value.length) return 0
  return Math.round(devices.value.reduce((s, d) => s + (d.cpu || 0), 0) / devices.value.length)
})
const avgMem = computed(() => {
  if (!devices.value.length) return 0
  return Math.round(devices.value.reduce((s, d) => s + (d.mem_used || 0), 0) / devices.value.length)
})

let lastGaugeSig = ''
function renderGauge() {
  if (!gaugeRef.value) return
  if (hoverPaused) return // 悬停期间不重绘
  gaugeChart ??= echarts.init(gaugeRef.value)
  // 数值无变化不重绘
  const sig = `${avgCpu.value}|${avgMem.value}`
  if (sig === lastGaugeSig) return
  lastGaugeSig = sig
  const ca = chartAxisColors()
  gaugeChart.setOption({
    backgroundColor: 'transparent',
    series: [
      {
        type: 'gauge',
        center: ['50%', '60%'],
        radius: '90%',
        min: 0, max: 100,
        startAngle: 200, endAngle: -20,
        progress: { show: true, width: 10 },
        axisLine: { lineStyle: { width: 10, color: [
          [0.6, cssVar('--np-success', '#3f9e63')],
          [0.85, cssVar('--np-warning', '#b5893a')],
          [1, cssVar('--np-danger', '#c0503d')]
        ] } },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        pointer: { show: true, length: '60%', width: 4 },
        detail: { valueAnimation: true, formatter: '{value}%', color: cssVar('--np-text-1', '#39424f'), fontSize: 22, offsetCenter: [0, '30%'] },
        title: { offsetCenter: [0, '60%'], color: ca.axis, fontSize: 12 },
        data: [{ value: avgCpu.value, name: t('dashboard.cpuAvg') }]
      }
    ]
  })
}

// 主题切换时重绘图表（ECharts canvas 不感知 CSS 变量变化）
watch(() => appStore.theme, () => {
  nextTick(() => renderCharts())
})

onMounted(async () => {
  await nextTick()
  // 悬停暂停：鼠标进入图表容器期间冻结刷新
  bindHoverPause(trendRef.value)
  bindHoverPause(healthRef.value)
  bindHoverPause(trafficRef.value)
  bindHoverPause(dbRef.value)
  bindHoverPause(gaugeRef.value)
  try { await loadOverview() } catch (e) { console.error('overview', e) }
  try { await loadTraffic() } catch (e) { console.error('traffic', e) }
  if (trendRef.value) { trendChart?.dispose(); trendChart = echarts.init(trendRef.value) }
  if (healthRef.value) { healthChart?.dispose(); healthChart = echarts.init(healthRef.value) }
  if (trafficRef.value) { trafficChart?.dispose(); trafficChart = echarts.init(trafficRef.value) }
  if (dbRef.value) { dbChart?.dispose(); dbChart = echarts.init(dbRef.value) }
  if (gaugeRef.value) { gaugeChart?.dispose(); gaugeChart = echarts.init(gaugeRef.value) }
  renderHealth(); renderDb(); renderTraffic(); renderGauge()
  try { await seedTrends() } catch (e) { console.error('seedTrends', e) }
  renderTrends()
  setTimeout(() => { trendChart?.resize(); healthChart?.resize(); trafficChart?.resize(); dbChart?.resize(); gaugeChart?.resize() }, 100)
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
    void loadOverview().then(() => { renderHealth(); renderDb(); renderGauge() }).catch(() => {})
    void loadTraffic().then(() => renderTraffic()).catch(() => {})
  }, 10000)
})

function onResize() { trendChart?.resize(); healthChart?.resize(); trafficChart?.resize(); dbChart?.resize(); gaugeChart?.resize() }

onBeforeUnmount(() => {
  ws?.close()
  if (timer) window.clearInterval(timer)
  if (wsLoadTimer) window.clearTimeout(wsLoadTimer)
  if (wsTrafficTimer) window.clearTimeout(wsTrafficTimer)
  window.removeEventListener('resize', onResize)
  trendChart?.dispose()
  healthChart?.dispose()
  trafficChart?.dispose()
  dbChart?.dispose()
  gaugeChart?.dispose()
})

function kpiCards() {
  const k = kpi.value
  if (!k) return []
  return [
    { label: t('dashboard.linkTotal'), value: k.link_total, sub: `${t('dashboard.linkUp')} ${k.link_up} · ${t('dashboard.linkDown')} ${k.link_down}`, color: cssVar('--np-primary', '#d95f1f'), icon: 'Link', go: '/linkdetect' },
    { label: t('dashboard.deviceTotal'), value: k.device_total, sub: `${t('dashboard.deviceOnline')} ${k.device_online} · ${t('dashboard.deviceOffline')} ${k.device_offline}`, color: cssVar('--np-success', '#3f9e63'), icon: 'Monitor', go: '/monitor' },
    { label: t('dashboard.dbTotal'), value: k.db_total, sub: `${t('dashboard.dbUp')} ${k.db_up} · ${t('dashboard.dbDown')} ${k.db_down}`, color: cssVar('--np-info', '#3a6fb0'), icon: 'Coin', go: '/dbmon' },
    { label: t('dashboard.userTotal'), value: k.user_total, sub: `${t('dashboard.ipUsed')} ${k.ip_used}`, color: cssVar('--np-warning', '#b5893a'), icon: 'User', go: '/ipam' }
  ]
}
</script>

<template>
  <div class="np-monitor-screen">
    <!-- 顶部栏：标题 + 返回按钮 -->
    <div class="np-screen-topbar">
      <div class="np-screen-title">
        <span class="np-title-tick"></span>
        {{ t('nav.dashboard') }}
      </div>
      <el-button type="primary" size="small" @click="router.push('/linkdetect')">
        {{ t('dashboard.backToAdmin') || '返回管理界面' }}
      </el-button>
    </div>

    <!-- KPI 卡片 -->
    <div class="np-kpi-grid">
      <div v-for="c in kpiCards()" :key="c.label" class="np-kpi-card" :style="{ '--accent': c.color }" @click="c.go && router.push(c.go)">
        <span class="np-kpi-icon">
          <el-icon><component :is="c.icon" /></el-icon>
        </span>
        <div class="np-kpi-card-body">
          <div class="np-kpi-value">{{ c.value }}</div>
          <div class="np-kpi-label">{{ c.label }}</div>
          <div class="np-kpi-sub">{{ c.sub }}</div>
        </div>
      </div>
    </div>

    <NodeHealth :data="nodeHealth" />

    <!-- 链路实时趋势 -->
    <div class="np-screen-card np-full">
      <div class="np-screen-title">{{ t('dashboard.linkTrends') }}（10分钟）</div>
      <div ref="trendRef" class="np-chart" style="height: 260px"></div>
    </div>

    <div class="np-grid-2">
      <!-- 设备健康 -->
      <div class="np-screen-card">
        <div class="np-screen-title">{{ t('dashboard.deviceHealth') }}</div>
        <div ref="healthRef" class="np-chart" style="height: 240px"></div>
      </div>
      <!-- 专线流量 -->
      <div class="np-screen-card">
        <div class="np-screen-title">{{ t('dashboard.traffic') }}</div>
        <div ref="trafficRef" class="np-chart" style="height: 240px"></div>
      </div>
    </div>

    <div class="np-grid-2">
      <!-- 数据库健康 -->
      <div class="np-screen-card">
        <div class="np-screen-title">{{ t('dashboard.dbHealth') }}</div>
        <div ref="dbRef" class="np-chart" style="height: 240px"></div>
      </div>
      <!-- 实时状态 -->
      <div class="np-screen-card np-realtime">
        <div class="np-screen-title">
          <span class="np-live-dot"></span>
          {{ t('dashboard.realtimeStatus') }}
        </div>
        <div class="np-realtime-body">
          <div class="np-gauge-wrap">
            <div ref="gaugeRef" style="width: 100%; height: 180px"></div>
          </div>
          <div class="np-status-rows">
            <div class="np-status-row" v-for="l in links.slice(0, 6)" :key="String(l.name)">
              <span class="np-dot" :class="l.up ? 'up' : 'down'"></span>
              <span class="np-status-name">{{ l.name }}</span>
              <span class="np-status-val">{{ l.rt_ms != null ? `${Number(l.rt_ms).toFixed(1)}ms` : '--' }}</span>
              <span class="np-status-loss" :class="{bad: Number(l.loss_pct) > 0}">{{ l.loss_pct != null ? `${Number(l.loss_pct).toFixed(0)}%` : '' }}</span>
            </div>
            <div v-if="!links.length" class="np-empty">{{ t('common.noData') }}</div>
          </div>
        </div>
        <div class="np-screen-time">{{ t('dashboard.updatedAt') }} {{ fmtTime(now) }}</div>
      </div>
    </div>
  </div>
</template>

<style lang="scss">
.np-monitor-screen {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: var(--np-monitor-bg);
  min-height: 100%;

  .np-screen-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 2px 2px 4px;

    .np-screen-title {
      color: var(--np-monitor-text);
      font-size: 20px;
      font-weight: 700;
      letter-spacing: -0.01em;
      display: flex;
      align-items: center;
      gap: 10px;
    }
  }

  .np-title-tick {
    width: 6px;
    height: 6px;
    transform: rotate(45deg);
    border-radius: 1px;
    background: var(--np-primary);
  }

  .np-kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 14px;
  }

  .np-kpi-card {
    position: relative;
    display: flex;
    align-items: center;
    gap: 14px;
    background: var(--np-card-bg);
    border: 1px solid var(--np-border);
    border-radius: $radius-lg;
    padding: 16px 18px;
    box-shadow: var(--np-shadow-sm);
    cursor: pointer;
    transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out, transform 0.2s ease-out;

    // 左侧信号标线（随指标状态色）
    &::before {
      content: '';
      position: absolute;
      left: -1px;
      top: 14px;
      bottom: 14px;
      width: 2px;
      border-radius: 2px;
      background: var(--accent, var(--np-primary));
    }

    &:hover {
      border-color: var(--np-border-strong);
      box-shadow: var(--np-shadow-md);
      transform: translateY(-1px);
    }

    .np-kpi-icon {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 40px;
      height: 40px;
      border-radius: $radius-md;
      font-size: 20px;
      color: var(--accent, var(--np-primary));
      background: color-mix(in oklch, var(--accent, var(--np-primary)) 12%, transparent);
    }

    .np-kpi-card-body {
      flex: 1;
      min-width: 0;
    }

    .np-kpi-value {
      font-size: 28px;
      font-weight: 700;
      line-height: 1.2;
      letter-spacing: -0.01em;
      margin: 2px 0 4px;
      font-variant-numeric: tabular-nums;
      color: var(--np-monitor-text);
    }
    .np-kpi-label {
      color: var(--np-text-3);
      font-size: 11px;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    .np-kpi-sub {
      margin-top: 6px;
      color: var(--np-text-2);
      font-size: 12px;
      font-variant-numeric: tabular-nums;
    }
  }

  .np-screen-card {
    background: var(--np-card-bg);
    border: 1px solid var(--np-border);
    border-radius: $radius-lg;
    padding: 16px 18px;
    box-shadow: var(--np-shadow-sm);

    .np-screen-title {
      color: var(--np-monitor-text);
      font-size: 14px;
      font-weight: 600;
      margin-bottom: 10px;
      display: flex;
      align-items: center;
      gap: 8px;
      &::before {
        content: '';
        width: 5px;
        height: 5px;
        transform: rotate(45deg);
        border-radius: 1px;
        background: var(--np-primary);
      }
    }
    .np-screen-time {
      color: var(--np-text-3);
      font-size: 12px;
      text-align: right;
      margin-top: 8px;
      font-variant-numeric: tabular-nums;
    }
  }

  .np-chart {
    width: 100%;
  }

  .np-live-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--np-success);
    animation: np-pulse 1.5s infinite;
  }

  .np-grid-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
  }

  .np-full {
    grid-column: 1 / -1;
  }

  .np-realtime-body {
    display: flex;
    gap: 18px;
    align-items: center;
  }

  .np-gauge-wrap {
    flex: 0 0 45%;
  }

  .np-status-rows {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 200px;
    overflow-y: auto;
  }
  .np-status-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--np-monitor-text);
    padding: 7px 10px;
    border-radius: $radius-md;
    background: var(--np-surface-1);
    border: 1px solid var(--np-border);
    transition: border-color 0.18s ease-out, background-color 0.18s ease-out;
    &:hover {
      background: var(--np-hover);
    }
    .np-status-name {
      flex: 1;
      @include ellipsis;
    }
    .np-status-val {
      width: 76px;
      text-align: right;
      color: var(--np-success);
      font-variant-numeric: tabular-nums;
    }
    .np-status-loss {
      width: 44px;
      text-align: right;
      color: var(--np-text-3);
      font-variant-numeric: tabular-nums;
      &.bad { color: var(--np-danger); }
    }
  }
}

@keyframes np-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
</style>
