<script setup lang="ts">
// 设备监控：SNMP采集（服务器/交换机/路由器）+ 快照 + 历史
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { useAppStore } from '@/stores/app'
import { fmtBps, fmtDuration, fmtTime, cssVar, chartAxisColors } from '@/utils'
import { histTooltip } from '@/utils/histTooltip'

const { t } = useI18n()
const appStore = useAppStore()

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
  ssh_user?: string; ssh_pass?: string; ssh_port?: number; bgp_enable?: boolean
  display_ifs?: string
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
  port: 161, interval: 60, group_id: 0, enable: true, remark: '',
  ssh_user: '', ssh_pass: '', ssh_port: 22, bgp_enable: false, display_ifs: ''
})
const displayIfArr = computed<string[]>({
  get: () => form.display_ifs ? form.display_ifs.split(',').filter(Boolean) : [],
  set: (v) => { form.display_ifs = (v || []).join(',') }
})

// 历史曲线
const histVisible = ref(false)
const histDevice = ref<MonitorDevice | null>(null)
const histChartRef = ref<HTMLDivElement>()
const histHours = ref(6)
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
    // 拉一次实时快照，把 CPU/MEM/磁盘/接口/uptime 补全（否则刷新后只剩数据库里的静态字段）
    try {
      const snapList = await getEnc<any[]>('/monitor/snapshots')
      for (const s of snapList) {
        if (snaps.value[s.device_id]) {
          snaps.value[s.device_id] = s
        }
      }
    } catch {}
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
  if (!form.ip) { ElMessage.warning(t('monitor.ipRequired')); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ok: boolean; uptime?: number; error?: string; message?: string}>('/monitor/test', {
      ip: form.ip, snmp_version: form.snmp_version, community: form.community,
      username: form.username, auth_proto: form.auth_proto, priv_proto: form.priv_proto,
      auth_pass: form.auth_pass, priv_pass: form.priv_pass, port: form.port,
      ssh_user: form.ssh_user, ssh_pass: form.ssh_pass, ssh_port: form.ssh_port
    })
    testResult.value = (r && r.ok) ? `${t('monitor.testSuccess')} (Uptime ${Math.floor((r.uptime||0)/86400)}${t('monitor.days')})` : `${t('monitor.testFail')}: ${(r && r.error) || t('monitor.testFail')}`
  } catch (e: any) { testResult.value = t('monitor.testFail') + ': ' + e.message } finally { testing.value = false }
}

const sshTesting = ref(false)
const sshTestResult = ref('')
async function testSsh() {
  if (!form.ip || !form.ssh_user) { ElMessage.warning(t('bgp.sshUserIpRequired')); return }
  sshTesting.value = true; sshTestResult.value = ''
  try {
    const r = await postEnc<{ok: boolean; count?: number; error?: string}>('/bgpmon/test', {
      ip: form.ip, ssh_user: form.ssh_user, ssh_pass: form.ssh_pass,
      ssh_port: form.ssh_port, vendor: form.vendor
    })
    sshTestResult.value = r.ok ? `OK (${r.count||0} peers)` : (r.error || 'fail')
  } catch (e: any) { sshTestResult.value = 'ERR: ' + (e.message||'') } finally { sshTesting.value = false }
}

async function showHistory(d: MonitorDevice) {
  histDevice.value = d
  histVisible.value = true
  histHours.value = 6
  await loadHistory()
}

