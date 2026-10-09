<template>
  <div class="page">
    <el-card>
      <template #header>
        <div class="head">
          <span>{{ t('dbmigrate.title') }}</span>
          <el-button type="primary" :loading="running" @click="run">{{ t('dbmigrate.runMigrate') }}</el-button>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item :label="t('dbmigrate.dbType')">{{ info.type || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dbmigrate.tsdb')">{{ info.tsdb || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dbmigrate.host')">{{ info.host || '-' }}:{{ info.port || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dbmigrate.dbNamePath')">{{ info.db || info.path || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dbmigrate.user')">{{ info.user || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dbmigrate.pool')">{{ info.inuse }} / {{ info.open }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card style="margin-top:12px">
      <template #header>{{ t('dbmigrate.switchDb') }}</template>
      <el-form :model="form" label-width="120px">
        <el-divider content-position="left">{{ t('dbmigrate.businessDb') }}</el-divider>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item :label="t('dbmigrate.type')">
              <el-select v-model="form.database.type">
                <el-option label="SQLite" value="sqlite" />
                <el-option label="MySQL" value="mysql" />
                <el-option label="PostgreSQL" value="postgres" />
                <el-option label="Oracle" value="oracle" />
                <el-option :label="t('dbmigrate.dm')" value="dm" />
                <el-option :label="t('dbmigrate.kingbase')" value="kingbase" />
                <el-option label="OceanBase" value="oceanbase" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type !== 'sqlite'">
            <el-form-item :label="t('dbmigrate.host')"><el-input v-model="form.database.host" /></el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type !== 'sqlite'">
            <el-form-item :label="t('dbmigrate.port')"><el-input v-model.number="form.database.port" /></el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type === 'sqlite'">
            <el-form-item :label="t('dbmigrate.filePath')"><el-input v-model="form.database.path" /></el-form-item>
          </el-col>
          <template v-if="form.database.type !== 'sqlite'">
            <el-col :span="8">
              <el-form-item :label="t('dbmigrate.dbName')"><el-input v-model="form.database.database" /></el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :label="t('dbmigrate.user')"><el-input v-model="form.database.user" /></el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :label="t('dbmigrate.password')">
                <el-input v-model="form.database.password" type="password" show-password />
              </el-form-item>
            </el-col>
          </template>
        </el-row>

        <el-divider content-position="left">{{ t('dbmigrate.tsDatabase') }}</el-divider>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item :label="t('dbmigrate.type')">
              <el-select v-model="form.tsdb.type">
                <el-option :label="t('dbmigrate.builtin')" value="builtin" />
                <el-option label="TDengine" value="tdengine" />
                <el-option label="InfluxDB" value="influxdb" />
              </el-select>
            </el-form-item>
          </el-col>
          <template v-if="form.tsdb.type !== 'builtin'">
            <el-col :span="8"><el-form-item :label="t('dbmigrate.host')"><el-input v-model="form.tsdb.host" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item :label="t('dbmigrate.port')"><el-input v-model.number="form.tsdb.port" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item :label="t('dbmigrate.dbName')"><el-input v-model="form.tsdb.db" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item :label="t('dbmigrate.user')"><el-input v-model="form.tsdb.user" /></el-form-item></el-col>
            <el-col :span="8">
              <el-form-item :label="t('dbmigrate.password')">
                <el-input v-model="form.tsdb.pass" type="password" show-password />
              </el-form-item>
            </el-col>
          </template>
        </el-row>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save(true)">{{ t('dbmigrate.saveRestart') }}</el-button>
          <el-button @click="save(false)">{{ t('dbmigrate.saveOnly') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top:12px">
      <template #header>{{ t('dbmigrate.tableStructure') }}</template>
      <el-table :data="tables" size="small" stripe>
        <el-table-column prop="name" :label="t('dbmigrate.tableName')" />
        <el-table-column :label="t('dbmigrate.tableComment')">
          <template #default="{ row }">{{ tableLabel(row.name) }}</template>
        </el-table-column>
        <el-table-column prop="count" :label="t('dbmigrate.rows')" width="150" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'
const { t } = useI18n()

const info = ref<any>({})
const tables = ref<any[]>([])
const running = ref(false)
const saving = ref(false)
const tableKeys: Record<string, string> = {
  users: 't_users', audit_logs: 't_audit', device_groups: 't_groups', monitor_devices: 't_devices',
  link_tasks: 't_link', topo_devices: 't_topoDev', topo_links: 't_topoLink', topo_link_ips: 't_topoIp',
  db_instances: 't_db', script_templates: 't_tpl', script_histories: 't_hist',
  subnets: 't_subnet', ip_records: 't_ip', bind_devices: 't_bind', traffic_rules: 't_traffic',
  ai_configs: 't_ai', mcp_agents: 't_mcp', email_configs: 't_email',
  report_records: 't_report', system_settings: 't_setting', docker_hosts: 't_docker',
  k8s_clusters: 't_k8s', ts_kv: 't_tskv',
  nodes: 't_nodes', config_backups: 't_backup', alert_rules: 't_alertRule', alert_events: 't_alertEvent', notify_configs: 't_notify',
}
function tableLabel(n: string) { const k = tableKeys[n]; return k ? t('dbmigrate.' + k) : '-' }
const form = reactive<any>({
  database: { type: 'sqlite', host: '127.0.0.1', port: 3306, user: '', password: '', database: 'netops', path: 'data/netops.db', params: '' },
  tsdb: { type: 'builtin', host: '127.0.0.1', port: 6041, user: 'root', pass: '', db: 'data', params: '' }
})

async function load() {
  info.value = await getEnc('/dbmigrate/info')
  const cfg = await getEnc<any>('/dbmigrate/config')
  if (cfg) { form.database = { ...form.database, ...cfg.database }; form.tsdb = { ...form.tsdb, ...cfg.tsdb } }
  const r = await getEnc<{tables: any[]}>('/dbmigrate/tables')
  tables.value = r.tables || []
}
async function run() {
  running.value = true
  try {
    const r: any = await postEnc('/dbmigrate/run', {})
    ElMessage.success(r?.msg || t('dbmigrate.migrateDone'))
    await load()
  } catch (e: any) { ElMessage.error(e.message || t('dbmigrate.failed')) }
  finally { running.value = false }
}
async function save(restart: boolean) {
  saving.value = true
  try {
    const r: any = await postEnc('/dbmigrate/save', { ...form, restart })
    ElMessage.success(r?.msg || t('dbmigrate.saved'))
    if (restart) {
      await ElMessageBox.alert(t('dbmigrate.restartMsg'), t('dbmigrate.restarting'), { type: 'info' })
      setTimeout(() => location.reload(), 3000)
    }
  } catch (e: any) { ElMessage.error(e.message || t('dbmigrate.failed')) }
  finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.head { display:flex; justify-content:space-between; align-items:center; }
.page { padding: 16px; }
</style>
