<script setup lang="ts">
// BGP / VPNv4 邻居只读监控（SSH 采集，不对设备做任何修改）
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getEnc, postEnc } from '@/utils/request'
import { fmtTime } from '@/utils'
import { ElMessage } from 'element-plus'

const { t } = useI18n()
const router = useRouter()

interface Peer {
  device_id: number; device_name: string; peer_ip: string; asn: string; v: string
  state: string; up_down: string; prefix_rcvd: number; prefix_sent: number
  is_rr: boolean; out_iface?: string; updated_at: string
}
interface Dev { id: number; name: string; ip: string; bgp_enable: boolean }
const peers = ref<Peer[]>([])
const devs = ref<Dev[]>([])
const testDevId = ref<number | undefined>(undefined)
const testing = ref(false)
let timer: any

async function load() {
  try { peers.value = await getEnc<Peer[]>('/bgpmon/peers') } catch {}
  try {
    const list = await getEnc<Dev[]>('/monitor/devices')
    devs.value = (list || []).filter((d: any) => d.bgp_enable)
    if (!testDevId.value && devs.value.length) testDevId.value = devs.value[0].id
  } catch {}
}
onMounted(() => { load(); timer = setInterval(load, 15000) })
onBeforeUnmount(() => clearInterval(timer))

async function runTest() {
  if (!testDevId.value) { ElMessage.warning(t('bgp.pickDevice')); return }
  testing.value = true
  try {
    const r = await postEnc<any>('/bgpmon/test', { device_id: testDevId.value })
    if (r.ok) {
      ElMessage.success(t('bgp.testOk', { n: r.count }))
      load()
    } else {
      ElMessage.error(r.error || t('bgp.testFail'))
    }
  } catch (e: any) {
    ElMessage.error(e?.message || t('bgp.testFail'))
  } finally { testing.value = false }
}

function stateType(s: string) {
  if (s === 'Established') return 'success'
  return 'danger'
}
</script>

<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('bgp.title') }}</div>
        <div class="np-page-head-desc">{{ t('bgp.readOnly') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-select v-model="testDevId" size="small" style="width:180px" :placeholder="devs.length ? t('bgp.pickDevice') : t('bgp.noEnabledDevice')" :disabled="!devs.length" clearable>
          <el-option v-for="d in devs" :key="d.id" :label="d.name + '(' + d.ip + ')'" :value="d.id" />
        </el-select>
        <el-button size="small" :loading="testing" :disabled="!testDevId" @click="runTest">{{ t('bgp.testBtn') }}</el-button>
        <el-button type="primary" @click="router.push('/monitor')">{{ t('bgp.goDevice') }}</el-button>
      </div>
    </div>

    <div class="np-kpi-grid">
      <div class="np-kpi">
        <div class="np-kpi-icon"><el-icon><Network /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ peers.length }}</div>
          <div class="np-kpi-label">{{ t('bgp.totalPeers') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ peers.filter(p => p.state === 'Established').length }}</div>
          <div class="np-kpi-label">{{ t('bgp.establishedPeers') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-danger"><el-icon><WarningFilled /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ peers.length - peers.filter(p => p.state === 'Established').length }}</div>
          <div class="np-kpi-label">{{ t('bgp.downPeers') }}</div>
        </div>
      </div>
    </div>

    <div class="np-card">
      <el-table :data="peers" stripe class="np-table bgp-table" v-if="peers.length">
        <el-table-column :label="t('bgp.device')" prop="device_name" min-width="140" />
        <el-table-column :label="t('bgp.peerIp')" prop="peer_ip" min-width="130" />
        <el-table-column :label="t('bgp.outIface')" prop="out_iface" min-width="170">
          <template #default="{ row }">{{ row.out_iface || '--' }}</template>
        </el-table-column>
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
      <div v-else class="bgp-empty">
        <div class="np-empty-box">
          <div class="np-empty-icon"><el-icon><Network /></el-icon></div>
          <div class="np-empty-title">{{ t('bgp.title') }}</div>
          <div class="np-empty-desc">{{ t('bgp.emptyHint') }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss">
.bgp-empty { padding: 0; }

// 表格数字等宽对齐（仪表读数）
.bgp-table .el-table__cell {
  font-variant-numeric: tabular-nums;
}
</style>
