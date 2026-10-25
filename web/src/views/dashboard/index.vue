<script setup lang="ts">
// 智能运维可视化大屏（参考运维大屏布局）
// 模块：7 KPI / 厂商·类型·告警分级分布 / 近7天流量趋势 / 设备排行 / 主机资源 / 告警趋势 / 告警热力图 / 实时告警 / 变更动态 / 运维摘要
// hover 零消失硬保障（五重）：
//  1. emphasis.disabled —— hover 不触发任何图形状态变化，画布零重绘
//  2. tooltip.axisPointer.type='none' —— 不画画布轴指针
//  3. animation:false —— 数据更新不播放动画
//  4. hoverPaused —— 鼠标在图表内时暂停 setOption 重绘（数据照常拉取）
//  5. 签名守卫 —— 数据无变化不 setOption
import { ref, onMounted, onBeforeUnmount, nextTick, watch, computed } from 'vue'
import * as echarts from '@/utils/echarts'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc, putEnc, delEnc, postEnc, http } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAppStore } from '@/stores/app'
import { ElMessage } from 'element-plus'
import { fmtTime, cssVar, chartAxisColors } from '@/utils'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

// ---------- 数据类型 ----------
interface ScreenData {
  kpi: {
    device_total: number; device_online: number; device_offline: number
    dev_type: Record<string, number>
    alert_today: number; alert_total: number; alert_critical: number
    ip_used: number; ip_total: number
    if_up: number; if_total: number
    template_total: number
  }
  vendor_dist: Array<{ name: string; value: number }>
  type_dist: Array<{ name: string; value: number }>
  alert_level: Array<{ name: string; value: number }>
  host_res: { cpu: number; mem: number; disk: number }
  alert_trend: Array<{ date: string; count: number }>
  alert_heat: { devices: string[]; data: number[][] }
  traffic_trend: { x: string[]; in: number[]; out: number[] }
  device_rank: Array<{ device_id: number; name: string; ip: string; up: boolean; cpu: number; mem: number; disk: number }>
  change_feed: Array<{ user: string; action: string; time: string }>
  summary: {
    alert_active: number; alert_today: number; task_total: number; task_success: number
    dev_online: number; dev_total: number; ip_used: number; if_up: number; if_total: number
  }
}
interface AlertItem { id: number; level: string; target: string; message: string; count: number; created_at: string }
interface GeoPoint { device_id: number; name: string; x: number; y: number; up: boolean }

const data = ref<ScreenData | null>(null)
const alerts = ref<AlertItem[]>([])
const now = ref(Date.now())

// ---------- 设备地理图 ----------
const geo = ref<{ image: string; points: GeoPoint[] }>({ image: '', points: [] })
const manageOpen = ref(false)
const placingDevice = ref<number | undefined>()
const deviceOptions = computed(() =>
  (data.value?.device_rank ?? []).map((d) => ({ device_id: d.device_id, name: d.name || d.ip || 'device-' + d.device_id }))
)
async function loadGeoMap() {
  const r = await getEnc<{ image: string; points: GeoPoint[] }>('/dashboard/geomap')
  geo.value = r
}
function openManage() {
  manageOpen.value = true
  void loadGeoMap().catch(() => {})
}
async function uploadGeoImage(options: { file: File }) {
  const fd = new FormData()
  fd.append('file', options.file)
  await http.post('/dashboard/geomap/upload', fd, {
    headers: { 'Content-Type': 'multipart/form-data', 'X-No-Enc': '1' }
  })
  ElMessage.success(t('common.success'))
  await loadGeoMap()
}
const geoUrl = ref('')
async function importGeoUrl() {
  const url = geoUrl.value.trim()
  if (!url) return
  await postEnc('/dashboard/geomap/import-url', { url })
  ElMessage.success(t('common.success'))
  geoUrl.value = ''
  await loadGeoMap()
}
function onGeoCanvasClick(e: MouseEvent) {
  if (!placingDevice.value) return
  const el = e.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  const x = Math.min(100, Math.max(0, Math.round(((e.clientX - rect.left) / rect.width) * 10000) / 100))
  const y = Math.min(100, Math.max(0, Math.round(((e.clientY - rect.top) / rect.height) * 10000) / 100))
  void putEnc('/dashboard/geomap/points', [{ device_id: placingDevice.value, x, y }])
    .then(() => loadGeoMap())
    .catch(() => ElMessage.error(t('common.failed')))
}
function removeGeoPoint(deviceId: number) {
  void delEnc(`/dashboard/geomap/points/${deviceId}`)
    .then(() => loadGeoMap())
    .catch(() => ElMessage.error(t('common.failed')))
}

// ---------- 图表引用 ----------
const vendorRef = ref<HTMLDivElement>()
const typeRef = ref<HTMLDivElement>()
const trafficRef = ref<HTMLDivElement>()
const hostRef = ref<HTMLDivElement>()
const alertTrendRef = ref<HTMLDivElement>()
const heatRef = ref<HTMLDivElement>()
let vendorChart: echarts.ECharts | null = null
let typeChart: echarts.ECharts | null = null
let trafficChart: echarts.ECharts | null = null
let hostChart: echarts.ECharts | null = null
let alertTrendChart: echarts.ECharts | null = null
let heatChart: echarts.ECharts | null = null
let ws: WSClient | null = null
let timer: number | null = null
let wsTimer: number | null = null
let ro: ResizeObserver | null = null

