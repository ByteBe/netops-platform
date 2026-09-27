<script setup lang="ts">
// 设备监控：SNMP采集（服务器/交换机/路由器）+ 快照 + 历史
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { fmtBps, fmtDuration, fmtTime } from '@/utils'

const { t } = useI18n()

interface MonitorDevice {
  id: number
  name: string
  ip: string
  type: string
  snmp_version: string
  community: string
  username: string
  auth_proto: string
  priv_proto: string
  auth_pass: string
  priv_pass: string
  port: number
  interval: number
  group_id: number
  enable: boolean
  remark: string
  online?: boolean
  cpu?: number
  mem?: number
  uptime?: number
  message?: string
  status?: string
  vendor?: string
}
interface DeviceSnap {
  device_id: number
  name: string
  ip: string
  up: boolean
  cpu: number
  mem_used: number
  uptime_s: number
  interfaces: Array<{ index: string; name: string; speed: number; oper: string; in_bps: number; out_bps: number }>
  disks: Array<{ mount: string; total: number; used: number; used_pct: number }>
  ts: string
  message: string
}

const devices = ref<MonitorDevice[]>([])
const snaps = ref<Record<number, DeviceSnap>>({})
const loading = ref(false)

const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive<MonitorDevice>({
  id: 0, name: '', ip: '', type: 'switch', vendor: '', snmp_version: '2c', community: 'public',
  username: '', auth_proto: 'md5', priv_proto: 'des', auth_pass: '', priv_pass: '',
  port: 161, interval: 60, group_id: 0, enable: true, remark: ''
})

// 历史曲线
const histVisible = ref(false)
const histDevice = ref<MonitorDevice | null>(null)
const histChartRef = ref<HTMLDivElement>()
let histChart: echarts.ECharts | null = null

let ws: WSClient | null = null
const testing = ref(false)
const testResult = ref('')

