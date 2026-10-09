<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps<{ self: any }>()
const visible = ref(false)
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>{{ t('node.selfInfo') }}</b></template>
    <el-descriptions :column="2" border>
      <el-descriptions-item :label="t('node.uuid')"><code>{{ self.node_uuid || '-' }}</code></el-descriptions-item>
      <el-descriptions-item :label="t('node.name')">{{ self.name || '-' }}</el-descriptions-item>
      <el-descriptions-item :label="t('node.parent')">{{ self.parent || t('node.parentNone') }}</el-descriptions-item>
      <el-descriptions-item :label="t('node.token')">
        <code>{{ visible ? self.token : '••••••••' }}</code>
        <el-button link type="primary" size="small" @click="visible=!visible" style="margin-left:6px">{{ visible ? t('node.hide') : t('node.show') }}</el-button>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>
