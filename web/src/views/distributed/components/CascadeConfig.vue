<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { putEnc, postEnc } from '@/utils/request'
const props = defineProps<{ cfg: any }>()
const emit = defineEmits(['saved'])
const checking = ref(false)
const status = ref('')
async function save() {
  try {
    await putEnc('/system/settings/self_node_name', { value: props.cfg.self_node_name })
    await putEnc('/system/settings/parent_url', { value: props.cfg.parent_url })
    await putEnc('/system/settings/cluster_token', { value: props.cfg.cluster_token })
    await putEnc('/system/settings/mqtt_broker', { value: props.cfg.mqtt_broker })
    await putEnc('/system/settings/mqtt_username', { value: props.cfg.mqtt_username })
    await putEnc('/system/settings/mqtt_password', { value: props.cfg.mqtt_password })
    ElMessage.success('已保存')
    emit('saved')
  } catch (e: any) { ElMessage.error('保存失败: ' + (e?.message || e)) }
}
async function pushNow() {
  await postEnc('/distributed/push-now', {})
  ElMessage.success('已触发上报')
}
async function checkMqtt() {
  checking.value = true; status.value = ''
  try {
    const r: any = await postEnc('/distributed/mqtt-check', { broker: props.cfg.mqtt_broker })
    status.value = r.ok ? `MQTT 连接正常 (${r.broker})` : `MQTT 连接失败: ${r.error}`
    if (r.ok) ElMessage.success('MQTT 连接正常'); else ElMessage.error('MQTT 连接失败: ' + r.error)
  } catch (e: any) { status.value = 'MQTT 连接失败: ' + (e?.message || e) }
  finally { checking.value = false }
}
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>级联配置</b></template>
    <el-form :model="cfg" inline>
      <el-form-item label="本节点名称"><el-input v-model="cfg.self_node_name" placeholder="如：新疆省运维中心" /></el-form-item>
      <el-form-item label="上级地址"><el-input v-model="cfg.parent_url" placeholder="https://域名 或 http://IPv4:端口" /></el-form-item>
      <el-form-item label="Token"><el-input v-model="cfg.cluster_token" type="password" show-password /></el-form-item>
      <el-form-item>
        <el-button type="primary" @click="save">保存</el-button>
        <el-button @click="pushNow">立即上报</el-button>
      </el-form-item>
    </el-form>
    <el-divider />
    <el-form :model="cfg" inline>
      <el-form-item label="MQTT Broker"><el-input v-model="cfg.mqtt_broker" placeholder="tcp://域名:1883 或 tcp://IPv4:1883" style="width:280px" /></el-form-item>
      <el-form-item label="用户名"><el-input v-model="cfg.mqtt_username" /></el-form-item>
      <el-form-item label="密码"><el-input v-model="cfg.mqtt_password" type="password" show-password /></el-form-item>
      <el-form-item><el-button @click="save">保存 MQTT</el-button>
        <el-button type="success" :loading="checking" @click="checkMqtt">检测连接</el-button>
      </el-form-item>
    </el-form>
    <div v-if="status" :style="{marginTop:'8px', fontSize:'13px', color: status.startsWith('MQTT 连接正常') ? '#67C23A' : '#F56C6C'}">{{ status }}</div>
  </el-card>
</template>
