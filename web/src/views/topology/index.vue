<script setup lang="ts">
// 网络拓扑图：设备名称、连线多IP显示、2D/3D 视图切换
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import { deviceIcons, getDeviceSvgDataUrl } from '@/assets/icons/device-icons'

const { t } = useI18n()

interface TopoNode {
  id: number
  name: string
  ip: string
  type: string
  x: number
  y: number
  z: number
  group_id: number
  icon: string
}
interface TopoEdge {
  id: number
  name: string
  source: number
  target: number
  color: string
  bandwidth: string
  status: string
  ips: string[]
}

const nodes = ref<TopoNode[]>([])
const edges = ref<TopoEdge[]>([])
const mode = ref<'2d' | '3d'>('2d')

// 2D SVG 视图
const svgRef = ref<SVGSVGElement>()
const selectedNode = ref<number | null>(null)
// 拖拽状态
let dragNode: TopoNode | null = null
let dragOffset = { x: 0, y: 0 }

function onNodeMouseDown(e: MouseEvent, n: TopoNode) {
  dragNode = n
  const rect = (svgRef.value as SVGSVGElement).getBoundingClientRect()
  const vb = svgViewBox()
  const sx = vb.width / rect.width
  const sy = vb.height / rect.height
  dragOffset.x = (e.clientX - rect.left) * sx - n.x
  dragOffset.y = (e.clientY - rect.top) * sy - n.y
  selectedNode.value = n.id
  e.preventDefault()
}
function onSvgMouseMove(e: MouseEvent) {
  if (!dragNode || !svgRef.value) return
  const rect = svgRef.value.getBoundingClientRect()
  const vb = svgViewBox()
  const sx = vb.width / rect.width
  const sy = vb.height / rect.height
  dragNode.x = Math.round((e.clientX - rect.left) * sx - dragOffset.x)
  dragNode.y = Math.round((e.clientY - rect.top) * sy - dragOffset.y)
}
function onSvgMouseUp() {
  if (dragNode) {
    const n = dragNode
    dragNode = null
    // 保存位置到后端
    putEnc(`/topology/devices/${n.id}`, { x: n.x, y: n.y }).catch(() => {})
  }
}

// 3D 视图
const threeRef = ref<HTMLDivElement>()
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let renderer: THREE.WebGLRenderer | null = null
let controls: OrbitControls | null = null
let animId = 0
const threeMeshes: THREE.Object3D[] = []

const nodeTypeMap: Record<string, string> = {
  router: '路由器',
  switch: '交换机',
  firewall: '防火墙',
  server: '服务器',
  other: '其他'
}

async function loadGraph() {
  const g = await getEnc<{ nodes: TopoNode[]; edges: TopoEdge[] }>('/topology/graph')
  nodes.value = g.nodes
  edges.value = g.edges
  if (mode.value === '2d') await nextTick()
}

async function autoDiscover() {
  const r = await postEnc('/topology/discover', {})
  ElMessage.success(`自动发现：新增设备 ${(r as any).devices || 0} 台，连线 ${(r as any).links || 0} 条`)
  await loadGraph()
}

function nodeColor(type: string): string {
  const m: Record<string, string> = {
    router: '#2f6bff',
    switch: '#22c55e',
    firewall: '#f59e0b',
    server: '#8b5cf6',
    other: '#64748b'
  }
  return m[type] || m.other
}

// ---- 设备 CRUD ----
const devDialog = ref(false)
const devForm = reactive<TopoNode>({ id: 0, name: '', ip: '', type: 'router', x: 0, y: 0, z: 0, group_id: 0, icon: '' })

