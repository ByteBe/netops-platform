<script setup lang="ts">
// 流量监控：单接口实时带宽卡（参考图样式：In/Out 速率 + 480s 曲线 + 单位切换 + 自动缩放）
import { ref, reactive, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAppStore } from '@/stores/app'
import { pickColor, fmtBps, chartAxisColors } from '@/utils'
import { histTooltip } from '@/utils/histTooltip'

const appStore = useAppStore()
const { t } = useI18n()

// 参考图配色：蓝=出站(Out)，绿=入站(In)
const IN_COLOR = '#3dd68c'
const OUT_COLOR = '#3b9df0'
const WINDOW_SEC = 480 // 曲线窗口（秒）
const WINDOW_MS = WINDOW_SEC * 1000

type Unit = 'mbps' | 'bytes'
interface CurPoint {
  in: [number, number][]
  out: [number, number][]
}

interface TrafficRule {
  id: number
  device_id: number
  interface: string
  display_name: string
  line_rate: number
  if_speed: number
  up_rate: number
  down_rate: number
  color: string
  enable: boolean
  remark: string
  device_name?: string
  device_ip?: string
}
interface IfInfo {
  index: string
  name: string
  speed: number
  oper: string
}

const rules = ref<TrafficRule[]>([])
const devices = ref<Array<{ id: number; name: string; ip: string }>>([])
const currents = ref<Record<number, { up: boolean; in_bps: number; out_bps: number; in_pct: number; out_pct: number }>>({})
const curves = ref<Record<number, CurPoint>>({})
const units = ref<Record<number, Unit>>({})
const autoscale = ref<Record<number, boolean>>({})
const nowTs = ref(Date.now())

const dialogVisible = ref(false)
const editingId = ref(0)
interface RuleForm {
  id: number
  device_id?: number
  interface: string
  display_name: string
  line_rate: number
  if_speed: number
  up_rate: number
  down_rate: number
  color: string
  enable: boolean
  remark: string
}
const form = reactive<RuleForm>({
  id: 0, interface: '', display_name: '', line_rate: 1000,
  if_speed: 1000, up_rate: 1000, down_rate: 1000, color: '', enable: true, remark: ''
})

const ifs = ref<IfInfo[]>([])
const ifLoading = ref(false)

// 历史弹窗
const histVisible = ref(false)
const histRule = ref<TrafficRule | null>(null)
const histRef = ref<HTMLDivElement>()
const histHours = ref(24)
let histChart: echarts.ECharts | null = null

// 每卡迷你曲线
const chartMap = new Map<number, echarts.ECharts>()
const chartEls = ref<Record<number, HTMLDivElement | undefined>>({})
let renderTimer: number | null = null
let ws: WSClient | null = null
let clockTimer: any = null
let pollTimer: any = null

function fmtBytes(bps: number, digits = 2): string {
  const b = bps / 8
  if (b <= 0) return '0 B/s'
  const unitsArr = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let v = b
  let i = 0
  while (v >= 1000 && i < unitsArr.length - 1) { v /= 1000; i++ }
  return `${v.toFixed(digits)} ${unitsArr[i]}`
}

function fmtRate(bps: number, u: Unit): string {
  if (u === 'bytes') return fmtBytes(bps)
  if (bps <= 0) return '0.00 Mbps'
  if (bps < 1e4) return (bps / 1e3).toFixed(1) + ' Kbps'
  return (bps / 1e6).toFixed(2) + ' Mbps'
}

function fmtAxis(v: number, u: Unit): string {
  if (u === 'bytes') return fmtBytes(v, 0)
  if (v >= 1e6) return (v / 1e6).toFixed(v >= 10e6 ? 0 : 2) + 'M'
  if (v >= 1e3) return (v / 1e3).toFixed(0) + 'K'
  return v.toFixed(0)
}

// 历史图纵轴：按量级自适应单位，避免小流量时全显示 0M
function fmtAxisSmart(bps: number): string {
  if (bps >= 1e9) return (bps / 1e9).toFixed(1) + 'G'
  if (bps >= 1e6) return (bps / 1e6).toFixed(1) + 'M'
  if (bps >= 1e3) return (bps / 1e3).toFixed(1) + 'K'
  return bps.toFixed(0)
}

