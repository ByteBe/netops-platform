<script setup lang="ts">
// Nginx 监控：可用性/状态、活跃连接、请求速率(QPS)、Reading/Writing/Waiting
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { fmtTime } from '@/utils'

const { t } = useI18n()

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
  await nextTick()
  const h = await getEnc<{
    conn: number[][]
    qps: number[][]
    req: number[][]
    reading: number[][]
    writing: number[][]
    waiting: number[][]
  }>(`/nginxmon/instances/${d.id}/history`)
  histChart ??= echarts.init(histRef.value as HTMLDivElement)
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: [t('nginx.activeConns'), t('nginx.qps'), t('nginx.requests'), t('nginx.reading'), t('nginx.writing'), t('nginx.waiting')] },
    grid: { left: 60, right: 20, top: 36, bottom: 28 },
    xAxis: { type: 'time' },
    yAxis: [{ type: 'value', name: t('nginx.activeConns') }, { type: 'value', name: t('nginx.qps'), splitLine: { show: false } }],
    series: [
      { name: t('nginx.activeConns'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#2f6bff' }, itemStyle: { color: '#2f6bff' }, data: h.conn || [] },
      { name: t('nginx.qps'), type: 'line', yAxisIndex: 1, showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#22c55e' }, itemStyle: { color: '#22c55e' }, data: h.qps || [] },
      { name: t('nginx.requests'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#f59e0b' }, itemStyle: { color: '#f59e0b' }, data: h.req || [] },
      { name: t('nginx.reading'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#8b5cf6' }, itemStyle: { color: '#8b5cf6' }, data: h.reading || [] },
      { name: t('nginx.writing'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#06b6d4' }, itemStyle: { color: '#06b6d4' }, data: h.writing || [] },
      { name: t('nginx.waiting'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#94a3b8' }, itemStyle: { color: '#94a3b8' }, data: h.waiting || [] }
    ]
  })
}

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
    <div class="np-toolbar">
      <div class="spacer"></div>
      <el-button type="primary" @click="openAdd">
        <el-icon><Plus /></el-icon>{{ t('nginx.addInstance') }}
      </el-button>
    </div>

    <div class="np-db-grid">
      <div v-for="d in instances" :key="d.id" class="np-card np-db-card">
        <div class="np-db-head">
          <span class="np-dot" :class="(snaps[d.id] && snaps[d.id].up) ? 'up' : 'down'"></span>
          <span class="np-db-name">{{ d.name }}</span>
          <el-tag size="small" effect="plain">Nginx</el-tag>
          <span class="np-db-host">{{ d.status_url }}</span>
        </div>
        <div class="np-db-body">
          <div class="np-db-metrics">
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.activeConns') }}</span><span>{{ snaps[d.id]?.active_conns ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.qps') }}</span><span>{{ snaps[d.id] ? snaps[d.id].qps.toFixed(1) : '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.requests') }}</span><span>{{ snaps[d.id]?.requests ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.reading') }}</span><span>{{ snaps[d.id]?.reading ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.writing') }}</span><span>{{ snaps[d.id]?.writing ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.waiting') }}</span><span>{{ snaps[d.id]?.waiting ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('nginx.state') }}</span><span :style="{ color: snaps[d.id]?.up ? '#22c55e' : '#ef4444' }">{{ snaps[d.id]?.up ? t('nginx.online') : t('nginx.offline') }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('monitor.interval') }}</span><span>{{ d.interval }}s</span></div>
          </div>
          <div class="np-db-msg" v-if="snaps[d.id]?.message">{{ snaps[d.id]!.message }}</div>
        </div>
        <div class="np-db-foot">
          <span>{{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
          <div>
            <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('link.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>
      <div v-if="!instances.length" class="np-card np-empty">{{ t('common.noData') }}</div>
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
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.indexOf(t('nginx.testSuccess')) >= 0 ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
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
      <div ref="histRef" class="np-chart" style="height: 340px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-db-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}

.np-db-card {
  .np-db-head {
    display: flex;
    align-items: center;
    gap: 8px;
    .np-db-name {
      font-size: 15px;
      font-weight: 600;
    }
    .np-db-host {
      margin-left: auto;
      color: var(--np-text-2);
      font-size: 12px;
      max-width: 200px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
  .np-db-body {
    margin-top: 10px;
  }
  .np-db-metrics {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px 16px;
    .np-db-m {
      display: flex;
      justify-content: space-between;
      font-size: 13px;
      border-bottom: 1px dashed var(--np-border);
      padding-bottom: 4px;
      .np-db-label {
        color: var(--np-text-2);
      }
    }
  }
  .np-db-msg {
    margin-top: 8px;
    color: #ef4444;
    font-size: 12px;
    word-break: break-all;
  }
  .np-db-foot {
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--np-border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    color: var(--np-text-2);
    font-size: 12px;
  }
}
</style>
