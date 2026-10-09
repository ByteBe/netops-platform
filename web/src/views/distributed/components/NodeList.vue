<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc } from '@/utils/request'
const { t } = useI18n()
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
    <template #header><b>{{ t('node.childNodes') }}</b></template>
    <el-table :data="nodes" stripe size="small">
      <el-table-column prop="name" :label="t('node.name')" min-width="140" />
      <el-table-column :label="t('node.group')" width="140">
        <template #default="{ row }">
          <el-select v-if="editing===row.id" v-model="draft" size="small" filterable allow-create default-first-option @change="setGroup(row)">
            <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
          </el-select>
          <el-button v-else link size="small" @click="editing=row.id; draft=row.group||''">{{ row.group || t('node.ungrouped') }}</el-button>
        </template>
      </el-table-column>
      <el-table-column :label="t('node.level')" width="80">
        <template #default="{ row }">{{ ['',t('node.l1'),t('node.l2'),t('node.l3'),t('node.l4')][row.level] || 'L'+row.level }}</template>
      </el-table-column>
      <el-table-column :label="t('common.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status==='online'?'success':'info'" size="small">{{ row.status==='online' ? t('node.online') : t('node.offline') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="device_cnt" :label="t('node.devices')" width="80" />
      <el-table-column prop="container_cnt" :label="t('node.containers')" width="80" />
      <el-table-column prop="last_seen_at" :label="t('node.lastSeen')" min-width="160" />
      <el-table-column :label="t('node.actions')" width="100">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="showDevices(row)">{{ t('node.deviceDetail') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
  <el-dialog v-model="dlg" :title="t('node.childDevices') + ' - ' + (cur?.name || '')" width="700px">
    <el-table :data="devices" stripe size="small">
      <el-table-column prop="name" :label="t('common.name')" />
      <el-table-column prop="ip" label="IP" />
      <el-table-column prop="type" :label="t('node.type')" />
    </el-table>
  </el-dialog>
</template>
