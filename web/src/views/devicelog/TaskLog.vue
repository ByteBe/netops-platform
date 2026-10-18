<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc } from '@/utils/request'
const { t } = useI18n()

interface TaskLog {
  id: number; device_id: number; device_name: string; ip: string
  type: string; success: boolean; duration_ms: number; message: string; created_at: string
}
const taskLogs = ref<TaskLog[]>([])
const taskDetailVisible = ref(false)
const taskDetailLog = ref<TaskLog | null>(null)
const tableRef = ref()

function fmtTime(s: string) { return s ? s.replace('T', ' ').substring(0, 19) : '' }
function showDetail(row: TaskLog) { taskDetailLog.value = row; taskDetailVisible.value = true }

async function load() {
  taskLogs.value = await getEnc<TaskLog[]>('/monitor/task-logs')
  await nextTick()
  tableRef.value?.doLayout?.()
  setTimeout(() => tableRef.value?.doLayout?.(), 200)
}
let autoTimer: number | undefined
onMounted(() => { load(); autoTimer = window.setInterval(load, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>
<template>
  <div>
    <el-table ref="tableRef" :data="taskLogs" stripe size="small" style="width:100%">
      <el-table-column :label="t('devicelog.time')" width="170">
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column :label="t('devicelog.status')" width="70">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? 'OK' : 'FAIL' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="device_name" :label="t('devicelog.device')" width="120" />
      <el-table-column prop="ip" label="IP" width="120" />
      <el-table-column prop="type" :label="t('devicelog.type')" width="80" />
      <el-table-column :label="t('devicelog.duration')" width="80">
        <template #default="{ row }">{{ row.duration_ms }}ms</template>
      </el-table-column>
      <el-table-column :label="t('devicelog.message')" min-width="200">
        <template #default="{ row }">
          <div class="log-msg" @click="showDetail(row)">{{ row.message }}</div>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog v-model="taskDetailVisible" title="采集任务详情" width="700px">
      <div v-if="taskDetailLog">
        <p><b>时间：</b>{{ fmtTime(taskDetailLog.created_at) }}　<b>状态：</b>{{ taskDetailLog.success ? '成功' : '失败' }}　<b>耗时：</b>{{ taskDetailLog.duration_ms }}ms</p>
        <p><b>设备：</b>{{ taskDetailLog.device_name }}（{{ taskDetailLog.ip }}）　<b>类型：</b>{{ taskDetailLog.type }}</p>
        <p><b>内容：</b></p>
        <pre class="log-detail-text">{{ taskDetailLog.message }}</pre>
      </div>
    </el-dialog>
  </div>
</template>
<style scoped>
.log-msg { cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-msg:hover { color: #409eff; }
.log-detail-text { background: #f5f7fa; padding: 10px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; max-height: 400px; overflow-y: auto; font-size: 13px; }
</style>
