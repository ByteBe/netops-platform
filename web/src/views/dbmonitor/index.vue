<script setup lang="ts">
// 数据库监控：可用性/状态、容量存储、事务日志、错误异常
import { ref, reactive, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { fmtBytes, fmtDuration, fmtTime, chartAxisColors, cssVar } from '@/utils'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

interface DBInstance {
  id: number
  name: string
  type: string
  host: string
  port: number
  user: string
  password: string
  db_name: string
  interval: number
  enable: boolean
  remark: string
}
interface DBSnap {
  instance_id: number
  name: string
  up: boolean
  version: string
  conns: number
  max_conns: number
  size_bytes: number
  table_count: number
  tx_rate: number
  deadlocks: number
  error_count: number
  uptime_s: number
  log_pos: string
  message: string
  cpu_percent: number
  mem_used: number
  mem_total: number
  ts: string
}

const instances = ref<DBInstance[]>([])
const snaps = ref<Record<number, DBSnap>>({})
const types = ref<Array<{label: string; value: string; port?: number}>>([])
const loading = ref(false)

const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive<DBInstance>({
  id: 0, name: '', type: 'mysql', host: '127.0.0.1', port: 3306, user: '', password: '', db_name: '',
  interval: 60, enable: true, remark: ''
})

const defaultPorts: Record<string, number> = {
  mysql: 3306, oracle: 1521, postgresql: 5432, sqlserver: 1433, tdengine: 6030, influxdb: 8086,
  mongodb: 27017, clickhouse: 8123, tidb: 4000, oceanbase: 2881,
  dm: 5236, kingbase: 54321, opengauss: 5432, gbase: 5258,
  db2: 50000, sybase: 5000, sqlite: 0
}
watch(() => form.type, (newType) => {
  if (!editingId.value) {
    const tp = types.value.find(t => t.value === newType)
    form.port = (tp && tp.port) ? tp.port : (defaultPorts[newType] || 3306)
  }
})
const histVisible = ref(false)
const testing = ref(false)
const testResult = ref('')
const histInstance = ref<DBInstance | null>(null)
const histRef = ref<HTMLDivElement>()
const histHours = ref(24)
const histData = ref<{ conn: number[][]; tx: number[][] }>({ conn: [], tx: [] })
let histChart: echarts.ECharts | null = null
let ws: WSClient | null = null

async function load() {
  loading.value = true
  try {
    const list = await getEnc<any[]>('/dbmonitor/instances')
    instances.value = list
    for (const d of list) {
      if (d.online !== undefined) {
        snaps.value[d.id] = {
          instance_id: d.id, name: d.name, up: d.online,
          version: d.version || '', conns: d.conns || 0, max_conns: 0,
          size_bytes: d.data_size || 0, table_count: d.table_count || 0,
          tx_rate: d.tx_per_sec || 0, deadlocks: d.deadlocks || 0,
          error_count: d.errors || 0, uptime_s: d.uptime || 0,
          log_pos: '', message: d.message || '', ts: '',
          cpu_percent: d.cpu_percent || 0, mem_used: d.mem_used || 0, mem_total: d.mem_total || 0
        }
      }
    }
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, {
    id: 0, name: '', type: 'mysql', host: '127.0.0.1', port: 3306, user: '', password: '',
    interval: 60, enable: true, remark: ''
  })
  dialogVisible.value = true
}
function openEdit(d: DBInstance) {
  editingId.value = d.id
  Object.assign(form, d)
  dialogVisible.value = true
}
async function save() {
  if (!form.name || !form.host) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editingId.value) {
    await putEnc(`/dbmonitor/instances/${editingId.value}`, form)
  } else {
    await postEnc('/dbmonitor/instances', form)
  }
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await load()
}
async function remove(d: DBInstance) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/dbmonitor/instances/${d.id}`)
  ElMessage.success(t('common.success'))
  await load()
}
async function probe(d: DBInstance) {
  await postEnc(`/dbmonitor/instances/${d.id}/probe`, {})
  ElMessage.success(t('common.success'))
}

async function testConn() {
  if (!form.host) { ElMessage.warning(t('dbmon.hostRequired')); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ok: boolean; version?: string; error?: string}>('/dbmonitor/test', {
      id: form.id, type: form.type, host: form.host, port: form.port, user: form.user, password: form.password 
    })
    testResult.value = r.ok ? `${t('dbmon.testSuccess')} ${r.version || ''}` : `${t('dbmon.testFail')}: ${r.error}`
    await load()
  } catch (e: any) { testResult.value = t('dbmon.testFail') + ': ' + e.message } finally { testing.value = false }
}

async function showHistory(d: DBInstance) {
  histInstance.value = d
  histVisible.value = true
  histHours.value = 24
  await loadDBHistory()
}

async function loadDBHistory() {
  const d = histInstance.value
  if (!d) return
  await nextTick()
  const h = await getEnc<{
    conn: number[][]
    tx: number[][]
  }>(`/dbmonitor/instances/${d.id}/history?hours=${histHours.value}`)
  histData.value = h
  if (!histRef.value) return
  histChart?.dispose()
  histChart = echarts.init(histRef.value as HTMLDivElement, null, { renderer: 'svg' })
  renderHist()
}

// 轴刻度数字格式化：整数不显示小数，避免右侧刻度过长被裁剪
function fmtAxisNum(v: number) { return Number.isInteger(v) ? String(v) : String(Math.round(v * 100) / 100) }

function renderHist() {
  if (!histRef.value || !histChart) return
  const ca = chartAxisColors()
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      confine: true,
      // 固定显示在图表右上角，不跟随鼠标：无论 hover 任何位置，曲线区域零遮挡
      position: ['right', 'top'],
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
        const rawTs = arr[0].axisValue
        let time = arr[0].axisValueLabel || ''
        if (!time && rawTs != null) {
          const d = new Date(Number(rawTs))
          if (!Number.isNaN(d.getTime())) {
            const p = (n: number) => String(n).padStart(2, '0')
            time = `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
          } else {
            time = String(rawTs)
          }
        }
        let html = `<div style="font-size:12px;line-height:1.6;margin-bottom:2px;color:${cssVar('--np-text-2', '#5a6070')}">${time}</div>`
        for (const p of arr) {
          const raw = Array.isArray(p.value) ? p.value[1] : p.value
          const v = raw == null || !Number.isFinite(Number(raw)) ? '--' : Number(raw).toFixed(2)
          const color = typeof p.color === 'string' ? p.color : '#999'
          html += `<div style="display:flex;align-items:center;gap:8px;font-size:12px;line-height:1.7">
            <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color}"></span>
            <span>${p.seriesName}</span>
            <span style="margin-left:auto;padding-left:12px;font-variant-numeric:tabular-nums;font-weight:600">${v}</span>
          </div>`
        }
        return html
      }
    },
    legend: { data: [t('dbmon.connections'), t('dbmon.txRate')], textStyle: { color: ca.axis } },
    grid: { left: 68, right: 56, top: 36, bottom: 60 },
    xAxis: { type: 'time', axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: [
      { type: 'value', name: t('dbmon.connections'), nameTextStyle: { color: ca.axis }, axisLabel: { color: ca.axis, formatter: fmtAxisNum }, splitLine: { lineStyle: { color: ca.split } } },
      { type: 'value', name: t('dbmon.txRate'), nameTextStyle: { color: ca.axis }, axisLabel: { color: ca.axis, formatter: fmtAxisNum }, splitLine: { show: false } }
    ],
    dataZoom: [
      // moveOnMouseMove 必须为 false：否则鼠标悬停会被当作数据窗口平移，图表重绘导致曲线"消失"
      { type: 'inside', start: 0, end: 100, moveOnMouseMove: false, zoomOnMouseWheel: true, moveOnMouseWheel: false },
      {
        type: 'slider', start: 0, end: 100, height: 24, bottom: 10,
        textStyle: { color: ca.axis },
        borderColor: ca.axisLine,
        fillerColor: 'transparent',
        handleStyle: { color: ca.axisLine },
        moveHandleStyle: { color: ca.axisLine },
        dataBackground: { lineStyle: { color: ca.axisLine }, areaStyle: { color: ca.split } }
      }
    ],
    series: [
      // emphasis disabled：hover 时曲线不做任何重绘（不显示强调点），避免部分环境 hover 瞬间曲线闪烁消失
      { name: t('dbmon.connections'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-primary', '#d95f1f') }, itemStyle: { color: cssVar('--np-primary', '#d95f1f') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: histData.value.conn || [] },
      { name: t('dbmon.txRate'), type: 'line', yAxisIndex: 1, showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-success', '#3f9e63') }, itemStyle: { color: cssVar('--np-success', '#3f9e63') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: histData.value.tx || [] }
    ]
  }, true)
}

// 主题切换时重绘历史图表（ECharts canvas 不感知 CSS 变量变化）
watch(() => appStore.theme, () => {
  nextTick(() => { if (histChart && histVisible.value) renderHist() })
})

let autoTimer: number | undefined
onMounted(async () => {
  await load()
  try {
    types.value = await getEnc<any[]>('/dbmonitor/types')
  } catch {
    types.value = [{label:'MySQL',value:'mysql'},{label:'PostgreSQL',value:'postgres'},{label:'Oracle',value:'oracle'},{label:'OceanBase MySQL模式',value:'oceanbase'},{label:'OceanBase Oracle模式',value:'oceanbase_oracle'},{label:'达梦 DM',value:'dm'},{label:'人大金仓 KingbaseES',value:'kingbase'},{label:'内置数据库',value:'sqlite'},{label:'TDengine',value:'tdengine'}]
  }
  ws = new WSClient(['dbmonitor'])
  ws.on('dbmonitor', 'db_snapshot', (msg) => {
    const s = msg.data as unknown as DBSnap
    snaps.value[s.instance_id] = s
  })
  ws.connect()
  autoTimer = window.setInterval(load, 30000) // 30s 刷新实例列表
})

onBeforeUnmount(() => {
  if (autoTimer) clearInterval(autoTimer)
  ws?.close()
  histChart?.dispose()
})
</script>

<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('dbmon.title') }}</div>
        <div class="np-page-head-desc">{{ t('dbmon.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon>{{ t('dbmon.addInstance') }}
        </el-button>
      </div>
    </div>

    <div class="np-kpi-grid">
      <div class="np-kpi">
        <div class="np-kpi-icon"><el-icon><Coin /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.length }}</div>
          <div class="np-kpi-label">{{ t('dbmon.totalInstances') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.filter(i => snaps[i.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('dbmon.onlineInstances') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-danger"><el-icon><CircleClose /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.length - instances.filter(i => snaps[i.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('dbmon.offlineInstances') }}</div>
        </div>
      </div>
    </div>

    <div class="db-grid">
      <div v-for="d in instances" :key="d.id" class="np-card db-card">
        <div class="db-head">
          <span class="np-dot" :class="(snaps[d.id] && snaps[d.id].up) ? 'up' : 'down'"></span>
          <span class="db-name">{{ d.name }}</span>
          <el-tag size="small" effect="plain">{{ d.type }}</el-tag>
          <span class="db-host np-ellipsis">{{ d.host }}:{{ d.port }}</span>
        </div>

        <div class="db-stats">
          <div class="db-stat">
            <div class="db-stat-value">{{ snaps[d.id] ? fmtBytes(snaps[d.id].size_bytes) : '--' }}</div>
            <div class="db-stat-label">{{ t('dbmon.capacity') }}</div>
          </div>
          <div class="db-stat">
            <div class="db-stat-value">{{ snaps[d.id]?.conns ?? '--' }}</div>
            <div class="db-stat-label">{{ t('dbmon.connections') }}</div>
          </div>
        </div>

        <div class="db-mini">
          <span class="db-mini-item"><em>{{ t('dbmon.version') }}</em><b>{{ snaps[d.id]?.version || '--' }}</b></span>
          <span class="db-mini-item"><em>{{ t('dbmon.tables') }}</em><b>{{ snaps[d.id]?.table_count ?? '--' }}</b></span>
          <span class="db-mini-item"><em>{{ t('dbmon.cpuUsage') }}</em><b>{{ snaps[d.id] && snaps[d.id].cpu_percent != null && !isNaN(snaps[d.id].cpu_percent) ? snaps[d.id].cpu_percent.toFixed(1) + '%' : '--' }}</b></span>
          <span class="db-mini-item"><em>{{ t('dbmon.memUsage') }}</em><b>{{ snaps[d.id] && snaps[d.id].mem_used > 0 ? fmtBytes(snaps[d.id].mem_used) : '--' }}</b></span>
          <span class="db-mini-item"><em>{{ t('dbmon.uptime') }}</em><b>{{ fmtDuration(snaps[d.id]?.uptime_s || 0) }}</b></span>
          <span class="db-mini-item"><em>{{ t('dbmon.status') }}</em><b :class="snaps[d.id]?.up ? 'np-text-success' : 'np-text-danger'">{{ snaps[d.id]?.up ? t('dbmon.running') : t('dbmon.down') }}</b></span>
        </div>

        <div class="db-msg" v-if="snaps[d.id]?.message">{{ snaps[d.id]!.message }}</div>

        <div class="db-foot">
          <span>{{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
          <div>
            <el-button size="small" text type="primary" @click="probe(d)">{{ t('common.test') }}</el-button>
            <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('link.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>
      <div v-if="!instances.length" class="np-card db-empty">
        <div class="np-empty-box">
          <div class="np-empty-icon"><el-icon><Coin /></el-icon></div>
          <div class="np-empty-title">{{ t('dbmon.title') }}</div>
          <div class="np-empty-desc">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('dbmon.editInstance') : t('dbmon.addInstance')" width="520px">
      <el-form :model="form" label-width="110px">
        <el-form-item :label="t('dbmon.instanceName')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('dbmon.dbType')">
          <el-select v-model="form.type" style="width: 100%">
            <el-option v-for="tp in types" :key="tp.value" :label="tp.label" :value="tp.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('dbmon.host')" required>
          <el-input v-model="form.host" />
        </el-form-item>
        <el-form-item :label="t('dbmon.port')">
          <el-input-number v-model="form.port" :min="1" :max="65535" :controls="false" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="t('dbmon.user')">
          <el-input v-model="form.user" />
        </el-form-item>
        <el-form-item :label="t('dbmon.dbName')">
          <el-input v-model="form.db_name" :placeholder="t('dbmon.dbNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('dbmon.password')">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label=" ">
          <el-button :loading="testing" @click="testConn">{{ t('dbmon.testConn') }}</el-button>
          <span v-if="testResult" class="db-test-result" :class="testResult.indexOf(t('dbmon.testSuccess')) >= 0 ? 'is-ok' : 'is-fail'">{{ testResult }}</span>
        </el-form-item>
        <el-form-item :label="t('monitor.interval')">
          <el-input-number v-model="form.interval" :min="10" :max="3600" />
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

    <el-dialog v-model="histVisible" :title="`${t('link.history')} - ${histInstance?.name}`" width="780px">
      <div style="margin-bottom:8px">
        <el-select v-model="histHours" size="small" style="width:140px" @change="loadDBHistory">
          <el-option :label="$t('monitor.lastNHours', {n: 1})" :value="1" />
          <el-option :label="$t('monitor.lastNHours', {n: 6})" :value="6" />
          <el-option :label="$t('monitor.lastNHours', {n: 12})" :value="12" />
          <el-option :label="$t('monitor.lastNHours', {n: 24})" :value="24" />
          <el-option :label="$t('monitor.lastNHours', {n: 72})" :value="72" />
          <el-option :label="$t('monitor.lastNHours', {n: 168})" :value="168" />
        </el-select>
      </div>
      <div ref="histRef" class="np-chart" style="height: 400px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.db-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}

.db-card {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .db-head {
    display: flex;
    align-items: center;
    gap: 8px;
    .db-name {
      font-size: 15px;
      font-weight: 600;
    }
    .db-host {
      margin-left: auto;
      color: var(--np-text-2);
      font-size: 12px;
      max-width: 200px;
    }
  }

  .db-stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    .db-stat {
      padding: 10px 12px;
      border-radius: 8px;
      background: var(--np-surface-1);
      border: 1px solid var(--np-border);
      .db-stat-value {
        font-size: 22px;
        font-weight: 700;
        line-height: 1.2;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
        word-break: break-all;
      }
      .db-stat-label {
        margin-top: 2px;
        font-size: 12px;
        color: var(--np-text-2);
      }
    }
  }

  .db-mini {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    .db-mini-item {
      display: flex;
      align-items: baseline;
      gap: 6px;
      font-size: 12px;
      em {
        font-style: normal;
        color: var(--np-text-2);
      }
      b {
        font-weight: 600;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
      }
    }
  }

  .db-msg {
    color: var(--np-danger);
    font-size: 12px;
    word-break: break-all;
  }

  .db-foot {
    margin-top: auto;
    padding-top: 10px;
    border-top: 1px solid var(--np-border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    color: var(--np-text-2);
    font-size: 12px;
  }
}

.db-test-result {
  margin-left: 12px;
  font-size: 13px;
  &.is-ok { color: var(--np-success); }
  &.is-fail { color: var(--np-danger); }
}

.db-empty {
  padding: 0;
}
</style>
