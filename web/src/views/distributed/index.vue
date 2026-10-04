<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc, putEnc } from '@/utils/request'

const dist = reactive({
  self: { node_uuid: '', name: '', parent: '', token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
  cfg: { self_node_name: '', parent_url: '', cluster_token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
  nodes: [] as any[],
})
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
      <template #header><b>本节点信息</b></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="节点 UUID"><code>{{ dist.self.node_uuid || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="本节点名称">{{ dist.self.name || '(未设置)' }}</el-descriptions-item>
        <el-descriptions-item label="上级地址">{{ dist.self.parent || '(未配置，为总部)' }}</el-descriptions-item>
        <el-descriptions-item label="通信 Token"><code>{{ dist.self.token || '(未设置)' }}</code></el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card shadow="never" class="mb-4">
      <template #header><b>级联配置</b></template>
      <el-form :model="dist.cfg" inline>
        <el-form-item label="本节点名称"><el-input v-model="dist.cfg.self_node_name" placeholder="如：新疆省运维中心" /></el-form-item>
        <el-form-item label="上级地址"><el-input v-model="dist.cfg.parent_url" placeholder="https://域名 或 http://IPv4:端口" /></el-form-item>
        <el-form-item label="Token"><el-input v-model="dist.cfg.cluster_token" placeholder="上下级共享密钥" /></el-form-item>
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
        <el-form-item><el-button @click="distSave">保存 MQTT</el-button></el-form-item>
      </el-form>
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
  </div>
</template>
