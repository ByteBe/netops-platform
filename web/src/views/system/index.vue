<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { fmtTime } from '@/utils'
import UserManagement from './components/UserManagement.vue'
import AIConfig from './components/AIConfig.vue'
import CertificatePanel from './components/CertificatePanel.vue'

const route = useRoute()
const { t } = useI18n()
const activeTab = ref((route.query.tab as string) || 'users')

// MCP
interface MCPAgent { id: number; name: string; transport: string; command: string; args: string; server_url: string; endpoint: string; auth_type: string; auth_token: string; enable: boolean; remark: string }
const mcpAgents = ref<MCPAgent[]>([])
const mcpInfo = ref<any>(null)
const mcpDialog = ref(false)
const mcpForm = reactive<MCPAgent>({ id: 0, name: '', transport: 'stdio', command: 'npx', args: '', server_url: '', endpoint: '/api', auth_type: 'none', auth_token: '', enable: true, remark: '' })
const mcpOut = ref('')
const mcpAgentTab = ref('Claude Code')
const mcpAgentTypes = ['Claude Code', 'Cursor', 'CodeBuddy Code', 'ZCode', 'TRAE', 'VS Code', 'Windsurf', 'Codex', 'DeepSeek Harness', 'OpenCode', 'Pi', 'Cherry']
const mcpConfigJson = computed(() => {  const srv = mcpInfo.value?.server || 'http://127.0.0.1:30821'
  const ep = mcpInfo.value?.tools_endpoint || '/mcp/tools'
  const url = srv + ep
  const map: Record<string, string> = {
    'Claude Code': JSON.stringify({ mcpServers: { netops: { type: 'sse', url } } }, null, 2),
    'Cursor': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'CodeBuddy Code': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'ZCode': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'TRAE': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'VS Code': JSON.stringify({ servers: { netops: { type: 'sse', url } } }, null, 2),
    'Windsurf': JSON.stringify({ mcpServers: { netops: { serverUrl: url } } }, null, 2),
    'Codex': 'netops --url ' + url,
    'DeepSeek Harness': JSON.stringify({ mcp: { netops: { url } } }, null, 2),
    'OpenCode': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'Pi': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2),
    'Cherry': JSON.stringify({ mcpServers: { netops: { url } } }, null, 2)
  }
  return map[mcpAgentTab.value] || url
})
async function copyMcp() {
  try { await navigator.clipboard.writeText(mcpConfigJson.value); ElMessage.success(t('system.copied')) }
  catch { ElMessage.warning(t('system.copyFail')) }
}
async function loadMCP() {
  mcpAgents.value = await getEnc<MCPAgent[]>('/system/mcp/agents')
  try { mcpInfo.value = await getEnc('/system/mcp/info') } catch {}
}
function openMCP(m?: MCPAgent) {
  mcpOut.value = ''
  if (m) Object.assign(mcpForm, m)
  else Object.assign(mcpForm, { id: 0, name: '', server_url: '', endpoint: '/api', auth_type: 'none', auth_token: '', enable: true, remark: '' })
  mcpDialog.value = true
}
async function saveMCP() {
  if (!mcpForm.name || !mcpForm.server_url) return ElMessage.warning(t('common.tip'))
  if (mcpForm.id) await putEnc(`/system/mcp/agents/${mcpForm.id}`, mcpForm); else await postEnc('/system/mcp/agents', mcpForm)
  ElMessage.success(t('common.success')); mcpDialog.value = false; await loadMCP()
}
async function removeMCP(m: MCPAgent) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/mcp/agents/${m.id}`); ElMessage.success(t('common.success')); await loadMCP()
}
async function testMCP(m: MCPAgent) { const r = await postEnc<any>(`/system/mcp/agents/${m.id}/test`, {}); mcpOut.value = r.output }

// Email
interface EmailConfig { id: number; name: string; type?: string; smtp_host: string; smtp_port: number; user: string; password: string; use_ssl: boolean; enable: boolean; default_to: string; webhook?: string }
const emailList = ref<EmailConfig[]>([])
const emailDialog = ref(false)
const emailForm = reactive<EmailConfig>({ id: 0, name: '', smtp_host: '', smtp_port: 465, user: '', password: '', use_ssl: true, enable: false, default_to: '' })
async function loadEmails() { emailList.value = await getEnc<EmailConfig[]>('/system/email/list') }
function openEmail(e?: EmailConfig) {
  if (e) Object.assign(emailForm, e); else Object.assign(emailForm, { id: 0, name: '', type: 'email', smtp_host: '', smtp_port: 465, user: '', password: '', use_ssl: true, enable: false, default_to: '', webhook: '' })
  emailDialog.value = true
}
async function saveEmail() {
  if (!emailForm.name) return ElMessage.warning(t('system.nameRequired'))
  if (!emailForm.smtp_host) return ElMessage.warning(t('system.smtpRequired'))
  if (emailForm.id) await putEnc(`/system/email/${emailForm.id}`, emailForm); else await postEnc('/system/email', emailForm)
  ElMessage.success(t('system.saved')); emailDialog.value = false; await loadEmails()
}
async function removeEmail(e: EmailConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/email/${e.id}`); ElMessage.success(t('common.success')); await loadEmails()
}
async function testEmail(e: EmailConfig) {
  try {
    const to = await ElMessageBox.prompt('请输入收件人邮箱', '发送测试邮件', {
      inputValue: e.default_to || e.user,
      confirmButtonText: '发送',
      cancelButtonText: '取消',
      inputPattern: /^[^@]+@[^@]+$/,
      inputErrorMessage: '邮箱格式不正确'
    })
    await postEnc(`/system/email/${e.id}/test`, { to: to.value })
    ElMessage.success('测试邮件已发送，请查收')
  } catch (err: any) {
    if (err !== 'cancel' && err?.message) ElMessage.error(err.message)
  }
}
// Notification channels (email receiver / webhook push targets)
interface NotifyConfig { id: number; name: string; type: string; addr: string; token?: string; enabled: boolean }
const notifyList = ref<NotifyConfig[]>([])
const notifyDialog = ref(false)
const notifyForm = reactive<NotifyConfig>({ id: 0, name: '', type: 'email', addr: '', enabled: true })
async function loadNotify() { notifyList.value = await getEnc<NotifyConfig[]>('/alert/notify') }
function openNotify(n?: NotifyConfig) {
  if (n) Object.assign(notifyForm, n)
  else Object.assign(notifyForm, { id: 0, name: '', type: 'email', addr: '', enabled: true })
  notifyDialog.value = true
}
async function saveNotify() {
  if (!notifyForm.name) return ElMessage.warning(t('system.nameRequired'))
  if (!notifyForm.addr) return ElMessage.warning(t('system.addrRequired'))
  if (notifyForm.id) await putEnc(`/alert/notify/${notifyForm.id}`, notifyForm); else await postEnc('/alert/notify', notifyForm)
  ElMessage.success(t('system.saved')); notifyDialog.value = false; await loadNotify()
}
async function removeNotify(n: NotifyConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/alert/notify/${n.id}`); ElMessage.success(t('common.success')); await loadNotify()
}
function notifyTypeLabel(tp: string) {
  switch (tp) {
    case 'email': return t('system.emailChannel')
    case 'feishu': return t('system.feishu')
    case 'dingtalk': return t('system.dingtalk')
    case 'wecom': return t('system.wecom')
    default: return tp
  }
}
function notifyAddrPlaceholder() {
  switch (notifyForm.type) {
    case 'email': return 'receiver@example.com'
    case 'feishu': return 'https://open.feishu.cn/open-apis/bot/v2/hook/xxx'
    case 'dingtalk': return 'https://oapi.dingtalk.com/robot/send?access_token=xxx'
    case 'wecom': return 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx'
    default: return 'https://example.com/webhook'
  }
}

// Audit
const audits = ref<any[]>([])
async function loadAudits() {
  try { const r = await getEnc<any>('/system/audit?page=1&size=200'); audits.value = r.list || [] } catch {}
}
function moduleFromPath(p?: string): string {
  if (!p) return '-'
  const parts = p.split('/').filter(Boolean)
  let seg = parts[0] || ''
  if (seg === 'api' && parts[1] === 'v1') seg = parts[2] || ''
  const map: Record<string, string> = { auth:'modAuth', linkdetect:'modLinkdetect', monitor:'modMonitor', dbmonitor:'modDbmonitor', resource:'modResource', script:'modScript', ipam:'modIpam', report:'modReport', traffic:'modTraffic', system:'modSystem', docker:'modDocker', k8s:'modK8s', dashboard:'modDashboard', user:'modUser', containermon:'modContainermon', nginxmon:'modNginxmon', bgpmon:'modBgpmon', alert:'modAlert', distributed:'modDistributed', devicelog:'modDevicelog', tasklog:'modTasklog', subnet:'modSubnet', setup:'modSetup', aiinspect:'modAiinspect' }
  const k = map[seg]
  return k ? t('system.' + k) : seg || '-'
}

// Update
const ver = ref<any>({})
const uploadResult = ref<any>(null)
const uploadPercent = ref(0)
const uploading = ref(false)
const applying = ref(false)

const roleList = ref([
  { key: 'admin', name: 'admin', desc: 'adminDesc', perms: ['*'] },
  { key: 'operator', name: 'operator', desc: 'operatorDesc', perms: ['monitor', 'alert:ack', 'config:backup', 'report:view'] },
  { key: 'viewer', name: 'viewer', desc: 'viewerDesc', perms: ['dashboard:view', 'monitor:view', 'report:view'] }
])
const allPerms = ['dashboard:view', 'monitor:view', 'monitor:edit', 'alert:view', 'alert:ack', 'alert:edit',
  'config:backup', 'config:rollback', 'report:view', 'report:edit', 'user:edit', 'system:edit', 'node:edit']
const permKeys: Record<string, string> = {
  'dashboard:view': 'permDashboardView', 'monitor:view': 'permMonitorView', 'monitor:edit': 'permMonitorEdit',
  'alert:view': 'permAlertView', 'alert:ack': 'permAlertAck', 'alert:edit': 'permAlertEdit',
  'config:backup': 'permConfigBackup', 'config:rollback': 'permConfigRollback',
  'report:view': 'permReportView', 'report:edit': 'permReportEdit',
  'user:edit': 'permUserEdit', 'system:edit': 'permSystemEdit', 'node:edit': 'permNodeEdit'
}
function permLabel(p: string): string {
  const key = permKeys[p]
  return key ? t('system.' + key) : p
}
async function saveRoles() { ElMessage.success(t('system.saved')) }
const checking = ref(false), upgrading = ref(false), latest = ref<any>(null)
async function loadVersion() { try { ver.value = await getEnc('/update/version') } catch {} }
async function checkUpdate() { checking.value = true; try { latest.value = await getEnc('/update/check') } catch(e:any){ ElMessage.error(e.message||t('system.checkFail')) } finally { checking.value = false } }
function isRestartNetworkError(e: any) {
  const msg = String(e?.message || e || '')
  return /Network Error|ERR_CONNECTION|timeout|Failed to fetch|Load failed/i.test(msg)
}
function redirectAfterRestart(delay = 6000) {
  ElMessage.success(t('system.upgrading'))
  setTimeout(() => { localStorage.clear(); location.href = '/login' }, delay)
}
async function doOnline() { upgrading.value = true; try { await postEnc('/update/online', { url: latest.value.download_url }); redirectAfterRestart(6000) } catch(e:any){ if (isRestartNetworkError(e)) { redirectAfterRestart(6000) } else { ElMessage.error(e.message||t('system.upgradeFail')); upgrading.value = false } } }
async function doUpload(opt: any) {
  if (!opt.file?.name?.toLowerCase().endsWith('.zip')) {
    ElMessage.warning('请上传 ZIP 更新包')
    return
  }
  uploading.value = true
  uploadPercent.value = 0
  uploadResult.value = null
  const fd = new FormData(); fd.append('file', opt.file)
  const xhr = new XMLHttpRequest()
  xhr.open('POST', '/api/v1/update/upload')
  xhr.setRequestHeader('Authorization', 'Bearer ' + (localStorage.getItem('np-token') || ''))
  xhr.setRequestHeader('X-Timestamp', String(Math.floor(Date.now() / 1000)))
  xhr.setRequestHeader('X-Nonce', Math.random().toString(36).slice(2, 15) + Date.now().toString(36))
  xhr.upload.onprogress = (e) => { if (e.lengthComputable) uploadPercent.value = Math.round(e.loaded / e.total * 100) }
  xhr.onload = () => {
    uploading.value = false
    try {
      const j = JSON.parse(xhr.responseText)
      if (j.code === 0) {
        uploadResult.value = j.data
        ElMessage.success(`${t('system.uploadOk')}（${(j.data.size/1024/1024).toFixed(2)} MB，完整性校验通过）`)
      } else ElMessage.error(j.message || t('system.uploadFail'))
    } catch { ElMessage.error(t('system.uploadFail')) }
  }
  xhr.onerror = () => { uploading.value = false; ElMessage.error(t('system.uploadFail')) }
  xhr.send(fd)
}
async function doApply() { applying.value = true; try { await postEnc('/update/apply', { tmp_path: uploadResult.value.tmp_path }); redirectAfterRestart(4000) } catch(e:any){ if (isRestartNetworkError(e)) { redirectAfterRestart(4000) } else { ElMessage.error(e.message||t('system.upgradeFail')); applying.value = false } } }
async function doRollback() {
  await ElMessageBox.confirm('回滚后系统将重启并恢复到上一版本，是否继续？', '回滚确认', { type: 'warning' })
  try { await postEnc('/update/rollback', {}); redirectAfterRestart(4000) } catch(e:any){ if (isRestartNetworkError(e)) redirectAfterRestart(4000); else ElMessage.error(e.message||t('system.upgradeFail')) }
}

let autoTimer: number | undefined
onMounted(() => { loadMCP(); loadEmails(); loadNotify(); loadAudits(); loadVersion(); autoTimer = window.setInterval(loadAudits, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>

<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('system.modSystem') }}</div>
        <div class="np-page-head-desc">{{ t('system.pageDesc') }}</div>
      </div>
    </div>
    <el-tabs v-model="activeTab">
      <el-tab-pane :label="t('system.users')" name="users"><UserManagement /></el-tab-pane>
      <el-tab-pane :label="t('system.roles')" name="roles">
        <div class="np-card">
          <div class="np-card-title">{{ t('system.roles') }}</div>
          <el-table :data="roleList" stripe class="np-table">
            <el-table-column prop="key" :label="t('system.roleKey')" width="120" />
            <el-table-column :label="t('system.roleName')" width="140">
              <template #default="{ row }">{{ t('system.' + row.name) }}</template>
            </el-table-column>
            <el-table-column :label="t('system.desc')" min-width="200">
              <template #default="{ row }">{{ t('system.' + row.desc) }}</template>
            </el-table-column>
            <el-table-column :label="t('system.perms')" min-width="400">
              <template #default="{ row }">
                <el-checkbox-group v-model="row.perms" :disabled="row.key==='admin'">
                  <el-checkbox v-for="p in allPerms" :key="p" :value="p" style="margin-right:8px">{{ permLabel(p) }}</el-checkbox>
                </el-checkbox-group>
              </template>
            </el-table-column>
          </el-table>
          <el-button style="margin-top:12px" type="primary" @click="saveRoles">{{ t('system.savePerms') }}</el-button>
        </div>
      </el-tab-pane>
      <el-tab-pane :label="t('system.ai')" name="ai"><AIConfig /></el-tab-pane>

      <el-tab-pane :label="t('system.mcp')" name="mcp">
        <div class="np-card">
          <div class="card-head-row">
            <div class="np-card-title">{{ t('system.mcpTitle') }}</div>
            <el-button type="primary" @click="openMCP()">{{ t('system.add') }}</el-button>
          </div>
          <el-alert v-if="mcpInfo" :closable="false" type="info" style="margin-bottom:12px" :title="t('system.mcpEndpoint', { s: mcpInfo.server, e: mcpInfo.tools_endpoint })" />
          <el-table :data="mcpAgents" stripe class="np-table">
            <el-table-column prop="name" :label="t('common.name')" min-width="110" />
            <el-table-column prop="server_url" :label="t('system.serverUrl')" min-width="220" />
            <el-table-column prop="auth_type" :label="t('system.auth')" width="90" />
            <el-table-column :label="t('common.enabled')" width="80"><template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?t('system.on'):t('system.off') }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="200">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testMCP(row)">{{ t('system.test') }}</el-button>
                <el-button size="small" text type="primary" @click="openMCP(row)">{{ t('system.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeMCP(row)">{{ t('system.del') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="mcpOut" class="code-out">{{ mcpOut }}</div>
        </div>

        <div class="np-card card-gap">
          <div class="np-card-title">{{ t('system.mcpConnect') }}</div>
          <p class="card-sub">{{ t('system.mcpConnectTip') }}</p>
          <el-tabs v-model="mcpAgentTab">
            <el-tab-pane v-for="a in mcpAgentTypes" :key="a" :label="a" :name="a" />
          </el-tabs>
          <div class="code-wrap">
            <pre class="code-pre">{{ mcpConfigJson }}</pre>
            <el-button size="small" class="code-copy-btn" @click="copyMcp">{{ t('system.copy') }}</el-button>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.notifyChannel')" name="email">
        <div class="np-card">
          <div class="card-head-row">
            <div class="np-card-title">{{ t('system.channels') }}</div>
            <el-button type="primary" @click="openNotify()">{{ t('system.add') }}</el-button>
          </div>
          <el-table :data="notifyList" stripe class="np-table">
            <el-table-column prop="name" :label="t('common.name')" min-width="110" />
            <el-table-column :label="t('system.channelType')" width="120"><template #default="{ row }">{{ notifyTypeLabel(row.type) }}</template></el-table-column>
            <el-table-column prop="addr" :label="t('system.channelAddr')" min-width="220" show-overflow-tooltip />
            <el-table-column :label="t('common.enabled')" width="80"><template #default="{ row }"><el-tag :type="row.enabled?'success':'info'" size="small">{{ row.enabled?t('system.on'):t('system.off') }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="130">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="openNotify(row)">{{ t('system.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeNotify(row)">{{ t('system.del') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="np-card card-gap">
          <div class="card-head-row">
            <div class="np-card-title">{{ t('system.smtpAccounts') }}</div>
            <el-button type="primary" @click="openEmail()">{{ t('system.add') }}</el-button>
          </div>
          <el-table :data="emailList" stripe class="np-table">
            <el-table-column prop="name" :label="t('common.name')" min-width="110" />
            <el-table-column prop="smtp_host" label="SMTP" min-width="180" />
            <el-table-column prop="smtp_port" :label="t('dbmigrate.port')" width="70" />
            <el-table-column prop="user" :label="t('system.account')" min-width="160" />
            <el-table-column :label="t('common.enabled')" width="80"><template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?t('system.on'):t('system.off') }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="200">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testEmail(row)">{{ t('system.test') }}</el-button>
                <el-button size="small" text type="primary" @click="openEmail(row)">{{ t('system.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeEmail(row)">{{ t('system.del') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.nodeCert')" name="cert">
        <CertificatePanel />
      </el-tab-pane>

      <el-tab-pane :label="t('system.audit')" name="audit">
        <div class="np-card">
          <div class="np-card-title">{{ t('system.audit') }}</div>
          <el-table :data="audits" stripe size="small" class="np-table">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="username" :label="t('system.auditUser')" width="110" />
            <el-table-column prop="action" :label="t('system.action')" min-width="200" show-overflow-tooltip />
            <el-table-column :label="t('system.module')" width="110"><template #default="{ row }">{{ moduleFromPath(row.path) }}</template></el-table-column>
            <el-table-column prop="created_at" :label="t('system.time')" width="170"><template #default="{ row }">{{ fmtTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.sysUpdate')" name="update">
        <div class="np-card update-card">
          <div class="np-card-title">{{ t('system.sysUpdate') }}</div>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item :label="t('system.version')">{{ ver.version || '--' }}</el-descriptions-item>
            <el-descriptions-item :label="t('system.buildTime')">{{ ver.build || '--' }}</el-descriptions-item>
          </el-descriptions>
          <el-divider />
          <div class="update-actions">
            <el-button :loading="checking" @click="checkUpdate">{{ t('system.checkUpdate') }}</el-button>
            <el-button type="warning" @click="doRollback">{{ t('system.rollbackUpdate') }}</el-button>
          </div>
          <div v-if="latest" class="latest-box">
            <div>{{ t('system.latestVer') }}: <b>{{ latest.latest }}</b></div>
            <el-button v-if="latest.has_update" type="primary" class="mt-8" :loading="upgrading" @click="doOnline">{{ t('system.upgradeNow') }}</el-button>
          </div>
          <el-upload :http-request="doUpload" :show-file-list="false" accept=".zip" class="mt-16" drag>
            <div>{{ t('system.uploadHint') }}</div>
          </el-upload>
          <el-progress v-if="uploading || uploadPercent > 0" :percentage="uploadPercent" class="mt-12" />
          <div v-if="uploadResult" class="upload-ok">
            已上传 {{ uploadResult.filename }}，大小 {{ (uploadResult.size/1024/1024).toFixed(2) }} MB，ZIP 完整性校验通过
            已上传 {{ uploadResult.filename }}，大小 {{ (uploadResult.size/1024/1024).toFixed(2) }} MB，ZIP 完整性校验通过
          </div>
          <el-button v-if="uploadResult" type="danger" style="margin-top:10px" :loading="applying" @click="doApply">{{ t('system.confirmApply') }}</el-button>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="mcpDialog" title="MCP Server" width="640px">
      <el-form :model="mcpForm" label-width="100px" label-position="top">
        <el-form-item :label="t('common.name')"><el-input v-model="mcpForm.name" /></el-form-item>
        <el-form-item :label="t('system.accessType')">
          <el-radio-group v-model="mcpForm.transport">
            <el-radio value="stdio">{{ t('system.localStdio') }}</el-radio>
            <el-radio value="http">{{ t('system.httpService') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="mcpForm.transport==='stdio'" :label="t('system.cmdPath')"><el-input v-model="mcpForm.command" placeholder="npx / C:\xxx\mcp.exe" /></el-form-item>
        <el-form-item v-if="mcpForm.transport==='stdio'" :label="t('system.runArgs')"><el-input v-model="mcpForm.args" /></el-form-item>
        <el-form-item v-if="mcpForm.transport==='http'" :label="t('system.serverUrl')"><el-input v-model="mcpForm.server_url" placeholder="http://127.0.0.1:8080" /></el-form-item>
        <el-form-item :label="t('system.apiPath')"><el-input v-model="mcpForm.endpoint" placeholder="/api" /></el-form-item>
        <el-form-item :label="t('system.authMethod')">
          <el-radio-group v-model="mcpForm.auth_type">
            <el-radio value="none">{{ t('system.none') }}</el-radio>
            <el-radio value="bearer">Bearer Token</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="mcpForm.auth_type==='bearer'" label="Token"><el-input v-model="mcpForm.auth_token" type="password" show-password /></el-form-item>
        <el-form-item :label="t('system.remark')"><el-input v-model="mcpForm.remark" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="mcpForm.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="mcpDialog=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="saveMCP">{{ t('system.save') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="emailDialog" :title="t('system.notifyChannel')" width="520px">
      <el-form :model="emailForm" label-width="110px">
        <el-form-item :label="t('common.name')"><el-input v-model="emailForm.name" /></el-form-item>
        <el-form-item :label="t('system.smtpHost')"><el-input v-model="emailForm.smtp_host" placeholder="smtp.qq.com / smtp.163.com" /></el-form-item>
        <el-form-item :label="t('system.smtpPort')"><el-input-number v-model="emailForm.smtp_port" /></el-form-item>
        <el-form-item :label="t('system.account')"><el-input v-model="emailForm.user" :placeholder="t('system.senderEmail')" /></el-form-item>
        <el-form-item :label="t('system.authCode')"><el-input v-model="emailForm.password" type="password" show-password :placeholder="t('system.authCode')" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="emailForm.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="emailDialog=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="saveEmail">{{ t('system.save') }}</el-button></template>
    </el-dialog>
    <el-dialog v-model="notifyDialog" :title="t('system.channels')" width="480px">
      <el-form :model="notifyForm" label-width="90px">
        <el-form-item :label="t('common.name')"><el-input v-model="notifyForm.name" /></el-form-item>
        <el-form-item :label="t('system.channelType')">
          <el-select v-model="notifyForm.type" style="width:100%">
            <el-option :label="t('system.emailChannel')" value="email" />
            <el-option :label="t('system.feishu')" value="feishu" />
            <el-option :label="t('system.dingtalk')" value="dingtalk" />
            <el-option :label="t('system.wecom')" value="wecom" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('system.channelAddr')"><el-input v-model="notifyForm.addr" :placeholder="notifyAddrPlaceholder()" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="notifyForm.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="notifyDialog=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="saveNotify">{{ t('system.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.card-head-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.card-head-row .np-card-title { margin-bottom: 0; }
.card-gap { margin-top: 12px; }
.card-sub { color: var(--np-text-2); font-size: 12px; margin: 0 0 12px; }
.mt-12 { margin-top: 12px; }

/* 代码块（输出/配置 JSON）：中性次级表面 + 发丝描边，走主题变量 */
.code-out {
  margin-top: 10px;
  background: var(--np-surface-1);
  color: var(--np-text-1);
  border: 1px solid var(--np-border);
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: Consolas, 'Courier New', monospace;
}
.code-wrap { position: relative; }
.code-pre {
  background: var(--np-surface-1);
  color: var(--np-text-1);
  border: 1px solid var(--np-border);
  padding: 14px 84px 14px 14px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.7;
  overflow: auto;
  margin: 0;
  font-family: Consolas, 'Courier New', monospace;
}
.code-copy-btn { position: absolute; right: 8px; top: 8px; }

.update-card { padding: 20px 24px; }
.update-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.latest-box {
  margin-top: 12px;
  padding: 12px 14px;
  background: var(--np-primary-weak);
  border: 1px solid var(--np-border);
  border-radius: 8px;
}
.upload-ok { margin-top: 8px; color: var(--np-success); font-size: 13px; }
</style>
