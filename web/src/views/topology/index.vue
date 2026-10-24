<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('topology.title') }}</div>
        <div class="np-page-head-desc">{{ t('topology.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-switch v-model="editMode" :active-text="t('topology.editMode')" inline-prompt class="te-switch" @change="onEditModeChange" />
        <el-button :icon="Refresh" size="default" @click="load(true)">{{ t('topology.refresh') }}</el-button>
        <el-button :icon="Position" size="default" @click="restartLayout">{{ t('topology.autoLayout') }}</el-button>
        <el-button :icon="FullScreen" size="default" @click="toggleFullscreen">{{ t(fs ? 'topology.exitFullscreen' : 'topology.fullscreen') }}</el-button>
        <el-button :icon="Download" size="default" @click="exportPng">{{ t('topology.exportPng') }}</el-button>
      </div>
    </div>

    <!-- 统计行 -->
    <div class="topo-stats">
      <div class="topo-stat"><span class="ts-num">{{ stats.deviceTotal }}</span><span class="ts-label">{{ t('topology.deviceTotal') }}</span></div>
      <div class="topo-stat ok"><span class="ts-num">{{ stats.deviceUp }}</span><span class="ts-label">{{ t('topology.online') }}</span></div>
      <div class="topo-stat bad"><span class="ts-num">{{ stats.deviceDown }}</span><span class="ts-label">{{ t('topology.offline') }}</span></div>
      <div class="topo-stat mute"><span class="ts-num">{{ stats.deviceUnknown }}</span><span class="ts-label">{{ t('topology.unknown') }}</span></div>
      <div class="topo-stat-sep"></div>
      <div class="topo-stat"><span class="ts-num">{{ stats.linkTotal }}</span><span class="ts-label">{{ t('topology.linkTotal') }}</span></div>
      <div class="topo-stat ok"><span class="ts-num">{{ stats.linkUp }}</span><span class="ts-label">{{ t('topology.normal') }}</span></div>
      <div class="topo-stat bad"><span class="ts-num">{{ stats.linkDown }}</span><span class="ts-label">{{ t('topology.abnormal') }}</span></div>
      <div class="topo-stat-sep"></div>
      <div class="topo-stat type-stat" v-for="(label, key) in typeLabels" :key="key">
        <span class="ts-dot" :style="{ background: typeColor[key] }"></span>
        <span class="ts-num" :style="{ color: typeColor[key] }">{{ typeStats[key] || 0 }}</span>
        <span class="ts-label">{{ label }}</span>
      </div>
      <div class="topo-stat alert-stat">
        <span class="ts-dot" style="background:#d9534f"></span>
        <span class="ts-num" style="color:#d9534f">{{ alertStats.total || 0 }}</span>
        <span class="ts-label">{{ t('topology.activeAlerts') }}</span>
      </div>
    </div>

    <!-- 主体：左导航面板 + 画布 -->
    <div class="topo-main" ref="mainRef">
      <!-- 左侧设备导航面板 -->
      <aside class="topo-side">
        <div class="ts-title">{{ t('topology.deviceList') }} <el-badge v-if="alertList.length" :value="alertList.length" type="danger" class="ts-alert-badge" /></div>
        <el-input v-model="kw" :placeholder="t('topology.searchDevice')" size="small" clearable class="ts-search">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <div class="ts-list">
          <div v-for="n in filteredDevs" :key="n.id" class="ts-item" :class="{ active: curId === n.id }" @click="focusDevice(n)">
            <span class="ts-ic" :style="{ background: statusColor(n.status) }">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="#fff"><path :d="iconOf(n.type)" /></svg>
            </span>
            <div class="ts-it">
              <div class="ts-n">{{ n.name }}</div>
              <div class="ts-m">{{ n.ip || t('topology.local') }}<span v-if="n.alerts" class="ts-a">{{ n.alerts }}</span></div>
            </div>
            <div class="ts-load" v-if="n.cpu || n.mem"><span>C{{ fmtPct(n.cpu) }}</span><span>M{{ fmtPct(n.mem) }}</span></div>
          </div>
          <div v-if="!filteredDevs.length" class="ts-empty">{{ kw ? t('topology.noMatch') : t('topology.empty') }}</div>
        </div>
        <el-button v-if="!nodes.length" size="small" type="primary" style="width:100%" @click="go('/monitor')">{{ t('topology.addDevice') }}</el-button>

        <!-- 活跃告警 -->
        <div class="ts-alerts">
          <div class="ts-title">{{ t('topology.activeAlerts') }} <span v-if="alertStats.critical" class="ts-crit">{{ alertStats.critical }}</span></div>
          <div v-if="alertList.length" class="ts-alert-list">
            <div v-for="a in alertList" :key="a.id" class="ts-alert-item" :title="a.message">
              <span class="ts-alv" :class="a.level">{{ a.level }}</span>
              <div class="ts-alert-body">
                <div class="ts-alert-msg">{{ a.message || a.target }}</div>
                <div class="ts-alert-time">{{ fmtTime(a.created_at) }}</div>
              </div>
            </div>
          </div>
          <div v-else class="ts-noalerts">{{ t('topology.noAlerts') }}</div>
        </div>
      </aside>

      <!-- 画布 -->
      <div class="topo-wrap" :class="{ fs: fs }">
        <div class="topo-canvas" ref="chartRef"></div>
        <!-- 编辑工具栏（编辑模式） -->
        <div v-if="editMode" class="topo-edittool">
          <el-button size="small" type="primary" @click="openAddNode()">
            <el-icon style="margin-right:4px"><Plus /></el-icon>{{ t('topology.addNode') }}
          </el-button>
          <el-button size="small" @click="openAddLink()">
            <el-icon style="margin-right:4px"><Link /></el-icon>{{ t('topology.addLinkManual') }}
          </el-button>
          <div class="te-tip">{{ t('topology.editTip') }}</div>
        </div>
        <div class="topo-legend">
          <div class="tl-title">{{ t('topology.legend') }}</div>
          <div class="tl-group">
            <div class="tl-name">{{ t('topology.nodeLegend') }}</div>
            <div class="tl-row"><span class="dot" style="background:#3f9e63"></span>{{ t('topology.up') }}</div>
            <div class="tl-row"><span class="dot" style="background:#d9534f"></span>{{ t('topology.down') }}</div>
            <div class="tl-row"><span class="dot" style="background:#9aa0a6"></span>{{ t('topology.unknown') }}</div>
          </div>
          <div class="tl-group">
            <div class="tl-name">{{ t('topology.linkLegend') }}</div>
            <div class="tl-row"><span class="dot" style="background:#3f9e63"></span>{{ t('topology.normal') }}</div>
            <div class="tl-row"><span class="dot" style="background:#d9534f"></span>{{ t('topology.abnormal') }}</div>
            <div class="tl-row"><span class="dot" style="background:#9aa0a6"></span>{{ t('topology.unknown') }}</div>
          </div>
        </div>
        <el-empty v-if="empty" :description="t('topology.empty')" :image-size="120">
          <el-button type="primary" @click="go('/monitor')">{{ t('topology.addDevice') }}</el-button>
          <el-button @click="go('/linkdetect')">{{ t('topology.addLink') }}</el-button>
        </el-empty>
      </div>
    </div>

    <!-- 设备/链路详情抽屉 -->
    <el-drawer v-model="drawer" :title="detailTitle" size="420px" :with-header="true">
      <div v-if="cur" class="topo-detail">
        <div class="td-hero">
          <div class="td-icon" :style="{ color: statusColor(cur.status) }">
            <svg viewBox="0 0 24 24" width="38" height="38" fill="currentColor"><path :d="iconOf(cur.type)" /></svg>
          </div>
          <div>
            <div class="td-name">{{ cur.name }}</div>
            <el-tag size="small" :type="cur.status === 'up' ? 'success' : cur.status === 'down' ? 'danger' : 'info'">
              {{ t('topology.' + (cur.status === 'up' ? 'up' : cur.status === 'down' ? 'down' : 'unknown')) }}
            </el-tag>
            <span v-if="cur.alerts" class="td-alert">{{ t('topology.alerts') }}: {{ cur.alerts }}</span>
          </div>
        </div>
        <el-tabs v-model="detailTab">
          <el-tab-pane :label="t('topology.basics')" name="basics">
            <el-descriptions :column="1" size="small" class="td-desc">
              <el-descriptions-item v-if="cur.ip" :label="t('topology.ip')">{{ cur.ip }}</el-descriptions-item>
              <el-descriptions-item :label="t('topology.type')">{{ typeName(cur.type) }}</el-descriptions-item>
              <el-descriptions-item v-if="cur.vendor" :label="t('topology.vendor')">{{ cur.vendor }}</el-descriptions-item>
              <template v-if="isDevice">
                <el-descriptions-item :label="t('topology.cpu')">{{ cur.cpu ? cur.cpu.toFixed(1) + '%' : '--' }}</el-descriptions-item>
                <el-descriptions-item :label="t('topology.mem')">{{ cur.mem ? cur.mem.toFixed(1) + '%' : '--' }}</el-descriptions-item>
                <el-descriptions-item v-if="cur.uptime" :label="t('topology.uptime')">{{ fmtUptime(cur.uptime) }}</el-descriptions-item>
                <el-descriptions-item :label="t('topology.node')">{{ cur.node_uuid ? cur.node_uuid : t('topology.local') }}</el-descriptions-item>
              </template>
              <template v-if="!isDevice">
                <el-descriptions-item :label="t('topology.method')">{{ cur.method }}</el-descriptions-item>
                <el-descriptions-item v-if="cur.port" :label="t('topology.port')">{{ cur.port }}</el-descriptions-item>
                <el-descriptions-item :label="t('topology.latency')">{{ cur.rt ? cur.rt.toFixed(1) + ' ' + t('topology.ms') : '--' }}</el-descriptions-item>
                <el-descriptions-item :label="t('topology.loss')">{{ cur.loss ? cur.loss.toFixed(1) + '%' : '--' }}</el-descriptions-item>
              </template>
            </el-descriptions>
            <template v-if="editMode && isManualNode">
              <el-button type="primary" style="width:100%;margin-top:16px" @click="openEditNode()">{{ t('topology.edit') }}</el-button>
              <el-button type="danger" style="width:100%;margin-top:8px" @click="delNode()">{{ t('topology.del') }}</el-button>
            </template>
            <template v-else-if="editMode && isManualLink">
              <el-button type="primary" style="width:100%;margin-top:16px" @click="openEditLink()">{{ t('topology.edit') }}</el-button>
              <el-button type="danger" style="width:100%;margin-top:8px" @click="delLink()">{{ t('topology.del') }}</el-button>
            </template>
            <el-button v-else-if="isDevice" type="primary" style="width:100%;margin-top:16px" @click="go('/monitor')">{{ t('topology.viewDevice') }}</el-button>
            <el-button v-else type="primary" style="width:100%;margin-top:16px" @click="go('/linkdetect')">{{ t('topology.viewLink') }}</el-button>
          </el-tab-pane>
          <el-tab-pane :label="t('topology.history')" name="history" lazy>
            <div class="th-range">
              <el-radio-group v-model="histRange" size="small" @change="loadHistory">
                <el-radio-button value="1">{{ t('topology.range1h') }}</el-radio-button>
                <el-radio-button value="24">{{ t('topology.range24h') }}</el-radio-button>
              </el-radio-group>
            </div>
            <div class="th-chart" ref="histRef"></div>
            <div class="th-loading" v-if="histLoading">{{ t('topology.loadFail') }}</div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>

    <!-- 手工节点：新增/编辑 -->
    <el-dialog :title="t(editingManual ? 'topology.edit' : 'topology.addNode')" v-model="nodeDlg" width="420px" append-to-body>
      <el-form label-width="84px" @submit.prevent>
        <el-form-item :label="t('topology.nodeName')" required>
          <el-input v-model="nodeForm.name" maxlength="128" />
        </el-form-item>
        <el-form-item :label="t('topology.nodeIp')">
          <el-input v-model="nodeForm.ip" maxlength="64" placeholder="0.0.0.0" />
        </el-form-item>
        <el-form-item :label="t('topology.typeDist')">
          <el-select v-model="nodeForm.type" style="width:100%">
            <el-option :label="t('topology.server')" value="server" />
            <el-option :label="t('topology.switch')" value="switch" />
            <el-option :label="t('topology.router')" value="router" />
            <el-option :label="t('topology.firewall')" value="firewall" />
            <el-option :label="t('topology.other')" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.status')">
          <el-select v-model="nodeForm.status" style="width:100%">
            <el-option :label="t('topology.statusUp')" value="up" />
            <el-option :label="t('topology.statusDown')" value="down" />
            <el-option :label="t('topology.statusUnknown')" value="unknown" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.remark')">
          <el-input v-model="nodeForm.remark" type="textarea" :rows="2" maxlength="256" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nodeDlg = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveNode()">{{ t('topology.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 手工链路：新增/编辑 -->
    <el-dialog :title="t(editingManual ? 'topology.edit' : 'topology.addLinkManual')" v-model="linkDlg" width="440px" append-to-body>
      <el-form label-width="84px" @submit.prevent>
        <el-form-item :label="t('topology.linkName')" required>
          <el-input v-model="linkForm.name" maxlength="128" />
        </el-form-item>
        <el-form-item :label="t('topology.chooseSource')" required>
          <el-select v-model="linkForm.source" filterable style="width:100%" :placeholder="t('topology.chooseSource')">
            <el-option v-for="o in nodeOptions" :key="o.id" :label="o.label" :value="o.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.chooseTarget')" required>
          <el-select v-model="linkForm.target" filterable style="width:100%" :placeholder="t('topology.chooseTarget')">
            <el-option v-for="o in nodeOptions" :key="o.id" :label="o.label" :value="o.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.method')">
          <el-select v-model="linkForm.method" style="width:100%">
            <el-option :label="t('topology.methodManual')" value="manual" />
            <el-option label="ICMP" value="icmp" />
            <el-option label="TCP" value="tcp" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.port')">
          <el-input-number v-model="linkForm.port" :min="0" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('topology.status')">
          <el-select v-model="linkForm.status" style="width:100%">
            <el-option :label="t('topology.statusUp')" value="up" />
            <el-option :label="t('topology.statusDown')" value="down" />
            <el-option :label="t('topology.statusUnknown')" value="unknown" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topology.color')">
          <el-color-picker v-model="linkForm.color" />
        </el-form-item>
        <el-form-item :label="t('topology.remark')">
          <el-input v-model="linkForm.remark" type="textarea" :rows="2" maxlength="256" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkDlg = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveLink()">{{ t('topology.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Refresh, Position, FullScreen, Download, Search, Plus, Link } from '@element-plus/icons-vue'
import * as echarts from '@/utils/echarts'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { useAppStore } from '@/stores/app'
import { cssVar, chartAxisColors, fmtTime } from '@/utils'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const dark = computed(() => appStore.theme === 'dark')

const chartRef = ref<HTMLDivElement>()
const mainRef = ref<HTMLDivElement>()
const histRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let histChart: echarts.ECharts | null = null
let timer: ReturnType<typeof setInterval> | null = null
let histTimer: ReturnType<typeof setInterval> | null = null

const nodes = ref<any[]>([])
const links = ref<any[]>([])
const typeStats = ref<Record<string, number>>({})
const alertStats = ref<{ total: number; critical: number }>({ total: 0, critical: 0 })
const alertList = ref<any[]>([])
const stats = ref({ deviceTotal: 0, deviceUp: 0, deviceDown: 0, deviceUnknown: 0, linkTotal: 0, linkUp: 0, linkDown: 0 })
const empty = computed(() => nodes.value.length === 0)

const kw = ref('')
const fs = ref(false)
// ===== 编辑模式：手工添加/修改节点与链路 =====
const editMode = ref(false)
const nodeDlg = ref(false)
const linkDlg = ref(false)
const editingManual = ref(false) // 当前弹窗是新增(false)还是编辑(true)
const nodeForm = reactive<any>({ id: 0, name: '', ip: '', type: 'server', status: 'unknown', x: 0, y: 0, remark: '' })
const linkForm = reactive<any>({ id: 0, name: '', source: '', target: '', method: 'manual', port: 0, status: 'unknown', color: '#3d9df2', remark: '' })
const nodeOptions = computed(() => nodes.value.map((n: any) => ({ id: n.id, label: `${n.name}${n.ip ? ' (' + n.ip + ')' : ''}` })))
const typeLabels: Record<string, string> = {
  server: t('topology.server'), switch: t('topology.switch'), router: t('topology.router'),
  firewall: t('topology.firewall'), other: t('topology.other')
}
const typeColor: Record<string, string> = {
  server: '#3d9df2', switch: '#5ab8ff', router: '#ff9a3d', firewall: '#d9534f', other: '#9aa0a6'
}
const devs = computed(() => nodes.value.filter((n: any) => !n.id.startsWith('ext_')))
const filteredDevs = computed(() => {
  const q = kw.value.trim().toLowerCase()
  if (!q) return devs.value
  return devs.value.filter((d: any) => (d.name + ' ' + (d.ip || '')).toLowerCase().includes(q))
})

// 详情抽屉
const drawer = ref(false)
const cur = ref<any>(null)
const curId = ref('')
const detailTab = ref('basics')
const histRange = ref('1')
const histLoading = ref(false)
const isDevice = computed(() => !cur.value || cur.value.kind === 'device')
const detailTitle = computed(() => t(isDevice.value ? 'topo.deviceDetail' : 'topo.linkDetail'))

const ICONS: Record<string, string> = {
  server: 'M3,5h18v4H3zM3,11h18v4H3zM3,17h18v4H3zM6,6.5h1v-1H6zM6,12.5h1v-1H6zM6,18.5h1v-1H6z',
  switch: 'M2,7h20v10H2zM4,9h3v6H4zM8,9h12v6H8zM8,11h1v2H8zM10,11h1v2h-1zM12,11h1v2h-1zM14,11h1v2h-1zM16,11h1v2h-1z',
  router: 'M5,9a7,7 0 0 1 14,0v6H5V9zM8,11h3M13,11h3M8,14h3M13,14h3M5,17h14v2H5z',
  firewall: 'M12,2l8,3v6c0,5 -3,8 -8,11c-5,-3 -8,-6 -8,-11V5zM9,12l2,2l4,-4',
  other: 'M12,12m-9,0a9,9 0 1 1 18,0a9,9 0 1 1 -18,0',
  target: 'M12,12m-9,0a9,9 0 1 1 18,0a9,9 0 1 1 -18,0M12,12m-4,0a4,4 0 1 1 8,0a4,4 0 1 1 -8,0'
}
const STATUS = { up: '#3f9e63', down: '#d9534f', unknown: '#9aa0a6' }
function statusColor(s: string) { return STATUS[s as keyof typeof STATUS] ?? STATUS.unknown }
function typeName(type: string) {
  if (type === 'server') return t('topology.server')
  if (type === 'switch') return t('topology.switch')
  if (type === 'router') return t('topology.router')
  if (type === 'firewall') return t('topology.firewall')
  if (type === 'target') return t('topology.targetType')
  return t('topology.other')
}
function iconOf(type: string) { return ICONS[type] ?? ICONS.other }
function fmtPct(v: number) { return v ? v.toFixed(0) + '%' : '--' }
function fmtUptime(sec: number) {
  if (!sec) return '--'
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60)
  return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : `${m}m`
}

async function load(forceLayout = false) {
  try {
    const r = await getEnc<any>('/topology/overview')
    if (!r || !r.data) return
    const data = r.data
    nodes.value = data.nodes ?? []
    links.value = data.links ?? []
    typeStats.value = data.typeStats ?? {}
    alertStats.value = data.alertStats ?? { total: 0, critical: 0 }
    alertList.value = data.alerts ?? []

    const devsArr = nodes.value.filter((n: any) => !n.id.startsWith('ext_'))
    stats.value = {
      deviceTotal: devsArr.length,
      deviceUp: devsArr.filter((d: any) => d.status === 'up').length,
      deviceDown: devsArr.filter((d: any) => d.status === 'down').length,
      deviceUnknown: devsArr.filter((d: any) => d.status !== 'up' && d.status !== 'down').length,
      linkTotal: links.value.length,
      linkUp: links.value.filter((l: any) => l.status === 'up').length,
      linkDown: links.value.filter((l: any) => l.status !== 'up').length
    }
    render(forceLayout)
  } catch { /* 静默 */ }
}

function render(forceLayout = false) {
  if (!chartRef.value) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
    chart.on('click', onChartClick)
    chart.on('mouseup', onChartMouseUp)
    window.addEventListener('resize', onResize)
  }
  const ax = chartAxisColors()

  // 固定布局：任一节点带坐标记录 → 全部用记录坐标（缺失节点落默认网格）；否则 force 自动布局
  const hasPos = nodes.value.some((n: any) => typeof n.x === 'number')
  const layout = hasPos ? 'none' : 'force'
  const defaultPos = (i: number): [number, number] => {
    const cols = 5, gap = 190, base = 120
    return [base + (i % cols) * gap, base + Math.floor(i / cols) * gap]
  }

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: cssVar('--np-bg-raise', 'rgba(255,255,255,0.96)'),
      borderColor: cssVar('--np-border', '#d0d4da'),
      borderWidth: 1,
      textStyle: { color: cssVar('--np-text', '#1a1d23'), fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        if (!d) return ''
        const st = d.status === 'up' ? t('topology.up') : d.status === 'down' ? t('topology.down') : t('topology.unknown')
        if (d.kind === 'link') {
          return `${d.name}<br/><span style="color:${cssVar('--np-text-2', '#5a6070')}">${st}${d.rt ? ' · ' + d.rt.toFixed(1) + 'ms' : ''}${d.loss ? ' · ' + d.loss.toFixed(1) + '%' : ''}</span>`
        }
        return `${d.name}<br/><span style="color:${cssVar('--np-text-2', '#5a6070')}">${d.ip ?? ''} · ${st}${d.alerts ? ' · ⚠' + d.alerts : ''}</span>`
      }
    },
    series: [{
      type: 'graph',
      layout,
      roam: true,
      draggable: editMode.value,
      symbolSize: 46,
      label: { show: true, position: 'bottom', color: cssVar('--np-text', '#1a1d23'), fontSize: 11 },
      edgeLabel: { show: true, color: cssVar('--np-text-2', '#5a6070'), fontSize: 10, formatter: (p: any) => (p.data.rt ? p.data.rt.toFixed(0) + 'ms' : '') },
      force: { repulsion: 260, edgeLength: 150, gravity: 0.1, layoutAnimation: forceLayout },
      emphasis: { focus: 'adjacency', label: { fontWeight: 600, fontSize: 13 } },
      data: nodes.value.map((n: any, i: number) => {
        const [dx, dy] = hasPos ? (typeof n.x === 'number' && typeof n.y === 'number' ? [n.x, n.y] : defaultPos(i)) : [undefined, undefined]
        return {
          id: n.id,
          name: n.name,
          ip: n.ip,
          vendor: n.vendor,
          type: n.type,
          status: n.status,
          cpu: n.cpu,
          mem: n.mem,
          uptime: n.uptime,
          alerts: n.alerts,
          node_uuid: n.node_uuid,
          manual: !!n.manual,
          kind: 'device',
          symbol: 'path://' + (ICONS[n.type] ?? ICONS.other),
          itemStyle: { color: statusColor(n.status) },
          label: { show: true, position: 'bottom', formatter: n.name, color: cssVar('--np-text', '#1a1d23'), fontSize: 11 },
          x: dx,
          y: dy
        }
      }),
      links: links.value.map((l: any) => ({
        id: l.id,
        source: l.source,
        target: l.target,
        name: l.name,
        method: l.method,
        port: l.port,
        status: l.status,
        rt: l.rt,
        loss: l.loss,
        manual: !!l.manual,
        kind: 'link',
        lineStyle: { color: l.color ?? statusColor(l.status), width: l.status === 'up' ? 2 : 2.5, opacity: l.status === 'up' ? 0.85 : 1, curveness: 0.15 },
        edgeLabel: { show: true, formatter: (p: any) => (l.rt ? l.rt.toFixed(0) + 'ms' : ''), color: cssVar('--np-text-2', '#5a6070'), fontSize: 10 }
      }))
    }]
  }, forceLayout)
}

