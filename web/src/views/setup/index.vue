<script setup lang="ts">
// 系统初始化向导：选择存储数据库 + 时序数据库 + 创建管理员
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc } from '@/utils/request'
import { validatePasswordStrength } from '@/utils'

const router = useRouter()
const { t } = useI18n()

interface InitStatus {
  initialized: boolean
  db_types: string[]
  tsdb_types: string[]
  web_port: number
  internal_port: number
}

const status = ref<InitStatus | null>(null)
const step = ref(0)
const testing = ref(false)
const submitting = ref(false)

const dbTypes: Record<string, string> = {
  sqlite: '内置数据库 (SQLite)',
  mysql: 'MySQL',
  oracle: 'Oracle',
  dm: '达梦 (DM8)',
  kingbase: '人大金仓 (KingbaseES)'
}
const tsdbTypes: Record<string, string> = {
  builtin: '内置时序库 (SQLite)',
  tdengine: 'TDengine',
  influxdb: 'InfluxDB'
}

const dbForm = reactive({
  type: 'sqlite',
  host: '127.0.0.1',
  port: 3306,
  user: '',
  password: '',
  database: 'netops',
  path: 'data/netops.db'
})
const tsdbForm = reactive({
  type: 'builtin',
  host: '127.0.0.1',
  port: 6041,
  user: 'root',
  pass: 'taosdata',
  db: 'data',
  params: ''
})
const adminForm = reactive({
  username: 'admin',
  password: '',
  email: '',
  employee_no: ''
})

const dbTypesList = ref<string[]>([])
const tsdbTypesList = ref<string[]>([])

async function testDb() {
  testing.value = true
  try {
    const body: Record<string, unknown> = { type: dbForm.type }
    if (dbForm.type === 'sqlite') {
      body.path = dbForm.path
    } else {
      Object.assign(body, {
        host: dbForm.host,
        port: dbForm.port,
        user: dbForm.user,
        password: dbForm.password,
        database: dbForm.database
      })
    }
    await postEnc('/setup/db-test', body)
    ElMessage.success(t('common.success'))
  } catch {
    /* 拦截器已提示 */
  } finally {
    testing.value = false
  }
}

async function testTsdb() {
  testing.value = true
  try {
    const body: Record<string, unknown> = {
      type: tsdbForm.type,
      host: tsdbForm.host,
      port: tsdbForm.port,
      user: tsdbForm.user,
      pass: tsdbForm.pass,
      db: tsdbForm.db,
      params: tsdbForm.params
    }
    await postEnc('/setup/tsdb-test', body)
    ElMessage.success(t('common.success'))
  } catch {
    /* 拦截器已提示 */
  } finally {
    testing.value = false
  }
}

async function submit() {
  const err = validatePasswordStrength(adminForm.password)
  if (err) {
    ElMessage.warning(err)
    return
  }
  submitting.value = true
  try {
    await postEnc('/setup/init', {
      database: dbForm,
      tsdb: tsdbForm,
      admin: adminForm
    })
    ElMessage.success('初始化完成')
    router.push('/login')
  } catch {
    /* 拦截器已提示 */
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  const s = await getEnc<InitStatus>('/setup/status')
  status.value = s
  dbTypesList.value = s.db_types
  tsdbTypesList.value = s.tsdb_types
  if (s.initialized) {
    ElMessage.warning('系统已完成初始化，跳转登录')
    router.push('/login')
  }
})
</script>

