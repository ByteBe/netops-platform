<script setup lang="ts">
// 资源管理：将纳管设备分组分类管理
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()

interface DeviceGroup {
  id: number
  name: string
  type: string
  color: string
  remark: string
  device_count: number
}

interface GroupDevice {
  id: number; name: string; ip: string; type: string; up: boolean
  cpu?: number; mem?: number; conns?: number; source?: string
}

const groups = ref<DeviceGroup[]>([])
const groupTypeMap: Record<string, string> = {
  switch: t('resource.switch'), router: t('resource.router'), firewall: t('resource.firewall'),
  server: t('resource.server'), database: t('resource.database'), other: t('resource.other')
}

const dialogVisible = ref(false)
const form = reactive<DeviceGroup>({ id: 0, name: '', type: 'other', color: '#2f6bff', remark: '', device_count: 0 })

const devicesVisible = ref(false)
const devicesLoading = ref(false)
const currentGroup = ref<DeviceGroup | null>(null)
const groupDevices = ref<GroupDevice[]>([])
const selected = ref<GroupDevice[]>([])
const importFile = ref<HTMLInputElement>()

async function load() {
  groups.value = await getEnc<DeviceGroup[]>('/resource/groups')
}

function openAdd() {
  Object.assign(form, { id: 0, name: '', type: 'other', color: '#2f6bff', remark: '', device_count: 0 })
  dialogVisible.value = true
}
function openEdit(g: DeviceGroup) {
  Object.assign(form, g)
  dialogVisible.value = true
}
async function save() {
  if (!form.name) { ElMessage.warning(t('common.tip')); return }
  if (form.id) await putEnc(`/resource/groups/${form.id}`, form)
  else await postEnc('/resource/groups', form)
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await load()
}
async function remove(g: DeviceGroup) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/resource/groups/${g.id}`)
  ElMessage.success(t('common.success'))
  await load()
}

async function showDevices(g: DeviceGroup) {
  currentGroup.value = g
  devicesVisible.value = true
  devicesLoading.value = true
  try {
    groupDevices.value = await getEnc<GroupDevice[]>(`/resource/groups/${g.id}/devices`)
  } finally {
    devicesLoading.value = false
  }
}

async function autoAssign() {
  const r = await postEnc<{ assigned: number }>('/resource/auto-assign', {})
  ElMessage.success(t('resource.autoAssign') + `: ${r.assigned} ${t('resource.devices')}`)
  await load()
}

async function batchAction(action: string) {
  if (!selected.value.length) { ElMessage.warning(t('resource.selectedRequired')); return }
  const ids = selected.value.map(d => d.id)
  if (action === 'test') {
    const r = await postEnc<any[]>('/resource/devices/batch', { ids, action })
    ElMessage.success(t('resource.testResult') + `: ${r.filter(x => x.up).length}/${r.length}`)
    return
  }
  await postEnc('/resource/devices/batch', { ids, action })
  ElMessage.success(t('common.success'))
  if (currentGroup.value) await showDevices(currentGroup.value)
}

function triggerImport() { importFile.value?.click() }

async function downloadTemplate() {
  const auth = useAuthStore()
  const resp = await fetch('/api/v1/resource/devices/template', {
    headers: {
      Authorization: `Bearer ${auth.token}`,
      'X-Timestamp': String(Math.floor(Date.now() / 1000)),
      'X-Nonce': Math.random().toString(36).substring(2, 15) + Date.now().toString(36)
    }
  })
  const blob = await resp.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'device-template.csv'
  a.click()
  URL.revokeObjectURL(url)
}
async function onImport(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const fd = new FormData()
  fd.append('file', f)
  const r = await postEnc<{ created: number }>('/resource/devices/import', fd)
  ElMessage.success(`${t('resource.importResult')}: ${r.created}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">{{ t('resource.desc') }}</span>
      <div class="spacer"></div>
      <el-button @click="autoAssign">
        <el-icon><MagicStick /></el-icon>{{ t('resource.autoAssign') }}
      </el-button>
      <el-button @click="triggerImport">
        <el-icon><Upload /></el-icon>{{ t('resource.importCsv') }}
      </el-button>
      <el-button @click="downloadTemplate">
        <el-icon><Download /></el-icon>{{ t('resource.downloadTemplate') }}
      </el-button>
      <input ref="importFile" type="file" accept=".csv" style="display:none" @change="onImport" />
      <el-button type="primary" @click="openAdd">
        <el-icon><Plus /></el-icon>{{ t('common.add') }}
      </el-button>
    </div>

    <div class="np-group-grid">
      <div v-for="g in groups" :key="g.id" class="np-card np-group-card" :style="{ borderTop: `4px solid ${g.color}` }" @click="showDevices(g)" style="cursor:pointer">
        <div class="np-group-head">
          <el-icon :size="26" :color="g.color"><Folder /></el-icon>
          <div>
            <div class="np-group-name">{{ g.name }}</div>
            <el-tag size="small" effect="plain">{{ groupTypeMap[g.type] || g.type }}</el-tag>
          </div>
        </div>
        <div class="np-group-count">{{ g.device_count || 0 }} {{ t('resource.devices') }}</div>
        <div class="np-group-actions" @click.stop>
          <el-button size="small" text type="primary" @click="openEdit(g)">{{ t('common.edit') }}</el-button>
          <el-button size="small" text type="danger" @click="remove(g)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
      <div v-if="!groups.length" class="np-card np-empty">{{ t('common.noData') }}</div>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? t('common.edit') : t('common.add')" width="460px">
      <el-form :model="form" label-width="90px">
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('common.type')">
          <el-select v-model="form.type">
            <el-option v-for="(label, key) in groupTypeMap" :key="key" :label="label" :value="key" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('resource.color')">
          <el-color-picker v-model="form.color" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="devicesVisible" :title="`${t('resource.groupDevices')} - ${currentGroup?.name}`" width="820px">
      <div style="margin-bottom:10px;display:flex;gap:8px">
        <el-button size="small" @click="batchAction('enable')">{{ t('resource.batchEnable') }}</el-button>
        <el-button size="small" @click="batchAction('disable')">{{ t('resource.batchDisable') }}</el-button>
        <el-button size="small" type="success" @click="batchAction('test')">{{ t('resource.batchTest') }}</el-button>
      </div>
      <el-table :data="groupDevices" v-loading="devicesLoading" size="small" stripe @selection-change="(v:any)=>selected=v">
        <el-table-column type="selection" width="40" />
        <el-table-column prop="name" :label="t('resource.name')" min-width="140" />
        <el-table-column prop="ip" :label="t('resource.ipAddr')" min-width="130" />
        <el-table-column :label="t('resource.type')" width="90">
          <template #default="{ row }">{{ groupTypeMap[row.type] || row.type }}</template>
        </el-table-column>
        <el-table-column :label="t('resource.status')" width="70">
          <template #default="{ row }">
            <el-tag size="small" :type="row.up ? 'success' : 'danger'">{{ row.up ? t('resource.online') : t('resource.offline') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('resource.metrics')" min-width="150">
          <template #default="{ row }">
            <span v-if="row.source === 'db'">{{ t('resource.connections') }}: {{ row.conns || 0 }}</span>
            <span v-else>CPU: {{ row.cpu ? row.cpu.toFixed(1)+'%' : '--' }} ｜ MEM: {{ row.mem ? row.mem.toFixed(1)+'%' : '--' }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!groupDevices.length && !devicesLoading" style="text-align:center;color:#999;padding:30px">{{ t('resource.empty') }}</div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-page-desc { color: var(--np-text-2); font-size: 13px; }
.np-group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 14px;
}
.np-group-card {
  display: flex; flex-direction: column; gap: 10px;
  transition: transform .15s, box-shadow .15s;
  &:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,.1); }
  .np-group-head {
    display: flex; align-items: center; gap: 10px;
    .np-group-name { font-size: 16px; font-weight: 600; }
  }
  .np-group-count { font-size: 13px; color: var(--np-text-2); }
  .np-group-actions {
    display: flex; justify-content: flex-end;
    border-top: 1px solid var(--np-border); padding-top: 6px;
  }
}
</style>
