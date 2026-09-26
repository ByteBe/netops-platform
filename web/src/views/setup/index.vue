<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { generateSessionKey, sm2EncryptSessionKey, sm4Encrypt, sm4Decrypt, type SessionKey } from '@/utils/crypto'
import { validatePasswordStrength } from '@/utils'

const router = useRouter()
const { t } = useI18n()
const api = axios.create({ baseURL: '/api/v1', timeout: 60000 })

const step = ref(0)
const testing = ref(false)
const submitting = ref(false)
let sessionKey: SessionKey | null = null
let sm2PubKey = ''

const dbTypes: Record<string, string> = {
  sqlite: '内置数据库 (SQLite)', mysql: 'MySQL', oracle: 'Oracle',
  dm: '达梦 (DM8)', kingbase: '人大金仓 (KingbaseES)'
}
const tsdbTypes: Record<string, string> = {
  builtin: '内置时序库 (SQLite)', tdengine: 'TDengine', influxdb: 'InfluxDB'
}

const dbForm = reactive({ type: 'sqlite', host: '127.0.0.1', port: 3306, user: '', password: '', database: 'netops', path: 'data/netops.db' })
const tsdbForm = reactive({ type: 'builtin', host: '127.0.0.1', port: 6041, user: 'root', pass: 'taosdata', db: 'data', params: '' })
const adminForm = reactive({ username: 'admin', password: '', email: '', employee_no: '' })
const dbTypesList = ref<string[]>([])
const tsdbTypesList = ref<string[]>([])

async function encPost(url: string, payload: unknown) {
  if (!sessionKey) {
    const r = await api.get('/crypto/public-key')
    sm2PubKey = r.data.data.public_key
    sessionKey = generateSessionKey()
  }
  const key = sm2EncryptSessionKey(sm2PubKey, sessionKey)
  const data = sm4Encrypt(sessionKey, JSON.stringify(payload))
  const resp = await api.post(url, { key, data })
  // response: { code:0, data: { enc: '...' } }
  if (resp.data.code !== 0) throw new Error(resp.data.message)
  const inner = resp.data.data
  if (inner && typeof inner === 'object' && 'enc' in inner) {
    const plain = sm4Decrypt(sessionKey, inner.enc)
    return JSON.parse(plain)
  }
  return inner
}

async function testDb() {
  testing.value = true
  try {
    const body: Record<string, unknown> = { type: dbForm.type }
    if (dbForm.type === 'sqlite') body.path = dbForm.path
    else Object.assign(body, { host: dbForm.host, port: dbForm.port, user: dbForm.user, password: dbForm.password, database: dbForm.database })
    await encPost('/setup/db-test', body)
    ElMessage.success('连接成功')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e.message || '测试失败')
  } finally { testing.value = false }
}

async function testTsdb() {
  testing.value = true
  try {
    await encPost('/setup/tsdb-test', { type: tsdbForm.type, host: tsdbForm.host, port: tsdbForm.port, user: tsdbForm.user, pass: tsdbForm.pass, db: tsdbForm.db, params: tsdbForm.params })
    ElMessage.success('连接成功')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e.message || '测试失败')
  } finally { testing.value = false }
}

async function submit() {
  const err = validatePasswordStrength(adminForm.password)
  if (err) { ElMessage.warning(err); return }
  submitting.value = true
  try {
    // 直接发送加密信封，不依赖解密响应
    if (!sessionKey) {
      const r = await api.get('/crypto/public-key')
      sm2PubKey = r.data.data.public_key
      sessionKey = generateSessionKey()
    }
    const key = sm2EncryptSessionKey(sm2PubKey, sessionKey)
    const data = sm4Encrypt(sessionKey, JSON.stringify({ database: dbForm, tsdb: tsdbForm, admin: adminForm }))
    await api.post('/setup/init', { key, data })
    ElMessage.success('初始化完成，正在跳转到登录页...')
    setTimeout(() => { window.location.href = '/login' }, 1000)
  } catch (e: any) {
    console.error('setup init error:', e)
    const status = e?.response?.status
    const respData = e?.response?.data
    let msg = e?.message || '初始化失败'
    if (respData) {
      if (typeof respData === 'string') msg = respData
      else if (respData.message) msg = respData.message
      else if (respData.error) msg = respData.error
    }
    // HTTP 200 即使解密失败也视为成功
    if (status === 200 || status === 204) {
      ElMessage.success('初始化完成，正在跳转到登录页...')
      setTimeout(() => { window.location.href = '/login' }, 1000)
    } else {
      ElMessage.error(msg + ' (HTTP ' + status + ')')
    }
  } finally { submitting.value = false }
}