<template>
  <div class="np-setup">
    <div class="np-setup-card">
      <div class="np-setup-head">
        <h1>{{ t('app.name') }}</h1>
        <p>系统初始化向导 · 请选择存储数据库与时序数据库，并创建管理员账号</p>
      </div>

      <el-steps :active="step" align-center class="np-steps">
        <el-step title="存储数据库" />
        <el-step title="时序数据库" />
        <el-step title="管理员账号" />
      </el-steps>

      <!-- Step 1: 存储库 -->
      <div v-show="step === 0" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="存储数据库" required>
            <el-select v-model="dbForm.type" style="width: 100%">
              <el-option v-for="k in dbTypesList" :key="k" :label="dbTypes[k] || k" :value="k" />
            </el-select>
          </el-form-item>
          <template v-if="dbForm.type === 'sqlite'">
            <el-form-item label="数据文件路径">
              <el-input v-model="dbForm.path" placeholder="data/netops.db" />
            </el-form-item>
          </template>
          <template v-else>
            <el-form-item label="主机地址">
              <el-input v-model="dbForm.host" />
            </el-form-item>
            <el-form-item label="端口">
              <el-input-number v-model="dbForm.port" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="dbForm.user" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="dbForm.password" type="password" show-password />
            </el-form-item>
            <el-form-item label="库名/服务名">
              <el-input v-model="dbForm.database" />
            </el-form-item>
          </template>
        </el-form>
        <div class="np-step-actions">
          <el-button type="primary" :loading="testing" @click="testDb">测试连接</el-button>
          <el-button type="primary" plain @click="step = 1">下一步</el-button>
        </div>
      </div>

      <!-- Step 2: 时序库 -->
      <div v-show="step === 1" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="时序数据库" required>
            <el-select v-model="tsdbForm.type" style="width: 100%">
              <el-option v-for="k in tsdbTypesList" :key="k" :label="tsdbTypes[k] || k" :value="k" />
            </el-select>
          </el-form-item>
          <template v-if="tsdbForm.type === 'builtin'">
            <el-form-item label="数据目录">
              <el-input v-model="tsdbForm.db" placeholder="data" />
            </el-form-item>
          </template>
          <template v-else>
            <el-form-item label="主机地址">
              <el-input v-model="tsdbForm.host" />
            </el-form-item>
            <el-form-item label="端口">
              <el-input-number v-model="tsdbForm.port" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="tsdbForm.user" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="tsdbForm.pass" type="password" show-password />
            </el-form-item>
            <el-form-item label="数据库">
              <el-input v-model="tsdbForm.db" />
            </el-form-item>
          </template>
        </el-form>
        <div class="np-step-actions">
          <el-button @click="step = 0">上一步</el-button>
          <el-button type="primary" :loading="testing" @click="testTsdb">测试连接</el-button>
          <el-button type="primary" plain @click="step = 2">下一步</el-button>
        </div>
      </div>

      <!-- Step 3: 管理员 -->
      <div v-show="step === 2" class="np-step-panel">
        <el-form label-width="120px" label-position="left">
          <el-form-item label="管理员用户名" required>
            <el-input v-model="adminForm.username" />
          </el-form-item>
          <el-form-item label="初始密码" required>
            <el-input v-model="adminForm.password" type="password" show-password placeholder="大于12位：大写+小写+数字+符号" />
          </el-form-item>
          <el-form-item label="工号">
            <el-input v-model="adminForm.employee_no" />
          </el-form-item>
          <el-form-item label="邮箱">
            <el-input v-model="adminForm.email" />
          </el-form-item>
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
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--np-bg);
  padding: 24px;

  &-card {
    width: 640px;
    background: var(--np-card-bg);
    border: 1px solid var(--np-border);
    border-radius: $radius-lg;
    box-shadow: 0 20px 60px rgba(15, 23, 42, 0.14);
    padding: 28px 36px 36px;
  }

  &-head {
    text-align: center;
    margin-bottom: 24px;
    h1 {
      font-size: 22px;
    }
    p {
      color: var(--np-text-2);
      font-size: 13px;
      margin-top: 6px;
    }
  }

  .np-step-panel {
    margin-top: 28px;
  }

  .np-step-actions {
    display: flex;
    justify-content: center;
    gap: 12px;
    margin-top: 20px;
  }
}
</style>