function openDevAdd() {
  devForm.id = 0
  devForm.name = ''
  devForm.ip = ''
  devForm.type = 'router'
  devForm.x = Math.round(Math.random() * 800) - 400
  devForm.y = Math.round(Math.random() * 500) - 250
  devForm.z = 0
  devDialog.value = true
}
function openDevEdit(n: TopoNode) {
  Object.assign(devForm, n)
  devDialog.value = true
}
// 上传图标
async function doUploadIcon(opt: any) {
  const fd = new FormData()
  fd.append('file', opt.file)
  const res = await fetch('/api/v1/topology/upload', {
    method: 'POST',
    body: fd,
    headers: { Authorization: 'Bearer ' + (localStorage.getItem('token') || '') }
  }).then(r => r.json())
  if (res.code === 0) {
    devForm.icon = res.data.url
    ElMessage.success('上传成功')
  } else {
    ElMessage.error(res.msg || '上传失败')
  }
}
function iconUrl(icon: string) {
  if (!icon) return ''
  if (icon.startsWith('http')) return icon
  return icon
}
function getNodeImg(n: TopoNode): string {
  if (n.icon) return iconUrl(n.icon)
  return getDeviceSvgDataUrl(n.type)
}
async function saveDev() {
  if (!devForm.name || !devForm.ip) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (devForm.id) {
    await putEnc(`/topology/devices/${devForm.id}`, devForm)
  } else {
    await postEnc('/topology/devices', devForm)
  }
  ElMessage.success(t('common.success'))
  devDialog.value = false
  await loadGraph()
}
async function removeDev(n: TopoNode) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/topology/devices/${n.id}`)
  ElMessage.success(t('common.success'))
  await loadGraph()
}

// ---- 连线 CRUD ----
const linkDialog = ref(false)
const linkForm = reactive({ id: 0, name: '', source: 0, target: 0, color: '#2f6bff', bandwidth: '', status: 'up', ips: '' })
function openLinkAdd() {
  linkForm.id = 0
  linkForm.name = ''
  linkForm.source = 0
  linkForm.target = 0
  linkForm.color = '#2f6bff'
  linkForm.bandwidth = ''
  linkForm.status = 'up'
  linkForm.ips = ''
  linkDialog.value = true
}
async function saveLink() {
  if (!linkForm.source || !linkForm.target || linkForm.source === linkForm.target) {
    ElMessage.warning('请选择两个不同设备')
    return
  }
  const body = {
    name: linkForm.name,
    source: linkForm.source,
    target: linkForm.target,
    color: linkForm.color,
    bandwidth: linkForm.bandwidth,
    status: linkForm.status,
    ips: linkForm.ips.split(',').map((s) => s.trim()).filter(Boolean)
  }
  if (linkForm.id) {
    await putEnc(`/topology/links/${linkForm.id}`, body)
  } else {
    await postEnc('/topology/links', body)
  }
  ElMessage.success(t('common.success'))
  linkDialog.value = false
  await loadGraph()
}
async function removeLink(e: TopoEdge) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/topology/links/${e.id}`)
  ElMessage.success(t('common.success'))
  await loadGraph()
}

function nodeById(id: number): TopoNode | undefined {
  return nodes.value.find((n) => n.id === id)
}

// ---- 3D 渲染 ----
function init3D() {
  if (!threeRef.value) return
  const w = threeRef.value.clientWidth || 800
  const h = threeRef.value.clientHeight || 500
  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x0f172a)
  camera = new THREE.PerspectiveCamera(50, w / h, 0.1, 5000)
  camera.position.set(300, 250, 400)
  camera.lookAt(0, 0, 0)
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(window.devicePixelRatio)
  renderer.setSize(w, h)
  threeRef.value.appendChild(renderer.domElement)
  controls = new OrbitControls(camera, renderer.domElement)
  controls.enableDamping = true
  controls.target.set(0, 0, 0)

  // 网格地面
  const grid = new THREE.GridHelper(800, 20, 0x475569, 0x334155)
  scene.add(grid)

  // 灯光
  scene.add(new THREE.AmbientLight(0xffffff, 0.9))
  const dir = new THREE.DirectionalLight(0xffffff, 1.0)
  dir.position.set(200, 400, 300)
  scene.add(dir)

  rebuild3D()
  animate()
}

