<script setup lang="ts">
// Nginx 监控：可用性/状态、活跃连接、请求速率(QPS)、Reading/Writing/Waiting
import { ref, reactive, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAppStore } from '@/stores/app'
import { fmtTime, cssVar, chartAxisColors } from '@/utils'
import { histTooltip } from '@/utils/histTooltip'

const { t } = useI18n()
const appStore = useAppStore()

interface NginxInstance {
  id: number
  name: string
  status_url: string
  interval: number
  enable: boolean
  status: string
  remark: string
}
// 列表接口合并实时快照后的条目
interface NginxListItem extends NginxInstance {
  online?: boolean
  active_conns?: number
  requests?: number
  qps?: number
  reading?: number
  writing?: number
  waiting?: number
  message?: string
  ts?: string
}
interface NginxSnap {
  instance_id: number
  name: string
  up: boolean
  active_conns: number
  requests: number
  qps: number
  reading: number
  writing: number
  waiting: number
  message: string
  ts: string
}

const instances = ref<NginxInstance[]>([])
const snaps = ref<Record<number, NginxSnap>>({})
const loading = ref(false)

const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive<NginxInstance>({
  id: 0, name: '', status_url: 'http://127.0.0.1/nginx_status', interval: 60, enable: true, status: '', remark: ''
})

const histVisible = ref(false)
const testing = ref(false)
const testResult = ref('')
const histInstance = ref<NginxInstance | null>(null)
const histRef = ref<HTMLDivElement>()
const histHours = ref(24)
let histChart: echarts.ECharts | null = null
let ws: WSClient | null = null

