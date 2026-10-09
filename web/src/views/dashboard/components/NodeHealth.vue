<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps<{ data: { total:number; online:number; offline:number; device_total:number; container_total:number } | null }>()
</script>
<template>
  <div v-if="data" class="np-screen-card" style="margin-bottom:16px;padding:10px 20px">
    <div style="display:flex;align-items:center;justify-content:space-around;flex-wrap:wrap">
      <div style="font-weight:600">{{ t('dashboard.nodeHealth') }}</div>
      <div><span style="color:#909399;font-size:12px">{{ t('dashboard.node') }} </span><b>{{ data.total }}</b></div>
      <div><span style="color:#67C23A;font-size:12px">{{ t('dashboard.online') }} </span><b style="color:#67C23A">{{ data.online }}</b></div>
      <div><span style="color:#F56C6C;font-size:12px">{{ t('dashboard.offline') }} </span><b style="color:#F56C6C">{{ data.offline }}</b></div>
      <div><span style="color:#409EFF;font-size:12px">{{ t('dashboard.device') }} </span><b>{{ data.device_total }}</b></div>
      <div><span style="color:#E6A23C;font-size:12px">{{ t('dashboard.container') }} </span><b>{{ data.container_total }}</b></div>
    </div>
  </div>
</template>
