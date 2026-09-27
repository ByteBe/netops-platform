<script lang="ts">
const rtHistoryGlobal: Record<number, Array<[number, number | null]>> = {}
</script>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { pickColor, fmtTime } from '@/utils'

const { t } = useI18n()

interface LinkTask {
  id: number; name: string; target: string; method: string; port: number
  interval: number; timeout: number; color: string; enabled: boolean; remark: string
}
interface LinkStatus {
  task_id: number; name: string; target: string; up: boolean
  rt_ms: number; loss_pct: number; ts: number; message: string; color: string
}

const tasks = ref<LinkTask[]>([])
const statuses = ref<Record<number, LinkStatus>>({})
const loading = ref(false)
const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive<LinkTask>({
  id: 0, name: '', target: '', method: 'cmd', port: 0,
  interval: 30, timeout: 3000, color: '', enabled: true, remark: ''
})
const historyVisible = ref(false)
const historyTask = ref<LinkTask | null>(null)
const historyData = ref<{ name: string; color: string; points: [number, number | null][] } | null>(null)
const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let ws: WSClient | null = null

async function loadTasks() {
  loading.value = true
  try {
    const list = await getEnc<Array<LinkTask & { status: boolean; rt: number; loss: number; message: string; last_ts: number }>>('/linkdetect/tasks')
    tasks.value = list
    const st: Record<number, LinkStatus> = {}
    for (const tt of list) {
      st[tt.id] = {
        task_id: tt.id, name: tt.name, target: tt.target,
        up: !!tt.status, rt_ms: tt.rt || 0, loss_pct: tt.loss || 0,
        ts: tt.last_ts || 0, message: tt.message || '', color: tt.color
      }
    }
    statuses.value = st
  } finally { loading.value = false }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, { id: 0, name: '', target: '', method: 'cmd', port: 0, interval: 30, timeout: 3000, color: pickColor(tasks.value.length), enabled: true, remark: '' })
  dialogVisible.value = true
}
function openEdit(row: LinkTask) { editingId.value = row.id; Object.assign(form, row); dialogVisible.value = true }

async function saveTask() {
  if (!form.name || !form.target) { ElMessage.warning(t('common.tip')); return }
  const payload = { name: form.name, target: form.target, method: form.method, port: form.port, interval: form.interval, timeout: form.timeout, color: form.color, enabled: form.enabled }
  if (editingId.value) await putEnc(`/linkdetect/tasks/${editingId.value}`, payload)
  else await postEnc('/linkdetect/tasks', payload)
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await loadTasks()
  renderRealtime()
}

