<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DeviceLog from './DeviceLog.vue'
import TaskLog from './TaskLog.vue'
const { t } = useI18n()
const tab = ref('device')
</script>
<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('devicelog.title') }}</div>
        <div class="np-page-head-desc">{{ t('devicelog.pageDesc') }}</div>
      </div>
    </div>
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
<style scoped>
.tab-bar {
  display: flex;
  gap: 24px;
  padding: 0 4px;
  border-bottom: 1px solid var(--np-border);
}
.tab-item {
  padding: 10px 0;
  cursor: pointer;
  color: var(--np-text-3);
  font-size: 14px;
  letter-spacing: 0.02em;
  position: relative;
  transition: color 0.18s ease-out;
}
.tab-item:hover {
  color: var(--np-text-2);
}
.tab-item.active {
  color: var(--np-primary);
  font-weight: 600;
}
.tab-item.active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  border-radius: 2px;
  background: var(--np-primary);
}
</style>