// hover 保护：鼠标进入任一图表容器时暂停全部 setOption 重绘
let hoverPaused = false
function bindHoverPause(el: HTMLDivElement | undefined) {
  if (!el) return
  el.addEventListener('mouseenter', () => { hoverPaused = true })
  el.addEventListener('mouseleave', () => { hoverPaused = false })
}

// ---------- 数据加载 ----------
async function loadScreen() {
  data.value = await getEnc<ScreenData>('/dashboard/screen')
  now.value = Date.now()
  renderAll()
}
async function loadAlerts() {
  const r = await getEnc<{ total: number; list: AlertItem[] }>('/alert/events?page=1&size=6&acked=false')
  alerts.value = r.list
  now.value = Date.now()
}

// ---------- 图表通用配置（hover 零消失核心） ----------
function tooltipCfg(fmt?: (v: number) => string) {
  return {
    trigger: 'axis' as const,
    triggerOn: 'mousemove|click' as const,
    confine: true,
    axisPointer: { type: 'none' as const },
    position: ['right', 'top'] as const,
    backgroundColor: cssVar('--np-card-bg', '#ffffff'),
    borderColor: cssVar('--np-border', '#e3e5e8'),
    textStyle: { color: cssVar('--np-text-1', '#39424f'), fontSize: 12 },
    ...(fmt ? { valueFormatter: (v: unknown) => fmt(Number(v)) } : {})
  }
}
function seriesOpt(extra: Record<string, unknown>) {
  return { animation: false, emphasis: { disabled: true }, ...extra }
}
function itemTooltip(fmt?: (v: number) => string) {
  return {
    trigger: 'item' as const,
    triggerOn: 'mousemove|click' as const,
    confine: true,
    backgroundColor: cssVar('--np-card-bg', '#ffffff'),
    borderColor: cssVar('--np-border', '#e3e5e8'),
    textStyle: { color: cssVar('--np-text-1', '#39424f'), fontSize: 12 },
    ...(fmt ? { valueFormatter: (v: unknown) => fmt(Number(v)) } : {})
  }
}

// 速率自适应短格式
function fmtShortBps(v: number): string {
  if (!v || v <= 0) return '0'
  if (v >= 1e6) return (v / 1e6).toFixed(2) + 'M'
  if (v >= 1e3) return (v / 1e3).toFixed(1) + 'K'
  return v.toFixed(0)
}

// ---------- 厂商分布（环形） ----------
let lastVendorSig = ''
function renderVendor() {
  if (!vendorRef.value || !data.value) return
  if (hoverPaused) return
  vendorChart ??= echarts.init(vendorRef.value)
  const list = data.value.vendor_dist
  const sig = JSON.stringify(list)
  if (sig === lastVendorSig) return
  lastVendorSig = sig
  const ca = chartAxisColors()
  vendorChart.setOption({
    backgroundColor: 'transparent',
    tooltip: itemTooltip(),
    legend: { bottom: 0, textStyle: { color: ca.axis, fontSize: 11 }, itemWidth: 10, itemHeight: 10 },
    series: [{
      name: t('dashboard.vendorDist'),
      type: 'pie',
      radius: ['42%', '68%'],
      center: ['50%', '44%'],
      avoidLabelOverlap: true,
      label: { show: false },
      emphasis: { disabled: true },
      itemStyle: { borderRadius: 4, borderColor: 'transparent' },
      data: list.length ? list : [{ name: t('common.noData'), value: 0 }]
    }]
  })
}

// ---------- 类型分布（柱状） ----------
let lastTypeSig = ''
function renderType() {
  if (!typeRef.value || !data.value) return
  if (hoverPaused) return
  typeChart ??= echarts.init(typeRef.value)
  const list = data.value.type_dist
  const sig = JSON.stringify(list)
  if (sig === lastTypeSig) return
  lastTypeSig = sig
  const ca = chartAxisColors()
  typeChart.setOption({
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => `${v} 台`),
    legend: { show: false },
    grid: { left: 44, right: 12, top: 12, bottom: 26, containLabel: true },
    xAxis: { type: 'category', data: list.map((x) => x.name), axisLabel: { color: ca.axis, interval: 0 }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', minInterval: 1, axisLabel: { color: ca.axis }, splitLine: { lineStyle: { color: ca.split } } },
    series: [seriesOpt({
      type: 'bar',
      barWidth: 20,
      label: { show: true, position: 'top', color: ca.axis, fontSize: 11 },
      itemStyle: { color: cssVar('--np-primary', '#d95f1f'), borderRadius: [4, 4, 0, 0] },
      data: list.map((x) => x.value)
    })]
  })
}

// ---------- 近 7 天流量趋势（折线） ----------
let lastTrafficSig = ''
function renderTraffic() {
  if (!trafficRef.value || !data.value) return
  if (hoverPaused) return
  trafficChart ??= echarts.init(trafficRef.value)
  const tt = data.value.traffic_trend
  const sig = JSON.stringify(tt)
  if (sig === lastTrafficSig) return
  lastTrafficSig = sig
  const ca = chartAxisColors()
  trafficChart.setOption({
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => fmtShortBps(v)),
    legend: { top: 0, textStyle: { color: ca.axis }, itemWidth: 12, itemHeight: 8 },
    grid: { left: 56, right: 14, top: 30, bottom: 24 },
    xAxis: { type: 'category', data: tt.x, axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', axisLabel: { color: ca.axis, formatter: (v: number) => fmtShortBps(v) }, splitLine: { lineStyle: { color: ca.split } } },
    series: [
      seriesOpt({ name: t('traffic.in'), type: 'line', smooth: true, showSymbol: true, symbolSize: 5, lineStyle: { width: 2, color: cssVar('--np-info', '#3a6fb0') }, itemStyle: { color: cssVar('--np-info', '#3a6fb0') }, data: tt.in }),
      seriesOpt({ name: t('traffic.out'), type: 'line', smooth: true, showSymbol: true, symbolSize: 5, lineStyle: { width: 2, color: cssVar('--np-warning', '#b5893a') }, itemStyle: { color: cssVar('--np-warning', '#b5893a') }, data: tt.out })
    ]
  })
}

