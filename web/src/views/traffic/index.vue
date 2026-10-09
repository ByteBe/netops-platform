<script setup lang="ts">
// 流量监控：专线规则（带宽/上下行独立可配）+ 实时速率 + 历史
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { WSClient } from '@/utils/ws'
import { fmtBps, pickColor } from '@/utils'

const fmtPct = (v: number) => {
  if (v < 0.01) return v.toFixed(3)
  if (v < 1) return v.toFixed(2)
  return v.toFixed(1)
}

const { t } = useI18n()

interface TrafficRule {
  id: number
  device_id: number
  interface: string
  display_name: string
  line_rate: number
  up_rate: number
  down_rate: number
  color: string
  enable: boolean
  remark: string
  device_name?: string
  device_ip?: string
}
interface IfInfo {
  index: string
  name: string
  speed: number
  oper: string
}

const rules = ref<TrafficRule[]>([])
const devices = ref<Array<{ id: number; name: string; ip: string }>>([])
const currents = ref<Record<number, { up: boolean; in_bps: number; out_bps: number; in_pct: number; out_pct: number }>>({})

const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive({
  id: 0, device_id: 0, interface: '', display_name: '', line_rate: 1000,
  up_rate: 1000, down_rate: 1000, color: '', enable: true, remark: ''
})

const ifs = ref<IfInfo[]>([])
const ifLoading = ref(false)

// 历史
const histVisible = ref(false)
const histRule = ref<TrafficRule | null>(null)
const histRef = ref<HTMLDivElement>()
let histChart: echarts.ECharts | null = null

let ws: WSClient | null = null

async function loadRules() {
  rules.value = await getEnc<TrafficRule[]>('/traffic/rules')
  // 拉取实时速率
  for (const r of rules.value) {
    if (!r.enable) continue
    try {
      const cur = await getEnc<{ up: boolean; in_bps: number; out_bps: number; in_pct: number; out_pct: number }>(
        `/traffic/rules/${r.id}/current`
      )
      currents.value[r.id] = cur
    } catch {
      /* 忽略 */
    }
  }
}

async function loadDevices() {
  devices.value = await getEnc<Array<{ id: number; name: string; ip: string }>>('/monitor/devices')
}

async function loadIfs() {
  if (!form.device_id) return
  ifLoading.value = true
  try {
    ifs.value = await getEnc<IfInfo[]>(`/traffic/devices/${form.device_id}/interfaces`)
  } finally {
    ifLoading.value = false
  }
}

