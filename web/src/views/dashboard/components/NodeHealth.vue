<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps<{ data: { total:number; online:number; offline:number; device_total:number; container_total:number } | null }>()
</script>

<template>
  <div v-if="data" class="np-node-health">
    <div class="np-node-head">
      <span class="np-node-tick"></span>
      <span>{{ t('dashboard.nodeHealth') }}</span>
    </div>
    <div class="np-node-stats">
      <div class="np-node-stat">
        <span class="np-node-num">{{ data.total }}</span>
        <span class="np-node-label">{{ t('dashboard.node') }}</span>
      </div>
      <div class="np-node-stat">
        <span class="np-node-num is-online">{{ data.online }}</span>
        <span class="np-node-label">{{ t('dashboard.online') }}</span>
      </div>
      <div class="np-node-stat">
        <span class="np-node-num is-offline">{{ data.offline }}</span>
        <span class="np-node-label">{{ t('dashboard.offline') }}</span>
      </div>
      <div class="np-node-stat">
        <span class="np-node-num is-device">{{ data.device_total }}</span>
        <span class="np-node-label">{{ t('dashboard.device') }}</span>
      </div>
      <div class="np-node-stat">
        <span class="np-node-num is-container">{{ data.container_total }}</span>
        <span class="np-node-label">{{ t('dashboard.container') }}</span>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.np-node-health {
  display: flex;
  align-items: center;
  gap: 24px;
  flex-wrap: wrap;
  background: var(--np-card-bg);
  border: 1px solid var(--np-border);
  border-radius: $radius-lg;
  padding: 14px 20px;
  box-shadow: var(--np-shadow-sm);
}

.np-node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--np-monitor-text);
  flex: none;

  .np-node-tick {
    width: 5px;
    height: 5px;
    transform: rotate(45deg);
    background: var(--np-primary);
    border-radius: 1px;
  }
}

.np-node-stats {
  display: flex;
  align-items: center;
  gap: 28px;
  flex-wrap: wrap;
  flex: 1;
  justify-content: space-around;
}

.np-node-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.np-node-num {
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
  color: var(--np-monitor-text);

  &.is-online { color: var(--np-success); }
  &.is-offline { color: var(--np-danger); }
  &.is-device { color: var(--np-info); }
  &.is-container { color: var(--np-warning); }
}

.np-node-label {
  font-size: 12px;
  color: var(--np-text-3);
}
</style>
