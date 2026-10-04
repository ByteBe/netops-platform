<script setup lang="ts">
import { ref } from 'vue'
const props = defineProps<{ self: any }>()
const visible = ref(false)
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>本节点信息</b></template>
    <el-descriptions :column="2" border>
      <el-descriptions-item label="节点 UUID"><code>{{ self.node_uuid || '-' }}</code></el-descriptions-item>
      <el-descriptions-item label="本节点名称">{{ self.name || '(未设置)' }}</el-descriptions-item>
      <el-descriptions-item label="上级地址">{{ self.parent || '(未配置，为总部)' }}</el-descriptions-item>
      <el-descriptions-item label="通信 Token">
        <code>{{ visible ? self.token : '••••••••' }}</code>
        <el-button link type="primary" size="small" @click="visible=!visible" style="margin-left:6px">{{ visible ? '隐藏' : '查看' }}</el-button>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>
