<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { fmtTime } from '@/utils'
import UserManagement from './components/UserManagement.vue'
import AIConfig from './components/AIConfig.vue'

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
  if (emailForm.type === 'email' && !emailForm.smtp_host) return ElMessage.warning(t('system.smtpRequired'))
  if (emailForm.type !== 'email' && !emailForm.webhook) return ElMessage.warning(t('system.webhookRequired'))
  if (emailForm.id) await putEnc(`/system/email/${emailForm.id}`, emailForm); else await postEnc('/system/email', emailForm)
  ElMessage.success(t('system.saved')); emailDialog.value = false; await loadEmails()
}
async function removeEmail(e: EmailConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/email/${e.id}`); ElMessage.success(t('common.success')); await loadEmails()
}
async function testEmail(e: EmailConfig) { await postEnc(`/system/email/${e.id}/test`, {}); ElMessage.success(t('system.testSent')) }
function webhookPlaceholder() {
  switch (emailForm.type) {
    case 'dingtalk': return 'https://oapi.dingtalk.com/robot/send?access_token=xxx'
    case 'wecom': return 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx'
    case 'feishu': return 'https://open.feishu.cn/open-apis/bot/v2/hook/xxx'
    case 'sms': return t('system.smsApi')
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
  const parts = p.replace('/api/v1/', '').split('/')
  const map: Record<string, string> = { auth:'modAuth', linkdetect:'modLinkdetect', topo:'modTopo', monitor:'modMonitor', dbmonitor:'modDbmonitor', resource:'modResource', script:'modScript', ipam:'modIpam', report:'modReport', traffic:'modTraffic', system:'modSystem', docker:'modDocker', k8s:'modK8s', dashboard:'modDashboard', user:'modUser' }
  const k = map[parts[0]]
  return k ? t('system.' + k) : parts[0] || '-'
}

// Security
const secForm = reactive({ tls_cert: '', tls_key: '', pwd_max_days: 90, login_max_fail: 5, login_lock_min: 15, audit_retention_days: 180, max_session_per_user: 1 })
async function loadSec() { try { Object.assign(secForm, await getEnc('/system/security')) } catch {} }
async function saveSec() { await putEnc('/system/security', secForm); ElMessage.success(t('system.saved')) }
async function downloadBackup() {
  const r = await fetch('/api/v1/system/backup', { headers: { Authorization: 'Bearer ' + localStorage.getItem('np-token') } })
  const blob = await r.blob(); const a = document.createElement('a')
  a.href = URL.createObjectURL(blob); a.download = 'netops-backup.zip'; a.click()
}

// Update
const ver = ref<any>({})
const uploadResult = ref<any>(null)
const applying = ref(false)

const roleList = ref([
  { key: 'admin', name: 'admin', desc: 'adminDesc', perms: ['*'] },
  { key: 'operator', name: 'operator', desc: 'operatorDesc', perms: ['monitor', 'alert:ack', 'config:backup', 'report:view', 'topo:edit'] },
  { key: 'viewer', name: 'viewer', desc: 'viewerDesc', perms: ['dashboard:view', 'monitor:view', 'report:view'] }
])
const allPerms = ['dashboard:view', 'monitor:view', 'monitor:edit', 'alert:view', 'alert:ack', 'alert:edit',
  'config:backup', 'config:rollback', 'report:view', 'report:edit', 'topo:edit', 'user:edit', 'system:edit', 'node:edit']
async function saveRoles() { ElMessage.success(t('system.saved')) }
const checking = ref(false), upgrading = ref(false), latest = ref<any>(null)
async function loadVersion() { try { ver.value = await getEnc('/update/version') } catch {} }
async function checkUpdate() { checking.value = true; try { latest.value = await getEnc('/update/check') } catch(e:any){ ElMessage.error(e.message||t('system.checkFail')) } finally { checking.value = false } }
async function doOnline() { upgrading.value = true; try { await postEnc('/update/online', { url: latest.value.download_url }); ElMessage.success(t('system.upgrading')); setTimeout(()=>{ localStorage.clear(); location.href='/login'; }, 6000) } catch(e:any){ ElMessage.error(e.message||t('system.upgradeFail')) } finally { upgrading.value = false } }
async function doUpload(opt: any) {
  const fd = new FormData(); fd.append('file', opt.file)
  const r = await fetch('/api/v1/update/upload', { method: 'POST', headers: { Authorization: 'Bearer ' + localStorage.getItem('np-token') || '' }, body: fd })
  const j = await r.json()
  if (j.code === 0) { uploadResult.value = j.data; ElMessage.success(t('system.uploadOk')) } else ElMessage.error(j.message || t('system.uploadFail'))
}
async function doApply() { applying.value = true; try { await postEnc('/update/apply', { tmp_path: uploadResult.value.tmp_path }); ElMessage.success(t('system.applyDone')); setTimeout(()=>{ localStorage.clear(); location.href='/login'; }, 4000) } finally { applying.value = false } }

onMounted(() => { loadMCP(); loadEmails(); loadAudits(); loadSec(); loadVersion() })
</script>

<template>
  <div class="np-page">
    <el-tabs v-model="activeTab">
      <el-tab-pane :label="t('system.users')" name="users"><UserManagement /></el-tab-pane>
      <el-tab-pane :label="t('system.roles')" name="roles">
        <div class="np-card">
          <el-table :data="roleList" stripe>
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
                  <el-checkbox v-for="p in allPerms" :key="p" :value="p" style="margin-right:8px">{{ p }}</el-checkbox>
                </el-checkbox-group>
              </template>
            </el-table-column>
          </el-table>
          <el-button style="margin-top:12px" type="primary" @click="saveRoles">{{ t('system.savePerms') }}</el-button>
        </div>
      </el-tab-pane>
      <el-tab-pane :label="t('system.ai')" name="ai"><AIConfig /></el-tab-pane>

      <el-tab-pane :label="t('system.mcp')" name="mcp">
        <div class="np-toolbar"><span class="np-page-desc">{{ t('system.mcpTitle') }}</span><div class="spacer"></div>
          <el-button type="primary" @click="openMCP()">{{ t('system.add') }}</el-button></div>
        <el-alert v-if="mcpInfo" :closable="false" type="info" style="margin-bottom:12px" :title="t('system.mcpEndpoint', { s: mcpInfo.server, e: mcpInfo.tools_endpoint })" />
        <div class="np-card">
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
          <div v-if="mcpOut" style="margin-top:10px;background:#0b1220;color:#7dd3fc;padding:10px;border-radius:4px;font-size:12px;white-space:pre-wrap">{{ mcpOut }}</div>
        </div>

        <div class="np-card" style="margin-top:12px">
          <h4 style="margin:0 0 8px">{{ t('system.mcpConnect') }}</h4>
          <p style="color:#888;font-size:12px;margin:0 0 12px">{{ t('system.mcpConnectTip') }}</p>
          <el-tabs v-model="mcpAgentTab">
            <el-tab-pane v-for="a in mcpAgentTypes" :key="a" :label="a" :name="a" />
          </el-tabs>
          <div style="position:relative">
            <pre style="background:#0b1220;color:#7dd3fc;padding:14px;padding-right:80px;border-radius:6px;font-size:13px;overflow:auto;margin:0">{{ mcpConfigJson }}</pre>
            <el-button size="small" style="position:absolute;right:8px;top:8px" @click="copyMcp">{{ t('system.copy') }}</el-button>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.notifyChannel')" name="email">
        <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="openEmail()">{{ t('system.add') }}</el-button></div>
        <div class="np-card">
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

      <el-tab-pane :label="t('system.security')" name="security">
        <div class="np-card" style="max-width:720px">
          <h3 style="margin:0 0 16px">HTTPS / TLS</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item :label="t('system.certPath')"><el-input v-model="secForm.tls_cert" /></el-form-item>
            <el-form-item :label="t('system.keyPath')"><el-input v-model="secForm.tls_key" /></el-form-item>
          </el-form>
          <el-divider /><h3 style="margin:0 0 16px">{{ t('system.pwdPolicy') }}</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item :label="t('system.pwdMaxDays')"><el-input-number v-model="secForm.pwd_max_days" :min="30" :max="365" /></el-form-item>
          </el-form>
          <el-divider /><h3 style="margin:0 0 16px">{{ t('system.loginGuard') }}</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item :label="t('system.maxFail')"><el-input-number v-model="secForm.login_max_fail" :min="3" :max="10" /></el-form-item>
            <el-form-item :label="t('system.lockMin')"><el-input-number v-model="secForm.login_lock_min" :min="5" :max="60" /></el-form-item>
          </el-form>
          <el-button type="warning" @click="downloadBackup">{{ t('system.downloadBackup') }}</el-button>
          <el-button type="primary" @click="saveSec" style="margin-left:10px">{{ t('system.save') }}</el-button>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.audit')" name="audit">
        <div class="np-card">
          <el-table :data="audits" stripe size="small" class="np-table">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="username" :label="t('system.auditUser')" width="110" />
            <el-table-column prop="action" :label="t('system.action')" min-width="140" />
            <el-table-column :label="t('system.module')" width="110"><template #default="{ row }">{{ moduleFromPath(row.path) }}</template></el-table-column>
            <el-table-column prop="created_at" :label="t('system.time')" width="170"><template #default="{ row }">{{ fmtTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.sysUpdate')" name="update">
        <div class="np-card" style="padding:24px">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item :label="t('system.version')">{{ ver.version || '--' }}</el-descriptions-item>
            <el-descriptions-item :label="t('system.buildTime')">{{ ver.build || '--' }}</el-descriptions-item>
          </el-descriptions>
          <el-divider />
          <el-button :loading="checking" @click="checkUpdate">{{ t('system.checkUpdate') }}</el-button>
          <div v-if="latest" style="margin-top:12px;padding:12px;background:#f0f9ff;border-radius:6px">
            <div>{{ t('system.latestVer') }}: <b>{{ latest.latest }}</b></div>
            <el-button v-if="latest.has_update" type="primary" style="margin-top:10px" :loading="upgrading" @click="doOnline">{{ t('system.upgradeNow') }}</el-button>
          </div>
          <el-upload :http-request="doUpload" :show-file-list="false" style="margin-top:20px" drag>
            <div>{{ t('system.uploadHint') }}</div>
          </el-upload>
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
        <el-form-item :label="t('system.channelType')">
          <el-select v-model="emailForm.type" style="width:100%" :placeholder="t('system.emailSmtp')">
            <el-option :label="t('system.emailSmtp')" value="email" />
            <el-option :label="t('system.dingtalk')" value="dingtalk" />
            <el-option :label="t('system.wecom')" value="wecom" />
            <el-option :label="t('system.feishu')" value="feishu" />
            <el-option :label="t('system.sms')" value="sms" />
            <el-option :label="t('system.webhook')" value="webhook" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('system.smtpHost')" v-if="emailForm.type==='email'"><el-input v-model="emailForm.smtp_host" placeholder="smtp.qq.com / smtp.163.com" /></el-form-item>
        <el-form-item :label="t('system.smtpPort')" v-if="emailForm.type==='email'"><el-input-number v-model="emailForm.smtp_port" /></el-form-item>
        <el-form-item :label="t('system.account')" v-if="emailForm.type==='email'"><el-input v-model="emailForm.user" :placeholder="t('system.senderEmail')" /></el-form-item>
        <el-form-item :label="t('dbmigrate.password')" v-if="emailForm.type==='email'"><el-input v-model="emailForm.password" type="password" show-password :placeholder="t('system.authCode')" /></el-form-item>
        <el-form-item label="Webhook" v-else><el-input v-model="emailForm.webhook" :placeholder="webhookPlaceholder()" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="emailForm.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="emailDialog=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="saveEmail">{{ t('system.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