// 拖拽结束（编辑模式）→ 防抖保存全部节点位置
let posTimer: any = null
function onChartMouseUp() {
  if (!editMode.value || !chart) return
  if (posTimer) clearTimeout(posTimer)
  posTimer = setTimeout(async () => {
    const opt: any = chart!.getOption()
    const data: any[] = opt?.series?.[0]?.data ?? []
    const positions = data
      .filter((d: any) => d?.id && typeof d?.x === 'number' && typeof d?.y === 'number')
      .map((d: any) => ({ id: d.id, x: d.x, y: d.y }))
    if (!positions.length) return
    try {
      await putEnc<any>('/topology/positions', { positions })
      ElMessage.success(t('topology.posSaved'))
    } catch { /* 静默 */ }
  }, 600)
}

function onChartClick(p: any) {
  if (!p || p.dataType === undefined) return
  const d = p.data
  if (!d) return
  cur.value = d.kind === 'link' ? { ...d, kind: 'link' } : { ...d, kind: 'device' }
  curId.value = d.kind === 'link' ? '' : d.id
  detailTab.value = 'basics'
  drawer.value = true
}

// ===== 手工编辑：节点 =====
function openAddNode() {
  editingManual.value = false
  Object.assign(nodeForm, { id: 0, name: '', ip: '', type: 'server', status: 'unknown', x: 0, y: 0, remark: '' })
  nodeDlg.value = true
}
function openEditNode() {
  const idStr = (cur.value?.id || '').toString().replace(/^tn_/, '')
  const id = Number(idStr)
  const n = nodes.value.find((x: any) => x.id === cur.value?.id)
  if (!id || !n) return
  editingManual.value = true
  Object.assign(nodeForm, { id, name: n.name, ip: n.ip ?? '', type: n.type, status: n.status, remark: '' })
  nodeDlg.value = true
}
async function saveNode() {
  if (!nodeForm.name.trim()) {
    ElMessage.warning(t('topology.nodeName') + ' ' + t('common.required'))
    return
  }
  if (editingManual.value) {
    await putEnc<any>(`/topology/nodes/${nodeForm.id}`, {
      name: nodeForm.name, ip: nodeForm.ip, type: nodeForm.type, status: nodeForm.status, remark: nodeForm.remark
    })
  } else {
    const cols = 5, gap = 190, base = 120
    const i = nodes.value.length
    await postEnc<any>('/topology/nodes', {
      name: nodeForm.name, ip: nodeForm.ip, type: nodeForm.type, status: nodeForm.status,
      x: base + (i % cols) * gap, y: base + Math.floor(i / cols) * gap, remark: nodeForm.remark
    })
  }
  nodeDlg.value = false
  drawer.value = false
  await load()
}
async function delNode() {
  await ElMessageBox.confirm(t('topology.delNodeTip'), t('common.warning'), { type: 'warning', confirmButtonText: t('topology.del'), cancelButtonText: t('common.cancel') })
  const id = Number((cur.value?.id || '').toString().replace(/^tn_/, ''))
  if (!id) return
  await delEnc<any>(`/topology/nodes/${id}`)
  drawer.value = false
  await load()
}