function fmtClock(ts: number): string {
  const d = new Date(ts)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}/${p(d.getDate())}/${d.getFullYear()} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function calcMax(inData: [number, number][], outData: [number, number][], as: boolean, lineRate: number): number {
  if (!as) return Math.max(1, lineRate) * 1e6
  let m = 1
  for (const p of inData) m = Math.max(m, p[1])
  for (const p of outData) m = Math.max(m, p[1])
  if (m <= 1) return 1e5 // 无数据：100 Kbps 空刻度
  // 数据驱动刻度（1/2/5 系列），留 25% 余量；下限 10 Kbps，小流量时曲线尽量铺满
  const nice = Math.pow(10, Math.floor(Math.log10(m)))
  const norm = m / nice
  let step = 10
  if (norm <= 1) step = 1
  else if (norm <= 2) step = 2
  else if (norm <= 5) step = 5
  return Math.max(1e4, step * nice * 1.25)
}

function mergeCurve(dst: CurPoint, src: CurPoint) {
  const merge = (a: [number, number][], b: [number, number][]) => {
    const m = new Map<number, number>()
    for (const p of a) m.set(p[0], p[1])
    for (const p of b) m.set(p[0], p[1])
    return Array.from(m.entries()).sort((x, y) => x[0] - y[0]) as [number, number][]
  }
  dst.in = merge(dst.in, src.in)
  dst.out = merge(dst.out, src.out)
}

function pushPoint(id: number, inBps: number, outBps: number, ts: number) {
  const c = curves.value[id]
  if (!c) return
  const cutoff = ts - WINDOW_MS
  const trim = (arr: [number, number][], v: number) => {
    const a = arr.filter((p) => p[0] > cutoff)
    const last = a[a.length - 1]
    if (last && ts - last[0] < 2000) last[1] = v
    else a.push([ts, v])
    return a
  }
  c.in = trim(c.in, inBps)
  c.out = trim(c.out, outBps)
}

function toggleUnit(id: number) {
  units.value[id] = units.value[id] === 'mbps' ? 'bytes' : 'mbps'
  renderRealtime()
}

function toggleScale(id: number) {
  autoscale.value[id] = autoscale.value[id] === false
  renderRealtime()
}

// 渲染节流：最多每秒一次
function renderRealtime() {
  if (renderTimer) return
  renderTimer = window.setTimeout(() => {
    renderTimer = null
    doRenderRealtime()
  }, 1000)
}

function doRenderRealtime() {
  ensureCharts()
  const now = Date.now()
  const cutoff = now - WINDOW_MS
  const ca = chartAxisColors()
  for (const r of rules.value) {
    if (!r.enable) continue
    const chart = chartMap.get(r.id)
    if (!chart || chart.isDisposed()) continue
    const c = curves.value[r.id]
    const u = units.value[r.id] || 'mbps'
    const as = autoscale.value[r.id] !== false
    const inData = (c?.in || []).filter((p) => p[0] >= cutoff)
    const outData = (c?.out || []).filter((p) => p[0] >= cutoff)
    chart.setOption({
      backgroundColor: 'transparent',
      grid: { left: 8, right: 46, top: 10, bottom: 8 },
      xAxis: { type: 'time', min: cutoff, max: now, axisLabel: { show: false }, axisLine: { show: false }, axisTick: { show: false }, splitLine: { show: false } },
      yAxis: {
        type: 'value', position: 'right', min: 0, max: calcMax(inData, outData, as, r.line_rate),
        splitNumber: 3,
        axisLabel: { color: ca.axis, fontSize: 10, formatter: (v: number) => fmtAxis(v, u) },
        splitLine: { lineStyle: { color: ca.split, type: 'dashed' } },
        axisLine: { show: false }, axisTick: { show: false }
      },
      series: [
        { type: 'line', showSymbol: false, smooth: true, animation: false, lineStyle: { width: 2, color: OUT_COLOR }, itemStyle: { color: OUT_COLOR }, data: outData },
        { type: 'line', showSymbol: false, smooth: true, animation: false, lineStyle: { width: 2, color: IN_COLOR }, itemStyle: { color: IN_COLOR }, data: inData }
      ]
    })
  }
}

