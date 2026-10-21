<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
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

function ctnDot(s: string) {
  if (s === 'running') return 'up'
  if (s === 'paused') return 'warn'
  if (s === 'exited' || s === 'dead') return 'down'
  return 'idle'
}

const config = reactive<Config>({ docker_enable: false, docker_interval: 30 })
const hosts = ref<DockerHost[]>([])
const containers = ref<ContainerInfo[]>([])
const loading = ref(false)
const dlgVisible = ref(false)
const editing = ref<any>({ id: 0, name: '', address: '', enabled: true, remark: '' })
const testing = ref(false)
const testResult = ref('')

const hostOnline = computed(() => hosts.value.filter(h => h.online).length)
const ctnRunning = computed(() => containers.value.filter(c => c.state === 'running').length)

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
    <!-- 统一页头 -->
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title"><el-icon><Box /></el-icon>{{ t('container.dockerEnable') }}</div>
        <div class="np-page-head-desc">{{ t('container.pageDesc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button @click="collect"><el-icon><Refresh /></el-icon>{{ t('container.collectNow') }}</el-button>
      </div>
    </div>

    <!-- 采集配置 -->
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

    <!-- KPI 概览 -->
    <div class="np-kpi-grid">
      <div class="np-kpi">
        <span class="np-kpi-icon is-info"><el-icon><Box /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ hosts.length }}</div>
          <div class="np-kpi-label">{{ t('container.hostTotal') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ hostOnline }}</div>
          <div class="np-kpi-label">{{ t('container.onlineHosts') }}</div>
          <div class="np-kpi-sub">{{ t('container.offline') }} {{ hosts.length - hostOnline }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon"><el-icon><Coin /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ containers.length }}</div>
          <div class="np-kpi-label">{{ t('container.ctnTotal') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <span class="np-kpi-icon is-success"><el-icon><CircleCheck /></el-icon></span>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ ctnRunning }}</div>
          <div class="np-kpi-label">{{ t('container.runningCtn') }}</div>
        </div>
      </div>
    </div>

    <!-- Docker 主机卡片 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Monitor /></el-icon><span>{{ t('container.dockerHosts') }}</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>{{ t('container.addHost') }}</el-button>
      </div>

      <div v-if="hosts.length" class="np-host-grid">
        <div v-for="h in hosts" :key="h.id" class="np-host-card">
          <div class="np-host-head">
            <span class="np-dot" :class="h.online ? 'up' : 'down'"></span>
            <span class="np-host-name">{{ h.name }}</span>
            <el-tag size="small" :type="h.enabled ? 'success' : 'info'">{{ h.enabled ? t('container.yes') : t('container.no') }}</el-tag>
          </div>
          <div class="np-host-meta">
            <div class="np-meta-row"><el-icon><Position /></el-icon><span>{{ h.address || t('container.local') }}</span></div>
            <div class="np-meta-row" v-if="h.remark"><el-icon><ChatDotRound /></el-icon><span>{{ h.remark }}</span></div>
          </div>
          <div class="np-host-foot">
            <el-button size="small" text type="primary" @click="openEdit(h)">{{ t('container.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="del(h)">{{ t('container.del') }}</el-button>
          </div>
        </div>
      </div>
      <div v-else class="np-empty-box">
        <el-icon class="np-empty-icon"><Box /></el-icon>
        <div class="np-empty-title">{{ t('container.hostEmptyTitle') }}</div>
        <div class="np-empty-desc">{{ t('container.hostEmptyDesc') }}</div>
      </div>
    </div>

    <!-- 容器卡片 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Box /></el-icon><span>{{ t('container.containers') }}</span>
        <el-tag size="small">{{ containers.length }}</el-tag>
      </div>

      <div v-if="containers.length" class="np-host-grid">
        <div v-for="c in containers" :key="c.id" class="np-host-card np-ctn-card">
          <div class="np-host-head">
            <span class="np-dot" :class="ctnDot(c.state)"></span>
            <span class="np-host-name np-ellipsis">{{ c.names }}</span>
            <el-tag size="small" :type="c.state === 'running' ? 'success' : 'info'">{{ stateText(c.state) }}</el-tag>
          </div>
          <div class="np-host-meta">
            <div class="np-meta-row"><el-icon><Picture /></el-icon><span class="np-ellipsis">{{ c.image }}</span></div>
            <div class="np-meta-row"><el-icon><Monitor /></el-icon><span>{{ hostName(c.host) }}</span></div>
          </div>
          <div class="np-ctn-metrics">
            <div class="np-metric">
              <span class="np-metric-value">{{ (c.cpu || 0).toFixed(1) }}%</span>
              <span class="np-metric-label">{{ t('container.cpu') }}</span>
            </div>
            <div class="np-metric">
              <span class="np-metric-value">{{ (c.mem_pct || 0).toFixed(1) }}%</span>
              <span class="np-metric-label">{{ t('container.mem') }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="np-empty-box">
        <el-icon class="np-empty-icon"><Box /></el-icon>
        <div class="np-empty-title">{{ t('container.ctnEmptyTitle') }}</div>
        <div class="np-empty-desc">{{ t('container.ctnEmptyDesc') }}</div>
      </div>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? t('container.editHost') : t('container.addHostTitle')" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item :label="t('container.name')" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item :label="t('container.address')">
          <el-input v-model="editing.address" :placeholder="t('container.addrPlaceholder')" />
        </el-form-item>
        <el-form-item>
          <el-button :loading="testing" @click="testConn">{{ t('container.testConn') }}</el-button>
          <span v-if="testResult" class="test-result-text" :class="testResult.indexOf(t('container.testSuccess')) >= 0 ? 'np-text-success' : 'np-text-danger'">{{ testResult }}</span>
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
