<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { postEnc } from '@/utils/request'
const { t } = useI18n()
const props = defineProps<{ modelValue: 'standalone' | 'distributed' }>()
const emit = defineEmits(['update:modelValue', 'changed'])
async function switchMode(m: string) {
  if (m === props.modelValue) return
  const modeName = m === 'distributed' ? t('node.distributedMode') : t('node.standaloneMode')
  try {
    await ElMessageBox.confirm(t('node.switchConfirm', { mode: modeName }), t('common.tip'), { type: 'warning' })
  } catch { return }
  try {
    await postEnc('/distributed/deploy-mode', { mode: m })
    emit('update:modelValue', m)
    ElMessage.success(t('node.switched'))
    setTimeout(() => window.location.reload(), 800)
  } catch (e: any) { ElMessage.error(t('node.switchFailed') + ': ' + (e?.message || e)) }
}
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>{{ t('node.deployMode') }}</b></template>
    <el-radio-group :model-value="modelValue" @change="(v: any) => switchMode(v)">
      <el-radio value="standalone">{{ t('node.standaloneMode') }}</el-radio>
      <el-radio value="distributed">{{ t('node.distributedMode') }}</el-radio>
    </el-radio-group>
    <div style="margin-top:8px;color:#909399;font-size:13px">
      {{ modelValue === 'distributed' ? t('node.distributedDesc') : t('node.standaloneDesc') }}
    </div>
  </el-card>
</template>
