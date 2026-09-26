<script setup lang="ts">
// 系统管理：用户管理 / AI 接入（云端+本地Ollama，多AI调度）/ MCP 配置 / 邮箱 / 审计日志
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { fmtTime } from '@/utils'

const { t } = useI18n()

const activeTab = ref('users')

// ===== 用户管理 =====
interface SysUser {
  id: number
  username: string
  employee_no: string
  email: string
  role: string
  status: string
  must_change_pwd: boolean
  last_login_at: string
  created_at: string
}
const users = ref<SysUser[]>([])
const userDialog = ref(false)
const userForm = reactive({ id: 0, username: '', employee_no: '', email: '', role: 'operator', status: 'active', password: '' })

async function loadUsers() {
  users.value = await getEnc<SysUser[]>('/user/users')
}
function openUser(u?: SysUser) {
  if (u) {
    Object.assign(userForm, { id: u.id, username: u.username, employee_no: u.employee_no, email: u.email, role: u.role, status: u.status, password: '' })
  } else {
    Object.assign(userForm, { id: 0, username: '', employee_no: '', email: '', role: 'operator', status: 'active', password: '' })
  }
  userDialog.value = true
}
async function saveUser() {
  if (!userForm.username || !userForm.email) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (userForm.id) {
    await putEnc(`/user/users/${userForm.id}`, {
      employee_no: userForm.employee_no, email: userForm.email, role: userForm.role, status: userForm.status
    })
  } else {
    await postEnc('/user/users', userForm)
  }
  ElMessage.success(t('common.success'))
  userDialog.value = false
  await loadUsers()
}
async function removeUser(u: SysUser) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/user/users/${u.id}`)
  ElMessage.success(t('common.success'))
  await loadUsers()
}
async function resetPwd(u: SysUser) {
  const { value } = await ElMessageBox.prompt('输入新密码（>12位，含大写+小写+数字+符号）', '重置密码', {
    inputType: 'password', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel')
  })
  await postEnc(`/user/users/${u.id}/reset-password`, { password: value })
  ElMessage.success(t('common.success'))
}

// ===== AI 接入 =====
interface AIConfig {
  id: number
  name: string
  provider: string
  base_url: string
  api_key: string
  model: string
  temperature: number
  priority: number
  enable: boolean
  remark: string
}
interface AIProvider {
  value: string
  label: string
  base_url: string
}
const aiList = ref<AIConfig[]>([])
const aiProviders = ref<AIProvider[]>([])
const aiDialog = ref(false)
const aiForm = reactive<AIConfig>({
  id: 0, name: '', provider: 'openai', base_url: '', api_key: '', model: '',
  temperature: 0.7, priority: 0, enable: true, remark: ''
})
const aiReply = ref('')

async function loadAI() {
  const [list, providers] = await Promise.all([
    getEnc<AIConfig[]>('/system/ai/list'),
    getEnc<AIProvider[]>('/system/ai/providers')
  ])
  aiList.value = list
  aiProviders.value = providers
}
function openAI(a?: AIConfig) {
  aiReply.value = ''
  if (a) {
    Object.assign(aiForm, a)
  } else {
    Object.assign(aiForm, {
      id: 0, name: '', provider: 'openai', base_url: 'https://api.openai.com/v1', api_key: '',
      model: '', temperature: 0.7, priority: aiList.value.length, enable: true, remark: ''
    })
  }
  aiDialog.value = true
}
function onProviderChange() {
  const p = aiProviders.value.find((x) => x.value === aiForm.provider)
  if (p?.base_url) aiForm.base_url = p.base_url
}
async function saveAI() {
  if (!aiForm.name || !aiForm.base_url) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (aiForm.id) {
    await putEnc(`/system/ai/${aiForm.id}`, aiForm)
  } else {
    await postEnc('/system/ai', aiForm)
  }
  ElMessage.success(t('common.success'))
  aiDialog.value = false
  await loadAI()
}
async function removeAI(a: AIConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/ai/${a.id}`)
  ElMessage.success(t('common.success'))
  await loadAI()
}
async function testAI(a: AIConfig) {
  const r = await postEnc<{ ok: boolean; reply: string }>(`/system/ai/${a.id}/test`, {})
  aiReply.value = r.reply
}