async function loadHistory() {
  if (!histDevice.value) return
  await nextTick()
  const h = await getEnc<{
    cpu: number[][]
    mem: number[][]
  }>(`/monitor/devices/${histDevice.value.id}/history?hours=${histHours.value}`)
  histChart ??= echarts.init(histChartRef.value as HTMLDivElement, null, { renderer: 'svg' })
  const ca = chartAxisColors()
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: histTooltip({ digits: 1, suffix: '%' }),
    legend: { data: ['CPU %', 'MEM %'], textStyle: { color: ca.axis } },
    grid: { left: 50, right: 16, top: 36, bottom: 28 },
    xAxis: { type: 'time', axisLabel: { color: ca.axis }, axisLine: { lineStyle: { color: ca.axisLine } } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: ca.axis }, splitLine: { lineStyle: { color: ca.split } } },
    series: [
      { name: 'CPU %', type: 'line', showSymbol: false, smooth: true, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-primary') }, itemStyle: { color: cssVar('--np-primary') }, data: h.cpu || [] },
      { name: 'MEM %', type: 'line', showSymbol: false, smooth: true, emphasis: { disabled: true }, lineStyle: { width: 2, color: cssVar('--np-success') }, itemStyle: { color: cssVar('--np-success') }, data: h.mem || [] }
    ]
  })
}

// 进度条信号色：超阈危险色，常态语义色（ECharts/组件不解析 CSS 变量，显式取色）
function cpuBarColor(v: number) { return v > 80 ? cssVar('--np-danger') : cssVar('--np-primary') }
function memBarColor(v: number) { return v > 80 ? cssVar('--np-danger') : cssVar('--np-success') }
function diskBarColor(v: number) { return v > 80 ? cssVar('--np-danger') : cssVar('--np-warning') }

// 主题切换时重绘历史曲线（canvas 不感知 CSS 变量变化）
watch(() => appStore.theme, () => { if (histChart) void loadHistory() })

function ifName(s: DeviceSnap, name: string): string {
  return s.interfaces.find((i) => i.name === name || i.index === name)?.name || name
}

function displayIfs(s?: DeviceSnap, d?: MonitorDevice) {
  if (!s?.interfaces) return []
  const virtual = /NULL|LoopBack|InLoopBack|Tunnel|Register|MEth|Vlan|Bridge|BSI|Dialer|Cellular/i
  let list = s.interfaces.filter((i) => !virtual.test(i.name))
  if (d?.display_ifs) {
    const sel = d.display_ifs.split(',').filter(Boolean)
    list = list.filter((i) => sel.includes(i.name))
  }
  return list
    .sort((a, b) => (b.in_bps + b.out_bps) - (a.in_bps + a.out_bps))
    .slice(0, 5)
}

let autoTimer: number | undefined
onMounted(async () => {
  await loadDevices()
  ws = new WSClient(['monitor'])
  ws.on('monitor', 'device_snapshot', (msg) => {
    const s = msg.data as unknown as DeviceSnap
    snaps.value[s.device_id] = s
  })
  ws.connect()
  autoTimer = window.setInterval(loadDevices, 30000) // 30s 刷新设备列表
})

onBeforeUnmount(() => {
  if (autoTimer) clearInterval(autoTimer)
  ws?.close()
  histChart?.dispose()
})
</script>

