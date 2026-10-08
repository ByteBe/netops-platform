<script setup lang="ts">
// 拓扑 3D 视图组件（重构版）
// 结构：scene/camera/renderer 独立管理；节点/连线/标签分层；支持自适应尺寸
import { ref, onMounted, onBeforeUnmount, watch, shallowRef } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { getDeviceSvgDataUrl } from '@/assets/icons/device-icons'

interface TopoNode { id: number; name: string; ip: string; type: string; x: number; y: number; z: number; icon: string }
interface TopoEdge { id: number; name: string; source: number; target: number; color: string; ips: string[] }

const props = defineProps<{ nodes: TopoNode[]; edges: TopoEdge[] }>()
const threeRef = ref<HTMLDivElement>()
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let renderer: THREE.WebGLRenderer | null = null
let controls: OrbitControls | null = null
let animId = 0
let ro: ResizeObserver | null = null
const group = shallowRef<THREE.Group | null>(null)

function nodeColor(type: string): number {
  const m: Record<string, number> = { router: 0x3b82f6, switch: 0x22c55e, firewall: 0xef4444, server: 0xf59e0b }
  return m[type] || 0x94a3b8
}
function nodeById(id: number) { return props.nodes.find(n => n.id === id) }

function init() {
  const el = threeRef.value!
  const w = el.clientWidth || 800
  const h = el.clientHeight || 500
  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x0b1220)
  scene.fog = new THREE.Fog(0x0b1220, 600, 1500)
  camera = new THREE.PerspectiveCamera(50, w / h, 0.1, 5000)
  camera.position.set(350, 280, 450)
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false })
  renderer.setPixelRatio(window.devicePixelRatio)
  renderer.setSize(w, h)
  renderer.shadowMap.enabled = true
  el.appendChild(renderer.domElement)
  controls = new OrbitControls(camera, renderer.domElement)
  controls.enableDamping = true
  controls.dampingFactor = 0.08
  // 灯光
  scene.add(new THREE.AmbientLight(0xffffff, 0.9))
  const dir = new THREE.DirectionalLight(0xffffff, 1.2)
  dir.position.set(200, 400, 300)
  scene.add(dir)
  group.value = new THREE.Group()
  scene.add(group.value)
  rebuild()
  animate()
  // 自适应
  ro = new ResizeObserver(() => {
    if (!el || !camera || !renderer) return
    camera.aspect = el.clientWidth / el.clientHeight
    camera.updateProjectionMatrix()
    renderer.setSize(el.clientWidth, el.clientHeight)
  })
  ro.observe(el)
}

function rebuild() {
  if (!group.value) return
  group.value.clear()
  const matCache = new Map<string, THREE.LineBasicMaterial>()
  // 连线
  for (const e of props.edges) {
    const a = nodeById(e.source); const b = nodeById(e.target)
    if (!a || !b) continue
    const color = e.color || '#22c55e'
    if (!matCache.has(color)) matCache.set(color, new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0.7 }))
    const pts = [new THREE.Vector3(a.x, a.z, a.y), new THREE.Vector3(b.x, b.z, b.y)]
    const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts), matCache.get(color)!)
    group.value.add(line)
    if (e.ips.length) {
      const mid = new THREE.Vector3().lerpVectors(pts[0], pts[1], 0.5)
      const sp = makeLabel(e.ips.join('\n'), 10, color)
      sp.position.copy(mid); group.value.add(sp)
    }
  }
  // 节点
  const cubeGeo = new THREE.BoxGeometry(44, 44, 44)
  for (const n of props.nodes) {
    const tex = n.icon ? makeCustom(n.icon) : makeIcon(n.type)
    const mesh = new THREE.Mesh(cubeGeo, new THREE.MeshPhongMaterial({
      color: nodeColor(n.type), emissive: nodeColor(n.type), emissiveIntensity: 0.2, map: tex
    }))
    mesh.position.set(n.x, n.z, n.y)
    mesh.castShadow = true
    group.value.add(mesh)
    const label = makeLabel(n.name, 14, '#e2e8f0')
    label.position.set(n.x, n.z + 38, n.y)
    group.value.add(label)
  }
}

function makeLabel(text: string, size: number, color: string): THREE.Sprite {
  const c = document.createElement('canvas')
  c.width = 512; c.height = 128
  const ctx = c.getContext('2d')!
  ctx.font = `bold ${size}px "Microsoft YaHei"`
  ctx.textAlign = 'center'; ctx.textBaseline = 'middle'
  ctx.fillStyle = 'rgba(11,18,32,0.7)'; ctx.fillRect(0, 0, 512, 128)
  ctx.fillStyle = color
  text.split('\n').forEach((line, i) => ctx.fillText(line, 256, 40 + i * 20))
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: new THREE.CanvasTexture(c), transparent: true }))
  sp.scale.set(220, 55, 1)
  return sp
}

function makeCustom(url: string): THREE.Texture {
  const c = document.createElement('canvas'); c.width = 128; c.height = 128
  const ctx = c.getContext('2d')!
  const img = new Image(); img.crossOrigin = 'anonymous'
  img.onload = () => ctx.drawImage(img, 16, 16, 96, 96)
  img.src = url
  return new THREE.CanvasTexture(c)
}
function makeIcon(type: string): THREE.Texture {
  const c = document.createElement('canvas'); c.width = 128; c.height = 128
  const ctx = c.getContext('2d')!
  const img = new Image(); img.src = getDeviceSvgDataUrl(type)
  if (img.complete) ctx.drawImage(img, 24, 24, 80, 80)
  else img.onload = () => ctx.drawImage(img, 24, 24, 80, 80)
  return new THREE.CanvasTexture(c)
}

function animate() {
  animId = requestAnimationFrame(animate)
  controls?.update()
  renderer?.render(scene!, camera!)
}

onMounted(() => setTimeout(init, 100))
onBeforeUnmount(() => {
  cancelAnimationFrame(animId)
  ro?.disconnect()
  controls?.dispose()
  renderer?.dispose()
  renderer?.domElement.remove()
})
watch(() => [props.nodes, props.edges], rebuild, { deep: true })
</script>

<template><div ref="threeRef" style="width:100%;height:75vh"></div></template>