// ---------- 主机资源（3 环形） ----------
let lastHostSig = ''
function renderHost() {
  if (!hostRef.value || !data.value) return
  if (hoverPaused) return
  hostChart ??= echarts.init(hostRef.value)
  const h = data.value.host_res
  const sig = JSON.stringify(h)
  if (sig === lastHostSig) return
  lastHostSig = sig
  const ca = chartAxisColors()
  const mk = (centerX: string, name: string, val: number, color: string) => [{
    name,
    type: 'pie' as const,
    center: [centerX, '50%'] as [string, string],
    radius: ['58%', '76%'],
    startAngle: 90,
    silent: true,
    label: {
      show: true, position: 'center', formatter: `{d|${Math.round(val)}%}\n{t|${name}}`,
      rich: {
        d: { fontSize: 20, fontWeight: 800, color: cssVar('--np-text-1', '#39424f'), lineHeight: 24 },
        t: { fontSize: 11, color: cssVar('--np-text-3', '#8b93a3'), lineHeight: 16 }
      }
    },
    data: [
      { value: val, itemStyle: { color, borderRadius: 6 } },
      { value: Math.max(100 - val, 0), itemStyle: { color: 'transparent' } }
    ]
  }]
  hostChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { show: false },
    series: [
      ...mk('20%', t('dashboard.cpuUsage'), h.cpu, cssVar('--np-primary', '#d95f1f')),
      ...mk('50%', t('dashboard.memUsage'), h.mem, cssVar('--np-info', '#3a6fb0')),
      ...mk('80%', t('dashboard.diskUsage'), h.disk, cssVar('--np-success', '#3f9e63'))
    ]
  })
}

// ---------- 告警趋势（折线） ----------
let lastAlertTrendSig = ''
function renderAlertTrend() {
  if (!alertTrendRef.value || !data.value) return
  if (hoverPaused) return
  alertTrendChart ??= echarts.init(alertTrendRef.value)
  const list = data.value.alert_trend
  const sig = JSON.stringify(list)
  if (sig === lastAlertTrendSig) return
  lastAlertTrendSig = sig
  const ca = chartAxisColors()
  alertTrendChart.setOption({
    backgroundColor: 'transparent',
    tooltip: tooltipCfg((v) => `${v} 条`),
    legend: { show: false },
    grid: { left: 40, right: 12, top: 12, bottom: 24, containLabel: true },
    xAxis: { type: 'category', data: list.map((x) => x.date), axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', minInterval: 1, axisLabel: { color: ca.axis }, splitLine: { lineStyle: { color: ca.split } } },
    series: [seriesOpt({
      type: 'line', smooth: true, showSymbol: true, symbolSize: 5,
      lineStyle: { width: 2, color: cssVar('--np-danger', '#d64545') },
      itemStyle: { color: cssVar('--np-danger', '#d64545') },
      areaStyle: { color: 'transparent' },
      data: list.map((x) => x.count)
    })]
  })
}

// ---------- 告警热力图（设备 × 小时） ----------
let lastHeatSig = ''
function renderHeat() {
  if (!heatRef.value || !data.value) return
  if (hoverPaused) return
  heatChart ??= echarts.init(heatRef.value)
  const ht = data.value.alert_heat
  const sig = JSON.stringify(ht)
  if (sig === lastHeatSig) return
  lastHeatSig = sig
  const ca = chartAxisColors()
  const hours = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
  heatChart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item' as const,
      triggerOn: 'mousemove|click' as const,
      confine: true,
      backgroundColor: cssVar('--np-card-bg', '#ffffff'),
      borderColor: cssVar('--np-border', '#e3e5e8'),
      textStyle: { color: cssVar('--np-text-1', '#39424f'), fontSize: 12 },
      formatter: (p: { value: number[] }) => {
        const d = ht.devices[p.value[0]]
        return `${d || ''} ${p.value[1]}:00<br/>${t('dashboard.alertCount')}: ${p.value[2]}`
      }
    },
    grid: { left: 86, right: 14, top: 10, bottom: 24 },
    xAxis: { type: 'category', data: hours, axisLabel: { color: ca.axis, interval: 3 }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'category', data: ht.devices, axisLabel: { color: ca.axis, fontSize: 10 }, splitLine: { show: false } },
    visualMap: {
      min: 0, max: Math.max(1, ...ht.data.map((d) => d[2])),
      calculable: false, orient: 'horizontal', left: 'center', bottom: 0,
      itemWidth: 10, itemHeight: 8, textStyle: { color: ca.axis, fontSize: 10 },
      inRange: { color: ['rgba(58,111,176,0.15)', cssVar('--np-primary', '#d95f1f'), cssVar('--np-danger', '#d64545')] }
    },
    series: [seriesOpt({
      type: 'heatmap',
      data: ht.data,
      label: { show: false },
      emphasis: { disabled: true },
      itemStyle: { borderWidth: 1, borderColor: 'transparent' }
    })]
  })
}

