<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DeviceLog from './DeviceLog.vue'
import TaskLog from './TaskLog.vue'
const { t } = useI18n()
const tab = ref('device')
</script>
<template>
  <div class="logcenter-page devicelog-page">
    <div class="np-toolbar"><h3>{{ t('devicelog.title') }}</h3></div>
    <div class="tab-bar">
      <div class="tab-item" :class="{active: tab==='device'}" @click="tab='device'">{{ t('devicelog.tabDevice') }}</div>
      <div class="tab-item" :class="{active: tab==='task'}" @click="tab='task'">{{ t('devicelog.tabTask') }}</div>
    </div>
    <div class="np-card">
      <DeviceLog v-if="tab==='device'" />
      <TaskLog v-if="tab==='task'" />
    </div>
  </div>
</template>
<style>
.devicelog-page .el-table,
.devicelog-page .el-table__inner-wrapper,
.devicelog-page .el-table__header-wrapper,
.devicelog-page .el-table__body-wrapper { width: 100% !important; }
.devicelog-page .el-table__header-wrapper table,
.devicelog-page .el-table__body-wrapper table { width: 100% !important; table-layout: fixed !important; }
.devicelog-page .el-table__header-wrapper col,
.devicelog-page .el-table__body-wrapper col { width: auto !important; }
</style>
<style scoped>
.logcenter-page { padding: 16px; }
.np-toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.np-toolbar h3 { margin: 0; }
.tab-bar { display: flex; gap: 24px; border-bottom: 1px solid var(--np-border); margin-bottom: 12px; }
.tab-item { padding: 8px 0; cursor: pointer; color: var(--np-text-secondary); font-size: 14px; }
.tab-item.active { color: var(--el-color-primary); border-bottom: 2px solid var(--el-color-primary); font-weight: 500; }
.np-card { background: var(--np-card-bg); border: 1px solid var(--np-border); border-radius: 8px; padding: 12px; }
</style>