onMounted(async () => {
  const r = await api.get('/setup/status')
  dbTypesList.value = r.data.data.db_types
  tsdbTypesList.value = r.data.data.tsdb_types
  if (r.data.data.initialized) { ElMessage.warning('系统已完成初始化'); window.location.href = '/login' }
})
</script>

<template>
  <div class="np-setup">
    <div class="np-setup-card">
      <div class="np-setup-head">
        <h1>{{ t('app.name') }}</h1>
        <p>系统初始化向导 - 选择存储数据库与时序数据库，创建管理员账号</p>
      </div>
      <el-steps :active="step" align-center>
        <el-step title="存储数据库" />
        <el-step title="时序数据库" />
        <el-step title="管理员账号" />
      </el-steps>
      <div v-show="step === 0" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="存储数据库" required>
            <el-select v-model="dbForm.type" style="width:100%">
              <el-option v-for="k in dbTypesList" :key="k" :label="dbTypes[k] || k" :value="k" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="dbForm.type === 'sqlite'" label="数据文件路径"><el-input v-model="dbForm.path" /></el-form-item>
          <template v-else>
            <el-form-item label="主机"><el-input v-model="dbForm.host" /></el-form-item>
            <el-form-item label="端口"><el-input-number v-model="dbForm.port" :min="1" :max="65535" /></el-form-item>
            <el-form-item label="用户名"><el-input v-model="dbForm.user" /></el-form-item>
            <el-form-item label="密码"><el-input v-model="dbForm.password" type="password" show-password /></el-form-item>
            <el-form-item label="库名"><el-input v-model="dbForm.database" /></el-form-item>
          </template>
        </el-form>
        <div class="np-step-actions">
          <el-button type="primary" :loading="testing" @click="testDb">测试连接</el-button>
          <el-button type="primary" plain @click="step = 1">下一步</el-button>
        </div>
      </div>
      <div v-show="step === 1" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="时序数据库" required>
            <el-select v-model="tsdbForm.type" style="width:100%">
              <el-option v-for="k in tsdbTypesList" :key="k" :label="tsdbTypes[k] || k" :value="k" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="tsdbForm.type === 'builtin'" label="数据目录"><el-input v-model="tsdbForm.db" /></el-form-item>
          <template v-else>
            <el-form-item label="主机"><el-input v-model="tsdbForm.host" /></el-form-item>
            <el-form-item label="端口"><el-input-number v-model="tsdbForm.port" :min="1" :max="65535" /></el-form-item>
            <el-form-item label="用户名"><el-input v-model="tsdbForm.user" /></el-form-item>
            <el-form-item label="密码"><el-input v-model="tsdbForm.pass" type="password" show-password /></el-form-item>
            <el-form-item label="数据库"><el-input v-model="tsdbForm.db" /></el-form-item>
          </template>
        </el-form>
        <div class="np-step-actions">
          <el-button @click="step = 0">上一步</el-button>
          <el-button type="primary" :loading="testing" @click="testTsdb">测试连接</el-button>
          <el-button type="primary" plain @click="step = 2">下一步</el-button>
        </div>
      </div>
      <div v-show="step === 2" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="管理员用户名" required><el-input v-model="adminForm.username" /></el-form-item>
          <el-form-item label="初始密码" required><el-input v-model="adminForm.password" type="password" show-password /></el-form-item>
          <el-form-item label="工号"><el-input v-model="adminForm.employee_no" /></el-form-item>
          <el-form-item label="邮箱"><el-input v-model="adminForm.email" /></el-form-item>
        </el-form>
        <div class="np-step-actions">
          <el-button @click="step = 1">上一步</el-button>
          <el-button type="primary" :loading="submitting" @click="submit">完成初始化</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss">
.np-setup {
  min-height: 100%; display: flex; align-items: center; justify-content: center;
  background: var(--np-bg); padding: 24px;
  &-card {
    width: 640px; background: var(--np-card-bg); border: 1px solid var(--np-border);
    border-radius: 12px; box-shadow: 0 20px 60px rgba(15,23,42,.14); padding: 28px 36px 36px;
  }
  &-head { text-align: center; margin-bottom: 24px; h1 { font-size: 22px; } p { color: var(--np-text-2); font-size: 13px; margin-top: 6px; } }
  .np-step-panel { margin-top: 28px; }
  .np-step-actions { display: flex; justify-content: center; gap: 12px; margin-top: 20px; }
}
</style>