// ===== MCP =====
interface MCPAgent {
  id: number
  name: string
  server_url: string
  endpoint: string
  auth_type: string
  auth_token: string
  enable: boolean
  remark: string
}
const mcpAgents = ref<MCPAgent[]>([])
const mcpInfo = ref<{ server: string; tools_endpoint: string; call_endpoint: string; tools: string[] } | null>(null)
const mcpDialog = ref(false)
const mcpForm = reactive<MCPAgent>({
  id: 0, name: '', server_url: '', endpoint: '/api', auth_type: 'none', auth_token: '', enable: true, remark: ''
})
const mcpOut = ref('')

async function loadMCP() {
  mcpAgents.value = await getEnc<MCPAgent[]>('/system/mcp/agents')
  try {
    mcpInfo.value = await getEnc<{ server: string; tools_endpoint: string; call_endpoint: string; tools: string[] }>('/system/mcp/info')
  } catch {
    /* 忽略 */
  }
}
function openMCP(m?: MCPAgent) {
  mcpOut.value = ''
  if (m) {
    Object.assign(mcpForm, m)
  } else {
    Object.assign(mcpForm, {
      id: 0, name: '', server_url: '', endpoint: '/api', auth_type: 'none', auth_token: '', enable: true, remark: ''
    })
  }
  mcpDialog.value = true
}
async function saveMCP() {
  if (!mcpForm.name || !mcpForm.server_url) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (mcpForm.id) {
    await putEnc(`/system/mcp/agents/${mcpForm.id}`, mcpForm)
  } else {
    await postEnc('/system/mcp/agents', mcpForm)
  }
  ElMessage.success(t('common.success'))
  mcpDialog.value = false
  await loadMCP()
}
async function removeMCP(m: MCPAgent) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/mcp/agents/${m.id}`)
  ElMessage.success(t('common.success'))
  await loadMCP()
}
async function testMCP(m: MCPAgent) {
  const r = await postEnc<{ ok: boolean; output: string }>(`/system/mcp/agents/${m.id}/test`, {})
  mcpOut.value = r.output
}

// ===== 邮箱 =====
interface EmailConfig {
  id: number
  name: string
  smtp_host: string
  smtp_port: number
  user: string
  password: string
  use_ssl: boolean
  enable: boolean
  default_to: string
  remark: string
}
const emailList = ref<EmailConfig[]>([])
const emailDialog = ref(false)
const emailForm = reactive<EmailConfig>({
  id: 0, name: '', smtp_host: '', smtp_port: 465, user: '', password: '',
  use_ssl: true, enable: false, default_to: '', remark: ''
})

async function loadEmails() {
  emailList.value = await getEnc<EmailConfig[]>('/system/email/list')
}
function openEmail(e?: EmailConfig) {
  if (e) {
    Object.assign(emailForm, e)
  } else {
    Object.assign(emailForm, {
      id: 0, name: '', smtp_host: '', smtp_port: 465, user: '', password: '',
      use_ssl: true, enable: false, default_to: '', remark: ''
    })
  }
  emailDialog.value = true
}
async function saveEmail() {
  if (!emailForm.name || !emailForm.smtp_host || !emailForm.user) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (emailForm.id) {
    await putEnc(`/system/email/${emailForm.id}`, emailForm)
  } else {
    await postEnc('/system/email', emailForm)
  }
  ElMessage.success(t('common.success'))
  emailDialog.value = false
  await loadEmails()
}
async function removeEmail(e: EmailConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/email/${e.id}`)
  ElMessage.success(t('common.success'))
  await loadEmails()
}
async function testEmail(e: EmailConfig) {
  const r = await postEnc<{ ok: boolean; to: string[] }>(`/system/email/${e.id}/test`, {})
  ElMessage.success(`测试邮件已发送至：${r.to.join(', ')}`)
}

// ===== 审计日志 =====
interface AuditRow {
  id: number
  username: string
  action: string
  module: string
  ip: string
  detail: string
  created_at: string
}
const audits = ref<AuditRow[]>([])

async function loadAudits() {
  try {
    const r = await getEnc<{ total: number; list: AuditRow[] }>('/system/audit?page=1&size=200')
    audits.value = r.list || []
  } catch {
    /* 非管理员忽略 */
  }
}