async function load() {
  loading.value = true
  try {
    const list = await getEnc<NginxListItem[]>(`/nginxmon/instances`)
    instances.value = list
    snaps.value = {}
    for (const d of list) {
      if (d.online !== undefined) {
        snaps.value[d.id] = {
          instance_id: d.id, name: d.name, up: !!d.online,
          active_conns: d.active_conns || 0,
          requests: d.requests || 0,
          qps: d.qps || 0,
          reading: d.reading || 0,
          writing: d.writing || 0,
          waiting: d.waiting || 0,
          message: d.message || '',
          ts: d.ts || ''
        }
      }
    }
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, { id: 0, name: '', status_url: 'http://127.0.0.1/nginx_status', interval: 60, enable: true, status: '', remark: '' })
  dialogVisible.value = true
}
function openEdit(d: NginxInstance) {
  editingId.value = d.id
  Object.assign(form, d)
  dialogVisible.value = true
}
async function save() {
  if (!form.name || !form.status_url) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editingId.value) {
    await putEnc(`/nginxmon/instances/${editingId.value}`, form)
  } else {
    await postEnc('/nginxmon/instances', form)
  }
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await load()
}
async function remove(d: NginxInstance) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/nginxmon/instances/${d.id}`)
  ElMessage.success(t('common.success'))
  await load()
}

async function testConn() {
  if (!form.status_url) { ElMessage.warning(t('nginx.urlRequired')); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ok: boolean; active_conns?: number; qps?: number; error?: string}>('/nginxmon/test', {
      status_url: form.status_url
    })
    testResult.value = r.ok
      ? `${t('nginx.testSuccess')} ${t('nginx.activeConns')}: ${r.active_conns ?? '--'} QPS: ${r.qps ?? '--'}`
      : `${t('nginx.testFail')}: ${r.error}`
  } catch (e: any) { testResult.value = t('nginx.testFail') + ': ' + e.message } finally { testing.value = false }
}

async function showHistory(d: NginxInstance) {
  histInstance.value = d
  histVisible.value = true
  histHours.value = 24
  await loadNginxHistory()
}

// 轴刻度数字格式化：整数不显示小数，避免右侧刻度过长被裁剪
function fmtAxisNum(v: number) { return Number.isInteger(v) ? String(v) : String(Math.round(v * 100) / 100) }

async function loadNginxHistory() {
  const d = histInstance.value
  if (!d) return
  await nextTick()
  const h = await getEnc<{
    conn: number[][]
    qps: number[][]
    req: number[][]
    reading: number[][]
    writing: number[][]
    waiting: number[][]
  }>(`/nginxmon/instances/${d.id}/history?hours=${histHours.value}`)
  if (!histRef.value) return
  histChart?.dispose()
  histChart = echarts.init(histRef.value as HTMLDivElement, null, { renderer: 'svg' })
  const ca = chartAxisColors()
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: histTooltip({ digits: 0 }),
    legend: { data: [t('nginx.activeConns'), t('nginx.qps'), t('nginx.requests'), t('nginx.reading'), t('nginx.writing'), t('nginx.waiting')], textStyle: { color: ca.axis } },
    grid: { left: 68, right: 56, top: 36, bottom: 60 },
    xAxis: { type: 'time', axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: [
      { type: 'value', name: t('nginx.activeConns'), nameTextStyle: { color: ca.axis }, axisLabel: { color: ca.axis, formatter: fmtAxisNum }, splitLine: { lineStyle: { color: ca.split } } },
      { type: 'value', name: t('nginx.qps'), nameTextStyle: { color: ca.axis }, axisLabel: { color: ca.axis, formatter: fmtAxisNum }, splitLine: { show: false } }
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
      { name: t('nginx.activeConns'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-primary') }, itemStyle: { color: cssVar('--np-primary') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.conn || [] },
      { name: t('nginx.qps'), type: 'line', yAxisIndex: 1, showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-success') }, itemStyle: { color: cssVar('--np-success') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.qps || [] },
      { name: t('nginx.requests'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-warning') }, itemStyle: { color: cssVar('--np-warning') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.req || [] },
      { name: t('nginx.reading'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-info') }, itemStyle: { color: cssVar('--np-info') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.reading || [] },
      { name: t('nginx.writing'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-text-2') }, itemStyle: { color: cssVar('--np-text-2') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.writing || [] },
      { name: t('nginx.waiting'), type: 'line', showSymbol: false, smooth: false, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-text-3') }, itemStyle: { color: cssVar('--np-text-3') }, areaStyle: { opacity: 0.08 }, connectNulls: false, data: h.waiting || [] }
    ]
  }, true)
}

// 主题切换时重绘历史曲线（canvas 不感知 CSS 变量变化）
watch(() => appStore.theme, () => {
  if (histVisible.value && histInstance.value) void showHistory(histInstance.value)
})

let autoTimer: number | undefined
onMounted(async () => {
  await load()
  ws = new WSClient(['nginxmon'])
  ws.on('nginxmon', 'nginx_snapshot', (msg) => {
    const s = msg.data as unknown as NginxSnap
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
        <div class="np-page-head-title">{{ t('nginx.title') }}</div>
        <div class="np-page-head-desc">{{ t('nginx.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon>{{ t('nginx.addInstance') }}
        </el-button>
      </div>
    </div>

    <div class="np-kpi-grid">
      <div class="np-kpi">
        <div class="np-kpi-icon"><el-icon><Box /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.length }}</div>
          <div class="np-kpi-label">{{ t('nginx.totalInstances') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.filter(i => snaps[i.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('nginx.onlineInstances') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-danger"><el-icon><CircleClose /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ instances.length - instances.filter(i => snaps[i.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('nginx.offlineInstances') }}</div>
        </div>
      </div>
    </div>

    <div class="nx-grid">
      <div v-for="d in instances" :key="d.id" class="np-card nx-card">
        <div class="nx-head">
          <span class="np-dot" :class="(snaps[d.id] && snaps[d.id].up) ? 'up' : 'down'"></span>
          <span class="nx-name">{{ d.name }}</span>
          <el-tag size="small" effect="plain">Nginx</el-tag>
          <span class="nx-host np-ellipsis">{{ d.status_url }}</span>
        </div>

        <div class="nx-stats">
          <div class="nx-stat">
            <div class="nx-stat-value">{{ snaps[d.id]?.active_conns ?? '--' }}</div>
            <div class="nx-stat-label">{{ t('nginx.activeConns') }}</div>
          </div>
          <div class="nx-stat is-accent">
            <div class="nx-stat-value">{{ snaps[d.id] ? snaps[d.id].qps.toFixed(1) : '--' }}</div>
            <div class="nx-stat-label">{{ t('nginx.qps') }}</div>
          </div>
        </div>

        <div class="nx-mini">
          <span class="nx-mini-item"><em>{{ t('nginx.reading') }}</em><b>{{ snaps[d.id]?.reading ?? '--' }}</b></span>
          <span class="nx-mini-item"><em>{{ t('nginx.writing') }}</em><b>{{ snaps[d.id]?.writing ?? '--' }}</b></span>
          <span class="nx-mini-item"><em>{{ t('nginx.waiting') }}</em><b>{{ snaps[d.id]?.waiting ?? '--' }}</b></span>
          <span class="nx-mini-item"><em>{{ t('nginx.requests') }}</em><b>{{ snaps[d.id]?.requests ?? '--' }}</b></span>
          <span class="nx-mini-item"><em>{{ t('monitor.interval') }}</em><b>{{ d.interval }}s</b></span>
        </div>

        <div class="nx-msg" v-if="snaps[d.id]?.message">{{ snaps[d.id]!.message }}</div>

        <div class="nx-foot">
          <span class="nx-time">{{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
          <div>
            <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('link.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>
      <div v-if="!instances.length" class="np-card nx-empty">
        <div class="np-empty-box">
          <div class="np-empty-icon"><el-icon><Monitor /></el-icon></div>
          <div class="np-empty-title">{{ t('nginx.title') }}</div>
          <div class="np-empty-desc">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('nginx.editInstance') : t('nginx.addInstance')" width="520px">
      <el-form :model="form" label-width="120px">
        <el-form-item :label="t('nginx.instanceName')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('nginx.statusUrl')" required>
          <el-input v-model="form.status_url" :placeholder="t('nginx.statusUrlTip')" />
        </el-form-item>
        <el-form-item label=" ">
          <el-button :loading="testing" @click="testConn">{{ t('nginx.testConn') }}</el-button>
          <span v-if="testResult" class="nx-test-result" :class="testResult.indexOf(t('nginx.testSuccess')) >= 0 ? 'is-ok' : 'is-fail'">{{ testResult }}</span>
        </el-form-item>
        <el-form-item :label="t('monitor.interval')">
          <el-input-number v-model="form.interval" :min="10" :max="3600" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enable" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="form.remark" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="histVisible" :title="`${t('link.history')} - ${histInstance?.name}`" width="820px">
      <div style="margin-bottom:8px">
        <el-select v-model="histHours" size="small" style="width:140px" @change="loadNginxHistory">
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
.nx-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}

.nx-card {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .nx-head {
    display: flex;
    align-items: center;
    gap: 8px;
    .nx-name {
      font-size: 15px;
      font-weight: 600;
    }
    .nx-host {
      margin-left: auto;
      color: var(--np-text-2);
      font-size: 12px;
      max-width: 200px;
    }
  }

  // 仪表读数条：单一凹陷表面 + 发丝分隔线，信号标线在左，数值墨色，微标签弱化
  .nx-stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    border: 1px solid var(--np-border);
    border-radius: $radius-md;
    background: var(--np-surface-1);
    overflow: hidden;
    .nx-stat {
      position: relative;
      padding: 12px 14px 12px 16px;
      & + & { border-left: 1px solid var(--np-border); }
      // 左侧信号标线（常态中性，QPS 项信号橙）
      &::before {
        content: '';
        position: absolute;
        left: 0;
        top: 12px;
        bottom: 12px;
        width: 2px;
        background: var(--np-border-strong);
      }
      &.is-accent::before { background: var(--np-primary); }
      .nx-stat-value {
        font-size: 24px;
        font-weight: 700;
        line-height: 1.2;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
      }
      .nx-stat-label {
        margin-top: 2px;
        font-size: 11px;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--np-text-3);
      }
    }
  }

  .nx-mini {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    .nx-mini-item {
      display: flex;
      align-items: baseline;
      gap: 6px;
      font-size: 12px;
      em {
        font-style: normal;
        font-size: 11px;
        letter-spacing: 0.06em;
        color: var(--np-text-3);
      }
      b {
        font-weight: 600;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
      }
    }
  }

  .nx-msg {
    color: var(--np-danger);
    font-size: 12px;
    word-break: break-all;
  }

  .nx-foot {
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

.nx-test-result {
  margin-left: 12px;
  font-size: 13px;
  &.is-ok { color: var(--np-success); }
  &.is-fail { color: var(--np-danger); }
}

.nx-empty {
  padding: 0;
}
</style>