// ---------- 派生展示值 ----------
function levelText(level: string): string {
  if (level === 'critical') return t('alert.critical')
  if (level === 'warning') return t('alert.warning')
  return t('alert.info')
}
function levelCount(level: string): number {
  return data.value?.alert_level.find((x) => x.name === level)?.value ?? 0
}
function alertLevelClass(level: string): string {
  return level === 'critical' ? 'critical' : level === 'warning' ? 'warning' : 'info'
}
function alertTime(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
function devTypeText(tp: string): string {
  if (tp === 'switch') return t('dashboard.devSwitch')
  if (tp === 'router') return t('dashboard.devRouter')
  if (tp === 'server') return t('dashboard.devServer')
  if (tp === 'firewall') return t('dashboard.devFirewall')
  return t('dashboard.devOther')
}
function ipRate(): string {
  const k = data.value?.kpi
  if (!k || !k.ip_total) return '--'
  return `${Math.round((k.ip_used / k.ip_total) * 100)}%`
}
function ifRate(): string {
  const k = data.value?.kpi
  if (!k || !k.if_total) return '--'
  return `${Math.round((k.if_up / k.if_total) * 100)}%`
}
function devRate(): string {
  const k = data.value?.kpi
  if (!k || !k.device_total) return '--'
  return `${Math.round((k.device_online / k.device_total) * 100)}%`
}
function taskRate(): string {
  const s = data.value?.summary
  if (!s || !s.task_total) return '--'
  return `${Math.round((s.task_success / s.task_total) * 100)}%`
}
function renderAll() {
  renderVendor(); renderType(); renderTraffic(); renderHost(); renderAlertTrend(); renderHeat()
}

// ---------- WS / 定时刷新 ----------
function wsReload() {
  if (wsTimer) return
  wsTimer = window.setTimeout(() => {
    wsTimer = null
    void loadScreen().catch(() => {})
    void loadAlerts().catch(() => {})
  }, 2000)
}

watch(() => appStore.theme, () => {
  nextTick(() => {
    for (const ch of [vendorChart, typeChart, trafficChart, hostChart, alertTrendChart, heatChart]) ch?.dispose()
    vendorChart = typeChart = trafficChart = hostChart = alertTrendChart = heatChart = null
    lastVendorSig = lastTypeSig = lastTrafficSig = lastHostSig = lastAlertTrendSig = lastHeatSig = ''
    renderAll()
  })
})

onMounted(async () => {
  await nextTick()
  for (const el of [vendorRef.value, typeRef.value, trafficRef.value, hostRef.value, alertTrendRef.value, heatRef.value]) {
    bindHoverPause(el)
  }
  try { await loadScreen() } catch (e) { console.error('screen', e) }
  try { await loadAlerts() } catch (e) { console.error('alerts', e) }
  // 图表容器稳定后初始化
  for (const refMap of [[vendorRef, vendorChart], [typeRef, typeChart], [trafficRef, trafficChart], [hostRef, hostChart], [alertTrendRef, alertTrendChart], [heatRef, heatChart]] as Array<[typeof vendorRef, echarts.ECharts | null]>) {
    if (refMap[0].value) {
      refMap[1]?.dispose()
      const ch = echarts.init(refMap[0].value)
      if (refMap[1] === vendorChart) vendorChart = ch
      else if (refMap[1] === typeChart) typeChart = ch
      else if (refMap[1] === trafficChart) trafficChart = ch
      else if (refMap[1] === hostChart) hostChart = ch
      else if (refMap[1] === alertTrendChart) alertTrendChart = ch
      else heatChart = ch
    }
  }
  renderAll()
  setTimeout(() => {
    vendorChart?.resize(); typeChart?.resize(); trafficChart?.resize()
    hostChart?.resize(); alertTrendChart?.resize(); heatChart?.resize()
  }, 100)
  window.addEventListener('resize', onResize)
  ro = new ResizeObserver(() => {
    vendorChart?.resize(); typeChart?.resize(); trafficChart?.resize()
    hostChart?.resize(); alertTrendChart?.resize(); heatChart?.resize()
  })
  for (const el of [vendorRef.value, typeRef.value, trafficRef.value, hostRef.value, alertTrendRef.value, heatRef.value]) {
    if (el) ro.observe(el)
  }
  try {
    ws = new WSClient(['dashboard', 'linkdetect', 'monitor', 'traffic'])
    ws.on('dashboard', 'overview', () => wsReload())
    ws.on('monitor', 'device_snapshot', () => wsReload())
    ws.on('traffic', 'if_rate', () => wsReload())
    ws.connect()
  } catch (e) { console.error('ws', e) }
  timer = window.setInterval(() => {
    void loadScreen().catch(() => {})
    void loadAlerts().catch(() => {})
    void loadGeoMap().catch(() => {})
  }, 30000)
})

function onResize() {
  vendorChart?.resize(); typeChart?.resize(); trafficChart?.resize()
  hostChart?.resize(); alertTrendChart?.resize(); heatChart?.resize()
}

onBeforeUnmount(() => {
  ws?.close()
  if (timer) window.clearInterval(timer)
  if (wsTimer) window.clearTimeout(wsTimer)
  window.removeEventListener('resize', onResize)
  ro?.disconnect()
  vendorChart?.dispose(); typeChart?.dispose(); trafficChart?.dispose()
  hostChart?.dispose(); alertTrendChart?.dispose(); heatChart?.dispose()
})
</script>

<template>
  <div class="np-screen">
    <!-- 顶部 -->
    <div class="np-topbar">
      <div class="np-brand">
        <span class="np-brand-mark"></span>
        <span class="np-brand-title">NETOPS {{ t('dashboard.entry') }}</span>
        <span class="np-brand-sub">Network Operations Center · {{ t('dashboard.realtimeStatus') }}</span>
      </div>
      <div class="np-top-right">
        <span class="np-clock">{{ fmtTime(now) }}</span>
        <span class="np-updated">{{ t('dashboard.updatedAt') }} {{ fmtTime(now) }}</span>
        <el-button size="small" @click="loadScreen()">{{ t('dashboard.refresh') }}</el-button>
        <el-button size="small" plain @click="router.push('/')">{{ t('dashboard.backToAdmin') }}</el-button>
      </div>
    </div>

    <!-- KPI 带 -->
    <div class="np-kpis" v-if="data">
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-dev"></span>{{ t('dashboard.deviceTotal') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big">{{ data.kpi.device_total }}</span><span class="np-kpi-unit">{{ t('dashboard.deviceUnit') }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.devSwitchRouter') }} {{ (data.kpi.dev_type['switch'] ?? 0) + (data.kpi.dev_type['router'] ?? 0) }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-rate"></span>{{ t('dashboard.onlineRate') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big">{{ devRate() }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.onlineCount') }} <b class="np-ok">{{ data.kpi.device_online }}</b> {{ t('dashboard.deviceUnit') }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-alert"></span>{{ t('dashboard.todayAlerts') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big" :class="{ 'np-bad': data.kpi.alert_today > 0 }">{{ data.kpi.alert_today }}</span><span class="np-kpi-unit">{{ t('dashboard.alertUnit') }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.totalAlerts') }} {{ data.kpi.alert_total }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-crit"></span>{{ t('dashboard.criticalAlerts') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big" :class="{ 'np-bad': data.kpi.alert_critical > 0 }">{{ data.kpi.alert_critical }}</span><span class="np-kpi-unit">{{ t('dashboard.alertUnit') }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.emergencyCount') }} {{ data.kpi.alert_critical }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-ip"></span>{{ t('dashboard.ipUsage') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big">{{ ipRate() }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.ipUsedCount') }} {{ data.kpi.ip_used }}/{{ data.kpi.ip_total }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-if"></span>{{ t('dashboard.interfaceRate') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big">{{ ifRate() }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.onlineCount') }} {{ data.kpi.if_up }}/{{ data.kpi.if_total }}</div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-head"><span class="np-kpi-ico np-ico-tpl"></span>{{ t('dashboard.templates') }}</div>
        <div class="np-kpi-main"><span class="np-kpi-big">{{ data.kpi.template_total }}</span><span class="np-kpi-unit">{{ t('dashboard.templateUnit') }}</span></div>
        <div class="np-kpi-sub">{{ t('dashboard.templateCount') }} {{ data.kpi.template_total }}</div>
      </div>
    </div>
    <div v-else class="np-empty">{{ t('common.loading') }}</div>

    <!-- 主体三列 -->
    <div class="np-body" v-if="data">
      <!-- 左列 -->
      <div class="np-col np-col-left">
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.vendorDist') }}</div>
          <div ref="vendorRef" class="np-chart np-chart-pie"></div>
        </div>
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.typeDist') }}</div>
          <div ref="typeRef" class="np-chart np-chart-bar"></div>
        </div>
        <div class="np-panel np-panel-alerts">
          <div class="np-panel-head">
            <span class="np-panel-tick"></span>{{ t('dashboard.alertCenter') }}
            <span class="np-panel-hint" :class="{ 'np-hint-bad': data.summary.alert_active > 0 }">{{ t('dashboard.activeAlerts') }} {{ data.summary.alert_active }} · {{ t('dashboard.todayAlerts') }} {{ data.summary.alert_today }}</span>
          </div>
          <div class="np-alert-stats">
            <span class="np-alert-stat critical"><i></i>{{ t('alert.critical') }} {{ levelCount('critical') }}</span>
            <span class="np-alert-stat warning"><i></i>{{ t('alert.warning') }} {{ levelCount('warning') }}</span>
            <span class="np-alert-stat info"><i></i>{{ t('alert.info') }} {{ levelCount('info') }}</span>
          </div>
          <div v-if="alerts.length" class="np-alert-list np-alert-list-col">
            <div v-for="a in alerts" :key="a.id" class="np-alert-row">
              <span class="np-alert-level" :class="alertLevelClass(a.level)">{{ levelText(a.level) }}</span>
              <span class="np-alert-target">{{ a.target }}</span>
              <span class="np-alert-msg">{{ a.message }}</span>
              <span v-if="a.count > 1" class="np-alert-count">×{{ a.count }}</span>
              <span class="np-alert-time">{{ alertTime(a.created_at) }}</span>
            </div>
          </div>
          <div v-else class="np-empty np-empty-flex">{{ t('dashboard.noAlerts') }}</div>
        </div>
      </div>

      <!-- 中列 -->
      <div class="np-col np-col-mid">
        <div class="np-panel np-panel-geo">
          <div class="np-panel-head">
            <span class="np-panel-tick"></span>{{ t('dashboard.geoMap') }}
            <span class="np-panel-hint">{{ t('dashboard.geoPlaced') }} {{ geo.points.length }}</span>
            <el-button class="np-geo-manage" size="small" @click="openManage">{{ t('dashboard.geoManage') }}</el-button>
          </div>
          <div class="np-geo">
            <img v-if="geo.image" :src="geo.image" class="np-geo-bg" alt="" />
            <div v-else class="np-geo-empty" @click="openManage">{{ t('dashboard.geoNoImage') }}</div>
            <div v-for="p in geo.points" :key="p.device_id" class="np-geo-point" :class="p.up ? 'up' : 'down'" :style="{ left: p.x + '%', top: p.y + '%' }">
              <span class="np-geo-dot"></span>
              <span class="np-geo-name">{{ p.name }}</span>
            </div>
          </div>
        </div>
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.trafficTrend') }}</div>
          <div ref="trafficRef" class="np-chart np-chart-trend"></div>
        </div>
      </div>

      <!-- 右列 -->
      <div class="np-col np-col-right">
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.hostRes') }}</div>
          <div ref="hostRef" class="np-chart np-chart-host"></div>
        </div>
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.alertTrend') }}</div>
          <div ref="alertTrendRef" class="np-chart np-chart-min"></div>
        </div>
        <div class="np-panel">
          <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.alertHeatmap') }}</div>
          <div ref="heatRef" class="np-chart np-chart-heat"></div>
        </div>
      </div>
    </div>

    <!-- 底部两栏 -->
    <div class="np-bottom" v-if="data">
      <div class="np-panel">
        <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.changeFeed') }}</div>
        <div v-if="data.change_feed.length" class="np-change-list">
          <div v-for="(c, i) in data.change_feed" :key="i" class="np-change-row">
            <span class="np-change-user">{{ c.user || '--' }}</span>
            <span class="np-change-action">{{ c.action }}</span>
            <span class="np-change-time">{{ c.time }}</span>
          </div>
        </div>
        <div v-else class="np-empty">{{ t('common.noData') }}</div>
      </div>

      <div class="np-panel">
        <div class="np-panel-head"><span class="np-panel-tick"></span>{{ t('dashboard.opsSummary') }}</div>
        <div class="np-summary-grid">
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.activeAlerts') }}</span>
            <span class="np-summary-val" :class="{ 'np-bad': data.summary.alert_active > 0 }">{{ data.summary.alert_active }}</span>
            <span class="np-summary-sub">{{ t('dashboard.alertUnit') }}</span>
          </div>
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.taskTotal') }}</span>
            <span class="np-summary-val">{{ data.summary.task_total }}</span>
            <span class="np-summary-sub">{{ t('dashboard.taskRate') }} {{ taskRate() }}</span>
          </div>
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.deviceOnline') }}</span>
            <span class="np-summary-val np-ok">{{ data.summary.dev_online }}/{{ data.summary.dev_total }}</span>
            <span class="np-summary-sub">{{ t('dashboard.onlineRate') }}</span>
          </div>
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.interfaceRate') }}</span>
            <span class="np-summary-val">{{ data.summary.if_up }}/{{ data.summary.if_total }}</span>
            <span class="np-summary-sub">{{ t('dashboard.onlineCount') }}</span>
          </div>
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.todayAlerts') }}</span>
            <span class="np-summary-val" :class="{ 'np-bad': data.summary.alert_today > 0 }">{{ data.summary.alert_today }}</span>
            <span class="np-summary-sub">{{ t('dashboard.alertUnit') }}</span>
          </div>
          <div class="np-summary-item">
            <span class="np-summary-label">{{ t('dashboard.ipUsed') }}</span>
            <span class="np-summary-val">{{ data.summary.ip_used }}</span>
            <span class="np-summary-sub">{{ t('dashboard.ipUsedCount') }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 设备地理图管理弹窗 -->
    <el-dialog v-model="manageOpen" :title="t('dashboard.geoManage')" width="780px" append-to-body>
      <div class="np-geo-editor">
        <div class="np-geo-toolbar">
          <el-upload :show-file-list="false" accept="image/*" :http-request="uploadGeoImage">
            <el-button size="small" type="primary">{{ t('dashboard.geoUpload') }}</el-button>
          </el-upload>
          <el-input v-model="geoUrl" size="small" :placeholder="t('dashboard.geoUrlPlaceholder')" clearable style="width: 260px" @keyup.enter="importGeoUrl" />
          <el-button size="small" :disabled="!geoUrl.trim()" @click="importGeoUrl">{{ t('dashboard.geoImportUrl') }}</el-button>
          <el-select v-model="placingDevice" size="small" :placeholder="t('dashboard.geoSelectDevice')" clearable style="width: 200px">
            <el-option v-for="d in deviceOptions" :key="d.device_id" :label="d.name" :value="d.device_id" />
          </el-select>
          <span class="np-geo-tip">{{ t('dashboard.geoClickPlace') }}</span>
        </div>
        <div class="np-geo-canvas" :class="{ 'np-geo-canvas-empty': !geo.image }" @click="onGeoCanvasClick">
          <img v-if="geo.image" :src="geo.image" class="np-geo-canvas-bg" alt="" />
          <div v-else class="np-geo-empty">{{ t('dashboard.geoNoImage') }}</div>
          <div v-for="p in geo.points" :key="p.device_id" class="np-geo-point" :class="p.up ? 'up' : 'down'" :style="{ left: p.x + '%', top: p.y + '%' }">
            <span class="np-geo-dot"></span>
            <span class="np-geo-name">{{ p.name }}</span>
          </div>
        </div>
        <div class="np-geo-list">
          <div v-for="p in geo.points" :key="p.device_id" class="np-geo-row">
            <span class="np-geo-row-name">{{ p.name }}</span>
            <span class="np-geo-row-pos">{{ p.x }}%, {{ p.y }}%</span>
            <span class="np-geo-row-status" :class="p.up ? 'up' : 'down'">{{ p.up ? t('dashboard.online') : t('dashboard.offline') }}</span>
            <el-button size="small" type="danger" link @click="removeGeoPoint(p.device_id)">{{ t('dashboard.geoRemove') }}</el-button>
          </div>
          <div v-if="!geo.points.length" class="np-empty">{{ t('common.noData') }}</div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
// ============ 智能运维可视化大屏 ============
.np-screen {
  padding: 16px 20px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: var(--np-bg);
  min-height: 100%;

  .np-topbar {
    display: flex; align-items: center; justify-content: space-between; gap: 12px;
    padding-bottom: 4px; border-bottom: 1px solid var(--np-border);
  }
  .np-brand { display: flex; align-items: center; gap: 10px; }
  .np-brand-mark {
    width: 12px; height: 12px; transform: rotate(45deg); border-radius: 2px;
    background: linear-gradient(135deg, var(--np-primary), color-mix(in oklch, var(--np-primary) 45%, var(--np-info)));
    box-shadow: 0 0 10px color-mix(in oklch, var(--np-primary) 50%, transparent);
  }
  .np-brand-title { font-size: 20px; font-weight: 800; letter-spacing: 0.02em; color: var(--np-text-1); }
  .np-brand-sub { font-size: 12px; color: var(--np-text-3); letter-spacing: 0.08em; }
  .np-top-right { display: flex; align-items: center; gap: 12px; }
  .np-clock { font-size: 14px; font-variant-numeric: tabular-nums; color: var(--np-text-1); font-weight: 600; }
  .np-updated { font-size: 12px; color: var(--np-text-3); font-variant-numeric: tabular-nums; }

  // KPI 带（7 卡）
  .np-kpis { display: grid; grid-template-columns: repeat(7, 1fr); gap: 12px; }
  .np-kpi {
    background: var(--np-card-bg); border: 1px solid var(--np-border); border-radius: $radius-lg;
    padding: 12px 14px; box-shadow: var(--np-shadow-sm); min-width: 0;
    transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out;
    &:hover { border-color: var(--np-border-strong); }
  }
  .np-kpi-head { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--np-text-2); letter-spacing: 0.04em; white-space: nowrap; }
  .np-kpi-ico { width: 14px; height: 14px; flex: none; border-radius: 4px; background: color-mix(in oklch, var(--np-primary) 12%, transparent); }
  .np-kpi-main { display: flex; align-items: baseline; gap: 5px; margin: 8px 0 4px; }
  .np-kpi-big { font-size: 28px; font-weight: 800; line-height: 1.1; font-variant-numeric: tabular-nums; color: var(--np-text-1); &.np-bad { color: var(--np-danger); } }
  .np-kpi-unit { font-size: 12px; color: var(--np-text-3); }
  .np-kpi-sub { font-size: 11px; color: var(--np-text-2); font-variant-numeric: tabular-nums; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    b.np-ok { color: var(--np-success); } }

  // 主体三列
  .np-body {
    display: grid;
    grid-template-columns: 1fr 1.9fr 1.15fr;
    gap: 12px;
    align-items: stretch;
  }
  .np-col { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .np-panel {
    background: var(--np-card-bg); border: 1px solid var(--np-border); border-radius: $radius-lg;
    padding: 14px 16px; box-shadow: var(--np-shadow-sm); min-width: 0;
    display: flex; flex-direction: column;
    transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out;
    &:hover { border-color: var(--np-border-strong); }
  }
  .np-panel-head {
    display: flex; align-items: center; gap: 8px;
    font-size: 14px; font-weight: 600; color: var(--np-text-1); margin-bottom: 10px;
    .np-panel-tick { width: 5px; height: 5px; transform: rotate(45deg); border-radius: 1px; background: var(--np-primary); }
    .np-panel-hint { margin-left: auto; font-size: 12px; font-weight: 400; color: var(--np-text-3); &.np-hint-bad { color: var(--np-danger); } }
  }
  .np-chart { width: 100%; flex: 1; min-height: 0; }
  .np-chart-pie { height: 180px; flex: none; }
  .np-chart-bar { height: 170px; flex: none; }
  .np-chart-trend { height: 200px; flex: none; }
  .np-chart-host { height: 150px; flex: none; }
  .np-chart-min { height: 140px; flex: none; }
  .np-chart-heat { height: 170px; flex: none; }

  // 设备地理图
  .np-panel-geo { flex: 2.2; }
  .np-geo-manage { margin-left: 8px; }
  .np-geo {
    position: relative; flex: 1; min-height: 0; overflow: hidden;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    img.np-geo-bg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; pointer-events: none; }
  }
  .np-geo-empty {
    position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
    color: var(--np-text-3); font-size: 13px; cursor: pointer;
    &:hover { color: var(--np-primary); }
  }
  .np-geo-point {
    position: absolute; transform: translate(-50%, -50%); display: flex; align-items: center; gap: 4px;
    z-index: 2; cursor: default; user-select: none;
    .np-geo-dot {
      width: 12px; height: 12px; border-radius: 50%; flex: none;
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--np-success) 22%, transparent), 0 0 8px color-mix(in oklch, var(--np-success) 55%, transparent);
    }
    &.down .np-geo-dot {
      background: var(--np-danger);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--np-danger) 22%, transparent), 0 0 8px color-mix(in oklch, var(--np-danger) 55%, transparent);
    }
    &.up .np-geo-dot { background: var(--np-success); }
    .np-geo-name {
      font-size: 11px; color: var(--np-text-1); max-width: 110px; @include ellipsis;
      padding: 1px 6px; border-radius: 999px; background: color-mix(in oklch, var(--np-card-bg) 88%, transparent);
    }
  }

  // 地理图管理弹窗
  .np-geo-editor { display: flex; flex-direction: column; gap: 10px; }
  .np-geo-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .np-geo-tip { font-size: 12px; color: var(--np-text-3); }
  .np-geo-canvas {
    position: relative; width: 100%; height: 380px; overflow: hidden; cursor: crosshair;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    img.np-geo-canvas-bg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; pointer-events: none; }
    &.np-geo-canvas-empty { cursor: default; }
  }
  .np-geo-list { display: flex; flex-direction: column; gap: 4px; max-height: 180px; overflow-y: auto; }
  .np-geo-row {
    display: flex; align-items: center; gap: 12px; padding: 5px 10px;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    font-size: 12px; color: var(--np-text-1);
    .np-geo-row-name { font-weight: 600; flex: 1; @include ellipsis; }
    .np-geo-row-pos { color: var(--np-text-2); font-variant-numeric: tabular-nums; }
    .np-geo-row-status { font-size: 11px; &.up { color: var(--np-success); } &.down { color: var(--np-danger); } }
  }

  // 最新告警（左列，带分级统计条）
  .np-panel-alerts { flex: 1; }
  .np-alert-stats { display: flex; gap: 6px; margin-bottom: 8px; }
  .np-alert-stat {
    display: flex; align-items: center; gap: 5px; flex: 1;
    font-size: 12px; color: var(--np-text-2); font-variant-numeric: tabular-nums;
    padding: 6px 8px; border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    i { width: 8px; height: 8px; border-radius: 2px; flex: none; }
    &.critical i { background: var(--np-danger); }
    &.warning i { background: var(--np-warning); }
    &.info i { background: var(--np-info); }
  }
  .np-alert-list { display: flex; flex-direction: column; gap: 5px; max-height: 168px; overflow-y: auto; }
  .np-alert-list-col { flex: 1; max-height: none; }
  .np-empty-flex { flex: 1; display: flex; align-items: center; justify-content: center; }

  // 底部两栏
  .np-bottom { display: grid; grid-template-columns: 1fr 1.2fr; gap: 12px; }
  .np-alert-row {
    display: flex; align-items: center; gap: 10px; padding: 7px 10px;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    font-size: 12px; color: var(--np-text-1);
    transition: background-color 0.18s ease-out, border-color 0.18s ease-out;
    &:hover { background: var(--np-hover); border-color: var(--np-border-strong); }
  }
  .np-alert-level { flex: none; padding: 1px 8px; border-radius: 999px; font-size: 11px; color: #fff;
    &.critical { background: var(--np-danger); } &.warning { background: var(--np-warning); } &.info { background: var(--np-info); } }
  .np-alert-target { flex: none; font-weight: 600; max-width: 160px; @include ellipsis; }
  .np-alert-msg { flex: 1; color: var(--np-text-2); @include ellipsis; }
  .np-alert-count { flex: none; color: var(--np-warning); font-weight: 700; }
  .np-alert-time { flex: none; color: var(--np-text-3); font-variant-numeric: tabular-nums; }

  .np-change-list { display: flex; flex-direction: column; gap: 5px; max-height: 168px; overflow-y: auto; }
  .np-change-row {
    display: flex; align-items: center; gap: 10px; padding: 7px 10px;
    border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
    font-size: 12px; color: var(--np-text-1);
    &:hover { background: var(--np-hover); }
    .np-change-user { flex: none; color: var(--np-info); font-weight: 600; }
    .np-change-action { flex: 1; color: var(--np-text-2); @include ellipsis; }
    .np-change-time { flex: none; color: var(--np-text-3); font-variant-numeric: tabular-nums; }
  }

  .np-summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; flex: 1; }
  .np-summary-item {
    display: flex; flex-direction: column; justify-content: center; gap: 2px;
    padding: 8px 10px; border-radius: $radius-sm; background: var(--np-surface-1); border: 1px solid var(--np-border);
  }
  .np-summary-label { font-size: 11px; color: var(--np-text-3); }
  .np-summary-val { font-size: 20px; font-weight: 800; font-variant-numeric: tabular-nums; color: var(--np-text-1); &.np-bad { color: var(--np-danger); } &.np-ok { color: var(--np-success); } }
  .np-summary-sub { font-size: 11px; color: var(--np-text-2); font-variant-numeric: tabular-nums; }

  .np-empty { color: var(--np-text-3); font-size: 13px; padding: 16px 0; text-align: center; }
}
</style>
