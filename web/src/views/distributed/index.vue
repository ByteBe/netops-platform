<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc, putEnc } from '@/utils/request'

const dist = reactive({
  self: { node_uuid: '', name: '', parent: '', token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
  cfg: { self_node_name: '', parent_url: '', cluster_token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
  nodes: [] as any[],
})
const mqttStatus = ref('')
const checkingMqtt = ref(false)
const tokenVisible = ref(false)
const deployMode = ref<'standalone' | 'distributed'>('standalone')
async function switchMode(m: string) {
  await postEnc('/distributed/deploy-mode', { mode: m })
  deployMode.value = m as any
  ElMessage.success('已切换为' + (m === 'distributed' ? '分布式级联模式' : '单机模式'))
  await loadDist()
}
async function checkMqtt() {
  checkingMqtt.value = true
  mqttStatus.value = ''
  try {
    const r: any = await postEnc('/distributed/mqtt-check', { broker: dist.cfg.mqtt_broker })
    mqttStatus.value = r.ok ? `MQTT 连接正常 (${r.broker})` : `MQTT 连接失败: ${r.error}`
    if (r.ok) ElMessage.success('MQTT 连接正常')
    else ElMessage.error('MQTT 连接失败: ' + r.error)
  } catch (e: any) {
    mqttStatus.value = 'MQTT 连接失败: ' + (e?.message || e)
  } finally {
    checkingMqtt.value = false
  }
}
async function loadDist() {
  try {
    dist.self = await getEnc('/distributed/self')
    dist.nodes = await getEnc('/distributed/nodes')
    dist.cfg.self_node_name = dist.self.name || ''
    dist.cfg.parent_url = dist.self.parent || ''
    dist.cfg.cluster_token = dist.self.token || ''
    dist.cfg.mqtt_broker = dist.self.mqtt_broker || ''
    dist.cfg.mqtt_username = dist.self.mqtt_username || ''
    dist.cfg.mqtt_password = dist.self.mqtt_password || ''
    deployMode.value = (dist.self as any).deploy_mode || 'standalone'
  } catch {}
}
async function distSave() {
  try {
    await putEnc('/system/settings/self_node_name', { value: dist.cfg.self_node_name })
    await putEnc('/system/settings/parent_url', { value: dist.cfg.parent_url })
    await putEnc('/system/settings/cluster_token', { value: dist.cfg.cluster_token })
    await putEnc('/system/settings/mqtt_broker', { value: dist.cfg.mqtt_broker })
    await putEnc('/system/settings/mqtt_username', { value: dist.cfg.mqtt_username })
    await putEnc('/system/settings/mqtt_password', { value: dist.cfg.mqtt_password })
    ElMessage.success('已保存')
    await loadDist()
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.message || e))
  }
}
async function distPushNow() {
  await postEnc('/distributed/push-now', {})
  ElMessage.success('已触发上报')
}
onMounted(loadDist)
</script>

<template>
  <div class="p-4">
    <el-card shadow="never" class="mb-4">
      <template #header><b>部署模式</b></template>
      <el-radio-group v-model="deployMode" @change="switchMode">
        <el-radio value="standalone">单机模式</el-radio>
        <el-radio value="distributed">分布式级联模式</el-radio>
      </el-radio-group>
      <div style="margin-top:8px;color:#909399;font-size:13px">
        {{ deployMode === 'distributed' ? '当前为分布式模式，本节点可与上下级节点同步数据。' : '当前为单机模式，仅本机采集与存储，不与其他节点通信。' }}
      </div>
    </el-card>

    <template v-if="deployMode === 'distributed'">
    <el-card shadow="never" class="mb-4">
      <template #header><b>本节点信息</b></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="节点 UUID"><code>{{ dist.self.node_uuid || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="本节点名称">{{ dist.self.name || '(未设置)' }}</el-descriptions-item>
        <el-descriptions-item label="上级地址">{{ dist.self.parent || '(未配置，为总部)' }}</el-descriptions-item>
        <el-descriptions-item label="通信 Token">
          <code>{{ tokenVisible ? dist.self.token : '••••••••' }}</code>
          <el-button link type="primary" size="small" @click="tokenVisible=!tokenVisible" style="margin-left:6px">{{ tokenVisible ? '隐藏' : '查看' }}</el-button>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card shadow="never" class="mb-4">
      <template #header><b>级联配置</b></template>
      <el-form :model="dist.cfg" inline>
        <el-form-item label="本节点名称"><el-input v-model="dist.cfg.self_node_name" placeholder="如：新疆省运维中心" /></el-form-item>
        <el-form-item label="上级地址"><el-input v-model="dist.cfg.parent_url" placeholder="https://域名 或 http://IPv4:端口" /></el-form-item>
        <el-form-item label="Token"><el-input v-model="dist.cfg.cluster_token" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="distSave">保存</el-button>
          <el-button @click="distPushNow">立即上报</el-button>
        </el-form-item>
      </el-form>

      <el-divider />
      <el-form :model="dist.cfg" inline>
        <el-form-item label="MQTT Broker"><el-input v-model="dist.cfg.mqtt_broker" placeholder="tcp://域名:1883 或 tcp://IPv4:1883" style="width:280px" /></el-form-item>
        <el-form-item label="用户名"><el-input v-model="dist.cfg.mqtt_username" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="dist.cfg.mqtt_password" type="password" show-password /></el-form-item>
        <el-form-item><el-button @click="distSave">保存 MQTT</el-button>
          <el-button type="success" :loading="checkingMqtt" @click="checkMqtt">检测连接</el-button>
        </el-form-item>
      </el-form>
      <div v-if="mqttStatus" :style="{marginTop:'8px', fontSize:'13px', color: mqttStatus.startsWith('MQTT 连接正常') ? '#67C23A' : '#F56C6C'}">{{ mqttStatus }}</div>
    </el-card>

    <el-card shadow="never">
      <template #header><b>下级节点</b></template>
      <el-table :data="dist.nodes" stripe size="small">
        <el-table-column prop="name" label="节点名称" min-width="140" />
        <el-table-column label="层级" width="80">
          <template #default="{ row }">{{ ['','总部','省级','市级','县级'][row.level] || 'L'+row.level }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status==='online'?'success':'info'" size="small">{{ row.status==='online'?'在线':'离线' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="device_cnt" label="设备数" width="80" />
        <el-table-column prop="container_cnt" label="容器数" width="80" />
        <el-table-column prop="last_seen_at" label="最后上报" min-width="160" />
      </el-table>
    </el-card>
    </template>
  </div>
</template>
