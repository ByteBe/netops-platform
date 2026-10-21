<script setup lang="ts">
import { reactive, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc } from '@/utils/request'
import DeployMode from './components/DeployMode.vue'
import SelfInfo from './components/SelfInfo.vue'
import CascadeConfig from './components/CascadeConfig.vue'
import NodeList from './components/NodeList.vue'

const { t } = useI18n()

const dist = reactive({
  self: { node_uuid: '', name: '', parent: '', token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '', mqtt_tls_ca_set: false, mqtt_tls_ca_info: '' },
  cfg: { self_node_name: '', parent_url: '', cluster_token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '', mqtt_tls_ca_set: false, mqtt_tls_ca_info: '' },
  nodes: [] as any[],
})
const deployMode = ref<'standalone' | 'distributed'>('standalone')
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
    dist.cfg.mqtt_tls_ca_set = !!dist.self.mqtt_tls_ca_set
    dist.cfg.mqtt_tls_ca_info = dist.self.mqtt_tls_ca_info || ''
    deployMode.value = (dist.self as any).deploy_mode || 'standalone'
  } catch {}
}
let autoTimer: number | undefined
onMounted(() => { loadDist(); autoTimer = window.setInterval(loadDist, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
</script>
<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('node.title') }}</div>
        <div class="np-page-head-desc">{{ t('node.pageDesc') }}</div>
      </div>
    </div>
    <DeployMode v-model="deployMode" />
    <template v-if="deployMode === 'distributed'">
      <SelfInfo :self="dist.self" />
      <CascadeConfig :cfg="dist.cfg" @saved="loadDist" />
      <NodeList :nodes="dist.nodes" @changed="loadDist" />
    </template>
  </div>
</template>
