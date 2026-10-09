<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { postEnc, getEnc } from '@/utils/request'
const { t } = useI18n()
const groups = ref<string[]>([])
const newGroup = ref('')
async function load() { groups.value = await getEnc('/distributed/groups') }
async function add() {
  if (!newGroup.value.trim()) return
  await postEnc('/distributed/groups', { group: newGroup.value.trim() })
  newGroup.value = ''; await load()
}
async function del(g: string) {
  await postEnc('/distributed/groups/delete', { group: g }); await load()
}
defineExpose({ reload: load })
onMounted(load)
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>{{ t('node.groups') }}</b></template>
    <el-input v-model="newGroup" :placeholder="t('node.groupPlaceholder')" size="small" style="width:280px;margin-bottom:12px" @keyup.enter="add">
      <template #append><el-button @click="add">{{ t('node.create') }}</el-button></template>
    </el-input>
    <div style="display:flex;flex-wrap:wrap;gap:10px">
      <div v-for="g in groups" :key="g" style="padding:8px 16px;background:#f5f7fa;border-radius:6px;display:flex;align-items:center;gap:8px">
        <el-tag size="small">{{ g }}</el-tag>
        <el-button link type="danger" size="small" @click="del(g)">{{ t('node.delete') }}</el-button>
      </div>
      <span v-if="!groups.length" style="color:#999;font-size:13px;line-height:32px">{{ t('node.noGroups') }}</span>
    </div>
  </el-card>
</template>