<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('monitor.title') }}</div>
        <div class="np-page-head-desc">{{ t('monitor.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-input :placeholder="t('common.search')" style="width: 220px" clearable />
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon>{{ t('monitor.addDevice') }}
        </el-button>
      </div>
    </div>

    <div class="np-kpi-grid">
      <div class="np-kpi">
        <div class="np-kpi-icon"><el-icon><Cpu /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ devices.length }}</div>
          <div class="np-kpi-label">{{ t('monitor.totalDevices') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-success"><el-icon><Connection /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ devices.filter(d => snaps[d.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('monitor.onlineDevices') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-danger"><el-icon><CircleClose /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ devices.length - devices.filter(d => snaps[d.id]?.up).length }}</div>
          <div class="np-kpi-label">{{ t('monitor.offlineDevices') }}</div>
        </div>
      </div>
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
            <div class="np-metric-top">
              <span class="np-metric-label">{{ t('monitor.cpu') }}</span>
              <span class="np-metric-value">{{ (snaps[d.id]?.cpu ?? 0).toFixed(0) }}%</span>
            </div>
            <el-progress :percentage="snaps[d.id]?.cpu ?? 0" :stroke-width="8" :color="cpuBarColor(snaps[d.id]?.cpu ?? 0)" />
          </div>
          <div class="np-metric">
            <div class="np-metric-top">
              <span class="np-metric-label">{{ t('monitor.mem') }}</span>
              <span class="np-metric-value">{{ (snaps[d.id]?.mem_used ?? 0).toFixed(0) }}%</span>
            </div>
            <el-progress :percentage="snaps[d.id]?.mem_used ?? 0" :stroke-width="8" :color="memBarColor(snaps[d.id]?.mem_used ?? 0)" />
          </div>
        </div>
        <div v-if="(snaps[d.id]?.disks || []).length" class="np-dev-disks">
          <div v-for="disk in snaps[d.id]?.disks" :key="disk.mount" class="np-disk-row">
            <span class="np-disk-mount">{{ disk.mount === '/' ? '磁盘' : disk.mount }}</span>
            <el-progress :percentage="disk.used_pct" :stroke-width="6" :color="diskBarColor(disk.used_pct)" style="flex:1" />
            <span class="np-disk-size">{{ (disk.used/1024/1024/1024).toFixed(1) }}G / {{ (disk.total/1024/1024/1024).toFixed(1) }}G</span>
          </div>
        </div>
        <div v-if="d.type !== 'server' && (snaps[d.id]?.interfaces || []).length" class="np-dev-ifs">
          <div v-for="ifs in displayIfs(snaps[d.id], d)" :key="ifs.index" class="np-if-row">
            <span class="np-if-name">{{ ifs.name }}</span>
            <span class="np-if-val">↓ {{ fmtBps(ifs.in_bps) }}</span>
            <span class="np-if-val">↑ {{ fmtBps(ifs.out_bps) }}</span>
            <span class="np-dot" :class="ifs.oper === 'up' ? 'up' : 'down'"></span>
          </div>
          <div v-if="!snaps[d.id]" class="np-if-empty">{{ t('common.loading') }}</div>
        </div>
        <div class="np-dev-foot">
          <span>{{ t('monitor.systemUptime') }}: {{ fmtDuration(snaps[d.id]?.uptime_s || 0) }}</span>
          <span>{{ t('monitor.updated') }}: {{ snaps[d.id] ? fmtTime(snaps[d.id].ts) : '--' }}</span>
        </div>
        <div class="np-dev-actions">
          <el-button size="small" text type="primary" @click="showHistory(d)">{{ t('monitor.history') }}</el-button>
          <el-button size="small" text type="primary" @click="openEdit(d)">{{ t('common.edit') }}</el-button>
          <el-button size="small" text type="danger" @click="remove(d)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
      <div v-if="!devices.length" class="np-card np-dev-empty">
        <div class="np-empty-box">
          <div class="np-empty-icon"><el-icon><Monitor /></el-icon></div>
          <div class="np-empty-title">{{ t('monitor.title') }}</div>
          <div class="np-empty-desc">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>

    <!-- 设备管理表格 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Cpu /></el-icon>
        <span>{{ t('monitor.deviceMgmt') }}</span>
      </div>
      <el-table :data="devices" v-loading="loading" stripe class="np-table">
        <el-table-column prop="name" :label="t('monitor.deviceName')" min-width="140" />
        <el-table-column prop="ip" :label="t('monitor.ip')" min-width="140" />
        <el-table-column prop="type" :label="t('monitor.deviceType')" width="110" />
        <el-table-column label="SNMP" width="80">
          <template #default="{ row }">v{{ row.snmp_version }}</template>
        </el-table-column>
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
            <el-option :label="t('monitor.switch')" value="switch" />
            <el-option :label="t('monitor.router')" value="router" />
            <el-option :label="t('monitor.linuxServer')" value="server" />
            <el-option :label="t('monitor.windowsServer')" value="windows" />
            <el-option :label="t('monitor.firewall')" value="firewall" />
            <el-option :label="t('monitor.other')" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('monitor.vendor')">
          <el-select v-model="form.vendor" :placeholder="t('monitor.autoDetect')" clearable>
            <el-option :label="t('monitor.autoDetect')" value="" />
            <el-option :label="t('monitor.cisco')" value="cisco" />
            <el-option :label="t('monitor.huawei')" value="huawei" />
            <el-option :label="t('monitor.h3c')" value="h3c" />
            <el-option label="Windows" value="windows" />
            <el-option label="Linux" value="linux" />
          </el-select>
        </el-form-item>
        <template v-if="form.type !== 'db'">
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
        </template>
        <el-form-item :label="t('monitor.interval')">
          <el-input-number v-model="form.interval" :min="10" :max="3600" />
        </el-form-item>
        <el-form-item label=" ">
          <el-button :loading="testing" @click="testConn">{{ t('monitor.testConn') }}</el-button>
          <span v-if="testResult" class="dev-test-result" :class="testResult.indexOf(t('monitor.testSuccess')) >= 0 ? 'is-ok' : 'is-fail'">{{ testResult }}</span>
        </el-form-item>
        <el-divider content-position="left">SSH 采集（Linux 系统运行时间 / BGP）</el-divider>
        <el-form-item :label="t('bgp.sshUser')">
          <el-input v-model="form.ssh_user" placeholder="root/admin" />
        </el-form-item>
        <el-form-item :label="t('bgp.sshPass')">
          <el-input v-model="form.ssh_pass" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('bgp.sshPort')">
          <el-input-number v-model="form.ssh_port" :min="1" :max="65535" style="margin-right:12px" />
          <el-button :loading="sshTesting" @click="testSsh">{{ t('bgp.testBtn') }}</el-button>
          <span v-if="sshTestResult" class="dev-test-result" :class="sshTestResult.indexOf('OK') >= 0 ? 'is-ok' : 'is-fail'">{{ sshTestResult }}</span>
        </el-form-item>
        <el-form-item :label="t('bgp.bgpEnable')">
          <el-switch v-model="form.bgp_enable" />
        </el-form-item>
        <el-form-item :label="t('monitor.displayIfs')">
          <el-select v-model="displayIfArr" multiple filterable :placeholder="t('monitor.displayIfsPlaceholder')" style="width:100%">
            <el-option v-for="i in (snaps[form.id]?.interfaces || [])" :key="i.index" :label="i.name" :value="i.name" />
          </el-select>
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
      <div style="margin-bottom:8px">
        <el-select v-model="histHours" size="small" style="width:140px" @change="loadHistory">
          <el-option :label="$t('monitor.lastNHours', {n: 1})" :value="1" />
          <el-option :label="$t('monitor.lastNHours', {n: 6})" :value="6" />
          <el-option :label="$t('monitor.lastNHours', {n: 12})" :value="12" />
          <el-option :label="$t('monitor.lastNHours', {n: 24})" :value="24" />
          <el-option :label="$t('monitor.lastNHours', {n: 72})" :value="72" />
        </el-select>
      </div>
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
    gap: 10px;
    padding: 12px;
    border-radius: $radius-md;
    background: var(--np-surface-1);
    border: 1px solid var(--np-border);
    .np-metric-top {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      margin-bottom: 4px;
      .np-metric-label {
        font-size: 11px;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--np-text-3);
      }
      .np-metric-value {
        font-size: 18px;
        font-weight: 700;
        font-variant-numeric: tabular-nums;
        color: var(--np-text-1);
      }
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

.dev-test-result {
  margin-left: 12px;
  font-size: 13px;
  &.is-ok { color: var(--np-success); }
  &.is-fail { color: var(--np-danger); }
}

.np-dev-empty {
  padding: 0;
}
</style>
