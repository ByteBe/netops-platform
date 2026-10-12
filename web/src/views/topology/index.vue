<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'

interface Node { id:number; name:string; ip:string; vendor:string; type:string; layer:string; status:string; x:number; y:number; icon:string; remark:string }
interface Edge { id:number; name:string; source:number; target:number; color:string; bandwidth:string; utilization:number; status:string; ips:string[] }

const nodes=ref<Node[]>([]), edges=ref<Edge[]>([])
const canvasRef=ref<HTMLCanvasElement>(), ctx=ref<CanvasRenderingContext2D|null>(null)
const selectedNode=ref<Node|null>(null), selectedEdge=ref<Edge|null>(null)
const keyword=ref(''), layerFilter=ref('all')
const legend=[{c:'#22c55e',t:'正常'},{c:'#f59e0b',t:'高负载'},{c:'#ef4444',t:'故障'}]
const imgMap:Record<string,HTMLImageElement>={}
let dragId=0, dragStart={x:0,y:0}, moved=false

const nmap=computed(()=>Object.fromEntries(nodes.value.map(n=>[n.id,n])))
const shown=computed(()=>nodes.value.filter(n=>(layerFilter.value==='all'||n.layer===layerFilter.value)&&(!keyword.value||n.name.includes(keyword.value)||n.ip.includes(keyword.value))))

function loadImg(src:string){ if(imgMap[src]) return imgMap[src]; const im=new Image(); im.src=src; im.onload=draw; imgMap[src]=im; return im }
function center(){ const c=canvasRef.value!; return {x:c.width/2, y:c.height/2} }
function toPx(n:Node){ const c=center(); return {x:c.x+n.x, y:c.y+n.y} }

function draw(){ const c=canvasRef.value; if(!c) return; const g=c.getContext('2d')!; g.clearRect(0,0,c.width,c.height); g.strokeStyle='#22c55e'; g.lineWidth=2; for(const e of edges.value){ if(!nmap.value[e.source]||!nmap.value[e.target]) continue; const a=toPx(nmap.value[e.source]), b=toPx(nmap.value[e.target]); g.beginPath(); g.moveTo(a.x,a.y); g.lineTo(b.x,b.y); g.stroke(); g.fillStyle='#22c55e'; const ang=Math.atan2(b.y-a.y,b.x-a.x); g.beginPath(); g.moveTo(b.x,b.y); g.lineTo(b.x-10*Math.cos(ang-0.4),b.y-10*Math.sin(ang-0.4)); g.lineTo(b.x-10*Math.cos(ang+0.4),b.y-10*Math.sin(ang+0.4)); g.fill() } for(const n of shown.value){ const p=toPx(n); const src=n.icon||`/topology-icons/${n.type}.png?v=2`; const im=loadImg(src); if(im.complete){ g.drawImage(im,p.x-24,p.y-24,48,48) } g.fillStyle='#1f2937'; g.font='12px sans-serif'; g.textAlign='center'; g.fillText(n.name,p.x,p.y+36); if(selectedNode.value?.id===n.id){ g.strokeStyle='#2563eb'; g.lineWidth=2; g.strokeRect(p.x-28,p.y-28,56,56) } } }

function hit(x:number,y:number):Node|undefined{ return shown.value.find(n=>{const p=toPx(n); return Math.abs(x-p.x)<=28 && Math.abs(y-p.y)<=28}) }
function onDown(e:MouseEvent){ const r=canvasRef.value!.getBoundingClientRect(); const x=e.clientX-r.left, y=e.clientY-r.top; const n=hit(x,y); if(n){ dragId=n.id; dragStart={x:x-n.x, y:y-n.y}; moved=false; selectedNode.value=n; selectedEdge.value=null } else { selectedNode.value=null; selectedEdge.value=null; draw() } }
function onMove(e:MouseEvent){ if(!dragId) return; const n=nodes.value.find(x=>x.id===dragId); if(!n) return; const r=canvasRef.value!.getBoundingClientRect(); n.x=Math.round(e.clientX-r.left-center().x-dragStart.x+center().x); n.y=Math.round(e.clientY-r.top-center().y-dragStart.y+center().y); moved=true; draw() }
function onUp(){ if(!dragId) return; const n=nodes.value.find(x=>x.id===dragId); if(n&&moved) putEnc('/topology/devices/'+n.id,{...n}); dragId=0 }

