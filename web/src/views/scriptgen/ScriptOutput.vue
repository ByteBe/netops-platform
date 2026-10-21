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
    <div class="out-head">
      <span>{{ t('script.generatedResult') }}</span>
      <el-button type="primary" plain size="small" :disabled="!output" @click="copy(output)">📋 {{ t('script.copy') }}</el-button>
    </div>
    <pre class="code-pre">{{ output || t('script.fillParamsTip') }}</pre>
    <div class="out-head">
      <span>{{ t('script.cleanVersion') }}</span>
      <el-button plain size="small" :disabled="!outputClean" @click="copy(outputClean)">📋 {{ t('script.copy') }}</el-button>
    </div>
    <pre class="code-pre">{{ outputClean || t('script.clean') }}</pre>
  </div>
</template>

<style scoped>
.out-head { display: flex; align-items: center; margin-bottom: 8px; font-weight: 600; color: var(--np-text-1); }
.out-head > span { flex: 1; font-size: 13px; letter-spacing: 0.04em; }
/* 代码输出块：凹陷次级表面 + 发丝描边，颜色全部走主题变量 */
.code-pre {
  background: var(--np-table-bg);
  border: 1px solid var(--np-border);
  color: var(--np-text-1);
  border-radius: 6px;
  padding: 14px;
  font-size: 12px;
  line-height: 1.7;
  max-height: 380px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0 0 12px;
  font-family: Consolas, 'Courier New', monospace;
  font-variant-numeric: tabular-nums;
}
.out-head + .code-pre { margin-bottom: 12px; }
</style>
