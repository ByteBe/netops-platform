<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
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

const clusterOnline = computed(() => clusters.value.filter(c => c.online).length)
const nodeReady = computed(() => nodes.value.filter(n => n.ready).length)

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
    <!-- 统一页头 -->
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title"><el-icon><Ship /></el-icon>{{ t('k8s.k8sMonitor') }}</div>
        <div class="np-page-head-desc">{{ t('k8s.pageDesc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button @click="collect"><el-icon><Refresh /></el-icon>{{ t('k8s.collectNow') }}</el-button>
      </div>
    </div>

    <!-- 采集配置 -->
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

    <!-- KPI 概览 -->
    <div class="np-kpi-grid">
      <div class="np-kpi">
        <span class="np-kpi-icon is-info"><el-icon><Ship /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ clusters.length }}</div>
          <div class="np-kpi-label">{{ t('k8s.clusterTotal') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ clusterOnline }}</div>
          <div class="np-kpi-label">{{ t('k8s.onlineCluster') }}</div>
          <div class="np-kpi-sub">{{ t('k8s.offline') }} {{ clusters.length - clusterOnline }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon"><el-icon><Coin /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ nodes.length }}</div>
          <div class="np-kpi-label">{{ t('k8s.nodeTotal') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon is-success"><el-icon><CircleCheck /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ nodeReady }}</div>
          <div class="np-kpi-label">{{ t('k8s.readyNode') }}</div>
        </div>
      </div>
    </div>

    <!-- 集群卡片 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>{{ t('k8s.clusters') }}</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>{{ t('k8s.addCluster') }}</el-button>
      </div>

      <div v-if="clusters.length" class="np-host-grid">
        <div v-for="k in clusters" :key="k.id" class="np-host-card">
          <div class="np-host-head">
            <span class="np-dot" :class="k.online ? 'up' : 'down'"></span>
            <span class="np-host-name">{{ k.name }}</span>
            <el-tag size="small" :type="k.enabled ? 'success' : 'info'">{{ k.enabled ? t('k8s.yes') : t('k8s.no') }}</el-tag>
          </div>
          <div class="np-host-meta">
            <div class="np-meta-row"><el-icon><Link /></el-icon><span class="np-ellipsis">{{ k.api_server }}</span></div>
            <div class="np-meta-row" v-if="k.remark"><el-icon><ChatDotRound /></el-icon><span>{{ k.remark }}</span></div>
          </div>
          <div class="np-host-foot">
            <el-button size="small" text type="primary" @click="openEdit(k)">{{ t('k8s.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="del(k)">{{ t('k8s.del') }}</el-button>
          </div>
        </div>
      </div>
      <div v-else class="np-empty-box">
        <el-icon class="np-empty-icon"><Ship /></el-icon>
        <div class="np-empty-title">{{ t('k8s.clusterEmptyTitle') }}</div>
        <div class="np-empty-desc">{{ t('k8s.clusterEmptyDesc') }}</div>
      </div>
    </div>

    <!-- 节点卡片 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>{{ t('k8s.nodeList') }}</span>
        <el-tag size="small">{{ nodes.length }}</el-tag>
      </div>

      <div v-if="nodes.length" class="np-host-grid">
        <div v-for="(n, i) in nodes" :key="i" class="np-host-card np-ctn-card">
          <div class="np-host-head">
            <span class="np-dot" :class="n.ready ? 'up' : 'idle'"></span>
            <span class="np-host-name np-ellipsis">{{ n.name }}</span>
            <el-tag size="small" :type="n.ready ? 'success' : 'danger'">{{ n.ready ? t('k8s.ready') : t('k8s.notReady') }}</el-tag>
          </div>
          <div class="np-host-meta">
            <div class="np-meta-row"><el-icon><Ship /></el-icon><span>{{ n.cluster }}</span></div>
            <div class="np-meta-row" v-if="n.role"><el-icon><User /></el-icon><span>{{ n.role }}</span></div>
          </div>
          <div class="np-ctn-metrics">
            <div class="np-metric">
              <span class="np-metric-value">{{ (n.cpu || 0).toFixed(1) }}%</span>
              <span class="np-metric-label">{{ t('k8s.cpu') }}</span>
            </div>
            <div class="np-metric">
              <span class="np-metric-value">{{ (n.mem || 0).toFixed(1) }}%</span>
              <span class="np-metric-label">{{ t('k8s.mem') }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="np-empty-box">
        <el-icon class="np-empty-icon"><Ship /></el-icon>
        <div class="np-empty-title">{{ t('k8s.nodeEmptyTitle') }}</div>
        <div class="np-empty-desc">{{ t('k8s.nodeEmptyDesc') }}</div>
      </div>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? t('k8s.editCluster') : t('k8s.addClusterTitle')" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item :label="t('k8s.clusterName')" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item label="API Server" required><el-input v-model="editing.api_server" placeholder="https://10.0.0.1:6443" /></el-form-item>
        <el-form-item :label="t('container.token')"><el-input v-model="editing.token" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button :loading="testing" @click="testConn">{{ t('k8s.testConn') }}</el-button>
          <span v-if="testResult" class="test-result-text" :class="testResult.indexOf(t('k8s.testSuccess')) >= 0 ? 'np-text-success' : 'np-text-danger'">{{ testResult }}</span>
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

<style lang="scss">
.np-host-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.np-host-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--np-border);
  border-radius: 8px;
  background: var(--np-card-bg);
  transition: transform 0.2s ease-out, border-color 0.2s ease-out, box-shadow 0.2s ease-out;

  &:hover {
    transform: translateY(-1px);
    border-color: var(--np-border-strong);
    box-shadow: var(--np-shadow-sm);
  }
}

.np-host-head {
  display: flex;
  align-items: center;
  gap: 8px;

  .np-host-name {
    flex: 1;
    font-size: 15px;
    font-weight: 600;
    color: var(--np-text-1);
  }
}

.np-host-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.np-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--np-text-2);

  .el-icon {
    flex: none;
    color: var(--np-primary);
  }

  span {
    @include ellipsis;
  }
}

.np-host-foot {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-top: 4px;
  border-top: 1px solid var(--np-border);
}

.np-ctn-metrics {
  display: flex;
  gap: 24px;
  padding-top: 10px;
  border-top: 1px solid var(--np-border);
}

.np-metric {
  display: flex;
  flex-direction: column;
  gap: 2px;

  .np-metric-value {
    font-size: 20px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    color: var(--np-text-1);
  }

  .np-metric-label {
    font-size: 12px;
    color: var(--np-text-2);
  }
}

.test-result-text {
  margin-left: 12px;
  font-size: 13px;
}
</style>
