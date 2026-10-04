<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
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
interface EmailConfig { id: number; name: string; smtp_host: string; smtp_port: number; user: string; password: string; use_ssl: boolean; enable: boolean; default_to: string }
const emailList = ref<EmailConfig[]>([])
const emailDialog = ref(false)
const emailForm = reactive<EmailConfig>({ id: 0, name: '', smtp_host: '', smtp_port: 465, user: '', password: '', use_ssl: true, enable: false, default_to: '' })
async function loadEmails() { emailList.value = await getEnc<EmailConfig[]>('/system/email/list') }
function openEmail(e?: EmailConfig) {
  if (e) Object.assign(emailForm, e); else Object.assign(emailForm, { id: 0, name: '', smtp_host: '', smtp_port: 465, user: '', password: '', use_ssl: true, enable: false, default_to: '' })
  emailDialog.value = true
}
async function saveEmail() {
  if (!emailForm.name || !emailForm.smtp_host) return ElMessage.warning(t('common.tip'))
  if (emailForm.id) await putEnc(`/system/email/${emailForm.id}`, emailForm); else await postEnc('/system/email', emailForm)
  ElMessage.success(t('common.success')); emailDialog.value = false; await loadEmails()
}
async function removeEmail(e: EmailConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/email/${e.id}`); ElMessage.success(t('common.success')); await loadEmails()
}
async function testEmail(e: EmailConfig) { await postEnc(`/system/email/${e.id}/test`, {}); ElMessage.success('测试邮件已发送') }

// Audit
const audits = ref<any[]>([])
async function loadAudits() {
  try { const r = await getEnc<any>('/system/audit?page=1&size=200'); audits.value = r.list || [] } catch {}
}
function moduleFromPath(p?: string): string {
  if (!p) return '-'
  const parts = p.replace('/api/v1/', '').split('/')
  const map: Record<string, string> = { auth:'认证', linkdetect:'链路检测', topo:'拓扑', monitor:'设备监控', dbmonitor:'数据库监控', resource:'资源管理', script:'脚本生成', ipam:'IP管理', report:'巡检报告', traffic:'流量监控', system:'系统管理', docker:'Docker监控', k8s:'K8s监控', dashboard:'数据大屏', user:'用户' }
  return map[parts[0]] || parts[0] || '-'
}

// Security
const secForm = reactive({ tls_cert: '', tls_key: '', pwd_max_days: 90, login_max_fail: 5, login_lock_min: 15, audit_retention_days: 180, max_session_per_user: 1 })
async function loadSec() { try { Object.assign(secForm, await getEnc('/system/security')) } catch {} }
async function saveSec() { await putEnc('/system/security', secForm); ElMessage.success('保存成功') }
async function downloadBackup() {
  const r = await fetch('/api/v1/system/backup', { headers: { Authorization: 'Bearer ' + localStorage.getItem('np-token') } })
  const blob = await r.blob(); const a = document.createElement('a')
  a.href = URL.createObjectURL(blob); a.download = 'netops-backup.zip'; a.click()
}

// Update
const ver = ref<any>({})
const uploadResult = ref<any>(null)
const applying = ref(false)
const checking = ref(false), upgrading = ref(false), latest = ref<any>(null)
async function loadVersion() { try { ver.value = await getEnc('/update/version') } catch {} }
async function checkUpdate() { checking.value = true; try { latest.value = await getEnc('/update/check') } catch(e:any){ ElMessage.error(e.message||'检查失败') } finally { checking.value = false } }
async function doOnline() { upgrading.value = true; try { await postEnc('/update/online', { url: latest.value.download_url }); ElMessage.success('正在升级'); setTimeout(()=>location.reload(), 8000) } catch(e:any){ ElMessage.error(e.message||'升级失败') } finally { upgrading.value = false } }
async function doUpload(opt: any) {
  const fd = new FormData(); fd.append('file', opt.file)
  const r = await fetch('/api/v1/update/upload', { method: 'POST', headers: { Authorization: 'Bearer ' + localStorage.getItem('np-token') || '' }, body: fd })
  const j = await r.json()
  if (j.code === 0) { uploadResult.value = j.data; ElMessage.success('上传成功') } else ElMessage.error(j.message || '上传失败')
}
async function doApply() { applying.value = true; try { await postEnc('/update/apply', { tmp_path: uploadResult.value.tmp_path }); ElMessage.success('更新完成'); setTimeout(()=>location.reload(), 5000) } finally { applying.value = false } }

onMounted(() => { loadMCP(); loadEmails(); loadAudits(); loadSec(); loadVersion() })
</script>

<template>
  <div class="np-page">
    <el-tabs v-model="activeTab">
      <el-tab-pane :label="t('system.users')" name="users"><UserManagement /></el-tab-pane>
      <el-tab-pane :label="t('system.ai')" name="ai"><AIConfig /></el-tab-pane>

      <el-tab-pane :label="t('system.mcp')" name="mcp">
        <div class="np-toolbar"><span class="np-page-desc">MCP 服务器配置</span><div class="spacer"></div>
          <el-button type="primary" @click="openMCP()">添加</el-button></div>
        <el-alert v-if="mcpInfo" :closable="false" type="info" style="margin-bottom:12px" :title="`本系统 MCP 端点：${mcpInfo.server}${mcpInfo.tools_endpoint}`" />
        <div class="np-card">
          <el-table :data="mcpAgents" stripe class="np-table">
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column prop="server_url" label="服务器地址" min-width="220" />
            <el-table-column prop="auth_type" label="认证" width="90" />
            <el-table-column :label="t('common.enabled')" width="80"><template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?'开':'关' }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="200">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testMCP(row)">测试</el-button>
                <el-button size="small" text type="primary" @click="openMCP(row)">编辑</el-button>
                <el-button size="small" text type="danger" @click="removeMCP(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="mcpOut" style="margin-top:10px;background:#0b1220;color:#7dd3fc;padding:10px;border-radius:4px;font-size:12px;white-space:pre-wrap">{{ mcpOut }}</div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.email')" name="email">
        <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="openEmail()">添加</el-button></div>
        <div class="np-card">
          <el-table :data="emailList" stripe class="np-table">
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column prop="smtp_host" label="SMTP" min-width="180" />
            <el-table-column prop="smtp_port" label="端口" width="70" />
            <el-table-column prop="user" label="账号" min-width="160" />
            <el-table-column :label="t('common.enabled')" width="80"><template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?'开':'关' }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="200">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testEmail(row)">测试</el-button>
                <el-button size="small" text type="primary" @click="openEmail(row)">编辑</el-button>
                <el-button size="small" text type="danger" @click="removeEmail(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="安全配置" name="security">
        <div class="np-card" style="max-width:720px">
          <h3 style="margin:0 0 16px">HTTPS / TLS</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item label="证书路径"><el-input v-model="secForm.tls_cert" /></el-form-item>
            <el-form-item label="私钥路径"><el-input v-model="secForm.tls_key" /></el-form-item>
          </el-form>
          <el-divider /><h3 style="margin:0 0 16px">密码策略</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item label="有效期(天)"><el-input-number v-model="secForm.pwd_max_days" :min="30" :max="365" /></el-form-item>
          </el-form>
          <el-divider /><h3 style="margin:0 0 16px">登录防护</h3>
          <el-form :model="secForm" label-width="140px">
            <el-form-item label="最大失败次数"><el-input-number v-model="secForm.login_max_fail" :min="3" :max="10" /></el-form-item>
            <el-form-item label="锁定(分钟)"><el-input-number v-model="secForm.login_lock_min" :min="5" :max="60" /></el-form-item>
          </el-form>
          <el-button type="warning" @click="downloadBackup">下载备份包</el-button>
          <el-button type="primary" @click="saveSec" style="margin-left:10px">保存</el-button>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('system.audit')" name="audit">
        <div class="np-card">
          <el-table :data="audits" stripe size="small" class="np-table">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="username" label="用户" width="110" />
            <el-table-column prop="action" label="操作" min-width="140" />
            <el-table-column label="模块" width="110"><template #default="{ row }">{{ moduleFromPath(row.path) }}</template></el-table-column>
            <el-table-column prop="created_at" label="时间" width="170"><template #default="{ row }">{{ fmtTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="系统更新" name="update">
        <div class="np-card" style="padding:24px">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="版本号">{{ ver.version || '--' }}</el-descriptions-item>
            <el-descriptions-item label="构建时间">{{ ver.build || '--' }}</el-descriptions-item>
          </el-descriptions>
          <el-divider />
          <el-button :loading="checking" @click="checkUpdate">检查更新</el-button>
          <div v-if="latest" style="margin-top:12px;padding:12px;background:#f0f9ff;border-radius:6px">
            <div>最新版本: <b>{{ latest.latest }}</b></div>
            <el-button v-if="latest.has_update" type="primary" style="margin-top:10px" :loading="upgrading" @click="doOnline">立即升级</el-button>
          </div>
          <el-upload :http-request="doUpload" :show-file-list="false" style="margin-top:20px" drag>
            <div>拖拽或点击上传 .exe 更新包</div>
          </el-upload>
          <el-button v-if="uploadResult" type="danger" style="margin-top:10px" :loading="applying" @click="doApply">确认更新并重启</el-button>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="mcpDialog" title="MCP" width="560px">
      <el-form :model="mcpForm" label-width="110px">
        <el-form-item label="名称"><el-input v-model="mcpForm.name" /></el-form-item>
        <el-form-item label="服务器地址"><el-input v-model="mcpForm.server_url" /></el-form-item>
        <el-form-item label="认证"><el-select v-model="mcpForm.auth_type"><el-option label="无" value="none" /><el-option label="Bearer" value="bearer" /></el-select></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="mcpForm.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="mcpDialog=false">取消</el-button><el-button type="primary" @click="saveMCP">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="emailDialog" title="邮箱" width="520px">
      <el-form :model="emailForm" label-width="110px">
        <el-form-item label="名称"><el-input v-model="emailForm.name" /></el-form-item>
        <el-form-item label="SMTP"><el-input v-model="emailForm.smtp_host" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="emailForm.smtp_port" /></el-form-item>
        <el-form-item label="账号"><el-input v-model="emailForm.user" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="emailForm.password" type="password" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="emailForm.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="emailDialog=false">取消</el-button><el-button type="primary" @click="saveEmail">保存</el-button></template>
    </el-dialog>
  </div>
</template>
