<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('topology.title') }}</div>
        <div class="np-page-head-desc">{{ t('topology.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button :icon="Refresh" size="default" @click="load(true)">{{ t('topology.refresh') }}</el-button>
        <el-button :icon="Position" size="default" @click="restartLayout">{{ t('topology.autoLayout') }}</el-button>
      </div>
    </div>

    <!-- 统计卡 -->
    <div class="topo-stats">
      <div class="topo-stat"><span class="ts-num">{{ stats.deviceTotal }}</span><span class="ts-label">{{ t('topology.deviceTotal') }}</span></div>
      <div class="topo-stat ok"><span class="ts-num">{{ stats.deviceUp }}</span><span class="ts-label">{{ t('topology.online') }}</span></div>
      <div class="topo-stat bad"><span class="ts-num">{{ stats.deviceDown }}</span><span class="ts-label">{{ t('topology.offline') }}</span></div>
      <div class="topo-stat mute"><span class="ts-num">{{ stats.deviceUnknown }}</span><span class="ts-label">{{ t('topology.unknown') }}</span></div>
      <div class="topo-stat-sep"></div>
      <div class="topo-stat"><span class="ts-num">{{ stats.linkTotal }}</span><span class="ts-label">{{ t('topology.linkTotal') }}</span></div>
      <div class="topo-stat ok"><span class="ts-num">{{ stats.linkUp }}</span><span class="ts-label">{{ t('topology.normal') }}</span></div>
      <div class="topo-stat bad"><span class="ts-num">{{ stats.linkDown }}</span><span class="ts-label">{{ t('topology.abnormal') }}</span></div>
    </div>

    <!-- 拓扑画布 -->
    <div class="topo-wrap">
      <div class="topo-canvas" ref="chartRef"></div>
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
      <el-empty v-if="empty" :description="t('topology.empty')" :image-size="120" />
    </div>

    <!-- 设备/链路详情抽屉 -->
    <el-drawer v-model="drawer" :title="detailTitle" size="380px" :with-header="true">
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
          </div>
        </div>
        <el-descriptions :column="1" size="small" class="td-desc">
          <el-descriptions-item v-if="cur.ip" :label="t('topology.ip')">{{ cur.ip }}</el-descriptions-item>
          <el-descriptions-item :label="t('topology.type')">{{ typeName(cur.type) }}</el-descriptions-item>
          <el-descriptions-item v-if="cur.vendor" :label="t('topology.vendor')">{{ cur.vendor }}</el-descriptions-item>
          <template v-if="isDevice">
            <el-descriptions-item :label="t('topology.cpu')">{{ cur.cpu ? cur.cpu.toFixed(1) + '%' : '--' }}</el-descriptions-item>
            <el-descriptions-item :label="t('topology.mem')">{{ cur.mem ? cur.mem.toFixed(1) + '%' : '--' }}</el-descriptions-item>
            <el-descriptions-item :label="t('topology.node')">{{ cur.node_uuid ? cur.node_uuid : t('topology.local') }}</el-descriptions-item>
          </template>
          <template v-if="!isDevice">
            <el-descriptions-item :label="t('topology.method')">{{ cur.method }}</el-descriptions-item>
            <el-descriptions-item v-if="cur.port" :label="t('topology.port')">{{ cur.port }}</el-descriptions-item>
            <el-descriptions-item :label="t('topology.latency')">{{ cur.rt ? cur.rt.toFixed(1) + ' ms' : '--' }}</el-descriptions-item>
            <el-descriptions-item :label="t('topology.loss')">{{ cur.loss ? cur.loss.toFixed(1) + '%' : '--' }}</el-descriptions-item>
          </template>
        </el-descriptions>
        <el-button v-if="isDevice" type="primary" style="width:100%;margin-top:16px" @click="go('/monitor')">
          {{ t('topology.viewDevice') }}
        </el-button>
        <el-button v-else type="primary" style="width:100%;margin-top:16px" @click="go('/linkdetect')">
          {{ t('topology.viewLink') }}
        </el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Refresh, Position } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { getEnc } from '@/utils/request'
import { useAppStore } from '@/stores/app'
import { cssVar, chartAxisColors } from '@/utils'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const dark = computed(() => appStore.theme === 'dark')

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let timer: ReturnType<typeof setInterval> | null = null

const nodes = ref<any[]>([])
const links = ref<any[]>([])
const stats = ref({ deviceTotal: 0, deviceUp: 0, deviceDown: 0, deviceUnknown: 0, linkTotal: 0, linkUp: 0, linkDown: 0 })
const empty = computed(() => nodes.value.length === 0)

// 详情抽屉
const drawer = ref(false)
const cur = ref<any>(null)
const isDevice = computed(() => !cur.value || cur.value.kind === 'device')
const detailTitle = computed(() => t(isDevice.value ? 'topo.deviceDetail' : 'topo.linkDetail'))

// 设备图标（ECharts path:// 24x24 视口）
const ICONS: Record<string, string> = {
  server: 'M3,5h18v4H3zM3,11h18v4H3zM3,17h18v4H3zM6,6.5h1v-1H6zM6,12.5h1v-1H6zM6,18.5h1v-1H6z',
  switch: 'M2,7h20v10H2zM4,9h3v6H4zM8,9h12v6H8zM8,11h1v2H8zM10,11h1v2h-1zM12,11h1v2h-1zM14,11h1v2h-1zM16,11h1v2h-1z',
  router: 'M5,9a7,7 0 0 1 14,0v6H5V9zM8,11h3M13,11h3M8,14h3M13,14h3M5,17h14v2H5z',
  firewall: 'M12,2l8,3v6c0,5 -3,8 -8,11c-5,-3 -8,-6 -8,-11V5zM9,12l2,2l4,-4',
  other: 'M12,12m-9,0a9,9 0 1 1 18,0a9,9 0 1 1 -18,0',
  target: 'M12,12m-9,0a9,9 0 1 1 18,0a9,9 0 1 1 -18,0M12,12m-4,0a4,4 0 1 1 8,0a4,4 0 1 1 -8,0'
}

const STATUS = { up: '#3f9e63', down: '#d9534f', unknown: '#9aa0a6' }
function statusColor(s: string) {
  return STATUS[s as keyof typeof STATUS] ?? STATUS.unknown
}

function typeName(type: string) {
  if (type === 'server') return t('topology.server')
  if (type === 'switch') return t('topology.switch')
  if (type === 'router') return t('topology.router')
  if (type === 'firewall') return t('topology.firewall')
  if (type === 'target') return t('topology.targetType')
  return t('topology.other')
}
function iconOf(type: string) {
  return ICONS[type] ?? ICONS.other
}

async function load(forceLayout = false) {
  try {
    const r = await getEnc<any>('/topology/overview')
    if (!r || !r.data) return
    const data = r.data
    nodes.value = data.nodes ?? []
    links.value = data.links ?? []

    const devs = nodes.value.filter((n: any) => !n.id.startsWith('ext_'))
    stats.value = {
      deviceTotal: devs.length,
      deviceUp: devs.filter((d: any) => d.status === 'up').length,
      deviceDown: devs.filter((d: any) => d.status === 'down').length,
      deviceUnknown: devs.filter((d: any) => d.status !== 'up' && d.status !== 'down').length,
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
    window.addEventListener('resize', onResize)
  }
  const ax = chartAxisColors()
  const g = cssVar('--np-bg-raise', '#ffffff')

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
          return `${d.name}<br/><span style="color:${cssVar('--np-text-2', '#5a6070')}">${st}${d.rt ? ' · ' + d.rt.toFixed(1) + 'ms' : ''}</span>`
        }
        return `${d.name}<br/><span style="color:${cssVar('--np-text-2', '#5a6070')}">${d.ip ?? ''} · ${st}</span>`
      }
    },
    series: [{
      type: 'graph',
      layout: forceLayout ? 'force' : 'force',
      roam: true,
      draggable: true,
      symbolSize: 46,
      label: { show: true, position: 'bottom', color: cssVar('--np-text', '#1a1d23'), fontSize: 11 },
      edgeLabel: { show: true, color: cssVar('--np-text-2', '#5a6070'), fontSize: 10, formatter: (p: any) => (p.data.rt ? p.data.rt.toFixed(0) + 'ms' : '') },
      force: { repulsion: 260, edgeLength: 150, gravity: 0.1, layoutAnimation: forceLayout },
      emphasis: { focus: 'adjacency', label: { fontWeight: 600, fontSize: 13 } },
      data: nodes.value.map((n: any) => ({
        id: n.id,
        name: n.name,
        ip: n.ip,
        vendor: n.vendor,
        type: n.type,
        status: n.status,
        cpu: n.cpu,
        mem: n.mem,
        node_uuid: n.node_uuid,
        kind: 'device',
        symbol: 'path://' + (ICONS[n.type] ?? ICONS.other),
        itemStyle: { color: statusColor(n.status) },
        label: { show: true, position: 'bottom', formatter: n.name, color: cssVar('--np-text', '#1a1d23'), fontSize: 11 }
      })),
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
        kind: 'link',
        lineStyle: { color: statusColor(l.status), width: l.status === 'up' ? 2 : 2.5, opacity: l.status === 'up' ? 0.85 : 1, curveness: 0.15 },
        edgeLabel: { show: true, formatter: (p: any) => (l.rt ? l.rt.toFixed(0) + 'ms' : ''), color: cssVar('--np-text-2', '#5a6070'), fontSize: 10 }
      }))
    }]
  }, forceLayout) // 常规刷新 merge 保留拖拽位置；自动布局 notMerge 重建
}