// 幂等初始化卡片图表（避免 ref 每次渲染重建实例）
function ensureCharts() {
  for (const r of rules.value) {
    if (!r.enable) continue
    const el = chartEls.value[r.id]
    if (!el) continue
    if (!chartMap.has(r.id)) chartMap.set(r.id, echarts.init(el, null, { renderer: 'svg' }))
  }
}

function chartElRef(r: TrafficRule) {
  return (el: unknown) => {
    chartEls.value[r.id] = el instanceof HTMLDivElement ? el : undefined
  }
}

async function loadRules() {
  rules.value = await getEnc<TrafficRule[]>('/traffic/rules')
  for (const r of rules.value) {
    if (!r.enable) continue
    try {
      const cur = await getEnc<{ up: boolean; in_bps: number; out_bps: number; in_pct: number; out_pct: number }>(
        `/traffic/rules/${r.id}/current`
      )
      currents.value[r.id] = cur
    } catch {
      /* 忽略 */
    }
  }
}

// 拉取 480 秒曲线；force=重置，否则与本地合并（补齐 WS 断档）
async function loadCurves(force: boolean) {
  const list = rules.value.filter((r) => r.enable)
  await Promise.all(list.map(async (r) => {
    try {
      const h = await getEnc<{ in: number[][]; out: number[][] }>(`/traffic/rules/${r.id}/realtime?seconds=${WINDOW_SEC}`)
      const src: CurPoint = {
        in: h.in.map((p) => [p[0], p[1]] as [number, number]),
        out: h.out.map((p) => [p[0], p[1]] as [number, number])
      }
      if (force || !curves.value[r.id]) curves.value[r.id] = src
      else mergeCurve(curves.value[r.id], src)
    } catch {
      if (!curves.value[r.id]) curves.value[r.id] = { in: [], out: [] }
    }
  }))
  renderRealtime()
}

async function loadDevices() {
  devices.value = await getEnc<Array<{ id: number; name: string; ip: string }>>('/monitor/devices')
}

async function loadIfs() {
  if (!form.device_id) return
  ifLoading.value = true
  try {
    ifs.value = await getEnc<IfInfo[]>(`/traffic/devices/${form.device_id}/interfaces`)
  } finally {
    ifLoading.value = false
  }
}

