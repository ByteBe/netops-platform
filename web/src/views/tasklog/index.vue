<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
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
let autoTimer: number | undefined
onMounted(() => { load(); autoTimer = window.setInterval(load, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>
<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('tasklog.title') }}</div>
        <div class="np-page-head-desc">{{ t('tasklog.pageDesc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button @click="load"><el-icon><Refresh /></el-icon>{{ t('tasklog.refresh') }}</el-button>
      </div>
    </div>
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Document /></el-icon>
        <span>{{ t('tasklog.title') }}</span>
      </div>
      <el-table :data="logs" stripe size="small" class="np-table">
        <el-table-column prop="created_at" :label="t('tasklog.time')" width="180">
          <template #default="{ row }"><span class="time-cell">{{ row.created_at }}</span></template>
        </el-table-column>
        <el-table-column :label="t('tasklog.status')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.success ? 'success' : 'danger'">{{ row.success ? t('tasklog.ok') : t('tasklog.fail') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="device_name" :label="t('tasklog.device')" width="160" />
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="type" :label="t('tasklog.type')" width="90" />
        <el-table-column prop="duration_ms" :label="t('tasklog.duration')" width="110">
          <template #default="{ row }"><span class="num-cell">{{ row.duration_ms }} ms</span></template>
        </el-table-column>
        <el-table-column prop="message" :label="t('tasklog.message')" min-width="280" show-overflow-tooltip />
      </el-table>
      <div v-if="!logs.length" class="np-empty-box">
        <el-icon class="np-empty-icon"><Document /></el-icon>
        <div class="np-empty-title">{{ t('common.noData') }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.time-cell,
.num-cell {
  font-variant-numeric: tabular-nums;
  font-feature-settings: 'tnum';
}
.num-cell { text-align: right; }
</style>
