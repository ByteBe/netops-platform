<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Lock } from '@element-plus/icons-vue'
import { putEnc, postEnc } from '@/utils/request'
const { t } = useI18n()
const props = defineProps<{ cfg: any; isRoot?: boolean }>()
const emit = defineEmits(['saved'])
const checking = ref(false)
const status = ref('')
const statusOk = ref(false)
const caStatus = ref<any>(null)
const uploading = ref(false)
async function onCaFile(file: any) {
  const raw = file?.raw
  if (!raw) return
  if (raw.size > 512 * 1024) {
    ElMessage.warning(t('node.mqttCaTooBig'))
    return
  }
  const text = await readFileText(raw)
  if (!/-----BEGIN [A-Z ]+-----/.test(text)) {
    ElMessage.warning(t('node.mqttCaNotPem'))
    return
  }
  if (/-----BEGIN (RSA |EC |ENCRYPTED )?PRIVATE KEY-----/.test(text)) {
    ElMessage.warning(t('node.mqttCaIsKey'))
    return
  }
  if (/-----BEGIN CERTIFICATE REQUEST-----/.test(text)) {
    ElMessage.warning(t('node.mqttCaIsCsr'))
    return
  }
  uploading.value = true
  caStatus.value = null
  try {
    const r: any = await postEnc('/distributed/mqtt-ca-upload', { ca: text.trim() })
    caStatus.value = r
    if (r.ok && r.valid) {
      ElMessage.success(t('node.mqttCaSaved'))
      emit('saved') // 刷新配置：界面仅显示证书摘要，不回显明文
    } else {
      ElMessage.error(r.message || t('node.mqttCaInvalid'))
    }
  } catch (e: any) {
    caStatus.value = { ok: false, message: e?.message || String(e) }
    ElMessage.error(t('node.mqttCaInvalid') + ': ' + (e?.message || e))
  } finally {
    uploading.value = false
  }
}
function readFileText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('read error'))
    reader.readAsText(file)
  })
}
function caSummary(certs: any[]) {
  if (!certs?.length) return ''
  const c = certs[0]
  const cn = (c.subject || '').split(',').find((p: string) => p.trim().startsWith('CN='))
  const name = cn ? cn.trim().slice(3) : c.subject
  return `${name} · ${fmtCaTime(c.not_before)} ~ ${fmtCaTime(c.not_after)}`
}
function fmtCaTime(tm: string) {
  return tm ? tm.replace('T', ' ').substring(0, 16) : '-'
}
async function removeCa() {
  try {
    await postEnc('/distributed/mqtt-ca-remove', {})
    caStatus.value = null
    ElMessage.success(t('common.success'))
    emit('saved')
  } catch (e: any) {
    ElMessage.error(t('node.saveFailed') + ': ' + (e?.message || e))
  }
}
async function save() {
  try {
    await putEnc('/system/settings/self_node_name', { value: props.cfg.self_node_name })
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
  checking.value = true; status.value = ''; statusOk.value = false
  try {
    const r: any = await postEnc('/distributed/mqtt-check', { broker: props.cfg.mqtt_broker })
    statusOk.value = !!r.ok
    status.value = r.ok ? `${t('node.mqttOk')} (${r.broker})` : `${t('node.mqttFail')}: ${r.error}`
    if (r.ok) ElMessage.success(t('node.mqttOk')); else ElMessage.error(t('node.mqttFail') + ': ' + r.error)
  } catch (e: any) { statusOk.value = false; status.value = t('node.mqttFail') + ': ' + (e?.message || e) }
  finally { checking.value = false }
}
</script>
<template>
  <div class="np-card">
    <div class="np-card-title">{{ t('node.cascadeConfig') }}</div>
    <el-form :model="cfg" inline>
      <el-form-item :label="t('node.selfNodeName')"><el-input v-model="cfg.self_node_name" :placeholder="t('node.selfNodePlaceholder')" /></el-form-item>
      <el-form-item label="Token"><el-input v-model="cfg.cluster_token" type="password" show-password /></el-form-item>
      <el-form-item>
        <el-button type="primary" @click="save">{{ t('node.save') }}</el-button>
        <el-button v-if="!isRoot" @click="pushNow">{{ t('node.pushNow') }}</el-button>
      </el-form-item>
    </el-form>
    <div class="cascade-tip">{{ isRoot ? t('node.cascadeRootTip') : t('node.cascadeHttpRemoved') }}</div>
    <template v-if="!isRoot">
      <el-divider />
      <el-form :model="cfg" label-width="110px">
        <el-form-item :label="t('node.mqttBroker')"><el-input v-model="cfg.mqtt_broker" :placeholder="t('node.mqttBrokerPlaceholder')" style="width:320px" /></el-form-item>
        <el-form-item :label="t('node.mqttUser')"><el-input v-model="cfg.mqtt_username" /></el-form-item>
        <el-form-item :label="t('node.mqttPass')"><el-input v-model="cfg.mqtt_password" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button @click="save">{{ t('node.saveMqtt') }}</el-button>
          <el-button type="success" :loading="checking" @click="checkMqtt">{{ t('node.checkMqtt') }}</el-button>
        </el-form-item>
      </el-form>
      <!-- MQTT TLS 安全：证书与 MQTT 强关联面板 -->
      <div class="mqtt-tls">
        <div class="mqtt-tls-head">
          <div class="mqtt-tls-title">
            <el-icon class="mqtt-tls-icon"><Lock /></el-icon>
            <span>{{ t('node.mqttTlsTitle') }}</span>
            <el-tag :type="cfg.mqtt_tls_ca_set ? 'success' : 'info'" size="small" effect="light">
              {{ cfg.mqtt_tls_ca_set ? t('node.mqttTlsEnabled') : t('node.mqttTlsDisabled') }}
            </el-tag>
          </div>
          <span class="mqtt-tls-sub">{{ t('node.mqttTlsTip') }}</span>
        </div>
        <div class="mqtt-tls-body">
          <div class="ca-box">
            <template v-if="cfg.mqtt_tls_ca_set">
              <div class="ca-state ok">
                <span class="ca-dot ok-dot" />{{ t('node.certCaSet') }}
              </div>
              <div class="ca-meta">{{ cfg.mqtt_tls_ca_info }}</div>
            </template>
            <template v-else>
              <div class="ca-state muted">{{ t('node.certCaNotSet') }}</div>
            </template>
          </div>
          <div class="ca-actions">
            <el-upload :auto-upload="false" :show-file-list="false" accept=".pem,.crt,.cer,.cert,.txt" :on-change="onCaFile" style="margin-bottom:8px">
              <el-button size="small" :loading="uploading">{{ t('node.mqttCaUpload') }}</el-button>
            </el-upload>
            <el-button v-if="cfg.mqtt_tls_ca_set" size="small" type="danger" plain @click="removeCa">{{ t('node.certCaRemove') }}</el-button>
            <div v-if="caStatus" class="ca-status" :class="caStatus.ok && caStatus.valid ? 'ok' : 'fail'">
              <template v-if="caStatus.ok && caStatus.valid">
                <div class="ca-dot ok-dot" />{{ t('node.mqttCaSaved') }}
                <div v-if="caStatus.certs?.length" class="ca-meta">{{ caSummary(caStatus.certs) }}</div>
              </template>
              <template v-else>
                <div class="ca-dot fail-dot" />{{ caStatus.message || t('node.mqttCaInvalid') }}
              </template>
            </div>
          </div>
        </div>
      </div>
    </template>
    <div v-if="status" class="mqtt-check-status" :class="statusOk ? 'ok' : 'fail'">{{ status }}</div>
  </div>
</template>

<style scoped>
.cascade-tip {
  margin: -2px 0 10px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--np-text-2);
}
.mqtt-tls {
  margin-top: 4px;
  border: 1px solid var(--np-border);
  border-radius: 10px;
  background: var(--np-surface-1);
  padding: 14px 16px;
}
.mqtt-tls-head { display: flex; flex-direction: column; gap: 6px; margin-bottom: 10px; }
.mqtt-tls-title { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600; color: var(--np-text-1); }
.mqtt-tls-icon { color: var(--np-primary); }
.mqtt-tls-sub { font-size: 12px; color: var(--np-text-2); line-height: 1.5; }
.mqtt-tls-body { display: flex; align-items: flex-start; gap: 12px; flex-wrap: wrap; }
.ca-box {
  flex: 1;
  min-width: 280px;
  border: 1px solid var(--np-border);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--np-card-bg);
}
.ca-state { font-size: 13px; font-weight: 600; line-height: 1.5; }
.ca-state.ok { color: var(--np-success); }
.ca-state.muted { color: var(--np-text-2); }
.ca-actions { display: flex; flex-direction: column; align-items: flex-start; min-width: 140px; }
.ca-status { display: flex; flex-direction: column; gap: 4px; font-size: 13px; line-height: 1.4; }
.ca-status.ok { color: var(--np-success); }
.ca-status.fail { color: var(--np-danger); max-width: 220px; }
.ca-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
.ca-dot.ok-dot { background: var(--np-success); }
.ca-dot.fail-dot { background: var(--np-danger); }
.ca-meta { color: var(--np-text-2); font-size: 12px; word-break: break-all; margin-top: 2px; }
.mqtt-check-status { margin-top: 8px; font-size: 13px; }
.mqtt-check-status.ok { color: var(--np-success); }
.mqtt-check-status.fail { color: var(--np-danger); }
</style>
