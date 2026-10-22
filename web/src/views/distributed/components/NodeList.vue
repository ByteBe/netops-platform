<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
const props = defineProps<{ nodes: any[] }>()
const emit = defineEmits(['changed'])
const dlg = ref(false)
const cur = ref<any>(null)
const devices = ref<any[]>([])
const ctnDlg = ref(false)
const containers = ref<any[]>([])
const linkDlg = ref(false)
const links = ref<any[]>([])
// 证书：已导入集合 + 导入弹窗（按节点导入，每个下级节点各自生成自己的证书）
const peersSet = ref<Set<string>>(new Set())
const peersTimer = ref<number | null>(null)
const importDlg = ref(false)
const importTarget = ref<any>(null) // 行内导入绑定的目标节点；顶部入口为 null（不校验归属）
const importPem = ref('')
const importing = ref(false)
const importResult = ref<any>(null)
onMounted(() => {
  loadPeers()
  peersTimer.value = window.setInterval(loadPeers, 30 * 1000)
})
onUnmounted(() => {
  if (peersTimer.value) window.clearInterval(peersTimer.value)
})
function openImport(row: any) {
  importTarget.value = row
  importPem.value = ''
  importResult.value = null
  importDlg.value = true
}
async function loadPeers() {
  try {
    const list: any[] = await getEnc('/distributed/cert/peers')
    peersSet.value = new Set((list || []).map((p: any) => p.node_uuid))
  } catch {}
}
function short(fp: string) { return fp ? fp.slice(0, 8) + '…' + fp.slice(-8) : '' }
function onImportFile(file: any) {
  const raw = file?.raw
  if (!raw) return
  const reader = new FileReader()
  reader.onload = () => { importPem.value = String(reader.result || '') }
  reader.readAsText(raw)
}
async function importCert() {
  if (!importPem.value.trim()) { ElMessage.warning(t('node.certImportPlaceholder')); return }
  importing.value = true
  importResult.value = null
  try {
    const r: any = await postEnc('/distributed/cert/import', {
      cert_pem: importPem.value.trim(),
      expect_node_uuid: importTarget.value?.node_uuid || '',
    })
    importResult.value = r
    if (r.ok) {
      ElMessage.success(t('node.certImportDone'))
      await loadPeers()
      emit('changed')
    } else {
      ElMessage.error(t('node.certImportFail') + ': ' + (r.error || ''))
    }
  } catch (e: any) {
    importResult.value = { ok: false, error: e?.message || e }
    ElMessage.error(t('node.certImportFail') + ': ' + (e?.message || e))
  } finally { importing.value = false }
}
async function showDevices(row: any) {
  cur.value = row
  devices.value = await getEnc(`/monitor/devices?node_uuid=${row.node_uuid}`)
  dlg.value = true
}
async function showContainers(row: any) {
  cur.value = row
  containers.value = await getEnc(`/distributed/nodes/${row.id}/containers`)
  ctnDlg.value = true
}
async function showLinks(row: any) {
  cur.value = row
  links.value = await getEnc(`/distributed/nodes/${row.id}/links`)
  linkDlg.value = true
}
function fmtUptime(s: any) {
  const v = Number(s || 0)
  if (!v || v <= 0) return '-'
  const d = Math.floor(v / 86400)
  const h = Math.floor((v % 86400) / 3600)
  const m = Math.floor((v % 3600) / 60)
  if (d > 0) return `${d}天${h}小时`
  if (h > 0) return `${h}小时${m}分`
  return `${m}分钟`
}
async function removeNode(row: any) {
  await delEnc(`/distributed/nodes/${row.id}`)
  ElMessage.success(t('node.deleted'))
  emit('changed')
}
</script>
<template>
  <div class="np-card">
    <div class="node-list-head">
      <div class="np-card-title">{{ t('node.childNodes') }}</div>
      <span class="cert-wait-tip">{{ t('node.certWaitTip') }}</span>
    </div>
    <el-table :data="nodes" stripe size="small" class="np-table">
      <el-table-column prop="name" :label="t('node.name')" min-width="140" />
      <el-table-column :label="t('node.certCol')" width="90">
        <template #default="{ row }">
          <el-tag v-if="peersSet.has(row.node_uuid)" type="success" size="small">{{ t('node.certImported') }}</el-tag>
          <el-tag v-else type="danger" size="small">{{ t('node.certNotImported') }}</el-tag>
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
      <el-table-column :label="t('node.actions')" width="340">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="showDevices(row)">{{ t('node.deviceDetail') }}</el-button>
          <el-button link type="primary" size="small" @click="showContainers(row)">{{ t('node.containerDetail') }}</el-button>
          <el-button link type="primary" size="small" @click="showLinks(row)">{{ t('node.linkDetail') }}</el-button>
          <el-button link :type="peersSet.has(row.node_uuid) ? 'warning' : 'danger'" size="small" @click="openImport(row)">{{ t('node.importCert') }}</el-button>
          <el-popconfirm :title="t('node.deleteConfirm', { name: row.name })" width="220" @confirm="removeNode(row)">
            <template #reference>
              <el-button link type="danger" size="small">{{ t('node.delete') }}</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
  </div>
  <!-- 导入下级证书（按节点：每个下级节点各自生成自己的证书） -->
  <el-dialog v-model="importDlg" :title="importTarget ? (t('node.certImportTitle') + ' - ' + importTarget.name) : t('node.certImportBtn')" width="640px">
    <div class="dialog-warn">
      {{ t('node.certWaitTip') }}
      <span v-if="importTarget" class="dialog-target">目标节点：{{ importTarget.name }}（{{ importTarget.node_uuid }}），请粘贴该节点自己生成的证书，证书节点标识不匹配将无法导入</span>
    </div>
    <el-input v-model="importPem" type="textarea" :rows="8" :placeholder="t('node.certImportPlaceholder')" class="pem-input" />
    <el-upload :auto-upload="false" :show-file-list="false" accept=".pem,.crt,.cer,.cert,.txt" :on-change="onImportFile" class="upload-row">
      <el-button size="small">{{ t('node.mqttCaUpload') }}</el-button>
    </el-upload>
    <div v-if="importResult" class="import-result" :class="importResult.ok ? 'ok' : 'fail'">
      <template v-if="importResult.ok">
        {{ t('node.certImportDone') }}：{{ importResult.name }} · {{ short(importResult.fingerprint) }}
        <div class="import-meta">
          {{ t('node.certIp') }}: {{ importResult.node_info?.ips?.join(', ') || '-' }}&nbsp;|&nbsp;
          {{ t('node.certMac') }}: {{ importResult.node_info?.macs?.join(', ') || '-' }}&nbsp;|&nbsp;
          {{ t('node.certOs') }}: {{ importResult.node_info?.os || '-' }}
        </div>
      </template>
      <template v-else>{{ importResult.error }}</template>
    </div>
    <template #footer>
      <el-button @click="importDlg=false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="importing" @click="importCert">{{ t('node.certImportBtn') }}</el-button>
    </template>
  </el-dialog>
  <!-- 设备明细 -->
  <el-dialog v-model="dlg" :title="t('node.childDevices') + ' - ' + (cur?.name || '')" width="900px">
    <el-table :data="devices" stripe size="small">
      <el-table-column prop="name" :label="t('common.name')" min-width="130" />
      <el-table-column prop="ip" label="IP" min-width="120" />
      <el-table-column prop="type" :label="t('node.type')" width="90" />
      <el-table-column prop="vendor" :label="t('node.vendor')" width="90" />
      <el-table-column :label="t('common.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.last_online ? 'success' : 'info'" size="small">{{ row.last_online ? t('node.online') : t('node.offline') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('node.cpu')" width="80">
        <template #default="{ row }">{{ row.last_cpu != null ? row.last_cpu.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('node.mem')" width="80">
        <template #default="{ row }">{{ row.last_mem != null ? row.last_mem.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('node.uptime')" min-width="110">
        <template #default="{ row }">{{ fmtUptime(row.last_uptime) }}</template>
      </el-table-column>
    </el-table>
  </el-dialog>
  <!-- 容器明细 -->
  <el-dialog v-model="ctnDlg" :title="t('node.childContainers') + ' - ' + (cur?.name || '')" width="820px">
    <el-table :data="containers" stripe size="small">
      <el-table-column prop="name" :label="t('common.name')" min-width="130" />
      <el-table-column prop="image" :label="t('node.image')" min-width="150" />
      <el-table-column prop="state" :label="t('node.state')" width="90" />
      <el-table-column :label="t('node.cpu')" width="80">
        <template #default="{ row }">{{ row.cpu != null ? row.cpu.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('node.mem')" width="80">
        <template #default="{ row }">{{ row.mem_pct != null ? row.mem_pct.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column prop="host" :label="t('node.host')" min-width="120" />
    </el-table>
  </el-dialog>
  <!-- 链路明细 -->
  <el-dialog v-model="linkDlg" :title="t('node.childLinks') + ' - ' + (cur?.name || '')" width="720px">
    <el-table :data="links" stripe size="small">
      <el-table-column prop="name" :label="t('common.name')" min-width="130" />
      <el-table-column prop="target" :label="t('node.target')" min-width="140" />
      <el-table-column prop="method" :label="t('node.method')" width="90" />
      <el-table-column prop="port" :label="t('node.port')" width="80" />
      <el-table-column :label="t('common.status')" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status==='up'?'success':'info'" size="small">{{ row.status === 'up' ? t('node.up') : (row.status === 'down' ? t('node.down') : '-') }}</el-tag>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<style scoped>
.node-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.node-list-head .np-card-title { margin-bottom: 0; }
.cert-wait-tip { color: var(--np-warning); font-size: 12px; line-height: 1.5; max-width: 520px; }

.dialog-warn { color: var(--np-warning); font-size: 12px; margin-bottom: 8px; line-height: 1.6; }
.dialog-target { display: block; color: var(--np-text-2); margin-top: 4px; }
.pem-input :deep(.el-textarea__inner) { font-family: Consolas, monospace; font-size: 12px; }
.upload-row { margin: 8px 0; }
.import-result { margin: 8px 0; font-size: 13px; line-height: 1.6; }
.import-result.ok { color: var(--np-success); }
.import-result.fail { color: var(--np-danger); }
.import-meta { color: var(--np-text-2); font-size: 12px; word-break: break-all; margin-top: 2px; }
</style>
