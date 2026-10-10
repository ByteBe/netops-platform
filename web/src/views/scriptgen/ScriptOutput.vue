<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
const { t } = useI18n()
defineProps<{ output: string; outputClean: string }>()

async function copy(s: string) {
  if (!s) return
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(s)
    } else {
      const ta = document.createElement('textarea')
      ta.value = s
      ta.style.position = 'fixed'
      ta.style.top = '0'
      ta.style.left = '0'
      ta.style.width = '1px'
      ta.style.height = '1px'
      ta.style.opacity = '0'
      ta.setAttribute('readonly', 'readonly')
      document.body.appendChild(ta)
      ta.focus()
      ta.select()
      ta.setSelectionRange(0, s.length)
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    ElMessage.success(t('script.copied'))
  } catch (e) {
    ElMessage.error(t('script.copyFail'))
  }
}
</script>

<template>
  <div class="np-card">
    <div style="display:flex;align-items:center;margin-bottom:8px;font-weight:600">
      <span>{{ t('script.generatedResult') }}</span><div style="flex:1"></div>
      <el-button type="primary" plain size="small" :disabled="!output" @click="copy(output)">📋 {{ t('script.copy') }}</el-button>
    </div>
    <pre style="background:#0b1220;color:#7dd3fc;border-radius:8px;padding:14px;font-size:12px;line-height:1.7;max-height:380px;overflow:auto;white-space:pre-wrap;margin:0">{{ output || t('script.fillParamsTip') }}</pre>
    <div style="display:flex;align-items:center;margin:12px 0 8px;font-weight:600">
      <span>{{ t('script.cleanVersion') }}</span><div style="flex:1"></div>
      <el-button type="success" plain size="small" :disabled="!outputClean" @click="copy(outputClean)">📋 {{ t('script.copy') }}</el-button>
    </div>
    <pre style="background:#0b1220;color:#7dd3fc;border-radius:8px;padding:14px;font-size:12px;line-height:1.7;max-height:380px;overflow:auto;white-space:pre-wrap;margin:0">{{ outputClean || t('script.clean') }}</pre>
  </div>
</template>