// ===== 手工编辑：链路 =====
function openAddLink() {
  editingManual.value = false
  Object.assign(linkForm, { id: 0, name: '', source: '', target: '', method: 'manual', port: 0, status: 'unknown', color: '#3d9df2', remark: '' })
  linkDlg.value = true
}
function openEditLink() {
  const idStr = (cur.value?.id || '').toString().replace(/^ml_/, '')
  const id = Number(idStr)
  const l = links.value.find((x: any) => x.id === cur.value?.id)
  if (!id || !l) return
  editingManual.value = true
  Object.assign(linkForm, {
    id, name: l.name, source: l.source, target: l.target,
    method: l.method || 'manual', port: l.port || 0, status: l.status, color: l.color || '#3d9df2', remark: ''
  })
  linkDlg.value = true
}
async function saveLink() {
  if (!linkForm.name.trim()) {
    ElMessage.warning(t('topology.linkName') + ' ' + t('common.required'))
    return
  }
  if (!linkForm.source || !linkForm.target) {
    ElMessage.warning(t('topology.chooseSource') + ' / ' + t('topology.chooseTarget'))
    return
  }
  if (linkForm.source === linkForm.target) {
    ElMessage.warning(t('topology.chooseSource') + ' / ' + t('topology.chooseTarget'))
    return
  }
  if (editingManual.value) {
    await putEnc<any>(`/topology/links/${linkForm.id}`, {
      name: linkForm.name, source: linkForm.source, target: linkForm.target,
      method: linkForm.method, port: Number(linkForm.port) || 0, status: linkForm.status, color: linkForm.color, remark: linkForm.remark
    })
  } else {
    await postEnc<any>('/topology/links', {
      name: linkForm.name, source: linkForm.source, target: linkForm.target,
      method: linkForm.method, port: Number(linkForm.port) || 0, status: linkForm.status, color: linkForm.color, remark: linkForm.remark
    })
  }
  linkDlg.value = false
  drawer.value = false
  await load()
}
async function delLink() {
  await ElMessageBox.confirm(t('topology.delLinkTip'), t('common.warning'), { type: 'warning', confirmButtonText: t('topology.del'), cancelButtonText: t('common.cancel') })
  const id = Number((cur.value?.id || '').toString().replace(/^ml_/, ''))
  if (!id) return
  await delEnc<any>(`/topology/links/${id}`)
  drawer.value = false
  await load()
}
const isManualNode = computed(() => !!cur.value && cur.value.kind === 'device' && (cur.value.id || '').toString().startsWith('tn_'))
const isManualLink = computed(() => !!cur.value && cur.value.kind === 'link' && (cur.value.id || '').toString().startsWith('ml_'))