async function load(){ const d=await getEnc<{nodes:Node[];edges:Edge[]}>('/topology/graph'); nodes.value=d.nodes; edges.value=d.edges; await nextTick(); resize(); draw() }
function resize(){ const c=canvasRef.value; if(!c) return; const r=c.parentElement!.getBoundingClientRect(); c.width=r.width; c.height=r.height; draw() }
async function layout(){ await postEnc('/topology/auto-layout',{}); ElMessage.success('已自动布局'); await load() }

const devDlg=ref(false), dev=reactive<Node>({id:0,name:'',ip:'',vendor:'h3c',type:'switch',layer:'access',status:'up',x:0,y:0,icon:'',remark:''})
function add(){ Object.assign(dev,{id:0,name:'',ip:'',vendor:'h3c',type:'switch',layer:'access',status:'up',x:Math.round(Math.random()*120-60),y:Math.round(Math.random()*80-40),icon:''}); devDlg.value=true }
async function save(){ if(!dev.name||!dev.ip) return ElMessage.warning('请填写名称和IP'); if(dev.id) await putEnc('/topology/devices/'+dev.id,dev); else await postEnc('/topology/devices',dev); devDlg.value=false; await load() }
function editNode(){ if(selectedNode.value){ Object.assign(dev,selectedNode.value); devDlg.value=true } }
async function removeNode(){ if(!selectedNode.value) return; await delEnc('/topology/devices/'+selectedNode.value.id); selectedNode.value=null; ElMessage.success('已删除'); await load() }
async function removeEdge(){ if(!selectedEdge.value) return; await delEnc('/topology/links/'+selectedEdge.value.id); selectedEdge.value=null; ElMessage.success('已删除'); await load() }

const linkDlg=ref(false), link=reactive<any>({id:0,source:0,target:0,name:''})
function addLink(){ Object.assign(link,{id:0,source:0,target:0}); linkDlg.value=true }
async function saveLink(){ if(!link.source||!link.target) return ElMessage.warning('请选择两端设备'); const body={name:link.name||'',source_id:link.source,target_id:link.target,status:'up'}; if(link.id) await putEnc('/topology/links/'+link.id,body); else await postEnc('/topology/links',body); linkDlg.value=false; ElMessage.success('链路已保存'); await load() }

onMounted(()=>{ load(); window.addEventListener('resize',resize); canvasRef.value?.addEventListener('mousedown',onDown); window.addEventListener('mousemove',onMove); window.addEventListener('mouseup',onUp) })
onUnmounted(()=>{ window.removeEventListener('resize',resize); canvasRef.value?.removeEventListener('mousedown',onDown); window.removeEventListener('mousemove',onMove); window.removeEventListener('mouseup',onUp) })
</script>

