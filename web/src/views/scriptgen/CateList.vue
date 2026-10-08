<script setup lang="ts">
// 脚本生成器：左栏功能选择
import type { Category, Group } from '@/types/scriptgen'

defineProps<{ groups: Group[]; selected: Category | null }>()
const emit = defineEmits<{ (e: 'pick', c: Category): void }>()
</script>

<template>
  <div class="np-card">
    <div v-for="g in groups" :key="g.type" style="margin-bottom:14px">
      <div style="font-size:12px;color:#909399;font-weight:600;margin-bottom:8px;padding-left:8px;border-left:3px solid #67c23a">{{ g.label }}</div>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:6px">
        <div v-for="c in g.items" :key="c.code"
          :style="{ display:'flex',flexDirection:'column',alignItems:'center',gap:'4px',padding:'10px 4px',borderRadius:'8px',cursor:'pointer',border:'1px solid #e4e7ed',background:'#fafafa' }"
          :class="{ on: selected?.code === c.code }"
          @click="emit('pick', c)">
          <span style="font-size:22px">{{ c.icon }}</span>
          <span style="font-size:11px;text-align:center">{{ c.name }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.on { background:#67c23a !important; border-color:#67c23a !important; }
.on span { color:#fff; }
</style>
