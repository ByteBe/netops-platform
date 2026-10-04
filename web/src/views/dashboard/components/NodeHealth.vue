<script setup lang="ts">
defineProps<{ data: { total:number; online:number; offline:number; device_total:number; container_total:number } | null }>()
</script>
<template>
  <div v-if="data" class="np-screen-card" style="margin-bottom:16px;padding:10px 20px">
    <div style="display:flex;align-items:center;justify-content:space-around;flex-wrap:wrap">
      <div style="font-weight:600">分布式节点健康度</div>
      <div><span style="color:#909399;font-size:12px">节点 </span><b>{{ data.total }}</b></div>
      <div><span style="color:#67C23A;font-size:12px">在线 </span><b style="color:#67C23A">{{ data.online }}</b></div>
      <div><span style="color:#F56C6C;font-size:12px">离线 </span><b style="color:#F56C6C">{{ data.offline }}</b></div>
      <div><span style="color:#409EFF;font-size:12px">设备 </span><b>{{ data.device_total }}</b></div>
      <div><span style="color:#E6A23C;font-size:12px">容器 </span><b>{{ data.container_total }}</b></div>
    </div>
  </div>
</template>
