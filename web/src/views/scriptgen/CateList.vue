<script setup lang="ts">
// 脚本生成器：左栏功能选择
import { useI18n } from 'vue-i18n'
import type { Category, Group } from '@/types/scriptgen'

const { t } = useI18n()
defineProps<{ groups: Group[]; selected: Category | null }>()
const emit = defineEmits<{ (e: 'pick', c: Category): void }>()
</script>

<template>
  <div class="np-card cate-card">
    <div v-for="g in groups" :key="g.type" class="cate-group">
      <div class="cate-group-label">{{ g.label }}</div>
      <div class="cate-grid">
        <div v-for="c in g.items" :key="c.code"
          class="cate-item"
          :class="{ on: selected?.code === c.code }"
          @click="emit('pick', c)">
          <span class="cate-icon">{{ c.icon }}</span>
          <span class="cate-name">{{ c.name }}</span>
        </div>
      </div>
    </div>
    <div v-if="!groups.length" class="np-empty-box">
      <div class="np-empty-icon"><el-icon><FolderOpened /></el-icon></div>
      <div class="np-empty-title">{{ t('script.selectFeature') }}</div>
    </div>
  </div>
</template>

<style scoped>
.cate-card { position: sticky; top: 0; }
.cate-group { margin-bottom: 14px; }
.cate-group:last-child { margin-bottom: 0; }
.cate-group-label {
  font-size: 12px;
  color: var(--np-text-2);
  font-weight: 600;
  margin-bottom: 8px;
  padding-left: 8px;
  border-left: 2px solid var(--np-primary);
}
.cate-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.cate-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 10px 4px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid var(--np-border);
  background: var(--np-surface-1);
  color: var(--np-text-1);
  transition: background-color 0.15s ease-out, border-color 0.15s ease-out;
}
.cate-item:hover {
  background: var(--np-hover);
  border-color: var(--np-border-strong);
}
.cate-icon { font-size: 22px; line-height: 1; }
.cate-name { font-size: 11px; text-align: center; line-height: 1.4; }
.on {
  background: var(--np-primary-weak) !important;
  border-color: var(--np-primary) !important;
  color: var(--np-primary);
  font-weight: 600;
}
</style>