async function removeTask(row: LinkTask) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/linkdetect/tasks/${row.id}`)
  ElMessage.success(t('common.success'))
  delete rtHistoryGlobal[row.id]
  await loadTasks()
}

async function showHistory(row: LinkTask) {
  historyTask.value = row
  historyVisible.value = true
  await nextTick()
  let points: [number, number | null][] = (rtHistoryGlobal[row.id] || []).slice()
  try {
    const h = await getEnc<{ rt: [number, number][] }>(`/linkdetect/tasks/${row.id}/realtime`)
    if (h.rt && h.rt.length > 0) {
      const merged = new Map<number, number | null>()
      for (const p of points) merged.set(p[0], p[1])
      for (const p of h.rt) merged.set(p[0], p[1] > 0 ? p[1] : null)
      points = Array.from(merged.entries()).sort((a, b) => a[0] - b[0]) as [number, number | null][]
    }
  } catch { }
  historyData.value = { name: row.name, color: row.color, points }
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value)
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + ' ms' },
    grid: { left: 50, right: 16, top: 20, bottom: 60 },
    xAxis: { type: 'time' },
    yAxis: { type: 'value', name: 'ms', min: 0 },
    dataZoom: [
      { type: 'inside', start: 0, end: 100, moveOnMouseMove: true, zoomOnMouseWheel: true, moveOnMouseWheel: false },
      { type: 'slider', start: 0, end: 100, height: 24, bottom: 10 }
    ],
    series: [{
      name: row.name, type: 'line', showSymbol: false, smooth: false,
      lineStyle: { width: 2, color: row.color }, itemStyle: { color: row.color },
      areaStyle: { opacity: 0.08 }, connectNulls: false,
      data: historyData.value?.points || []
    }]
  }, true)
}

const rtChartRef = ref<HTMLDivElement>()
let rtChart: echarts.ECharts | null = null
const rtHistory = rtHistoryGlobal

function pushPoint(taskId: number, val: number | null) {
  const now = Date.now()
  const cutoff = now - 10 * 60 * 1000
  let hist = (rtHistory[taskId] || []).filter((p) => p[0] > cutoff)
  const last = hist[hist.length - 1]
  if (val === null) {
    if (!last || now - last[0] > 2000) hist.push([now, null])
    else last[1] = null
  } else {
    if (!last || now - last[0] > 2000) hist.push([now, val])
    else last[1] = val
  }
  rtHistory[taskId] = hist
}

let renderTimer: number | null = null
let pollTimer: any = null
function renderRealtime() {
  if (!rtChartRef.value) return
  rtChart ??= echarts.init(rtChartRef.value)
  // 节流：最多每秒渲染一次
  if (renderTimer) return
  renderTimer = window.setTimeout(() => {
    renderTimer = null
    doRenderRealtime()
  }, 1000)
}

function doRenderRealtime() {
  if (!rtChart || rtChart.isDisposed()) return
  const cutoff = Date.now() - 10 * 60 * 1000
  const list = tasks.value.map((tk) => {
    const hist = (rtHistory[tk.id] || []).filter((p) => p[0] > cutoff)
    return { name: tk.name, color: tk.color, data: hist }
  })
  const nowMs = Date.now()
  const axisStart = nowMs - 10 * 60 * 1000
  rtChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v == null ? '--' : v.toFixed(1) + ' ms' },
    legend: { type: 'scroll', top: 0 },
    grid: { left: 60, right: 20, top: 40, bottom: 24 },
    xAxis: { type: 'time', min: axisStart, max: nowMs, axisLabel: { formatter: '{HH}:{mm}:{ss}' } },
    yAxis: { type: 'value', name: 'ms', min: 0 },
    series: list.map((s) => ({
      name: s.name, type: 'line', showSymbol: false, smooth: false,
      lineStyle: { width: 2, color: s.color }, itemStyle: { color: s.color },
      areaStyle: { opacity: 0.08 }, connectNulls: false, data: s.data
    }))
  }, true)
}

async function seedHistory() {
  for (const tk of tasks.value) {
    try {
      const h = await getEnc<{ rt: [number, number][] }>(`/linkdetect/tasks/${tk.id}/realtime`)
      if (h.rt && h.rt.length > 0) {
        const existing = rtHistory[tk.id] || []
        const merged = new Map<number, number | null>()
        for (const p of existing) merged.set(p[0], p[1])
        for (const p of h.rt) merged.set(p[0], p[1] > 0 ? p[1] : null)
        rtHistory[tk.id] = Array.from(merged.entries()).sort((a, b) => a[0] - b[0]) as [number, number | null][]
      }
    } catch { }
  }
}

function onResize() { rtChart?.resize() }

onMounted(async () => {
  chart = null
  await loadTasks()
  await seedHistory()
  await nextTick()
  if (rtChartRef.value) {
    rtChart?.dispose()
    rtChart = echarts.init(rtChartRef.value)
  }
  renderRealtime()
  setTimeout(() => rtChart?.resize(), 100)

  // WebSocket 实时推送（唯一实时数据源）
  const onResult = (msg: any) => {
    const s = msg.data as any
    if (!s || !s.task_id) return
    statuses.value[s.task_id] = {
      task_id: s.task_id, name: s.name, target: s.target,
      up: !!s.up, rt_ms: s.rt_ms || 0, loss_pct: s.loss_pct || 0,
      ts: s.ts || Date.now(), message: s.message || '', color: s.color || ''
    }
    pushPoint(s.task_id, s.up ? (s.rt_ms || 0) : null)
    renderRealtime()
  }
  try {
    ws = new WSClient(['linkdetect'])
    ws.on('linkdetect', 'result', onResult)
    ws.on('linkdetect', 'status_change', onResult)
    ws.on('linkdetect', 'link_status', onResult)
    ws.connect()
  } catch { }

  window.addEventListener('resize', onResize)

  // 每5秒轮询状态（WebSocket断连兜底，新任务状态自动更新）
  pollTimer = setInterval(async () => { await loadTasks(); renderRealtime() }, 5000)
})

onBeforeUnmount(() => {
  if (renderTimer) { clearTimeout(renderTimer); renderTimer = null }
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  ws?.close()
  chart?.dispose()
  rtChart?.dispose()
  window.removeEventListener('resize', onResize)
})

function taskColor(row: LinkTask) { return row.color || '#64748b' }
</script>

<template>
  <div class="np-page">
    <div class="np-card np-full">
      <div class="np-card-title">
        <el-icon><Monitor /></el-icon>
        <span>{{ t('link.realtime') }}</span>
        <span class="np-sub">{{ t('link.realtimeDesc') }}</span>
      </div>
      <div ref="rtChartRef" style="height: 320px"></div>
      <div class="np-status-table">
        <div v-for="tk in tasks" :key="tk.id" class="np-status-item" :style="{ borderLeft: `4px solid ${taskColor(tk)}` }">
          <span class="np-dot" :class="!statuses[tk.id] ? 'idle' : (statuses[tk.id].up ? 'up' : 'down')"></span>
          <span class="np-si-name">{{ tk.name }}</span>
          <span class="np-si-target">{{ tk.target }}</span>
          <span class="np-si-val">{{ statuses[tk.id] ? (statuses[tk.id].rt_ms || 0).toFixed(1) + 'ms' : '--' }}</span>
          <span class="np-si-val">{{ statuses[tk.id] ? (statuses[tk.id].loss_pct || 0).toFixed(0) + '%' : '--' }}</span>
          <span class="np-si-time">{{ statuses[tk.id] ? fmtTime(statuses[tk.id].ts) : '--' }}</span>
        </div>
        <div v-if="!tasks.length" class="np-empty">{{ t('common.noData') }}</div>
      </div>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Connection /></el-icon>
        <span>{{ t('link.taskName') }}</span>
        <div class="spacer"></div>
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon>{{ t('link.addTask') }}
        </el-button>
      </div>
      <el-table :data="tasks" v-loading="loading" size="default" stripe class="np-table">
        <el-table-column prop="name" :label="t('link.taskName')" min-width="140" />
        <el-table-column prop="target" :label="t('link.target')" min-width="160">
          <template #default="{ row }">
            <span :style="{ color: taskColor(row), fontWeight: 600 }">{{ row.target }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('link.method')" width="130">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.method === 'tcp' ? t('link.tcp') : row.method === 'icmp' ? t('link.icmp') : t('link.cmd') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="interval" :label="t('link.interval')" width="110" />
        <el-table-column prop="timeout" :label="t('link.timeout') + '(ms)'" width="110" />
        <el-table-column :label="t('common.status')" width="90">
          <template #default="{ row }">
            <span class="np-dot" :class="!statuses[row.id] ? 'idle' : (statuses[row.id].up ? 'up' : 'down')"></span>
            <span v-if="statuses[row.id]">{{ statuses[row.id].up ? t('link.up') : t('link.down') }}</span><span v-else class="np-text-2">--</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="210" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="showHistory(row)">{{ t('link.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="removeTask(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('link.editTask') : t('link.addTask')" width="520px">
      <el-form :model="form" label-width="110px">
        <el-form-item :label="t('link.taskName')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('link.target')" required>
          <el-input v-model="form.target" placeholder="IP或域名" />
        </el-form-item>
        <el-form-item :label="t('link.method')">
          <el-select v-model="form.method">
            <el-option label="系统Ping命令" value="cmd" />
            <el-option label="TCP" value="tcp" />
            <el-option label="ICMP" value="icmp" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('link.port')" v-if="form.method === 'tcp'">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item :label="t('link.interval')">
          <el-input-number v-model="form.interval" :min="5" :max="3600" />
        </el-form-item>
        <el-form-item :label="t('link.timeout') + '(ms)'">
          <el-input-number v-model="form.timeout" :min="500" :max="30000" :step="500" />
        </el-form-item>
        <el-form-item :label="t('link.color')">
          <el-color-picker v-model="form.color" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTask">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="historyVisible" :title="`${t('link.history')} - ${historyTask?.name}`" width="760px">
      <div ref="chartRef" class="np-chart" style="height: 400px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-sub { font-size: 12px; color: var(--np-text-2); font-weight: 400; }
.np-status-table { margin-top: 10px; display: flex; flex-direction: column; gap: 6px; max-height: 260px; overflow-y: auto; }
.np-status-item {
  display: flex; align-items: center; gap: 10px;
  background: var(--np-bg); border-radius: $radius-sm;
  padding: 8px 12px; font-size: 13px;
  .np-si-name { width: 160px; font-weight: 600; }
  .np-si-target { flex: 1; color: var(--np-text-2); }
  .np-si-val { width: 100px; text-align: right; font-variant-numeric: tabular-nums; }
  .np-si-time { width: 160px; text-align: right; color: var(--np-text-2); font-size: 12px; }
}
.spacer { flex: 1; }
</style>