function onEditModeChange() {
  if (!editMode.value) {
    drawer.value = false
  }
  render()
}

// 左侧列表点击：定位并高亮画布节点 + 打开详情
function focusDevice(n: any) {
  cur.value = { ...n, kind: 'device' }
  curId.value = n.id
  detailTab.value = 'basics'
  drawer.value = true
  if (chart) {
    const idx = nodes.value.findIndex((x: any) => x.id === n.id)
    if (idx >= 0) {
      chart.dispatchAction({ type: 'downplay', seriesIndex: 0 })
      chart.dispatchAction({ type: 'highlight', seriesIndex: 0, dataIndex: idx })
    }
  }
}

// 历史趋势
async function loadHistory() {
  if (!histRef.value || !cur.value) return
  const idRaw = (cur.value.id || '').toString()
  // 手工节点/链路无监控历史
  if (idRaw.startsWith('tn_') || idRaw.startsWith('ml_')) {
    histLoading.value = false
    histChart?.clear()
    if (histChart) {
      histChart.setOption({ title: { text: t('topology.noHistory'), left: 'center', top: 'middle', textStyle: { color: cssVar('--np-text-2', '#5a6070'), fontSize: 12, fontWeight: 400 } } })
    }
    return
  }
  const idStr = idRaw.replace(/^(dev_|link_)/, '')
  if (!idStr) return
  histLoading.value = true
  try {
    const kind = isDevice.value ? 'device' : 'link'
    const r = await getEnc<any>(`/topology/history?kind=${kind}&id=${idStr}&hours=${histRange.value}`)
    if (!histChart) histChart = echarts.init(histRef.value)
    const ax = chartAxisColors()
    const isDev = isDevice.value
    const s1 = isDev ? (r.data?.cpu ?? []) : (r.data?.rt ?? [])
    const s2 = isDev ? (r.data?.mem ?? []) : (r.data?.loss ?? [])
    const name1 = isDev ? t('topology.cpu') + ' %' : t('topology.latency') + ' (' + t('topology.ms') + ')'
    const name2 = isDev ? t('topology.mem') + ' %' : t('topology.loss') + ' %'
    const c1 = isDev ? '#ff9a3d' : '#3d9df2'
    const c2 = isDev ? '#8a4bff' : '#d9534f'
    histChart.setOption({
      tooltip: {
        trigger: 'axis',
        confine: true,
        backgroundColor: cssVar('--np-bg-raise', 'rgba(255,255,255,0.96)'),
        borderColor: cssVar('--np-border', '#d0d4da'),
        borderWidth: 1,
        textStyle: { color: cssVar('--np-text', '#1a1d23'), fontSize: 12 },
        valueFormatter: (v: any) => (typeof v === 'number' ? (isDev ? v.toFixed(1) + '%' : v.toFixed(1)) : '-')
      },
      legend: { data: [name1, name2], textStyle: { color: cssVar('--np-text-2', '#5a6070') }, top: 0 },
      grid: { left: 8, right: 8, top: 30, bottom: 4, containLabel: true },
      xAxis: {
        type: 'time',
        axisLine: { lineStyle: { color: ax.axis } },
        axisLabel: { color: ax.axis, fontSize: 10 },
        splitLine: { show: false }
      },
      yAxis: {
        type: 'value',
        scale: true,
        axisLine: { show: false },
        axisLabel: { color: ax.axis, fontSize: 10 },
        splitLine: { lineStyle: { color: ax.split } }
      },
      series: [
        { name: name1, type: 'line', data: s1.map((p: any) => [p[0], p[1]]), smooth: true, showSymbol: false, lineStyle: { width: 2, color: c1 }, itemStyle: { color: c1 }, areaStyle: { opacity: 0.08, color: c1 } },
        { name: name2, type: 'line', data: s2.map((p: any) => [p[0], p[1]]), smooth: true, showSymbol: false, lineStyle: { width: 2, color: c2 }, itemStyle: { color: c2 }, areaStyle: { opacity: 0.08, color: c2 } }
      ]
    }, true)
  } catch {
    histChart?.clear()
  } finally {
    histLoading.value = false
  }
}

