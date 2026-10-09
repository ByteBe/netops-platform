<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { putEnc, postEnc } from '@/utils/request'
const { t } = useI18n()
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
    ElMessage.success(t('common.success'))
    emit('saved')
  } catch (e: any) { ElMessage.error(t('node.saveFailed') + ': ' + (e?.message || e)) }
}
async function pushNow() {
  await postEnc('/distributed/push-now', {})
  ElMessage.success(t('node.pushed'))
}
async function checkMqtt() {
  checking.value = true; status.value = ''
  try {
    const r: any = await postEnc('/distributed/mqtt-check', { broker: props.cfg.mqtt_broker })
    status.value = r.ok ? `${t('node.mqttOk')} (${r.broker})` : `${t('node.mqttFail')}: ${r.error}`
    if (r.ok) ElMessage.success(t('node.mqttOk')); else ElMessage.error(t('node.mqttFail') + ': ' + r.error)
  } catch (e: any) { status.value = t('node.mqttFail') + ': ' + (e?.message || e) }
  finally { checking.value = false }
}
</script>
<template>
  <el-card shadow="never" class="mb-4">
    <template #header><b>{{ t('node.cascadeConfig') }}</b></template>
    <el-form :model="cfg" inline>
      <el-form-item :label="t('node.selfNodeName')"><el-input v-model="cfg.self_node_name" :placeholder="t('node.selfNodePlaceholder')" /></el-form-item>
      <el-form-item :label="t('node.parentUrl')"><el-input v-model="cfg.parent_url" :placeholder="t('node.parentPlaceholder')" /></el-form-item>
      <el-form-item label="Token"><el-input v-model="cfg.cluster_token" type="password" show-password /></el-form-item>
      <el-form-item>
        <el-button type="primary" @click="save">{{ t('node.save') }}</el-button>
        <el-button @click="pushNow">{{ t('node.pushNow') }}</el-button>
      </el-form-item>
    </el-form>
    <el-divider />
    <el-form :model="cfg" inline>
      <el-form-item :label="t('node.mqttBroker')"><el-input v-model="cfg.mqtt_broker" placeholder="tcp://host:1883" style="width:280px" /></el-form-item>
      <el-form-item :label="t('node.mqttUser')"><el-input v-model="cfg.mqtt_username" /></el-form-item>
      <el-form-item :label="t('node.mqttPass')"><el-input v-model="cfg.mqtt_password" type="password" show-password /></el-form-item>
      <el-form-item><el-button @click="save">{{ t('node.saveMqtt') }}</el-button>
        <el-button type="success" :loading="checking" @click="checkMqtt">{{ t('node.checkMqtt') }}</el-button>
      </el-form-item>
    </el-form>
    <div v-if="status" :style="{marginTop:'8px', fontSize:'13px', color: status.indexOf(t('node.mqttOk')) >= 0 ? '#67C23A' : '#F56C6C'}">{{ status }}</div>
  </el-card>
</template>
