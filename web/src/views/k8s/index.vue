<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()

interface Config { k8s_enable: boolean; k8s_interval: number }
interface K8sCluster { id: number; name: string; api_server: string; token: string; enabled: boolean; remark: string; online: boolean }

const config = reactive<Config>({ k8s_enable: false, k8s_interval: 60 })
const clusters = ref<K8sCluster[]>([])
const nodes = ref<any[]>([])
const loading = ref(false)
const dlgVisible = ref(false)
const editing = ref<any>({ id: 0, name: '', api_server: '', token: '', enabled: true, remark: '' })
const testing = ref(false)
const testResult = ref('')

async function load() {
  loading.value = true
  try {
    const c = await getEnc<Config>('/containermon/config')
    config.k8s_enable = c.k8s_enable; config.k8s_interval = c.k8s_interval
    clusters.value = await getEnc<K8sCluster[]>('/containermon/k8s/clusters')
    nodes.value = await getEnc('/containermon/k8s/nodes')
  } finally { loading.value = false }
}

function openAdd() { editing.value = { id: 0, name: '', api_server: '', token: '', enabled: true, remark: '' }; testResult.value = ''; dlgVisible.value = true }
function openEdit(k: K8sCluster) { editing.value = { ...k }; testResult.value = ''; dlgVisible.value = true }
async function save() {
  if (!editing.value.name) { ElMessage.warning(t('k8s.nameRequired')); return }
  if (!editing.value.api_server) { ElMessage.warning(t('k8s.apiServerRequired')); return }
  if (editing.value.id) await putEnc(`/containermon/k8s/clusters/${editing.value.id}`, editing.value)
  else await postEnc('/containermon/k8s/clusters', editing.value)
  ElMessage.success(t('k8s.saved')); dlgVisible.value = false; await load()
}
async function del(k: K8sCluster) { await ElMessageBox.confirm(`${t('k8s.confirmDelete')} ${k.name}?`, t('common.tip'), { type: 'warning' }); await delEnc(`/containermon/k8s/clusters/${k.id}`); await load() }

async function testConn() {
  if (!editing.value.api_server) { ElMessage.warning(t('k8s.fillApiServer')); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ ok: boolean; version?: string; error?: string }>('/containermon/k8s/test', { api_server: editing.value.api_server, token: editing.value.token })
    testResult.value = r.ok ? `${t('k8s.testSuccess')} v${r.version}` : `${t('k8s.testFail')}: ${r.error}`
  } catch (e: any) { testResult.value = t('k8s.testFail') + ': ' + e.message } finally { testing.value = false }
}

async function saveConfig() {
  const full = await getEnc<any>('/containermon/config')
  await postEnc('/containermon/config', { ...full, k8s_enable: config.k8s_enable, k8s_interval: config.k8s_interval })
  ElMessage.success(t('k8s.saved')); await load()
}
async function collect() { await postEnc('/containermon/k8s/collect', {}); ElMessage.success(t('k8s.done')); await load() }
let _timer: any = null
onMounted(() => { load(); _timer = setInterval(load, 10000) })
onBeforeUnmount(() => { if (_timer) clearInterval(_timer) })
</script>

<template>
  <div class="np-page">
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>{{ t('k8s.k8sMonitor') }}</span>
        <el-tag :type="config.k8s_enable ? 'success' : 'info'" size="small">{{ config.k8s_enable ? t('k8s.enabled') : t('k8s.notEnabled') }}</el-tag>
      </div>
      <el-form inline>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="config.k8s_enable" /></el-form-item>
        <el-form-item :label="t('container.interval')"><el-input-number v-model="config.k8s_interval" :min="10" :max="3600" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig">{{ t('k8s.saveConfig') }}</el-button>
          <el-button @click="collect">{{ t('k8s.collectNow') }}</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>{{ t('k8s.clusters') }}</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>{{ t('k8s.addCluster') }}</el-button>
      </div>
      <el-table :data="clusters" stripe class="np-table">
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{ row }">
            <el-tag :type="row.online ? 'success' : 'danger'" size="small">{{ row.online ? t('k8s.online') : t('k8s.offline') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('k8s.clusterName')" min-width="140" />
        <el-table-column prop="api_server" label="API Server" min-width="200" />
        <el-table-column prop="remark" :label="t('k8s.remark')" min-width="150" />
        <el-table-column :label="t('common.enabled')" width="70"><template #default="{ row }"><el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? t('k8s.yes') : t('k8s.no') }}</el-tag></template></el-table-column>
        <el-table-column :label="t('k8s.actions')" width="150">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">{{ t('k8s.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="del(row)">{{ t('k8s.del') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="np-card">
      <div class="np-card-title"><el-icon><Ship /></el-icon><span>{{ t('k8s.nodeList') }}</span><el-tag size="small">{{ nodes.length }}</el-tag></div>
      <el-table :data="nodes" v-loading="loading" stripe class="np-table">
        <el-table-column prop="cluster" :label="t('k8s.cluster')" width="120" />
        <el-table-column prop="name" label="Node" min-width="160" />
        <el-table-column :label="t('k8s.ready')" width="80">
          <template #default="{ row }">
            <el-tag :type="row.ready ? 'success' : 'danger'" size="small">{{ row.ready ? t('k8s.ready') : t('k8s.notReady') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="100"><template #default="{ row }">{{ (row.cpu||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column :label="t('k8s.mem')" width="100"><template #default="{ row }">{{ (row.mem||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column prop="role" :label="t('k8s.role')" width="90" />
      </el-table>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? t('k8s.editCluster') : t('k8s.addClusterTitle')" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item :label="t('k8s.clusterName')" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item label="API Server" required><el-input v-model="editing.api_server" placeholder="https://10.0.0.1:6443" /></el-form-item>
        <el-form-item :label="t('container.token')"><el-input v-model="editing.token" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button :loading="testing" @click="testConn">{{ t('k8s.testConn') }}</el-button>
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.indexOf(t('k8s.testSuccess')) >= 0 ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
        </el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="editing.enabled" /></el-form-item>
        <el-form-item :label="t('k8s.remark')"><el-input v-model="editing.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ t('k8s.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('k8s.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss"></style>
