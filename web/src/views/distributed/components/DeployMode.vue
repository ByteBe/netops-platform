<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { postEnc } from '@/utils/request'
const props = defineProps<{ modelValue: 'standalone' | 'distributed' }>()
const emit = defineEmits(['update:modelValue', 'changed'])
async function switchMode(m: string) {
  if (m === props.modelValue) return
  try {
    await ElMessageBox.confirm(`切换为${m === 'distributed' ? '分布式级联' : '单机'}模式后刷新页面生效，确认继续？`, '确认切换', { type: 'warning' })
  } catch { return }
  try {
    await postEnc('/distributed/deploy-mode', { mode: m })
    emit('update:modelValue', m)
    ElMessage.success('已切换')
    setTimeout(() => window.location.reload(), 800)
  } catch (e: any) { ElMessage.error('切换失败: ' + (e?.message || e)) }
}
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>部署模式</b></template>
    <el-radio-group :model-value="modelValue" @change="(v: any) => switchMode(v)">
      <el-radio value="standalone">单机模式</el-radio>
      <el-radio value="distributed">分布式级联模式</el-radio>
    </el-radio-group>
    <div style="margin-top:8px;color:#909399;font-size:13px">
      {{ modelValue === 'distributed' ? '当前为分布式模式，本节点可与上下级节点同步数据。' : '当前为单机模式，仅本机采集与存储，不与其他节点通信。' }}
    </div>
  </el-card>
</template>