async function loadDevices() {
  loading.value = true
  try {
    const list = await getEnc<MonitorDevice[]>('/monitor/devices')
    devices.value = list
    for (const d of list) {
      snaps.value[d.id] = {
        device_id: d.id, name: d.name, ip: d.ip,
        up: d.online || false,
        cpu: d.cpu || 0, mem_used: d.mem || 0,
        uptime_s: d.uptime || 0, message: d.message || '',
        interfaces: [], disks: [], ts: new Date().toISOString()
      }
    }
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, {
    id: 0, name: '', ip: '', type: 'switch', vendor: '', snmp_version: '2c', community: 'public',
    username: '', auth_proto: 'md5', priv_proto: 'des', auth_pass: '', priv_pass: '',
    port: 161, interval: 60, group_id: 0, enable: true, remark: ''
  })
  dialogVisible.value = true
}
function openEdit(d: MonitorDevice) {
  editingId.value = d.id
  Object.assign(form, d)
  dialogVisible.value = true
}
async function save() {
  if (!form.name || !form.ip) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editingId.value) {
    await putEnc(`/monitor/devices/${editingId.value}`, form)
  } else {
    await postEnc('/monitor/devices', form)
  }
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await loadDevices()
}
async function remove(d: MonitorDevice) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/monitor/devices/${d.id}`)
  ElMessage.success(t('common.success'))
  await loadDevices()
}

async function testConn() {
  if (!form.ip) { ElMessage.warning('请填写IP地址'); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ok: boolean; uptime?: number; error?: string; message?: string}>('/monitor/test', {
      ip: form.ip, snmp_version: form.snmp_version, community: form.community,
      username: form.username, auth_proto: form.auth_proto, priv_proto: form.priv_proto,
      auth_pass: form.auth_pass, priv_pass: form.priv_pass, port: form.port
    })
    testResult.value = r.ok ? `连接成功 (Uptime ${Math.floor((r.uptime||0)/86400)}天)` : `连接失败: ${r.error}`
  } catch (e: any) { testResult.value = '连接失败: ' + e.message } finally { testing.value = false }
}

async function showHistory(d: MonitorDevice) {
  histDevice.value = d
  histVisible.value = true
  await nextTick()
  const h = await getEnc<{
    cpu: number[][]
    mem: number[][]
  }>(`/monitor/devices/${d.id}/history?hours=6`)
  histChart ??= echarts.init(histChartRef.value as HTMLDivElement)
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: ['CPU %', 'MEM %'] },
    grid: { left: 50, right: 16, top: 36, bottom: 28 },
    xAxis: { type: 'time' },
    yAxis: { type: 'value', max: 100 },
    series: [
      { name: 'CPU %', type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#2f6bff' }, itemStyle: { color: '#2f6bff' }, data: h.cpu || [] },
      { name: 'MEM %', type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#22c55e' }, itemStyle: { color: '#22c55e' }, data: h.mem || [] }
    ]
  })
}

function ifName(s: DeviceSnap, name: string): string {
  return s.interfaces.find((i) => i.name === name || i.index === name)?.name || name
}

onMounted(async () => {
  await loadDevices()
  ws = new WSClient(['monitor'])
  ws.on('monitor', 'device_snapshot', (msg) => {
    const s = msg.data as unknown as DeviceSnap
    snaps.value[s.device_id] = s
  })
  ws.connect()
})

onBeforeUnmount(() => {
  ws?.close()
  histChart?.dispose()
})
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <el-input :placeholder="t('common.search')" style="width: 260px" clearable />
      <div class="spacer"></div>
      <el-button type="primary" @click="openAdd">
        <el-icon><Plus /></el-icon>{{ t('monitor.addDevice') }}
      </el-button>
    </div>

    <!-- 设备快照卡片 -->
    <div class="np-dev-grid">
      <div v-for="d in devices" :key="d.id" class="np-card np-dev-card">
        <div class="np-dev-head">
          <span class="np-dot" :class="snaps[d.id] ? (snaps[d.id].up ? 'up' : 'down') : 'idle'"></span>
          <span class="np-dev-name">{{ d.name }}</span>
          <span class="np-dev-ip">{{ d.ip }}</span>
          <el-tag size="small" effect="plain">{{ d.type }}</el-tag>
        </div>
        <div class="np-dev-metrics">
          <div class="np-metric">
            <div class="np-metric-label">CPU</div>
            <el-progress :percentage="snaps[d.id]?.cpu ?? 0" :stroke-width="8" :color="(snaps[d.id]?.cpu ?? 0) > 80 ? '#ef4444' : '#2f6bff'" />
          </div>
          <div class="np-metric">
            <div class="np-metric-label">MEM</div>
            <el-progress :percentage="snaps[d.id]?.mem_used ?? 0" :stroke-width="8" :color="(snaps[d.id]?.mem_used ?? 0) > 80 ? '#ef4444' : '#22c55e'" />
          </div>
        </div>
        <div v-if="(snaps[d.id]?.disks || []).length" class="np-dev-disks">
          <div v-for="disk in snaps[d.id]?.disks" :key="disk.mount" class="np-disk-row">
            <span class="np-disk-mount">{{ disk.mount }}</span>
            <el-progress :percentage="disk.used_pct" :stroke-width="6" :color="disk.used_pct > 80 ? '#ef4444' : '#f59e0b'" style="flex:1" />
            <span class="np-disk-size">{{ (disk.used/1024/1024/1024).toFixed(1) }}G / {{ (disk.total/1024/1024/1024).toFixed(1) }}G</span>
          </div>
        </div>
        <div class="np-dev-ifs">
          <div v-for="ifs in (snaps[d.id]?.interfaces || []).slice(0, 5)" :key="ifs.index" class="np-if-row">
            <span class="np-if-name">{{ ifs.name }}</span>
            <span class="np-if-val">↓ {{ fmtBps(ifs.in_bps) }}</span>
            <span class="np-if-val">↑ {{ fmtBps(ifs.out_bps) }}</span>
            <span class="np-dot" :class="ifs.oper === 'up' ? 'up' : 'down'"></span>
          </div>
          <div v-if="!snaps[d.id]" class="np-if-empty">{{ t('common.loading') }}</div>
        </div>
        <div class="np-dev-foot">
          <span>系统运行: {{ fmtDuration(snaps[d.id]?.uptime_s || 0) }}</span>
          <span>更新: {{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
        </div>
        <div class="np-dev-actions">
          <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('monitor.history') }}</el-button>
          <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
          <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
      <div v-if="!devices.length" class="np-card np-empty">{{ t('common.noData') }}，请 {{ t('common.add') }}</div>
    </div>

    <!-- 设备管理表格 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Cpu /></el-icon>
        <span>设备管理</span>
      </div>
      <el-table :data="devices" v-loading="loading" stripe class="np-table">
        <el-table-column prop="name" :label="t('monitor.deviceName')" min-width="140" />
        <el-table-column prop="ip" :label="t('monitor.ip')" min-width="140" />
        <el-table-column prop="type" :label="t('monitor.deviceType')" width="110" />
        <el-table-column prop="snmp_version" label="SNMP" width="80" />
        <el-table-column prop="interval" :label="t('monitor.interval')" width="110" />
        <el-table-column :label="t('common.status')" width="90">
          <template #default="{ row }">
            <span class="np-dot" :class="snaps[row.id] ? (snaps[row.id].up ? 'up' : 'down') : 'idle'"></span>
            <span>{{ snaps[row.id] ? (snaps[row.id].up ? 'up' : 'down') : '--' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="t('common.remark')" min-width="140" />
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('common.edit') : t('monitor.addDevice')" width="560px">
      <el-form :model="form" label-width="130px">
        <el-form-item :label="t('monitor.deviceName')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('monitor.ip')" required>
          <el-input v-model="form.ip" />
        </el-form-item>
        <el-form-item :label="t('monitor.deviceType')">
          <el-select v-model="form.type">
            <el-option label="交换机" value="switch" />
            <el-option label="路由器" value="router" />
            <el-option label="Linux服务器" value="server" />
            <el-option label="Windows服务器" value="windows" />
            <el-option label="防火墙" value="firewall" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="厂商">
          <el-select v-model="form.vendor" placeholder="自动识别" clearable>
            <el-option label="自动识别" value="" />
            <el-option label="Cisco 思科" value="cisco" />
            <el-option label="Huawei 华为" value="huawei" />
            <el-option label="H3C 华三" value="h3c" />
            <el-option label="Windows" value="windows" />
            <el-option label="Linux" value="linux" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('monitor.snmpVersion')">
          <el-select v-model="form.snmp_version" style="width: 120px">
            <el-option label="v1" value="1" />
            <el-option label="v2c" value="2c" />
            <el-option label="v3" value="3" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('monitor.community')" v-if="form.snmp_version !== '3'">
          <el-input v-model="form.community" />
        </el-form-item>
        <template v-if="form.snmp_version === '3'">
          <el-form-item :label="t('monitor.username')">
            <el-input v-model="form.username" />
          </el-form-item>
          <el-form-item :label="t('monitor.authProto')">
            <el-select v-model="form.auth_proto">
              <el-option label="MD5" value="md5" />
              <el-option label="SHA" value="sha" />
              <el-option label="SHA256" value="sha256" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('monitor.privProto')">
            <el-select v-model="form.priv_proto">
              <el-option label="DES" value="des" />
              <el-option label="AES" value="aes" />
              <el-option label="AES256" value="aes256" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('monitor.authPass')">
            <el-input v-model="form.auth_pass" type="password" show-password />
          </el-form-item>
          <el-form-item :label="t('monitor.privPass')">
            <el-input v-model="form.priv_pass" type="password" show-password />
          </el-form-item>
        </template>
        <el-form-item :label="t('monitor.port')">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item :label="t('monitor.interval')">
          <el-input-number v-model="form.interval" :min="10" :max="3600" />
        </el-form-item>
        <el-form-item label=" ">
          <el-button :loading="testing" @click="testConn">测试连接</el-button>
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.startsWith('连接成功') ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enable" />
        </el-form-item>
        <el-form-item :label="t('common.remark')">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="histVisible" :title="`${t('monitor.history')} - ${histDevice?.name}`" width="760px">
      <div ref="histChartRef" class="np-chart" style="height: 320px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-dev-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 14px;
}

.np-dev-card {
  display: flex;
  flex-direction: column;
  gap: 10px;

  .np-dev-head {
    display: flex;
    align-items: center;
    gap: 8px;
    .np-dev-name {
      font-weight: 600;
      font-size: 15px;
    }
    .np-dev-ip {
      flex: 1;
      color: var(--np-text-2);
      font-size: 13px;
    }
  }

  .np-dev-metrics {
    display: flex;
    flex-direction: column;
    gap: 8px;
    .np-metric-label {
      font-size: 12px;
      color: var(--np-text-2);
      margin-bottom: 2px;
    }
  }

  .np-dev-disks {
    display: flex;
    flex-direction: column;
    gap: 4px;
    .np-disk-row {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      .np-disk-mount {
        width: 60px;
        color: var(--np-text-2);
        @include ellipsis;
      }
      .np-disk-size {
        color: var(--np-text-2);
        font-size: 11px;
        width: 110px;
        text-align: right;
      }
    }
  }

  .np-dev-ifs {
    border-top: 1px dashed var(--np-border);
    padding-top: 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    .np-if-row {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      .np-if-name {
        flex: 1;
        @include ellipsis;
      }
      .np-if-val {
        color: var(--np-text-2);
        font-variant-numeric: tabular-nums;
        width: 96px;
        text-align: right;
      }
    }
    .np-if-empty {
      color: var(--np-text-2);
      font-size: 12px;
      text-align: center;
      padding: 6px;
    }
  }

  .np-dev-foot {
    display: flex;
    justify-content: space-between;
    color: var(--np-text-2);
    font-size: 12px;
  }

  .np-dev-actions {
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid var(--np-border);
    padding-top: 6px;
  }
}
</style>