function openAdd() {
  editingId.value = 0
  Object.assign(form, {
    id: 0, device_id: 0, interface: '', display_name: '', line_rate: 1000,
    up_rate: 1000, down_rate: 1000, color: pickColor(rules.value.length), enable: true, remark: ''
  })
  dialogVisible.value = true
  ifs.value = []
}
function openEdit(r: TrafficRule) {
  editingId.value = r.id
  Object.assign(form, r)
  dialogVisible.value = true
  loadIfs()
}
async function save() {
  if (!form.device_id || !form.interface) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (editingId.value) {
    await putEnc(`/traffic/rules/${editingId.value}`, form)
  } else {
    await postEnc('/traffic/rules', form)
  }
  ElMessage.success(t('common.success'))
  dialogVisible.value = false
  await loadRules()
}
async function remove(r: TrafficRule) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/traffic/rules/${r.id}`)
  ElMessage.success(t('common.success'))
  await loadRules()
}

async function showHistory(r: TrafficRule) {
  histRule.value = r
  histVisible.value = true
  await nextTick()
  const h = await getEnc<{
    in: number[][]
    out: number[][]
  }>(`/traffic/rules/${r.id}/history`)
  histChart ??= echarts.init(histRef.value as HTMLDivElement)
  histChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: [t('traffic.in'), t('traffic.out')] },
    grid: { left: 80, right: 20, top: 36, bottom: 28 },
    xAxis: { type: 'time' },
    yAxis: { type: 'value', axisLabel: { formatter: (v: number) => (v / 1e6).toFixed(0) + 'M' } },
    series: [
      { name: t('traffic.in'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#06b6d4' }, itemStyle: { color: '#06b6d4' }, areaStyle: { opacity: 0.08 }, data: h.in },
      { name: t('traffic.out'), type: 'line', showSymbol: false, smooth: true, lineStyle: { width: 2, color: '#f59e0b' }, itemStyle: { color: '#f59e0b' }, areaStyle: { opacity: 0.08 }, data: h.out }
    ]
  })
}

let _timer: any = null
onMounted(async () => {
  await Promise.all([loadDevices(), loadRules()])
  ws = new WSClient(['traffic'])
  ws.on('traffic', 'if_rate', (msg) => {
    const d = msg.data as unknown as { device_id: number; if_name: string; in_bps: number; out_bps: number }
    const r = rules.value.find((x) => x.device_id === d.device_id && (x.interface === d.if_name))
    if (r) {
      currents.value[r.id] = {
        up: true, in_bps: d.in_bps, out_bps: d.out_bps,
        in_pct: r.down_rate ? (d.in_bps / (r.down_rate * 1e6)) * 100 : 0,
        out_pct: r.up_rate ? (d.out_bps / (r.up_rate * 1e6)) * 100 : 0
      }
    }
  })
  ws.connect()
})

onBeforeUnmount(() => {
  if (_timer) clearInterval(_timer)
  ws?.close()
  histChart?.dispose()
})
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <span class="np-page-desc">{{ t('traffic.desc') }}</span>
      <div class="spacer"></div>
      <el-button type="primary" @click="openAdd">
        <el-icon><Plus /></el-icon>{{ t('traffic.addRule') }}
      </el-button>
    </div>

    <!-- 实时流量卡片 -->
    <div class="np-traffic-grid">
      <div v-for="r in rules" :key="r.id" class="np-card np-traffic-card" :style="{ borderLeft: `4px solid ${r.color}` }">
        <div class="np-tc-head">
          <div class="np-tc-head-left">
            <div class="np-tc-devname">{{ r.device_name || t('traffic.unknownDevice') }}</div>
            <div class="np-tc-meta">
              <el-tag size="small" effect="plain">{{ r.device_ip || '-' }}</el-tag>
              <el-tag size="small" type="info" effect="dark">{{ r.interface }}</el-tag>
              <span v-if="r.display_name && r.display_name !== r.interface" class="np-tc-alias">{{ r.display_name }}</span>
            </div>
          </div>
          <span class="np-tc-status" :class="{up: currents[r.id]?.up}">{{ currents[r.id]?.up ? t('traffic.online') : t('traffic.offline') }}</span>
        </div>
        <div class="np-tc-rates">
          <div class="np-tc-item">
            <div class="np-tc-label">↓ {{ t('traffic.in') }}</div>
            <div class="np-tc-value" style="color: #06b6d4">{{ fmtBps(currents[r.id]?.in_bps || 0) }}</div>
            <el-progress :percentage="Math.min(currents[r.id]?.in_pct || 0, 100)" :stroke-width="6" color="#06b6d4" :show-text="false" />
            <div class="np-tc-pct">{{ fmtPct(currents[r.id]?.in_pct || 0) }}% / {{ r.down_rate }}M</div>
          </div>
          <div class="np-tc-item">
            <div class="np-tc-label">↑ {{ t('traffic.out') }}</div>
            <div class="np-tc-value" style="color: #f59e0b">{{ fmtBps(currents[r.id]?.out_bps || 0) }}</div>
            <el-progress :percentage="Math.min(currents[r.id]?.out_pct || 0, 100)" :stroke-width="6" color="#f59e0b" :show-text="false" />
            <div class="np-tc-pct">{{ fmtPct(currents[r.id]?.out_pct || 0) }}% / {{ r.up_rate }}M</div>
          </div>
        </div>
        <div class="np-tc-actions">
          <el-button size="small" text type="primary" @click="showHistory(r)">{{ t('traffic.history') }}</el-button>
          <el-button size="small" text type="primary" @click="openEdit(r)">{{ t('common.edit') }}</el-button>
          <el-button size="small" text type="danger" @click="remove(r)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
      <div v-if="!rules.length" class="np-card np-empty">{{ t('common.noData') }}</div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? t('traffic.editRule') : t('traffic.addRule')" width="520px">
      <el-form :model="form" label-width="130px">
        <el-form-item :label="t('traffic.device')" required>
          <el-select v-model="form.device_id" filterable style="width: 100%" @change="loadIfs">
            <el-option v-for="d in devices" :key="d.id" :label="`${d.name} (${d.ip})`" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('traffic.interface')" required>
          <el-select v-model="form.interface" filterable :loading="ifLoading" style="width: 100%">
            <el-option v-for="i in ifs" :key="i.index" :label="`${i.name} (${i.oper})`" :value="i.name" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('traffic.ruleName')">
          <el-input v-model="form.display_name" />
        </el-form-item>
        <el-form-item :label="t('traffic.lineRate')">
          <el-input-number v-model="form.line_rate" :min="1" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('traffic.upRate')">
          <el-input-number v-model="form.up_rate" :min="1" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('traffic.downRate')">
          <el-input-number v-model="form.down_rate" :min="1" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('traffic.color')">
          <el-color-picker v-model="form.color" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="histVisible" :title="`${t('traffic.history')} - ${histRule?.display_name}`" width="780px">
      <div ref="histRef" class="np-chart" style="height: 320px"></div>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-traffic-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}

.np-traffic-card {
  .np-tc-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 12px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--np-border, #eee);
    .np-tc-head-left {
      min-width: 0;
    }
    .np-tc-devname {
      font-size: 16px;
      font-weight: 700;
      margin-bottom: 6px;
    }
    .np-tc-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
      .np-tc-alias {
        color: var(--np-text-2);
        font-size: 12px;
      }
    }
    .np-tc-status {
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 10px;
      background: #fef0f0;
      color: #f56c6c;
      &.up {
        background: #f0f9eb;
        color: #67c23a;
      }
    }
  }
  .np-tc-rates {
    display: flex;
    gap: 18px;
    .np-tc-item {
      flex: 1;
      .np-tc-label {
        font-size: 12px;
        color: var(--np-text-2);
      }
      .np-tc-value {
        font-size: 20px;
        font-weight: 700;
        margin: 2px 0 6px;
        font-variant-numeric: tabular-nums;
      }
      .np-tc-pct {
        font-size: 11px;
        color: var(--np-text-2);
        margin-top: 2px;
        text-align: right;
      }
    }
  }
  .np-tc-actions {
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid var(--np-border);
    margin-top: 10px;
    padding-top: 4px;
  }
}
</style>