function restartLayout() { render(true) }
function go(path: string) { drawer.value = false; router.push(path) }
function onResize() { chart?.resize() }

// 全屏
function toggleFullscreen() {
  if (!mainRef.value) return
  if (document.fullscreenElement) {
    document.exitFullscreen().catch(() => {})
    fs.value = false
  } else {
    mainRef.value.requestFullscreen().catch(() => {})
    fs.value = true
  }
}
function onFsChange() { fs.value = !!document.fullscreenElement }
function exportPng() {
  if (!chart) return
  const url = chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: cssVar('--np-bg', '#f5f6f8') })
  const a = document.createElement('a')
  a.href = url
  a.download = 'topology-' + new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-') + '.png'
  a.click()
}

watch(dark, () => nextTick(() => { render(); if (histChart && drawer.value) loadHistory() }))
watch(drawer, (v) => { if (v && detailTab.value === 'history') nextTick(loadHistory) })
watch(detailTab, (v) => { if (v === 'history') nextTick(loadHistory) })

onMounted(() => {
  load(true)
  timer = setInterval(() => load(), 30000)
  document.addEventListener('fullscreenchange', onFsChange)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (histTimer) clearInterval(histTimer)
  window.removeEventListener('resize', onResize)
  document.removeEventListener('fullscreenchange', onFsChange)
  chart?.dispose()
  chart = null
  histChart?.dispose()
  histChart = null
})
</script>

