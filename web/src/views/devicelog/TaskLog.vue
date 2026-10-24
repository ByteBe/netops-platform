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
function showDetail(row: Partial<TaskLog>) { taskDetailLog.value = row as TaskLog; taskDetailVisible.value = true }

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
    <el-table ref="tableRef" :data="taskLogs" stripe size="small" class="np-table">
      <el-table-column :label="t('devicelog.time')" width="170">
        <template #default="{ row }"><span class="time-cell">{{ fmtTime(row.created_at) }}</span></template>
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
        <template #default="{ row }"><span class="num-cell">{{ row.duration_ms }}ms</span></template>
      </el-table-column>
      <el-table-column :label="t('devicelog.message')" min-width="200">
        <template #default="{ row }">
          <div class="log-msg" @click="showDetail(row)">{{ row.message }}</div>
        </template>
      </el-table-column>
    </el-table>
    <div v-if="!taskLogs.length" class="np-empty-box">
      <el-icon class="np-empty-icon"><Document /></el-icon>
      <div class="np-empty-title">{{ t('common.noData') }}</div>
    </div>
    <el-dialog v-model="taskDetailVisible" :title="t('devicelog.detailTitle')" width="700px">
      <div v-if="taskDetailLog">
        <p class="detail-meta"><b>时间：</b>{{ fmtTime(taskDetailLog.created_at) }}　<b>状态：</b>{{ taskDetailLog.success ? '成功' : '失败' }}　<b>耗时：</b>{{ taskDetailLog.duration_ms }}ms</p>
        <p class="detail-meta"><b>设备：</b>{{ taskDetailLog.device_name }}（{{ taskDetailLog.ip }}）　<b>类型：</b>{{ taskDetailLog.type }}</p>
        <p class="detail-meta"><b>内容：</b></p>
        <pre class="log-detail-text">{{ taskDetailLog.message }}</pre>
      </div>
    </el-dialog>
  </div>
</template>
<style scoped>
.time-cell,
.num-cell { font-variant-numeric: tabular-nums; font-feature-settings: 'tnum'; }
.time-cell { color: var(--np-text-2); }
.num-cell { text-align: right; }
.log-msg { cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--np-text-1); transition: color 0.15s ease-out; }
.log-msg:hover { color: var(--np-primary); }
.detail-meta { margin: 0 0 8px; font-size: 13px; line-height: 1.7; color: var(--np-text-2); }
.detail-meta b { color: var(--np-text-1); font-weight: 600; }
/* 任务输出：中性次级表面 + 发丝描边 + 等宽技术感，全部走主题变量 */
.log-detail-text {
  margin: 6px 0 0;
  background: var(--np-surface-1);
  color: var(--np-text-1);
  border: 1px solid var(--np-border);
  padding: 10px 12px;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 400px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.7;
  font-family: Consolas, 'Courier New', monospace;
  font-variant-numeric: tabular-nums;
}
</style>
