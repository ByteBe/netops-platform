<template>
  <div class="page">
    <el-card>
      <template #header>
        <div class="head">
          <span>数据库迁移</span>
          <el-button type="primary" :loading="running" @click="run">在当前库执行迁移</el-button>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="数据库类型">{{ info.type || '-' }}</el-descriptions-item>
        <el-descriptions-item label="时序数据库">{{ info.tsdb || '-' }}</el-descriptions-item>
        <el-descriptions-item label="主机">{{ info.host || '-' }}:{{ info.port || '-' }}</el-descriptions-item>
        <el-descriptions-item label="库名/路径">{{ info.db || info.path || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户">{{ info.user || '-' }}</el-descriptions-item>
        <el-descriptions-item label="连接池">{{ info.inuse }} / {{ info.open }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card style="margin-top:12px">
      <template #header>切换数据库（保存后自动重启）</template>
      <el-form :model="form" label-width="120px">
        <el-divider content-position="left">业务数据库</el-divider>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="类型">
              <el-select v-model="form.database.type">
                <el-option label="SQLite" value="sqlite" />
                <el-option label="MySQL" value="mysql" />
                <el-option label="PostgreSQL" value="postgres" />
                <el-option label="Oracle" value="oracle" />
                <el-option label="达梦 DM" value="dm" />
                <el-option label="人大金仓 Kingbase" value="kingbase" />
                <el-option label="OceanBase" value="oceanbase" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type !== 'sqlite'">
            <el-form-item label="主机"><el-input v-model="form.database.host" /></el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type !== 'sqlite'">
            <el-form-item label="端口"><el-input v-model.number="form.database.port" /></el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.database.type === 'sqlite'">
            <el-form-item label="文件路径"><el-input v-model="form.database.path" /></el-form-item>
          </el-col>
          <template v-if="form.database.type !== 'sqlite'">
            <el-col :span="8">
              <el-form-item label="库名"><el-input v-model="form.database.database" /></el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="用户"><el-input v-model="form.database.user" /></el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="密码">
                <el-input v-model="form.database.password" type="password" show-password />
              </el-form-item>
            </el-col>
          </template>
        </el-row>

        <el-divider content-position="left">时序数据库</el-divider>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="类型">
              <el-select v-model="form.tsdb.type">
                <el-option label="内置 (memory)" value="builtin" />
                <el-option label="TDengine" value="tdengine" />
                <el-option label="InfluxDB" value="influxdb" />
              </el-select>
            </el-form-item>
          </el-col>
          <template v-if="form.tsdb.type !== 'builtin'">
            <el-col :span="8"><el-form-item label="主机"><el-input v-model="form.tsdb.host" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="端口"><el-input v-model.number="form.tsdb.port" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="库名"><el-input v-model="form.tsdb.db" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="用户"><el-input v-model="form.tsdb.user" /></el-form-item></el-col>
            <el-col :span="8">
              <el-form-item label="密码">
                <el-input v-model="form.tsdb.pass" type="password" show-password />
              </el-form-item>
            </el-col>
          </template>
        </el-row>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save(true)">保存并重启</el-button>
          <el-button @click="save(false)">仅保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top:12px">
      <template #header>表结构</template>
      <el-table :data="tables" size="small" stripe>
        <el-table-column prop="name" label="表名" />
        <el-table-column label="表中文名称">
          <template #default="{ row }">{{ tableLabel(row.name) }}</template>
        </el-table-column>
        <el-table-column prop="count" label="行数" width="150" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc } from '@/utils/request'

const info = ref<any>({})
const tables = ref<any[]>([])
const running = ref(false)
const saving = ref(false)
const tableNames: Record<string, string> = {
  users: '用户表', audit_logs: '审计日志', device_groups: '设备分组', monitor_devices: '监控设备',
  link_tasks: '链路检测任务', topo_devices: '拓扑设备', topo_links: '拓扑链路', topo_link_ips: '拓扑链路IP',
  db_instances: '数据库实例', script_templates: '脚本模板', script_histories: '脚本执行历史',
  subnets: '子网记录', ip_records: 'IP记录', bind_devices: '绑定设备', traffic_rules: '流量策略',
  ai_configs: 'AI配置', mcp_agents: 'MCP代理', email_configs: '邮件配置',
  report_records: '巡检报告记录', system_settings: '系统设置', docker_hosts: 'Docker主机',
  k8s_clusters: 'K8S集群', ts_kv: '时序数据',
  nodes: '节点表', config_backups: '配置备份', alert_rules: '告警规则', alert_events: '告警事件', notify_configs: '通知渠道配置',
}
function tableLabel(n: string) { return tableNames[n] || '-' }
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
    ElMessage.success(r?.msg || '迁移完成')
    await load()
  } catch (e: any) { ElMessage.error(e.message || '失败') }
  finally { running.value = false }
}
async function save(restart: boolean) {
  saving.value = true
  try {
    const r: any = await postEnc('/dbmigrate/save', { ...form, restart })
    ElMessage.success(r?.msg || '已保存')
    if (restart) {
      await ElMessageBox.alert('服务正在重启，请稍候刷新页面', '重启中', { type: 'info' })
      setTimeout(() => location.reload(), 3000)
    }
  } catch (e: any) { ElMessage.error(e.message || '失败') }
  finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.head { display:flex; justify-content:space-between; align-items:center; }
.page { padding: 16px; }
</style>
