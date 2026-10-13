<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc } from '@/utils/request'
const { t } = useI18n()
interface Log {
  id: number
  device_id: number
  device_name: string
  ip: string
  type: string
  success: boolean
  duration_ms: number
  message: string
  created_at: string
}
const logs = ref<Log[]>([])
async function load() {
  logs.value = await getEnc<Log[]>('/monitor/task-logs')
}
onMounted(load)
</script>
<template>
  <div class="tasklog-page">
    <div class="np-toolbar">
      <h3>{{ t('tasklog.title') }}</h3>
      <div class="spacer"></div>
      <el-button @click="load">{{ t('tasklog.refresh') }}</el-button>
    </div>
    <div class="np-card">
      <el-table :data="logs" stripe>
        <el-table-column prop="created_at" :label="t('tasklog.time')" width="180" />
        <el-table-column :label="t('tasklog.status')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.success ? 'success' : 'danger'">{{ row.success ? t('tasklog.ok') : t('tasklog.fail') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="device_name" :label="t('tasklog.device')" width="160" />
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="type" :label="t('tasklog.type')" width="90" />
        <el-table-column prop="duration_ms" :label="t('tasklog.duration')" width="110">
          <template #default="{ row }">{{ row.duration_ms }} ms</template>
        </el-table-column>
        <el-table-column prop="message" :label="t('tasklog.message')" min-width="280" show-overflow-tooltip />
      </el-table>
    </div>
  </div>
</template>
<style scoped>
.tasklog-page { padding: 16px; }
.np-toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.np-toolbar h3 { margin: 0; }
.spacer { flex: 1; }
.np-card { background: var(--np-card-bg); border: 1px solid var(--np-border); border-radius: 8px; padding: 12px; }
</style>