function rebuild3D() {
  const s = scene
  if (!s) return
  threeMeshes.forEach((m) => s.remove(m))
  threeMeshes.length = 0
  const edgeMatCache = new Map<string, THREE.LineBasicMaterial>()

  // 连线（多IP标签用 Sprite 显示）
  for (const e of edges.value) {
    const a = nodeById(e.source)
    const b = nodeById(e.target)
    if (!a || !b) continue
    const color = e.color || '#22c55e'
    if (!edgeMatCache.has(color)) edgeMatCache.set(color, new THREE.LineBasicMaterial({ color }))
    const points = [new THREE.Vector3(a.x, a.z, a.y), new THREE.Vector3(b.x, b.z, b.y)]
    const geo = new THREE.BufferGeometry().setFromPoints(points)
    const line = new THREE.Line(geo, edgeMatCache.get(color))
    s.add(line)
    threeMeshes.push(line)
    // 中点显示IP列表
    const mid = new THREE.Vector3().lerpVectors(points[0], points[1], 0.5)
    if (e.ips.length) {
      const sprite = makeLabelSprite(e.ips.join('\n'), 10, color)
      sprite.position.copy(mid)
      s.add(sprite)
      threeMeshes.push(sprite)
    }
  }

  // 设备节点（立方体 + 图标纹理 + 名称标签）
  const cubeGeo = new THREE.BoxGeometry(44, 44, 44)
  for (const n of nodes.value) {
    const tex = n.icon ? makeCustomIconTexture(n.icon) : makeIconTexture(n.type)
    const mat = new THREE.MeshPhongMaterial({
      color: nodeColor(n.type),
      emissive: nodeColor(n.type),
      emissiveIntensity: 0.18,
      map: tex
    })
    const mesh = new THREE.Mesh(cubeGeo, mat)
    mesh.position.set(n.x, n.z, n.y)
    s.add(mesh)
    threeMeshes.push(mesh)
    const label = makeLabelSprite(n.name, 14, '#e2e8f0')
    label.position.set(n.x, n.z + 38, n.y)
    s.add(label)
    threeMeshes.push(label)
  }
}

