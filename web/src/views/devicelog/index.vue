<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc } from '@/utils/request'
import { ElMessage, ElMessageBox } from 'element-plus'
const { t } = useI18n()
const tab = ref('device')

// ===== 设备日志 =====
interface Log {
  id: number; source: string; device_name: string; ip: string
  level: string; facility: string; message: string; created_at: string
}
const list = ref<Log[]>([])
const total = ref(0)
const page = ref(1); const size = ref(50)
const source = ref(''); const level = ref(''); const device = ref(''); const keyword = ref('')
const loading = ref(false)
const levelType: Record<string, string> = { critical: 'danger', warning: 'warning', info: 'info', debug: '' }
const levelText: Record<string, string> = { critical: '严重', warning: '警告', info: '信息', debug: '调试' }
async function loadDevice() {
  loading.value = true
  try {
    const p = new URLSearchParams({ page: String(page.value), size: String(size.value) })
    if (source.value) p.set('source', source.value)
    if (level.value) p.set('level', level.value)
    if (device.value) p.set('device', device.value)
    if (keyword.value) p.set('keyword', keyword.value)
    const r = await getEnc<{ list: Log[]; total: number }>(`/devicelog/list?${p}`)
    list.value = r.list || []; total.value = r.total || 0
  } finally { loading.value = false }
}
function searchDevice() { page.value = 1; loadDevice() }
async function clearDevice() {
  await ElMessageBox.confirm(t('devicelog.clearConfirm'), t('common.tip'), { type: 'warning' })
  await postEnc('/devicelog/clear', {})
  ElMessage.success(t('common.ok')); loadDevice()
}

// ===== 采集任务日志 =====
interface TaskLog {
  id: number; device_id: number; device_name: string; ip: string
  type: string; success: boolean; duration_ms: number; message: string; created_at: string
}
const taskLogs = ref<TaskLog[]>([])
async function loadTask() {
  taskLogs.value = await getEnc<TaskLog[]>('/monitor/task-logs')
}
const sources = ref<{label:string,value:string}[]>([])
async function loadSources() {
  sources.value = await getEnc<{label:string,value:string}[]>('/devicelog/sources')
}
const sourceLabel: Record<string,string> = {
  syslog:'Syslog', docker:'Docker', k8s:'K8s', switch:'交换机', router:'路由器',
  server:'服务器', mysql:'MySQL', postgres:'PostgreSQL', mongodb:'MongoDB',
  redis:'Redis', tdengine:'TDengine', influxdb:'InfluxDB', prometheus:'Prometheus', elasticsearch:'Elasticsearch'
}
function fmtTime(s: string) { return s ? s.replace('T', ' ').substring(0, 19) : '' }
function switchTab(v: string) {
  if (v === 'device') loadDevice()
  else loadTask()
}
onMounted(() => { loadSources(); loadDevice() })
</script>
<template>
  <div class="logcenter-page">
    <div class="np-toolbar"><h3>{{ t('devicelog.title') }}</h3></div>
    <el-tabs v-model="tab" @tab-change="switchTab">
      <!-- 设备日志 -->
      <el-tab-pane :label="t('devicelog.tabDevice')" name="device">
        <div class="filter-bar np-card">
          <el-select v-model="source" placeholder="来源" clearable style="width:140px">
            <el-option v-for="s in sources" :key="s.value" :label="sourceLabel[s.value] || s.value" :value="s.value" />
          </el-select>
          <el-select v-model="level" placeholder="级别" clearable style="width:120px">
            <el-option label="严重" value="critical" />
            <el-option label="警告" value="warning" />
            <el-option label="信息" value="info" />
          </el-select>
          <el-input v-model="device" :placeholder="t('devicelog.devicePh')" clearable style="width:160px" @keyup.enter="searchDevice" />
          <el-input v-model="keyword" :placeholder="t('devicelog.keywordPh')" clearable style="width:200px" @keyup.enter="searchDevice" />
          <el-button type="primary" @click="searchDevice">{{ t('common.search') }}</el-button>
          <div style="flex:1"></div>
          <el-button @click="loadDevice">{{ t('common.refresh') }}</el-button>
          <el-button type="danger" @click="clearDevice">{{ t('devicelog.clear') }}</el-button>
        </div>
        <div class="np-card">
          <el-table :data="list" v-loading="loading" stripe size="small">
            <el-table-column :label="t('devicelog.time')" width="170">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="source" :label="t('devicelog.source')" width="80" />
            <el-table-column :label="t('devicelog.level')" width="90">
              <template #default="{ row }"><el-tag :type="levelType[row.level] || 'info'" size="small">{{ levelText[row.level] || row.level }}</el-tag></template>
            </el-table-column>
            <el-table-column prop="device_name" :label="t('devicelog.device')" width="150" />
            <el-table-column prop="ip" label="IP" width="130" />
            <el-table-column prop="facility" :label="t('devicelog.facility')" width="80" />
            <el-table-column prop="message" :label="t('devicelog.message')" min-width="300" show-overflow-tooltip />
          </el-table>
          <el-pagination style="margin-top:12px;justify-content:flex-end" layout="total,prev,pager,next"
            :total="total" :page-size="size" :current-page="page" @current-change="(p:number)=>{page=p;loadDevice()}" />
        </div>
      </el-tab-pane>
      <!-- 采集任务日志 -->
      <el-tab-pane :label="t('devicelog.tabTask')" name="task">
        <div class="np-card">
          <el-table :data="taskLogs" stripe size="small">
            <el-table-column :label="t('devicelog.time')" width="180">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column :label="t('devicelog.status')" width="80">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? 'OK' : 'FAIL' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="device_name" :label="t('devicelog.device')" width="160" />
            <el-table-column prop="ip" label="IP" width="140" />
            <el-table-column prop="type" :label="t('devicelog.type')" width="90" />
            <el-table-column prop="duration_ms" :label="t('devicelog.duration')" width="100">
              <template #default="{ row }">{{ row.duration_ms }}ms</template>
            </el-table-column>
            <el-table-column prop="message" :label="t('devicelog.message')" min-width="280" show-overflow-tooltip />
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<style scoped>
.logcenter-page { padding: 16px; }
.np-toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.np-toolbar h3 { margin: 0; }
.filter-bar { display: flex; gap: 8px; margin-bottom: 12px; align-items: center; }
.np-card { background: var(--np-card-bg); border: 1px solid var(--np-border); border-radius: 8px; padding: 12px; }
</style>
