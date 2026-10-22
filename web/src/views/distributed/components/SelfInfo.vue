<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps<{ self: any }>()
const visible = ref(false)
</script>
<template>
  <div class="np-card">
    <div class="np-card-title">{{ t('node.selfInfo') }}</div>
    <el-descriptions :column="2" border>
      <el-descriptions-item :label="t('node.uuid')"><code class="mono">{{ self.node_uuid || '-' }}</code></el-descriptions-item>
      <el-descriptions-item :label="t('node.name')">{{ self.name || '-' }}</el-descriptions-item>
      <el-descriptions-item :label="t('node.token')">
        <code class="mono">{{ visible ? self.token : '••••••••' }}</code>
        <el-button link type="primary" size="small" class="ml-6" @click="visible = !visible">{{ visible ? t('node.hide') : t('node.show') }}</el-button>
      </el-descriptions-item>
    </el-descriptions>
  </div>
</template>

<style scoped>
.mono {
  font-family: Consolas, monospace;
  font-size: 12px;
  background: var(--np-surface-1);
  border: 1px solid var(--np-border);
  padding: 2px 6px;
  border-radius: 4px;
  word-break: break-all;
}
.ml-6 { margin-left: 6px; }
</style>
