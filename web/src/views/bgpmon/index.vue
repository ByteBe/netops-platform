<script setup lang="ts">
// BGP / VPNv4 邻居只读监控（SSH 采集，不对设备做任何修改）
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc } from '@/utils/request'
import { fmtTime } from '@/utils'

const { t } = useI18n()
const router = useRouter()

interface Peer {
  device_id: number; device_name: string; peer_ip: string; asn: string; v: string
  state: string; up_down: string; prefix_rcvd: number; prefix_sent: number
  is_rr: boolean; updated_at: string
}
const peers = ref<Peer[]>([])
let timer: any

async function load() {
  try { peers.value = await getEnc<Peer[]>('/bgpmon/peers') } catch {}
}
onMounted(() => { load(); timer = setInterval(load, 15000) })
onBeforeUnmount(() => clearInterval(timer))

function stateType(s: string) {
  if (s === 'Established') return 'success'
  return 'danger'
}
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">{{ t('bgp.title') }}</span>
      <span class="np-tip">{{ t('bgp.readOnly') }}</span>
      <div class="spacer"></div>
      <el-button type="primary" @click="router.push('/monitor')">{{ t('bgp.goDevice') }}</el-button>
    </div>
    <div class="np-card">
      <el-table :data="peers" stripe class="np-table" v-if="peers.length">
        <el-table-column :label="t('bgp.device')" prop="device_name" min-width="140" />
        <el-table-column :label="t('bgp.peerIp')" prop="peer_ip" min-width="130" />
        <el-table-column label="ASN" prop="asn" width="100" />
        <el-table-column label="V" prop="v" width="60" />
        <el-table-column :label="t('bgp.state')" width="120">
          <template #default="{ row }">
            <el-tag :type="stateType(row.state)" size="small">{{ row.state }}</el-tag>
            <el-tag v-if="row.is_rr" type="warning" size="small" style="margin-left:6px">RR</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('bgp.upDown')" prop="up_down" width="140" />
        <el-table-column :label="t('bgp.prefixRcvd')" prop="prefix_rcvd" width="110" />
        <el-table-column :label="t('bgp.prefixSent')" prop="prefix_sent" width="110" />
        <el-table-column :label="t('bgp.updatedAt')" width="170">
          <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="t('bgp.emptyHint')" />
    </div>
  </div>
</template>