function moduleFromPath(p?: string): string {
  if (!p) return '-'
  const parts = p.replace('/api/v1/', '').split('/')
  const map: Record<string, string> = {
    auth: '认证', linkdetect: '链路检测', topo: '拓扑', monitor: '设备监控',
    dbmonitor: '数据库监控', resource: '资源管理', script: '脚本生成',
    ipam: 'IP管理', report: '巡检报告', traffic: '流量监控',
    system: '系统管理', docker: 'Docker监控', k8s: 'K8s监控',
    dashboard: '数据大屏', user: '用户'
  }
  return map[parts[0]] || parts[0] || '-'
}

// ===== 系统更新 =====
const ver = ref<any>({})
const uploadResult = ref<any>(null)
const applying = ref(false)

async function loadVersion() {
  ver.value = await getEnc('/update/version')
}

async function doUpload(opt: any) {
  const fd = new FormData()
  fd.append('file', opt.file)
  try {
    const r = await fetch('/api/v1/update/upload', {
      method: 'POST',
      headers: { 'Authorization': 'Bearer ' + localStorage.getItem('np-token') || '' },
      body: fd
    })
    const j = await r.json()
    if (j.code === 0) {
      uploadResult.value = j.data
      ElMessage.success('上传成功')
    } else {
      ElMessage.error(j.message || '上传失败')
    }
  } catch {
    ElMessage.error('上传失败')
  }
}

async function doApply() {
  applying.value = true
  try {
    await postEnc('/update/apply', { tmp_path: uploadResult.value.tmp_path })
    ElMessage.success('更新完成，系统将在几秒后重启')
    setTimeout(() => { location.reload() }, 5000)
  } finally {
    applying.value = false
  }
}

onMounted(() => {
  loadUsers()
  loadVersion()
  loadAI()
  loadMCP()
  loadEmails()
  loadAudits()
})
</script>

