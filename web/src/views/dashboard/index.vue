<script setup lang="ts">
// 数据大屏：KPI + 链路趋势 + 设备健康 + 专线流量 + 数据库健康（右上角入口）
import { ref, onMounted, onBeforeUnmount, nextTick, computed } from 'vue'
import * as echarts from 'echarts'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAuthStore } from '@/stores/auth'
import { fmtBps, fmtTime } from '@/utils'
import NodeHealth from './components/NodeHealth.vue'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

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
  topo_devices: number
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

// 链路趋势本地缓冲（WebSocket 实时累积，与链路页面一致）
interface TrendSeries { name: string; color: string; points: Array<[number, number | null]> }
const trendBuffer: Record<number, TrendSeries & { taskId: number }> = {}

async function loadOverview() {
  const o = await getEnc<Overview>('/dashboard/overview')
  kpi.value = o.kpi
  links.value = o.links as unknown as Snapshot[]
  devices.value = o.devices
  dbs.value = o.dbs as unknown as Snapshot[]
  try { nodeHealth.value = await getEnc('/distributed/health') } catch {}
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
  trendChart ??= echarts.init(trendRef.value)
  const nowMs = Date.now()
  const startMs = nowMs - 10 * 60 * 1000
  const series = Object.values(trendBuffer).map((s) => ({
    name: s.name,
    type: 'line' as const,
    showSymbol: false,
    smooth: true,
    lineStyle: { width: 2, color: s.color || '#2f6bff' },
    itemStyle: { color: s.color || '#2f6bff' },
    areaStyle: { opacity: 0.15 },
    connectNulls: false,
    data: s.points
  }))
  trendChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + ' ms' },
    legend: { textStyle: { color: '#94a3b8' }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 24 },
    xAxis: { type: 'time', min: startMs, max: nowMs, axisLine: { lineStyle: { color: '#334155' } } },
    yAxis: { type: 'value', name: 'ms', min: 0, splitLine: { lineStyle: { color: '#1e293b' } } },
    series
  }, true)
}

function renderHealth() {
  if (!healthRef.value) return
  healthChart ??= echarts.init(healthRef.value)
  healthChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + '%' },
    legend: { textStyle: { color: '#94a3b8' }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 40 },
    xAxis: { type: 'category', data: devices.value.map((d) => d.name), axisLabel: { color: '#94a3b8', rotate: 30 } },
    yAxis: { type: 'value', max: 100, splitLine: { lineStyle: { color: '#1e293b' } } },
    series: [
      {
        name: 'CPU %',
        type: 'bar',
        barWidth: 12,
        itemStyle: { color: '#2f6bff', borderRadius: [4, 4, 0, 0] },
        data: devices.value.map((d) => d.cpu)
      },
      {
        name: 'MEM %',
        type: 'bar',
        barWidth: 12,
        itemStyle: { color: '#22c55e', borderRadius: [4, 4, 0, 0] },
        data: devices.value.map((d) => d.mem_used)
      }
    ]
  }, true)
}

function renderTraffic() {
  if (!trafficRef.value) return
  trafficChart ??= echarts.init(trafficRef.value)
  trafficChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v == null ? '--' : (v/1e6).toFixed(2) + ' Mbps' },
    legend: { textStyle: { color: '#94a3b8' }, top: 0 },
    grid: { left: 70, right: 16, top: 32, bottom: 24 },
    xAxis: { type: 'category', data: trafficData.value.map((x) => x.name), axisLabel: { color: '#94a3b8' } },
    yAxis: {
      type: 'value',
      name: 'Mbps',
      axisLabel: { color: '#94a3b8', formatter: (v: number) => (v / 1e6).toFixed(0) },
      splitLine: { lineStyle: { color: '#1e293b' } }
    },
    series: [
      {
        name: t('traffic.in'),
        type: 'bar',
        barWidth: 10,
        itemStyle: { color: '#06b6d4', borderRadius: [4,4,0,0] },
        data: trafficData.value.map((x) => x.in_bps)
      },
      {
        name: t('traffic.out'),
        type: 'bar',
        barWidth: 10,
        itemStyle: { color: '#f59e0b', borderRadius: [4,4,0,0] },
        data: trafficData.value.map((x) => x.out_bps)
      }
    ]
  }, true)
}

const trafficData = ref<Array<{ name: string; in_bps: number; out_bps: number }>>([])

async function loadTraffic() {
  const list = await getEnc<Array<{ name: string; in_bps: number; out_bps: number }>>('/dashboard/traffic')
  trafficData.value = list
  if (trafficChart) renderTraffic()
}

function renderDb() {
  if (!dbRef.value) return
  dbChart ??= echarts.init(dbRef.value)
  dbChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item' },
    legend: { textStyle: { color: '#94a3b8' }, top: 0 },
    grid: { left: 50, right: 16, top: 32, bottom: 40 },
    xAxis: { type: 'category', data: dbs.value.map((d) => d.name), axisLabel: { color: '#94a3b8', rotate: 20 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#1e293b' } } },
    series: [
      {
        name: t('dbmon.connections'),
        type: 'bar',
        itemStyle: { color: '#8b5cf6', borderRadius: [4, 4, 0, 0] },
        data: dbs.value.map((d) => Number((d as unknown as { conns?: number }).conns || 0))
      }
    ]
  }, true)
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

function renderGauge() {
  if (!gaugeRef.value) return
  gaugeChart ??= echarts.init(gaugeRef.value)
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
        axisLine: { lineStyle: { width: 10, color: [[0.6, '#22c55e'], [0.85, '#f59e0b'], [1, '#ef4444']] } },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        pointer: { show: true, length: '60%', width: 4 },
        detail: { valueAnimation: true, formatter: '{value}%', color: '#e2e8f0', fontSize: 22, offsetCenter: [0, '30%'] },
        title: { offsetCenter: [0, '60%'], color: '#94a3b8', fontSize: 12 },
        data: [{ value: avgCpu.value, name: 'CPU 平均' }]
      }
    ]
  }, true)
}

