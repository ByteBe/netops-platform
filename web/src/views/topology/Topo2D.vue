<script setup lang="ts">
// 拓扑 2D SVG 视图组件
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEnc, putEnc } from '@/utils/request'
const { t } = useI18n()

interface TopoNode { id: number; name: string; ip: string; type: string; x: number; y: number; z: number; icon: string }
interface TopoEdge { id: number; name: string; source: number; target: number; color: string; ips: string[]; status: string }

const props = defineProps<{ nodes: TopoNode[]; edges: TopoEdge[] }>()
const svgRef = ref<SVGSVGElement>()
const selectedNode = ref<number | null>(null)
let dragNode: TopoNode | null = null
let dragOffset = { x: 0, y: 0 }

function nodeColor(type: string) {
  const m: Record<string, string> = { router: '#3b82f6', switch: '#22c55e', firewall: '#ef4444', server: '#f59e0b' }
  return m[type] || '#94a3b8'
}
function getNodeImg(n: TopoNode) { return n.icon || `/assets/device-${n.type}.svg` }
function nodeById(id: number) { return props.nodes.find(n => n.id === id) }
function vb() { return { width: 900, height: 520 } }

function onNodeMouseDown(e: MouseEvent, n: TopoNode) {
  dragNode = n
  const rect = (svgRef.value as SVGSVGElement).getBoundingClientRect()
  const s = vb().width / rect.width
  dragOffset.x = (e.clientX - rect.left) * s - n.x
  dragOffset.y = (e.clientY - rect.top) * s - n.y
  selectedNode.value = n.id
  e.preventDefault()
}
function onSvgMouseMove(e: MouseEvent) {
  if (!dragNode || !svgRef.value) return
  const rect = svgRef.value.getBoundingClientRect()
  const s = vb().width / rect.width
  dragNode.x = Math.round((e.clientX - rect.left) * s - dragOffset.x)
  dragNode.y = Math.round((e.clientY - rect.top) * s - dragOffset.y)
}
function onSvgMouseUp() {
  if (dragNode) {
    const n = dragNode
    dragNode = null
    putEnc(`/topology/devices/${n.id}`, { x: n.x, y: n.y }).catch(() => {})
  }
}
</script>

<template>
  <div class="np-card">
    <svg ref="svgRef" :viewBox="`0 0 ${vb().width} ${vb().height}`" style="width:100%;height:75vh;background:#0f172a"
      @mousemove="onSvgMouseMove" @mouseup="onSvgMouseUp" @mouseleave="onSvgMouseUp">
      <g v-for="e in edges" :key="'e' + e.id">
        <line :x1="nodeById(e.source)?.x ?? 0" :y1="nodeById(e.source)?.y ?? 0"
          :x2="nodeById(e.target)?.x ?? 0" :y2="nodeById(e.target)?.y ?? 0"
          :stroke="e.color || '#22c55e'" stroke-width="3" />
        <g :transform="`translate(${(nodeById(e.source)?.x ?? 0 + (nodeById(e.target)?.x ?? 0)) / 2}, ${((nodeById(e.source)?.y ?? 0) + (nodeById(e.target)?.y ?? 0)) / 2})`">
          <rect x="-70" y="-14" width="140" height="28" rx="6" fill="rgba(15,23,42,0.6)" />
          <text text-anchor="middle" dominant-baseline="middle" fill="#e2e8f0" font-size="11">
            {{ e.ips.join(', ') || e.name || e.status }}
          </text>
        </g>
      </g>
      <g v-for="n in nodes" :key="'n' + n.id" :transform="`translate(${n.x}, ${n.y})`"
        @click="selectedNode = n.id" @mousedown="onNodeMouseDown($event, n)">
        <rect x="-30" y="-30" width="60" height="60" rx="12" :fill="nodeColor(n.type)"
          :stroke="selectedNode === n.id ? '#ffffff' : 'transparent'" stroke-width="3" opacity="0.92" />
        <image :href="getNodeImg(n)" x="-20" y="-20" width="40" height="40" />
        <text text-anchor="middle" y="46" fill="#e2e8f0" font-size="13" font-weight="600">{{ n.name }}</text>
        <text text-anchor="middle" y="62" fill="#94a3b8" font-size="11">{{ n.ip }}</text>
      </g>
      <text v-if="!nodes.length" x="450" y="260" text-anchor="middle" fill="#94a3b8">{{ t('topo.noData') }}</text>
    </svg>
  </div>
</template>
