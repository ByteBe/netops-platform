<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc } from '@/utils/request'
import { ElMessage, ElMessageBox } from 'element-plus'
const { t } = useI18n()

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
const sources = ref<{label:string,value:string}[]>([])
const sourceLabel: Record<string,string> = {
  syslog:'Syslog', docker:'Docker', k8s:'K8s', switch:'交换机', router:'路由器',
  server:'服务器', mysql:'MySQL', postgres:'PostgreSQL', mongodb:'MongoDB',
  redis:'Redis', tdengine:'TDengine', influxdb:'InfluxDB', prometheus:'Prometheus', elasticsearch:'Elasticsearch'
}
const detailVisible = ref(false)
const detailLog = ref<Log | null>(null)
const tableRef = ref()

function fmtTime(s: string) { return s ? s.replace('T', ' ').substring(0, 19) : '' }
function showDetail(row: Log) { detailLog.value = row; detailVisible.value = true }

async function load() {
  loading.value = true
  try {
    const p = new URLSearchParams({ page: String(page.value), size: String(size.value) })
    if (source.value) p.set('source', source.value)
    if (level.value) p.set('level', level.value)
    if (device.value) p.set('device', device.value)
    if (keyword.value) p.set('keyword', keyword.value)
    const r = await getEnc<{ list: Log[]; total: number }>(`/devicelog/list?${p}`)
    list.value = r.list || []; total.value = r.total || 0
    await nextTick()
    tableRef.value?.doLayout?.()
    setTimeout(() => tableRef.value?.doLayout?.(), 200)
  } finally { loading.value = false }
}
function search() { page.value = 1; load() }
async function clear() {
  await ElMessageBox.confirm(t('devicelog.clearConfirm'), t('common.tip'), { type: 'warning' })
  await postEnc('/devicelog/clear', {})
  ElMessage.success(t('common.ok')); load()
}
let autoTimer: number | undefined
onMounted(async () => {
  sources.value = await getEnc<{label:string,value:string}[]>('/devicelog/sources')
  load()
  autoTimer = window.setInterval(load, 30000)
})
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>
<template>
  <div>
    <div class="filter-bar">
      <el-select v-model="source" placeholder="来源" clearable style="width:140px">
        <el-option v-for="s in sources" :key="s.value" :label="sourceLabel[s.value] || s.value" :value="s.value" />
      </el-select>
      <el-select v-model="level" placeholder="级别" clearable style="width:120px">
        <el-option label="严重" value="critical" />
        <el-option label="警告" value="warning" />
        <el-option label="信息" value="info" />
      </el-select>
      <el-input v-model="device" :placeholder="t('devicelog.devicePh')" clearable style="width:160px" @keyup.enter="search" />
      <el-input v-model="keyword" :placeholder="t('devicelog.keywordPh')" clearable style="width:200px" @keyup.enter="search" />
      <el-button type="primary" @click="search">{{ t('common.search') }}</el-button>
      <div style="flex:1"></div>
      <el-button @click="load">{{ t('common.refresh') }}</el-button>
      <el-button type="danger" @click="clear">{{ t('devicelog.clear') }}</el-button>
    </div>
    <el-table ref="tableRef" :data="list" v-loading="loading" stripe size="small" style="width:100%">
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
      <el-table-column :label="t('devicelog.message')" min-width="200">
        <template #default="{ row }">
          <div class="log-msg" @click="showDetail(row)">{{ row.message }}</div>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination style="margin-top:12px;justify-content:flex-end" layout="total,prev,pager,next"
      :total="total" :page-size="size" :current-page="page" @current-change="(p:number)=>{page=p;load()}" />
    <el-dialog v-model="detailVisible" title="日志详情" width="700px">
      <div v-if="detailLog">
        <p><b>时间：</b>{{ fmtTime(detailLog.created_at) }}</p>
        <p><b>来源：</b>{{ detailLog.source }}　<b>级别：</b>{{ levelText[detailLog.level] || detailLog.level }}　<b>设备：</b>{{ detailLog.device_name }}（{{ detailLog.ip }}）</p>
        <p><b>内容：</b></p>
        <pre class="log-detail-text">{{ detailLog.message }}</pre>
      </div>
    </el-dialog>
  </div>
</template>
<style scoped>
.filter-bar { display: flex; gap: 8px; margin-bottom: 12px; align-items: center; }
.log-msg { cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-msg:hover { color: #409eff; }
.log-detail-text { background: #f5f7fa; padding: 10px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; max-height: 400px; overflow-y: auto; font-size: 13px; }
</style>