<style scoped>
.np-page { display: flex; flex-direction: column; gap: 16px; }
.np-page-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.np-page-head-info { display: flex; flex-direction: column; gap: 4px; }
.np-page-head-title { font-size: 20px; font-weight: 600; letter-spacing: 0.01em; }
.np-page-head-desc { font-size: 13px; color: var(--np-text-2, #5a6070); }
.np-page-head-actions { display: flex; gap: 8px; flex-wrap: wrap; }

/* 统计行 */
.topo-stats { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.topo-stat {
  display: flex; align-items: baseline; gap: 8px; padding: 10px 16px; border-radius: 10px;
  background: var(--np-bg-raise, #fff); border: 1px solid var(--np-border, #e3e6ea);
}
.ts-num { font-size: 20px; font-weight: 700; font-variant-numeric: tabular-nums; }
.ts-label { font-size: 12px; color: var(--np-text-2, #5a6070); }
.topo-stat.ok .ts-num { color: #3f9e63; }
.topo-stat.bad .ts-num { color: #d9534f; }
.topo-stat.mute .ts-num { color: #9aa0a6; }
.topo-stat-sep { width: 1px; height: 32px; background: var(--np-border, #e3e6ea); }
.type-stat .ts-num { font-size: 16px; }
.alert-stat { border-color: rgba(217, 83, 79, 0.4); }
.ts-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }

/* 主体布局 */
.topo-main { display: flex; gap: 12px; align-items: stretch; }
.topo-main.fs { gap: 0; }

/* 左侧面板 */
.topo-side {
  width: 250px; flex: none; display: flex; flex-direction: column; gap: 10px;
  padding: 12px; border-radius: 12px; background: var(--np-bg-raise, #fff);
  border: 1px solid var(--np-border, #e3e6ea); min-height: 0;
}
.ts-title { font-size: 13px; font-weight: 600; display: flex; align-items: center; gap: 6px; }
.ts-alert-badge { margin-left: auto; }
.ts-search { width: 100%; }
.ts-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; min-height: 120px; }
.ts-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 8px; cursor: pointer;
  border: 1px solid transparent; transition: background 0.15s;
}
.ts-item:hover { background: var(--np-bg-hover, rgba(0,0,0,0.04)); }
.ts-item.active { background: var(--np-primary-weak, rgba(64,158,255,0.12)); border-color: rgba(64,158,255,0.4); }
.ts-ic { width: 22px; height: 22px; border-radius: 6px; display: flex; align-items: center; justify-content: center; flex: none; }
.ts-it { flex: 1; min-width: 0; }
.ts-n { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ts-m { font-size: 11px; color: var(--np-text-2, #5a6070); display: flex; align-items: center; gap: 6px; }
.ts-a { background: #d9534f; color: #fff; border-radius: 8px; padding: 0 5px; font-size: 10px; line-height: 14px; }
.ts-load { display: flex; gap: 4px; font-size: 10px; color: var(--np-text-2, #5a6070); font-variant-numeric: tabular-nums; }
.ts-empty { color: var(--np-text-2, #5a6070); font-size: 12px; text-align: center; padding: 20px 0; }

/* 活跃告警 */
.ts-alerts { border-top: 1px solid var(--np-border, #e3e6ea); padding-top: 10px; max-height: 200px; display: flex; flex-direction: column; }
.ts-crit { background: #d9534f; color: #fff; border-radius: 8px; padding: 0 6px; font-size: 10px; line-height: 15px; }
.ts-alert-list { overflow-y: auto; display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
.ts-alert-item { display: flex; gap: 6px; align-items: flex-start; font-size: 12px; padding: 4px 6px; border-radius: 6px; background: var(--np-bg, #f5f6f8); }
.ts-alv { font-size: 10px; line-height: 14px; border-radius: 4px; padding: 0 4px; color: #fff; flex: none; text-transform: uppercase; }
.ts-alv.warning { background: #e6a23c; }
.ts-alv.critical { background: #d9534f; }
.ts-alv.info { background: #3d9df2; }
.ts-alert-body { min-width: 0; }
.ts-alert-msg { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--np-text-1, #1a1d23); }
.ts-alert-time { font-size: 10px; color: var(--np-text-2, #5a6070); }
.ts-noalerts { color: var(--np-text-2, #5a6070); font-size: 12px; padding: 8px 0; }

/* 画布 */
.topo-wrap { position: relative; flex: 1; min-width: 0; }
.topo-canvas { width: 100%; height: calc(100vh - 340px); min-height: 420px; border-radius: 12px; border: 1px solid var(--np-border, #e3e6ea); background: var(--np-bg, #f5f6f8); }
.topo-legend {
  position: absolute; top: 12px; right: 12px; padding: 10px 12px; border-radius: 10px;
  background: var(--np-bg-raise, rgba(255,255,255,0.96)); border: 1px solid var(--np-border, #e3e6ea);
  font-size: 12px; display: flex; flex-direction: column; gap: 8px; z-index: 5;
}
.tl-title { font-weight: 600; margin-bottom: 2px; }
.tl-group { display: flex; flex-direction: column; gap: 3px; }
.tl-name { color: var(--np-text-2, #5a6070); font-size: 11px; }
.tl-row { display: flex; align-items: center; gap: 6px; }
.dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }

/* 详情 */
.topo-detail { display: flex; flex-direction: column; gap: 12px; }
.td-hero { display: flex; align-items: center; gap: 12px; }
.td-icon { font-size: 34px; line-height: 1; width: 46px; text-align: center; }
.td-name { font-size: 16px; font-weight: 600; margin-bottom: 4px; }
.td-alert { margin-left: 8px; color: #d9534f; font-size: 12px; }
.td-desc { width: 100%; }
.th-range { display: flex; justify-content: flex-end; margin-bottom: 8px; }
.th-chart { height: 240px; width: 100%; }
.th-loading { color: var(--np-text-2, #5a6070); font-size: 12px; text-align: center; padding: 40px 0; }

/* 编辑模式 */
.te-switch { margin-right: 8px; }
.topo-edittool {
  position: absolute; top: 12px; left: 12px; display: flex; align-items: center; gap: 8px;
  padding: 8px 10px; border-radius: 10px; z-index: 6;
  background: var(--np-bg-raise, rgba(255,255,255,0.96)); border: 1px solid var(--np-border, #e3e6ea);
  box-shadow: 0 4px 16px rgba(31, 45, 61, 0.08);
}
.te-tip { color: var(--np-text-2, #5a6070); font-size: 11px; max-width: 300px; line-height: 1.4; }
</style>