<template>
  <div class="np-page">
    <el-tabs v-model="activeTab">
      <!-- 用户管理 -->
      <el-tab-pane :label="t('system.users')" name="users">
        <div class="np-toolbar">
          <div class="spacer"></div>
          <el-button type="primary" @click="openUser()">
            <el-icon><Plus /></el-icon>{{ t('system.addUser') }}
          </el-button>
        </div>
        <div class="np-card">
          <el-table :data="users" stripe class="np-table">
            <el-table-column prop="username" label="用户名" min-width="120" />
            <el-table-column prop="employee_no" label="工号" width="120" />
            <el-table-column prop="email" label="邮箱" min-width="180" />
            <el-table-column prop="role" label="角色" width="90" />
            <el-table-column prop="status" label="状态" width="90" />
            <el-table-column :label="t('common.actions')" width="230" fixed="right">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="openUser(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" text type="warning" @click="resetPwd(row)">重置密码</el-button>
                <el-button size="small" text type="danger" :disabled="row.username === 'admin'" @click="removeUser(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- AI 接入 -->
      <el-tab-pane :label="t('system.ai')" name="ai">
        <div class="np-toolbar">
          <span class="np-page-desc">云端 API + 本地 Ollama/LLM，支持主流大模型，多 AI 自动调度（失败切换）</span>
          <div class="spacer"></div>
          <el-button type="primary" @click="openAI()">
            <el-icon><Plus /></el-icon>{{ t('common.add') }}
          </el-button>
        </div>
        <div class="np-card">
          <el-table :data="aiList" stripe class="np-table">
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column prop="provider" label="提供方" width="100" />
            <el-table-column prop="base_url" label="Base URL" min-width="240" show-overflow-tooltip />
            <el-table-column prop="model" label="模型" min-width="140" />
            <el-table-column prop="priority" label="优先级" width="80" />
            <el-table-column :label="t('common.enabled')" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.enable ? 'success' : 'info'">{{ row.enable ? '开' : '关' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testAI(row)">测试</el-button>
                <el-button size="small" text type="primary" @click="openAI(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeAI(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="aiReply" class="np-test-out">{{ aiReply }}</div>
        </div>
      </el-tab-pane>

      <!-- MCP 配置 -->
      <el-tab-pane :label="t('system.mcp')" name="mcp">
        <div class="np-toolbar">
          <span class="np-page-desc">支持主流 Agent 接入：服务器地址 + 接口形式</span>
          <div class="spacer"></div>
          <el-button type="primary" @click="openMCP()">
            <el-icon><Plus /></el-icon>{{ t('common.add') }}
          </el-button>
        </div>
        <el-alert v-if="mcpInfo" :closable="false" type="info" style="margin-bottom: 12px"
          :title="`本系统 MCP 端点：${mcpInfo.server}${mcpInfo.tools_endpoint}（工具：${mcpInfo.tools.join('、')}）`" />
        <div class="np-card">
          <el-table :data="mcpAgents" stripe class="np-table">
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column prop="server_url" label="服务器地址" min-width="220" show-overflow-tooltip />
            <el-table-column prop="endpoint" label="接口" min-width="120" />
            <el-table-column prop="auth_type" label="认证" width="90" />
            <el-table-column :label="t('common.enabled')" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.enable ? 'success' : 'info'">{{ row.enable ? '开' : '关' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testMCP(row)">测试</el-button>
                <el-button size="small" text type="primary" @click="openMCP(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeMCP(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="mcpOut" class="np-test-out">{{ mcpOut }}</div>
        </div>
      </el-tab-pane>

      <!-- 邮箱 -->
      <el-tab-pane :label="t('system.email')" name="email">
        <div class="np-toolbar">
          <span class="np-page-desc">通过用户填写的邮箱发送告警与巡检报告，支持启用开关</span>
          <div class="spacer"></div>
          <el-button type="primary" @click="openEmail()">
            <el-icon><Plus /></el-icon>{{ t('common.add') }}
          </el-button>
        </div>
        <div class="np-card">
          <el-table :data="emailList" stripe class="np-table">
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column prop="smtp_host" label="SMTP 服务器" min-width="180" />
            <el-table-column prop="smtp_port" label="端口" width="70" />
            <el-table-column prop="user" label="发件账号" min-width="160" />
            <el-table-column :label="t('common.enabled')" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.enable ? 'success' : 'info'">{{ row.enable ? '开' : '关' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="testEmail(row)">测试</el-button>
                <el-button size="small" text type="primary" @click="openEmail(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" text type="danger" @click="removeEmail(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- 审计日志 -->
      <el-tab-pane :label="t('system.audit')" name="audit">
        <div class="np-card">
          <el-table :data="audits" stripe size="small" class="np-table">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="username" label="用户" width="110" />
            <el-table-column prop="action" label="操作" min-width="140" />
            <el-table-column label="模块" width="110"><template #default="{ row }">{{ moduleFromPath(row.path) }}</template></el-table-column>
            <el-table-column label="IP" width="130"><template #default="{ row }">{{ row.ip === "::1" ? "127.0.0.1" : row.ip }}</template></el-table-column>
            <el-table-column label="详情" min-width="200" show-overflow-tooltip><template #default="{ row }">{{ (!row.detail || row.detail === "{}") ? "-" : row.detail }}</template></el-table-column>
            <el-table-column prop="created_at" label="时间" width="170">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- 系统更新 -->
      <el-tab-pane label="系统更新" name="update">
        <div class="np-card" style="padding:24px">
          <div style="margin-bottom:20px">
            <h3 style="margin:0 0 8px">当前版本</h3>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="版本号">{{ ver.version || '--' }}</el-descriptions-item>
              <el-descriptions-item label="构建时间">{{ ver.build || '--' }}</el-descriptions-item>
              <el-descriptions-item label="系统">{{ ver.os || '--' }} / {{ ver.arch || '--' }}</el-descriptions-item>
            </el-descriptions>
          </div>
          <el-divider></el-divider>
          <h3 style="margin:0 0 12px">上传更新包</h3>
          <el-upload
            :http-request="doUpload"
            :show-file-list="false"
            accept=".exe"
            drag>
            <el-icon :size="40"><UploadFilled /></el-icon>
            <div style="margin-top:8px">拖拽或点击上传新的二进制文件（.exe）</div>
            <div style="color:#999;font-size:12px;margin-top:4px">上传后将备份旧版本并自动重启</div>
          </el-upload>
          <div v-if="uploadResult" style="margin-top:16px;padding:12px;background:#f0f9ff;border-radius:6px">
            <div>文件: {{ uploadResult.filename }}</div>
            <div>大小: {{ (uploadResult.size/1024/1024).toFixed(1) }} MB</div>
            <div>校验: {{ uploadResult.sha256.substring(0,16) }}...</div>
            <el-button type="danger" style="margin-top:10px" :loading="applying" @click="doApply">确认更新并重启</el-button>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 用户对话框 -->
    <el-dialog v-model="userDialog" :title="userForm.id ? t('common.edit') : t('system.addUser')" width="460px">
      <el-form :model="userForm" label-width="90px">
        <el-form-item label="用户名" required>
          <el-input v-model="userForm.username" :disabled="userForm.id > 0" />
        </el-form-item>
        <el-form-item label="工号">
          <el-input v-model="userForm.employee_no" />
        </el-form-item>
        <el-form-item label="邮箱" required>
          <el-input v-model="userForm.email" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="userForm.role">
            <el-option label="管理员" value="admin" />
            <el-option label="操作员" value="operator" />
            <el-option label="只读" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态" v-if="userForm.id">
          <el-select v-model="userForm.status">
            <el-option label="启用" value="active" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item label="初始密码" v-if="!userForm.id">
          <el-input v-model="userForm.password" type="password" show-password placeholder="留空系统生成，首次登录强制修改" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="userDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveUser">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- AI 对话框 -->
    <el-dialog v-model="aiDialog" :title="aiForm.id ? t('common.edit') : t('system.addAI')" width="540px">
      <el-form :model="aiForm" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="aiForm.name" placeholder="如 本地Ollama / 云端DeepSeek" />
        </el-form-item>
        <el-form-item label="提供方">
          <el-select v-model="aiForm.provider" style="width: 100%" @change="onProviderChange">
            <el-option v-for="p in aiProviders" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="Base URL" required>
          <el-input v-model="aiForm.base_url" />
        </el-form-item>
        <el-form-item label="API Key">
          <el-input v-model="aiForm.api_key" type="password" show-password />
        </el-form-item>
        <el-form-item label="模型">
          <el-input v-model="aiForm.model" placeholder="如 gpt-3.5-turbo / qwen-plus" />
        </el-form-item>
        <el-form-item label="温度">
          <el-input-number v-model="aiForm.temperature" :min="0" :max="2" :step="0.1" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="aiForm.priority" :min="0" :max="99" />
          <span class="np-hint">越小越优先，失败自动切换下一个</span>
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="aiForm.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="aiDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveAI">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- MCP 对话框 -->
    <el-dialog v-model="mcpDialog" :title="mcpForm.id ? t('common.edit') : t('system.addMCP')" width="520px">
      <el-form :model="mcpForm" label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="mcpForm.name" />
        </el-form-item>
        <el-form-item label="服务器地址" required>
          <el-input v-model="mcpForm.server_url" placeholder="https://agent.example.com" />
        </el-form-item>
        <el-form-item label="接口">
          <el-input v-model="mcpForm.endpoint" />
        </el-form-item>
        <el-form-item label="认证方式">
          <el-select v-model="mcpForm.auth_type">
            <el-option label="无" value="none" />
            <el-option label="Bearer" value="bearer" />
            <el-option label="Basic" value="basic" />
          </el-select>
        </el-form-item>
        <el-form-item label="认证令牌">
          <el-input v-model="mcpForm.auth_token" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="mcpForm.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="mcpDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveMCP">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 邮箱对话框 -->
    <el-dialog v-model="emailDialog" :title="emailForm.id ? t('common.edit') : t('system.addEmail')" width="520px">
      <el-form :model="emailForm" label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="emailForm.name" />
        </el-form-item>
        <el-form-item label="SMTP 服务器" required>
          <el-input v-model="emailForm.smtp_host" placeholder="smtp.example.com" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="emailForm.smtp_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="发件账号" required>
          <el-input v-model="emailForm.user" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item label="密码/授权码">
          <el-input v-model="emailForm.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="SSL">
          <el-switch v-model="emailForm.use_ssl" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="emailForm.enable" />
          <span class="np-hint">关闭后不发送告警与巡检报告</span>
        </el-form-item>
        <el-form-item label="默认收件人">
          <el-input v-model="emailForm.default_to" placeholder="多个邮箱用逗号分隔，留空使用用户邮箱" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="emailDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveEmail">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-hint {
  margin-left: 10px;
  color: var(--np-text-2);
  font-size: 12px;
}

.np-test-out {
  margin-top: 10px;
  background: #0b1220;
  color: #7dd3fc;
  border-radius: $radius-sm;
  padding: 10px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow: auto;
}
</style>