function makeLabelSprite(text: string, size: number, color: string): THREE.Sprite {
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')!
  canvas.width = 512
  canvas.height = 128
  ctx.font = `bold ${size}px "Microsoft YaHei", sans-serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillStyle = 'rgba(15,23,42,0.55)'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  ctx.fillStyle = color
  text.split('\n').forEach((line, i) => {
    ctx.fillText(line, canvas.width / 2, canvas.height / 2 + (i - (text.split('\n').length - 1) / 2) * size * 1.4)
  })
  const tex = new THREE.CanvasTexture(canvas)
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true }))
  sprite.scale.set(220, 55, 1)
  return sprite
}

// 用自定义图片生成 3D 纹理
function makeCustomIconTexture(url: string): THREE.Texture {
  const canvas = document.createElement('canvas')
  canvas.width = 128
  canvas.height = 128
  const ctx = canvas.getContext('2d')!
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => ctx.drawImage(img, 16, 16, 96, 96)
  img.src = url
  return new THREE.CanvasTexture(canvas)
}

// 用 SVG dataURL 生成 3D 立方体贴图
function makeIconTexture(type: string): THREE.Texture {
  const canvas = document.createElement('canvas')
  canvas.width = 128
  canvas.height = 128
  const ctx = canvas.getContext('2d')!
  const img = new Image()
  const url = getDeviceSvgDataUrl(type)
  // 同步加载（dataURL 立即可用）
  img.src = url
  if (img.complete) {
    ctx.drawImage(img, 24, 24, 80, 80)
  } else {
    img.onload = () => ctx.drawImage(img, 24, 24, 80, 80)
  }
  return new THREE.CanvasTexture(canvas)
}

function animate() {
  animId = requestAnimationFrame(animate)
  controls?.update()
  renderer?.render(scene!, camera!)
}

function resize3D() {
  if (!threeRef.value || !camera || !renderer) return
  const w = threeRef.value.clientWidth
  const h = threeRef.value.clientHeight
  camera.aspect = w / h
  camera.updateProjectionMatrix()
  renderer.setSize(w, h)
}

async function switchMode(m: '2d' | '3d') {
  mode.value = m
  await nextTick()
  if (m === '3d') {
    await new Promise(r => setTimeout(r, 100))
    if (!scene) init3D()
    else {
      rebuild3D()
      resize3D()
    }
  }
}

onMounted(async () => {
  await loadGraph()
  await nextTick()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animId)
  controls?.dispose()
  renderer?.dispose()
  renderer?.domElement.remove()
})

// SVG 视图计算
function svgViewBox() {
  return { width: 900, height: 520 }
}
</script>

<template>
  <div class="np-page">
    <div class="np-toolbar">
      <el-radio-group :model-value="mode" @update:model-value="(v: string) => switchMode(v as '2d' | '3d')">
        <el-radio-button value="2d">{{ t('topo.mode2d') }}</el-radio-button>
        <el-radio-button value="3d">{{ t('topo.mode3d') }}</el-radio-button>
      </el-radio-group>
      <div class="spacer"></div>
      <el-button type="warning" @click="autoDiscover">
        <el-icon><Search /></el-icon>自动发现
      </el-button>
      <el-button type="primary" @click="openDevAdd">
        <el-icon><Plus /></el-icon>{{ t('topo.addDevice') }}
      </el-button>
      <el-button type="success" @click="openLinkAdd">
        <el-icon><Link /></el-icon>{{ t('topo.addLink') }}
      </el-button>
    </div>

    <!-- 2D SVG 拓扑 -->
    <div v-show="mode === '2d'" class="np-card np-topo-2d">
      <svg ref="svgRef" :viewBox="`0 0 ${svgViewBox().width} ${svgViewBox().height}`" class="np-topo-svg" @mousemove="onSvgMouseMove" @mouseup="onSvgMouseUp" @mouseleave="onSvgMouseUp">
        <!-- 连线 -->
        <g v-for="e in edges" :key="'e' + e.id">
          <line
            :x1="nodeById(e.source)?.x ?? 0" :y1="nodeById(e.source)?.y ?? 0"
            :x2="nodeById(e.target)?.x ?? 0" :y2="nodeById(e.target)?.y ?? 0"
            :stroke="e.color || '#22c55e'" stroke-width="3" class="np-topo-line"
          />
          <!-- 连线中点：多IP地址显示 -->
          <g :transform="`translate(${(nodeById(e.source)?.x ?? 0 + (nodeById(e.target)?.x ?? 0)) / 2}, ${((nodeById(e.source)?.y ?? 0) + (nodeById(e.target)?.y ?? 0)) / 2})`">
            <rect x="-70" y="-14" width="140" height="28" rx="6" fill="rgba(15,23,42,0.6)" />
            <text text-anchor="middle" dominant-baseline="middle" fill="#e2e8f0" font-size="11">
              {{ e.ips.join(', ') || e.name || e.status }}
            </text>
          </g>
        </g>
        <!-- 节点 -->
        <g
          v-for="n in nodes" :key="'n' + n.id"
          class="np-topo-node"
          :transform="`translate(${n.x}, ${n.y})`"
          @click="selectedNode = n.id" @mousedown="onNodeMouseDown($event, n)"
        >
          <rect x="-30" y="-30" width="60" height="60" rx="12" :fill="nodeColor(n.type)"
            :stroke="selectedNode === n.id ? '#ffffff' : 'transparent'" stroke-width="3" opacity="0.92" />
          <image :href="getNodeImg(n)" x="-20" y="-20" width="40" height="40" />
          <text text-anchor="middle" y="46" fill="var(--np-text-1)" font-size="13" font-weight="600">{{ n.name }}</text>
          <text text-anchor="middle" y="62" fill="var(--np-text-2)" font-size="11">{{ n.ip }}</text>
        </g>
        <text v-if="!nodes.length" x="450" y="260" text-anchor="middle" fill="var(--np-text-2)">{{ t('common.noData') }}</text>
      </svg>
    </div>

    <!-- 3D 视图 -->
    <div v-show="mode === '3d'" ref="threeRef" class="np-card np-topo-3d"></div>

    <!-- 设备列表 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Share /></el-icon>
        <span>{{ t('topo.devices') }}</span>
      </div>
      <el-table :data="nodes" size="default" stripe class="np-table">
        <el-table-column prop="name" :label="t('topo.deviceName')" min-width="140" />
        <el-table-column prop="ip" :label="t('topo.deviceIp')" min-width="150" />
        <el-table-column :label="t('topo.deviceType')" width="110">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ nodeTypeMap[row.type] || row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="x" :label="t('topo.x')" width="80" />
        <el-table-column prop="y" :label="t('topo.y')" width="80" />
        <el-table-column :label="t('common.actions')" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openDevEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="removeDev(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 连线列表 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Link /></el-icon>
        <span>{{ t('topo.links') }}</span>
      </div>
      <el-table :data="edges" size="default" stripe class="np-table">
        <el-table-column :label="t('topo.from')" min-width="120">
          <template #default="{ row }">{{ nodeById(row.source)?.name }}</template>
        </el-table-column>
        <el-table-column :label="t('topo.to')" min-width="120">
          <template #default="{ row }">{{ nodeById(row.target)?.name }}</template>
        </el-table-column>
        <el-table-column prop="name" :label="t('link.taskName')" min-width="120" />
        <el-table-column :label="t('topo.ipList')" min-width="200">
          <template #default="{ row }">
            <el-tag v-for="ip in row.ips" :key="ip" size="small" style="margin: 2px" effect="plain">{{ ip }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="danger" @click="removeLink(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 设备弹窗 -->
    <el-dialog v-model="devDialog" :title="devForm.id ? t('common.edit') : t('topo.addDevice')" width="480px">
      <el-form :model="devForm" label-width="100px">
        <el-form-item :label="t('topo.deviceName')" required>
          <el-input v-model="devForm.name" />
        </el-form-item>
        <el-form-item :label="t('topo.deviceIp')" required>
          <el-input v-model="devForm.ip" />
        </el-form-item>
        <el-form-item :label="t('topo.deviceType')">
          <el-select v-model="devForm.type">
            <el-option v-for="(label, key) in nodeTypeMap" :key="key" :label="label" :value="key" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topo.x')">
          <el-input-number v-model="devForm.x" :min="-800" :max="800" />
        </el-form-item>
        <el-form-item :label="t('topo.y')">
          <el-input-number v-model="devForm.y" :min="-400" :max="400" />
        </el-form-item>
        <el-form-item label="设备图标">
          <el-upload
            :show-file-list="false"
            :http-request="doUploadIcon"
            accept=".jpg,.jpeg,.png,.svg,.vsdx,.gif"
          >
            <el-button type="primary" plain>上传图标</el-button>
            <span style="margin-left:10px;color:var(--np-text-2);font-size:12px">支持 jpg/png/svg/vsdx（留空使用默认图标）</span>
          </el-upload>
          <div v-if="devForm.icon" style="margin-top:8px;display:flex;align-items:center;gap:8px">
            <img :src="iconUrl(devForm.icon)" style="width:36px;height:36px;border-radius:6px;background:#fff" />
            <span style="color:var(--np-text-2);font-size:12px;word-break:break-all">{{ devForm.icon }}</span>
            <el-button link type="danger" @click="devForm.icon = ''">清除</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="devDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveDev">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 连线弹窗 -->
    <el-dialog v-model="linkDialog" :title="t('topo.addLink')" width="480px">
      <el-form :model="linkForm" label-width="100px">
        <el-form-item :label="t('topo.from')" required>
          <el-select v-model="linkForm.source" filterable placeholder="选择源设备">
            <el-option v-for="n in nodes" :key="n.id" :label="`${n.name} (${n.ip})`" :value="n.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('topo.to')" required>
          <el-select v-model="linkForm.target" filterable placeholder="选择目标设备">
            <el-option v-for="n in nodes" :key="n.id" :label="`${n.name} (${n.ip})`" :value="n.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('link.taskName')">
          <el-input v-model="linkForm.name" />
        </el-form-item>
        <el-form-item :label="t('topo.ipList')">
          <el-input v-model="linkForm.ips" placeholder="多个IP用逗号分隔" />
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-radio-group v-model="linkForm.status">
            <el-radio value="up">up</el-radio>
            <el-radio value="down">down</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveLink">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.np-topo-2d {
  overflow: hidden;
}
.np-topo-svg {
  width: 100%;
  height: 520px;
  display: block;
  background:
    radial-gradient(circle at 50% 50%, rgba(47, 107, 255, 0.06), transparent 60%);
}
.np-topo-node {
  cursor: pointer;
  &:hover text {
    fill: var(--np-primary);
  }
}
.np-topo-line {
  opacity: 0.8;
}
.np-topo-3d {
  height: 520px;
  overflow: hidden;
  canvas {
    display: block;
  }
}
</style>