function onInterfaceChange() {
  const sel = ifs.value.find(i => i.name === form.interface)
  if (sel && sel.speed > 0) {
    form.if_speed = sel.speed
  }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, {
    id: 0, device_id: undefined, interface: '', display_name: '', line_rate: 1000,
    if_speed: 1000, up_rate: 1000, down_rate: 1000, color: pickColor(rules.value.length), enable: true, remark: ''
  })
  dialogVisible.value = true
  ifs.value = []
}
function openEdit(r: TrafficRule) {
  editingId.value = r.id
  Object.assign(form, r)
  dialogVisible.value = true
  loadIfs()
}
async function save() {
  if (!form.device_id || !form.interface) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editingId.value) {
    await putEnc(`/traffic/rules/${editingId.value}`, form)
  } else {
    await postEnc('/traffic/rules', form)
  }
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await loadRules()
  await loadCurves(true)
}
async function remove(r: TrafficRule) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/traffic/rules/${r.id}`)
  delete curves.value[r.id]
  chartMap.get(r.id)?.dispose()
  chartMap.delete(r.id)
  ElMessage.success(t('common.success'))
  await loadRules()
}

async function showHistory(r: TrafficRule) {
  histRule.value = r
  histVisible.value = true
  histHours.value = 24
  await loadTrafficHistory()
}

async function loadTrafficHistory() {
  if (!histRule.value) return
  await nextTick()
  const h = await getEnc<{
    in: number[][]
    out: number[][]
  }>(`/traffic/rules/${histRule.value.id}/history?hours=${histHours.value}`)
  histChart ??= echarts.init(histRef.value as HTMLDivElement, null, { renderer: 'svg' })
  const ca = chartAxisColors()
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: histTooltip({ digits: 2, fmt: (v) => fmtBps(v) }),
    legend: { data: [t('traffic.in'), t('traffic.out')], textStyle: { color: ca.axis } },
    grid: { left: 80, right: 20, top: 36, bottom: 28 },
    xAxis: { type: 'time', axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', axisLabel: { color: ca.axis, formatter: (v: number) => fmtAxisSmart(v) }, splitLine: { lineStyle: { color: ca.split } } },
    series: [
      { name: t('traffic.in'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: IN_COLOR }, itemStyle: { color: IN_COLOR }, areaStyle: { opacity: 0.08 }, data: h.in },
      { name: t('traffic.out'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: OUT_COLOR }, itemStyle: { color: OUT_COLOR }, areaStyle: { opacity: 0.08 }, data: h.out }
    ]
  })
}

// 主题切换时重绘曲线
watch(() => appStore.theme, () => {
  nextTick(() => {
    doRenderRealtime()
    if (histVisible.value) void loadTrafficHistory()
  })
})

onMounted(async () => {
  nowTs.value = Date.now()
  clockTimer = setInterval(() => { nowTs.value = Date.now() }, 1000)
  await Promise.all([loadDevices(), loadRules()])
  await loadCurves(true)
  ws = new WSClient(['traffic'])
  ws.on('traffic', 'if_rate', (msg) => {
    const d = msg.data as unknown as { device_id: number; if_name: string; in_bps: number; out_bps: number }
    const r = rules.value.find((x) => x.device_id === d.device_id && (x.interface === d.if_name))
    if (r) {
      currents.value[r.id] = {
        up: true, in_bps: d.in_bps, out_bps: d.out_bps,
        in_pct: r.down_rate ? (d.in_bps / (r.down_rate * 1e6)) * 100 : 0,
        out_pct: r.up_rate ? (d.out_bps / (r.up_rate * 1e6)) * 100 : 0
      }
      pushPoint(r.id, d.in_bps, d.out_bps, Date.now())
      renderRealtime()
    }
  })
  ws.connect()
  pollTimer = setInterval(async () => { await loadDevices(); await loadRules(); await loadCurves(false) }, 30000)
})

onBeforeUnmount(() => {
  if (clockTimer) clearInterval(clockTimer)
  if (pollTimer) clearInterval(pollTimer)
  if (renderTimer) { clearTimeout(renderTimer); renderTimer = null }
  ws?.close()
  histChart?.dispose()
  for (const ch of chartMap.values()) ch.dispose()
  chartMap.clear()
})
</script>

<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('traffic.title') }}</div>
        <div class="np-page-head-desc">{{ t('traffic.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon>{{ t('traffic.addRule') }}
        </el-button>
      </div>
    </div>

    <!-- 实时带宽卡片（参考图样式） -->
    <div class="np-traffic-grid">
      <div v-for="r in rules" :key="r.id" class="np-card np-tf-card">
        <div class="np-tf-top">
          <div class="np-tf-rates">
            <div class="np-tf-rate is-in">
              <span class="np-tf-k">In</span>
              <span class="np-tf-v">{{ fmtRate(currents[r.id]?.in_bps || 0, units[r.id] || 'mbps') }}</span>
            </div>
            <div class="np-tf-rate is-out">
              <span class="np-tf-k">Out</span>
              <span class="np-tf-v">{{ fmtRate(currents[r.id]?.out_bps || 0, units[r.id] || 'mbps') }}</span>
            </div>
          </div>
          <div class="np-tf-mid">
            <div class="np-tf-ts">{{ fmtClock(nowTs) }}</div>
            <div class="np-tf-tools">
              <a class="np-tf-tool" @click="toggleUnit(r.id)">{{ units[r.id] === 'bytes' ? t('traffic.switchMbps') : t('traffic.switchBytes') }}</a>
              <a class="np-tf-tool" @click="toggleScale(r.id)">{{ autoscale[r.id] === false ? t('traffic.autoScaleFixed') : t('traffic.autoScaleFollow') }}</a>
            </div>
            <div class="np-tf-note">{{ t('traffic.graphSeconds', { n: WINDOW_SEC }) }}</div>
          </div>
          <div class="np-tf-title">
            <div class="np-tf-ifname">{{ r.interface }}</div>
            <div class="np-tf-devname">{{ r.device_name || r.display_name || t('traffic.unknownDevice') }}</div>
          </div>
        </div>
        <div class="np-tf-chart" :ref="chartElRef(r)"></div>
        <div class="np-tf-bottom">
          <div class="np-tf-ip">{{ r.device_ip || '-' }}</div>
          <div class="np-tf-ops">
            <el-button size="small" text type="primary" @click="showHistory(r)">{{ t('traffic.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(r)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(r)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>
      <div v-if="!rules.length" class="np-card tc-empty">
        <div class="np-empty-box">
          <div class="np-empty-icon"><el-icon><TrendCharts /></el-icon></div>
          <div class="np-empty-title">{{ t('traffic.title') }}</div>
          <div class="np-empty-desc">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('traffic.editRule') : t('traffic.addRule')" width="520px">
      <el-form :model="form" label-width="130px">
        <el-form-item :label="t('traffic.device')" required>
          <el-select v-model="form.device_id" filterable style="width: 100%" @change="loadIfs" :placeholder="t('common.selectPlaceholder')">
            <el-option v-for="d in devices" :key="d.id" :label="`${d.name} (${d.ip})`" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('traffic.interface')" required>
          <el-select v-model="form.interface" filterable :loading="ifLoading" style="width: 100%" @change="onInterfaceChange">
            <el-option v-for="i in ifs" :key="i.index" :label="`${i.name} (${i.oper})`" :value="i.name" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('traffic.ruleName')">
          <el-input v-model="form.display_name" />
        </el-form-item>
        <el-form-item :label="t('traffic.lineRate')">
          <el-input-number v-model="form.line_rate" :min="1" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('traffic.ifSpeed')">
          <el-input-number v-model="form.if_speed" :min="1" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('traffic.color')">
          <el-color-picker v-model="form.color" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="histVisible" :title="`${t('traffic.history')} - ${histRule?.display_name}`" width="780px">
      <div style="margin-bottom:8px">
        <el-select v-model="histHours" size="small" style="width:140px" @change="loadTrafficHistory">
          <el-option :label="$t('monitor.lastNHours', {n: 1})" :value="1" />
          <el-option :label="$t('monitor.lastNHours', {n: 6})" :value="6" />
          <el-option :label="$t('monitor.lastNHours', {n: 12})" :value="12" />
          <el-option :label="$t('monitor.lastNHours', {n: 24})" :value="24" />
          <el-option :label="$t('monitor.lastNHours', {n: 72})" :value="72" />
          <el-option :label="$t('monitor.lastNHours', {n: 168})" :value="168" />
        </el-select>
      </div>
      <div ref="histRef" class="np-chart" style="height: 320px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-traffic-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 14px;
}

.np-tf-card {
  display: flex;
  flex-direction: column;
  padding: 12px 14px;
  min-width: 0;

  .np-tf-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 10px;
  }

  .np-tf-rates {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: none;

    .np-tf-rate {
      display: flex;
      align-items: baseline;
      gap: 6px;

      .np-tf-k {
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.05em;
      }
      .np-tf-v {
        font-size: 17px;
        font-weight: 700;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
      }
      &.is-in .np-tf-k { color: #3dd68c; }
      &.is-out .np-tf-k { color: #3b9df0; }
    }
  }

  .np-tf-mid {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 0;

    .np-tf-ts {
      font-size: 12px;
      color: var(--np-text-3);
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }
    .np-tf-tools {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      justify-content: center;

      .np-tf-tool {
        font-size: 11px;
        color: var(--np-text-2);
        cursor: pointer;
        white-space: nowrap;
        &:hover { color: var(--np-primary); }
      }
    }
    .np-tf-note {
      font-size: 11px;
      color: var(--np-text-3);
    }
  }

  .np-tf-title {
    flex: none;
    text-align: right;
    min-width: 0;

    .np-tf-ifname {
      font-size: 14px;
      font-weight: 700;
      color: var(--np-text-1);
      word-break: break-all;
    }
    .np-tf-devname {
      font-size: 12px;
      color: var(--np-text-2);
      margin-top: 2px;
      word-break: break-all;
    }
  }

  .np-tf-chart {
    height: 150px;
    margin-top: 8px;
  }

  .np-tf-bottom {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid var(--np-border);
    margin-top: 8px;
    padding-top: 6px;

    .np-tf-ip {
      font-size: 12px;
      color: var(--np-text-2);
      font-variant-numeric: tabular-nums;
    }
    .np-tf-ops {
      display: flex;
    }
  }
}

.tc-empty {
  padding: 0;
}
</style>
