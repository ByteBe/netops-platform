<script setup lang="ts">
// 数据库监控：可用性/状态、容量存储、事务日志、错误异常
import { ref, reactive, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { fmtBytes, fmtDuration, fmtTime } from '@/utils'

const { t } = useI18n()

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
          log_pos: '', message: d.message || '', ts: ''
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
  await nextTick()
  const h = await getEnc<{
    conn: number[][]
    tx: number[][]
  }>(`/dbmonitor/instances/${d.id}/history?hours=6`)
  histChart ??= echarts.init(histRef.value as HTMLDivElement)
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: ['Connections', 'Tx/s'] },
    grid: { left: 60, right: 20, top: 36, bottom: 28 },
    xAxis: { type: 'time' },
    yAxis: [{ type: 'value', name: 'Conn' }, { type: 'value', name: 'Tx/s', splitLine: { show: false } }],
    series: [
      { name: 'Connections', type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#2f6bff' }, itemStyle: { color: '#2f6bff' }, data: h.conn || [] },
      { name: 'Tx/s', type: 'line', yAxisIndex: 1, showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#22c55e' }, itemStyle: { color: '#22c55e' }, data: h.tx || [] }
    ]
  })
}

onMounted(async () => {
  await load()
  try {
    types.value = await getEnc<any[]>('/dbmonitor/types')
  } catch {
    types.value = [{label:'MySQL',value:'mysql'},{label:'Oracle',value:'oracle'},{label:'PostgreSQL',value:'postgresql'},{label:'Microsoft SQL Server',value:'sqlserver'},{label:'MongoDB',value:'mongodb'},{label:'ClickHouse',value:'clickhouse'},{label:'TiDB',value:'tidb'},{label:'OceanBase',value:'oceanbase'},{label:'达梦 DM',value:'dm'},{label:'人大金仓 KingbaseES',value:'kingbase'},{label:'openGauss',value:'opengauss'},{label:'GBase 8s/8a',value:'gbase'},{label:'IBM DB2',value:'db2'},{label:'SAP Sybase',value:'sybase'},{label:'内置数据库',value:'sqlite'},{label:'TDengine',value:'tdengine'},{label:'InfluxDB',value:'influxdb'}]
  }
  ws = new WSClient(['dbmonitor'])
  ws.on('dbmonitor', 'db_snapshot', (msg) => {
    const s = msg.data as unknown as DBSnap
    snaps.value[s.instance_id] = s
  })
  ws.connect()
})

onBeforeUnmount(() => {
  ws?.close()
  histChart?.dispose()
})
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <div class="spacer"></div>
      <el-button type="primary" @click="openAdd">
        <el-icon><Plus /></el-icon>{{ t('dbmon.addInstance') }}
      </el-button>
    </div>

    <div class="np-db-grid">
      <div v-for="d in instances" :key="d.id" class="np-card np-db-card">
        <div class="np-db-head">
          <span class="np-dot" :class="(snaps[d.id] && snaps[d.id].up) ? 'up' : 'down'"></span>
          <span class="np-db-name">{{ d.name }}</span>
          <el-tag size="small" effect="plain">{{ d.type }}</el-tag>
          <span class="np-db-host">{{ d.host }}:{{ d.port }}</span>
        </div>
        <div class="np-db-body">
          <div class="np-db-metrics">
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.version') }}</span><span>{{ snaps[d.id]?.version || '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.connections') }}</span><span>{{ snaps[d.id]?.conns ?? '--' }} / {{ snaps[d.id]?.max_conns ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.capacity') }}</span><span>{{ snaps[d.id] ? fmtBytes(snaps[d.id].size_bytes) : '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.tables') }}</span><span>{{ snaps[d.id]?.table_count ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.txRate') }}</span><span>{{ snaps[d.id]?.tx_rate ?? '--' }}/s</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.deadlocks') }}</span><span :style="{ color: (snaps[d.id]?.deadlocks || 0) > 0 ? '#ef4444' : '' }">{{ snaps[d.id]?.deadlocks ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.errors') }}</span><span :style="{ color: (snaps[d.id]?.error_count || 0) > 0 ? '#ef4444' : '' }">{{ snaps[d.id]?.error_count ?? '--' }}</span></div>
            <div class="np-db-m"><span class="np-db-label">{{ t('dbmon.uptime') }}</span><span>{{ fmtDuration(snaps[d.id]?.uptime_s || 0) }}</span></div>
          </div>
          <div class="np-db-msg" v-if="snaps[d.id]?.message">{{ snaps[d.id]!.message }}</div>
        </div>
        <div class="np-db-foot">
          <span>{{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
          <div>
            <el-button size="small" text type="primary" @click="probe(d)">{{ t('common.test') }}</el-button>
            <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('link.history') }}</el-button>
            <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>
      <div v-if="!instances.length" class="np-card np-empty">{{ t('common.noData') }}</div>
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
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.indexOf(t('dbmon.testSuccess')) >= 0 ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
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
      <div ref="histRef" class="np-chart" style="height: 320px"></div>
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
