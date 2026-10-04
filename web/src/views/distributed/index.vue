<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { getEnc } from '@/utils/request'
import DeployMode from './components/DeployMode.vue'
import SelfInfo from './components/SelfInfo.vue'
import CascadeConfig from './components/CascadeConfig.vue'
import NodeGroups from './components/NodeGroups.vue'
import NodeList from './components/NodeList.vue'

const dist = reactive({
  self: { node_uuid: '', name: '', parent: '', token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
  cfg: { self_node_name: '', parent_url: '', cluster_token: '', mqtt_broker: '', mqtt_username: '', mqtt_password: '' },
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
    deployMode.value = (dist.self as any).deploy_mode || 'standalone'
  } catch {}
}
onMounted(loadDist)
</script>
<template>
  <div class="p-4">
    <DeployMode v-model="deployMode" />
    <template v-if="deployMode === 'distributed'">
      <SelfInfo :self="dist.self" />
      <CascadeConfig :cfg="dist.cfg" @saved="loadDist" />
      <NodeGroups />
      <NodeList :nodes="dist.nodes" @changed="loadDist" />
    </template>
  </div>
</template>