function onChartClick(p: any) {
  if (!p || p.dataType === undefined) return
  const d = p.data
  if (!d) return
  if (d.kind === 'link') {
    cur.value = { ...d, kind: 'link' }
  } else {
    cur.value = { ...d, kind: 'device' }
  }
  drawer.value = true
}

function restartLayout() {
  render(true)
}

function go(path: string) {
  drawer.value = false
  router.push(path)
}

function onResize() {
  chart?.resize()
}

watch(dark, () => nextTick(() => render()))

onMounted(() => {
  load(true)
  timer = setInterval(() => load(), 30000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  window.removeEventListener('resize', onResize)
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.np-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.np-page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.np-page-head-info { display: flex; flex-direction: column; gap: 4px; }
.np-page-head-title { font-size: 20px; font-weight: 600; letter-spacing: 0.01em; }
.np-page-head-desc { font-size: 13px; color: var(--np-text-2, #5a6070); }
.np-page-head-actions { display: flex; gap: 8px; }

/* 统计卡 */
.topo-stats {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.topo-stat {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 10px 16px;
  border-radius: 10px;
  background: var(--np-bg-raise, #fff);
  border: 1px solid var(--np-border, #e3e6ea);
}
.ts-num { font-size: 20px; font-weight: 700; font-variant-numeric: tabular-nums; }
.ts-label { font-size: 12px; color: var(--np-text-2, #5a6070); }
.topo-stat.ok .ts-num { color: #3f9e63; }
.topo-stat.bad .ts-num { color: #d9534f; }
.topo-stat.mute .ts-num { color: #9aa0a6; }
.topo-stat-sep { width: 1px; height: 32px; background: var(--np-border, #e3e6ea); }

/* 画布 */
.topo-wrap { position: relative; }
.topo-canvas { width: 100%; height: calc(100vh - 300px); min-height: 420px; border-radius: 12px; border: 1px solid var(--np-border, #e3e6ea); background: var(--np-bg, #f5f6f8); }
.topo-legend {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--np-bg-raise, rgba(255,255,255,0.96));
  border: 1px solid var(--np-border, #e3e6ea);
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  z-index: 5;
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
.td-desc { width: 100%; }
</style>