<template>
<div class="np-page topo-huawei">
  <div class="toolbar">
    <el-input v-model="keyword" placeholder="搜索设备名/IP" style="width:200px"/>
    <el-radio-group v-model="layerFilter" size="small" @change="draw">
      <el-radio-button value="all">全部</el-radio-button>
      <el-radio-button value="core">核心</el-radio-button>
      <el-radio-button value="access">接入</el-radio-button>
    </el-radio-group>
    <div class="spacer"/>
    <span v-for="l in legend" :key="l.t" class="legend"><i :style="{background:l.c}"/>{{l.t}}</span>
    <el-button size="small" type="primary" @click="add">新增设备</el-button>
    <el-button size="small" @click="addLink">新增链路</el-button>
    <el-button size="small" @click="layout">自动布局</el-button>
    <el-button size="small" @click="load">刷新</el-button>
  </div>
  <div class="body">
    <div class="left">
      <h4>设备层级</h4>
      <div class="stat-row"><span>核心</span><b>{{nodes.filter(n=>n.layer==='core').length}}</b></div>
      <div class="stat-row"><span>接入</span><b>{{nodes.filter(n=>n.layer==='access').length}}</b></div>
      <div class="stat-row danger"><span>故障</span><b>{{nodes.filter(n=>n.status==='down').length}}</b></div>
    </div>
    <div class="canvas"><canvas ref="canvasRef"/></div>
    <div class="right">
      <h4>详情</h4>
      <div v-if="selectedNode">
        <p>名称：{{selectedNode.name}}</p><p>IP：{{selectedNode.ip}}</p><p>厂商：{{selectedNode.vendor}}</p><p>层级：{{selectedNode.layer}}</p><p>状态：{{selectedNode.status}}</p>
        <el-button size="small" type="primary" @click="editNode">修改</el-button>
        <el-button size="small" type="danger" @click="removeNode">删除</el-button>
      </div>
      <div v-else-if="selectedEdge">
        <p>源：{{nmap[selectedEdge.source]?.name}}</p><p>目标：{{nmap[selectedEdge.target]?.name}}</p>
        <el-button size="small" type="danger" @click="removeEdge">删除链路</el-button>
      </div>
      <p v-else>点击节点或链路查看</p>
    </div>
  </div>

  <el-dialog v-model="devDlg" title="设备" width="500px">
    <el-form :model="dev" label-width="80px">
      <el-form-item label="名称"><el-input v-model="dev.name"/></el-form-item>
      <el-form-item label="IP"><el-input v-model="dev.ip"/></el-form-item>
      <el-form-item label="厂商"><el-select v-model="dev.vendor"><el-option label="H3C" value="h3c"/><el-option label="华为" value="huawei"/><el-option label="思科" value="cisco"/></el-select></el-form-item>
      <el-form-item label="层级"><el-select v-model="dev.layer"><el-option label="核心" value="core"/><el-option label="接入" value="access"/></el-select></el-form-item>
      <el-form-item label="类型"><el-select v-model="dev.type"><el-option label="路由器" value="router"/><el-option label="交换机" value="switch"/><el-option label="防火墙" value="firewall"/><el-option label="服务器" value="server"/><el-option label="漏洞扫描" value="vuln"/><el-option label="日志审计" value="audit"/><el-option label="堡垒机" value="bastion"/><el-option label="广域网" value="wan"/><el-option label="AC" value="ac"/></el-select></el-form-item>
    </el-form>
    <template #footer><el-button @click="devDlg=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
  </el-dialog>
  <el-dialog v-model="linkDlg" title="链路" width="500px">
    <el-form :model="link" label-width="80px">
      <el-form-item label="源"><el-select v-model="link.source"><el-option v-for="n in nodes" :key="n.id" :label="n.name" :value="n.id"/></el-select></el-form-item>
      <el-form-item label="目标"><el-select v-model="link.target"><el-option v-for="n in nodes" :key="n.id" :label="n.name" :value="n.id"/></el-select></el-form-item>
    </el-form>
    <template #footer><el-button @click="linkDlg=false">取消</el-button><el-button type="primary" @click="saveLink">保存</el-button></template>
  </el-dialog>
</div>
</template>

<style scoped>
.topo-huawei{display:flex;flex-direction:column;height:calc(100vh - 110px)}
.toolbar{display:flex;align-items:center;gap:10px;background:#fff;padding:10px;border-radius:8px;margin-bottom:10px}
.body{display:grid;grid-template-columns:180px 1fr 230px;gap:10px;flex:1;min-height:0}
.left,.right{background:#fff;border-radius:10px;padding:16px;color:#475569;font-size:13px;line-height:1.6}
.left h4,.right h4{font-size:14px;font-weight:600;color:#1e293b;margin:0 0 12px}
.stat-row{display:flex;justify-content:space-between;padding:8px 10px;border-radius:6px;background:#f8fafc;margin-bottom:6px}
.stat-row b{color:#2563eb}.stat-row.danger b{color:#ef4444}
.right p{margin:6px 0;color:#475569}
.canvas{position:relative;background:#fff;border-radius:8px;overflow:hidden;min-height:400px}
.canvas canvas{position:absolute;inset:0;width:100%;height:100%;cursor:grab}
.legend{display:flex;align-items:center;gap:4px;font-size:12px}
.legend i{width:10px;height:10px;border-radius:50%;display:inline-block}
</style>
