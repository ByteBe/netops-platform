<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()

interface Config { docker_enable: boolean; docker_interval: number }
interface DockerHost { id: number; name: string; address: string; enabled: boolean; remark: string; online: boolean }
interface ContainerInfo { id: string; names: string; image: string; state: string; cpu: number; mem_pct: number; host: string }

function stateText(s: string) {
  const m: Record<string, string> = {
    running: t('container.running'), paused: t('container.paused'), exited: t('container.exited'),
    dead: t('container.dead'), created: t('container.created'), restarting: t('container.restarting')
  }
  return m[s] || s || '-'
}

const config = reactive<Config>({ docker_enable: false, docker_interval: 30 })
const hosts = ref<DockerHost[]>([])
const containers = ref<ContainerInfo[]>([])
const loading = ref(false)
const dlgVisible = ref(false)
const editing = ref<any>({ id: 0, name: '', address: '', enabled: true, remark: '' })
const testing = ref(false)
const testResult = ref('')

async function load() {
  loading.value = true
  try {
    const c = await getEnc<Config>('/containermon/config')
    config.docker_enable = c.docker_enable; config.docker_interval = c.docker_interval
    hosts.value = await getEnc<DockerHost[]>('/containermon/docker/hosts')
    containers.value = await getEnc<ContainerInfo[]>('/containermon/docker/containers')
  } finally { loading.value = false }
}

function openAdd() { editing.value = { id: 0, name: '', address: '', enabled: true, remark: '' }; testResult.value = ''; dlgVisible.value = true }
function openEdit(h: DockerHost) { editing.value = { ...h }; testResult.value = ''; dlgVisible.value = true }
async function save() {
  if (!editing.value.name) { ElMessage.warning(t('container.nameRequired')); return }
  editing.value.id ? await putEnc(`/containermon/docker/hosts/${editing.value.id}`, editing.value) : await postEnc('/containermon/docker/hosts', editing.value)
  ElMessage.success(t('container.saved')); dlgVisible.value = false; await load()
}
async function del(h: DockerHost) {
  await ElMessageBox.confirm(`${t('container.confirmDelete')} ${h.name}?`, t('common.tip'), { type: 'warning' })
  await delEnc(`/containermon/docker/hosts/${h.id}`); await load()
}

async function testConn() {
  if (!editing.value.address) { ElMessage.info(t('container.addrEmptyLocal')); }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ ok: boolean; version?: string; error?: string }>('/containermon/docker/test', { host: editing.value.address || '' })
    testResult.value = r.ok ? `${t('container.testSuccess')} Docker v${r.version}` : `${t('container.testFail')}: ${r.error}`
  } catch (e: any) { testResult.value = t('container.testFail') + ': ' + e.message } finally { testing.value = false }
}
async function saveConfig() {
  const full = await getEnc<any>('/containermon/config')
  await postEnc('/containermon/config', { ...full, docker_enable: config.docker_enable, docker_interval: config.docker_interval })
  ElMessage.success(t('container.saved')); await load()
}
async function collect() { await postEnc('/containermon/docker/collect', {}); ElMessage.success(t('container.done')); await load() }
function hostName(addr: string): string {
  const h = hosts.value.find(x => x.address === addr)
  return h ? h.name : addr
}
let _timer: any = null
onMounted(() => { load(); _timer = setInterval(load, 5000) })
onBeforeUnmount(() => { if (_timer) clearInterval(_timer) })
</script>

<template>
  <div class="np-page">
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Box /></el-icon><span>{{ t('container.dockerEnable') }}</span>
        <el-tag :type="config.docker_enable ? 'success' : 'info'" size="small">{{ config.docker_enable ? t('container.enabled') : t('container.notEnabled') }}</el-tag>
      </div>
      <el-form inline>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="config.docker_enable" /></el-form-item>
        <el-form-item :label="t('container.interval')"><el-input-number v-model="config.docker_interval" :min="10" :max="3600" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig">{{ t('container.saveConfig') }}</el-button>
          <el-button @click="collect">{{ t('container.collectNow') }}</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Monitor /></el-icon><span>{{ t('container.dockerHosts') }}</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>{{ t('container.addHost') }}</el-button>
      </div>
      <el-table :data="hosts" stripe class="np-table">
        <el-table-column :label="t('common.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.online ? 'success' : 'danger'" size="small">{{ row.online ? t('container.online') : t('container.offline') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="name" :label="t('container.hostName')" min-width="140" />
        <el-table-column :label="t('container.address')" min-width="200"><template #default="{ row }">{{ row.address || t('container.local') }}</template></el-table-column>
        <el-table-column prop="remark" :label="t('container.remark')" min-width="150" />
        <el-table-column :label="t('common.enabled')" width="70"><template #default="{ row }"><el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? t('container.yes') : t('container.no') }}</el-tag></template></el-table-column>
        <el-table-column :label="t('container.actions')" width="150">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">{{ t('container.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="del(row)">{{ t('container.del') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="np-card">
      <div class="np-card-title"><el-icon><Box /></el-icon><span>{{ t('container.containers') }}</span><el-tag size="small">{{ containers.length }}</el-tag></div>
      <el-table :data="containers" v-loading="loading" stripe class="np-table">
        <el-table-column :label="t('container.host')" width="140">
          <template #default="{ row }">{{ hostName(row.host) }}</template>
        </el-table-column>
        <el-table-column prop="names" :label="t('container.name')" min-width="140" />
        <el-table-column prop="image" :label="t('container.image')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('container.status')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.state === 'running' ? 'success' : 'info'" size="small">{{ stateText(row.state) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="100"><template #default="{ row }">{{ (row.cpu||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column :label="t('container.mem')" width="100"><template #default="{ row }">{{ (row.mem_pct||0).toFixed(1) }}%</template></el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? t('container.editHost') : t('container.addHostTitle')" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item :label="t('container.name')" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item :label="t('container.address')">
          <el-input v-model="editing.address" :placeholder="t('container.addrPlaceholder')" />
        </el-form-item>
        <el-form-item>
          <el-button :loading="testing" @click="testConn">{{ t('container.testConn') }}</el-button>
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.indexOf(t('container.testSuccess')) >= 0 ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
        </el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="editing.enabled" /></el-form-item>
        <el-form-item :label="t('container.remark')"><el-input v-model="editing.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ t('container.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('container.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss"></style>