onMounted(async () => {
  await nextTick()
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
    ws.on('dashboard', 'overview', () => void loadOverview())
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
    ws.on('monitor', 'device_snapshot', () => void loadOverview())
    ws.on('traffic', 'if_rate', () => void loadTraffic())
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
    { label: t('dashboard.linkTotal'), value: k.link_total, sub: `${t('dashboard.linkUp')} ${k.link_up} · ${t('dashboard.linkDown')} ${k.link_down}`, color: '#2f6bff', go: '/linkdetect' },
    { label: t('dashboard.deviceTotal'), value: k.device_total, sub: `${t('dashboard.deviceOnline')} ${k.device_online} · ${t('dashboard.deviceOffline')} ${k.device_offline}`, color: '#22c55e', go: '/monitor' },
    { label: t('dashboard.dbTotal'), value: k.db_total, sub: `${t('dashboard.dbUp')} ${k.db_up} · ${t('dashboard.dbDown')} ${k.db_down}`, color: '#8b5cf6', go: '/dbmon' },
    { label: t('dashboard.userTotal'), value: k.user_total, sub: `${t('dashboard.ipUsed')} ${k.ip_used}`, color: '#f59e0b', go: '/ipam' }
  ]
}
</script>

<template>
  <div class="np-monitor-screen">
    <!-- 顶部栏：标题 + 返回按钮 -->
    <div class="np-screen-topbar">
      <div class="np-screen-title">
        <span class="np-title-dot"></span>
        {{ t('nav.dashboard') }}
      </div>
      <el-button type="primary" round size="small" @click="router.push('/linkdetect')">
        {{ t('dashboard.backToAdmin') || '返回管理界面' }}
      </el-button>
    </div>

    <!-- KPI 卡片 -->
    <div class="np-kpi-grid">
      <div v-for="c in kpiCards()" :key="c.label" class="np-kpi-card" :style="{ borderTop: `3px solid ${c.color}`, cursor: 'pointer' }" @click="c.go && router.push(c.go)">
        <div class="np-kpi-label">{{ c.label }}</div>
        <div class="np-kpi-value" :style="{ color: c.color }">{{ c.value }}</div>
        <div class="np-kpi-sub">{{ c.sub }}</div>
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
          实时状态
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
        <div class="np-screen-time">更新于 {{ fmtTime(now) }}</div>
      </div>
    </div>
  </div>
</template>

<style lang="scss">
.np-monitor-screen {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: var(--np-monitor-bg);
  min-height: 100%;

  .np-screen-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 4px;

    .np-screen-title {
      color: #e2e8f0;
      font-size: 20px;
      font-weight: 700;
      display: flex;
      align-items: center;
      gap: 10px;
    }
  }

  .np-title-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: #22c55e;
    box-shadow: 0 0 10px #22c55e;
    animation: pulse 2s infinite;
  }

  .np-kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 14px;
  }

  .np-kpi-card {
    background: linear-gradient(160deg, rgba(30, 41, 59, 0.9), rgba(15, 23, 42, 0.95));
    border: 1px solid #1e293b;
    border-radius: $radius-md;
    padding: 16px 20px;
    transition: transform 0.2s;

    &:hover { transform: translateY(-2px); }

    .np-kpi-label {
      color: #94a3b8;
      font-size: 13px;
    }
    .np-kpi-value {
      font-size: 34px;
      font-weight: 700;
      margin: 8px 0 4px;
      font-variant-numeric: tabular-nums;
    }
    .np-kpi-sub {
      color: #64748b;
      font-size: 12px;
    }
  }

  .np-screen-card {
    background: rgba(30, 41, 59, 0.9);
    border: 1px solid #1e293b;
    border-radius: $radius-md;
    padding: 14px 16px;

    .np-screen-title {
      color: #e2e8f0;
      font-size: 14px;
      font-weight: 600;
      margin-bottom: 8px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .np-screen-time {
      color: #64748b;
      font-size: 12px;
      text-align: right;
      margin-top: 6px;
    }
  }

  .np-live-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #22c55e;
    box-shadow: 0 0 8px #22c55e;
    animation: pulse 1.5s infinite;
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
    gap: 16px;
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
    padding: 4px 8px;
    border-radius: 4px;
    background: rgba(15, 23, 42, 0.4);
    .np-status-name {
      flex: 1;
      @include ellipsis;
    }
    .np-status-val {
      width: 70px;
      text-align: right;
      color: #22c55e;
      font-variant-numeric: tabular-nums;
    }
    .np-status-loss {
      width: 40px;
      text-align: right;
      color: #64748b;
      font-variant-numeric: tabular-nums;
      &.bad { color: #ef4444; }
    }
  }
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
</style>
