<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getEnc, postEnc } from '@/utils/request'
const props = defineProps<{ nodes: any[] }>()
const emit = defineEmits(['changed'])
const groups = ref<string[]>([])
const editing = ref<number | null>(null)
const draft = ref('')
const dlg = ref(false)
const cur = ref<any>(null)
const devices = ref<any[]>([])
onMounted(async () => { groups.value = await getEnc('/distributed/groups') })
async function setGroup(row: any) {
  await postEnc(`/distributed/nodes/${row.id}/group`, { group: draft.value })
  editing.value = null; emit('changed')
}
async function showDevices(row: any) {
  cur.value = row
  devices.value = await getEnc(`/monitor/devices?node_uuid=${row.node_uuid}`)
  dlg.value = true
}
defineExpose({ reloadGroups: async () => { groups.value = await getEnc('/distributed/groups') } })
</script>
<template>
  <el-card shadow="never">
    <template #header><b>下级节点</b></template>
    <el-table :data="nodes" stripe size="small">
      <el-table-column prop="name" label="节点名称" min-width="140" />
      <el-table-column label="分组" width="140">
        <template #default="{ row }">
          <el-select v-if="editing===row.id" v-model="draft" size="small" filterable allow-create default-first-option @change="setGroup(row)">
            <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
          </el-select>
          <el-button v-else link size="small" @click="editing=row.id; draft=row.group||''">{{ row.group || '未分组' }}</el-button>
        </template>
      </el-table-column>
      <el-table-column label="层级" width="80">
        <template #default="{ row }">{{ ['','总部','省级','市级','县级'][row.level] || 'L'+row.level }}</template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status==='online'?'success':'info'" size="small">{{ row.status==='online'?'在线':'离线' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="device_cnt" label="设备数" width="80" />
      <el-table-column prop="container_cnt" label="容器数" width="80" />
      <el-table-column prop="last_seen_at" label="最后上报" min-width="160" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="showDevices(row)">设备明细</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
  <el-dialog v-model="dlg" :title="'下级设备 - ' + (cur?.name || '')" width="700px">
    <el-table :data="devices" stripe size="small">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="ip" label="IP" />
      <el-table-column prop="type" label="类型" />
    </el-table>
  </el-dialog>
</template>
