<script setup lang="ts">
import { ref, reactive, onMounted, computed, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'

interface Node { id:number; name:string; ip:string; vendor:string; type:string; layer:string; status:string; x:number; y:number; icon:string; remark:string }
interface Edge { id:number; name:string; source:number; target:number; color:string; bandwidth:string; utilization:number; status:string; ips:string[] }

const nodes=ref<Node[]>([]), edges=ref<Edge[]>([])
const chartEl=ref<HTMLDivElement>(), chart=ref<any>()
const selectedNode=ref<Node|null>(null), selectedEdge=ref<Edge|null>(null)
const keyword=ref(''), layerFilter=ref('all')
const legend=[{c:'#22c55e',t:'正常'},{c:'#f59e0b',t:'高负载'},{c:'#ef4444',t:'故障'}]

async function load(){ const d=await getEnc<{nodes:Node[];edges:Edge[]}>('/topology/graph'); nodes.value=d.nodes; edges.value=d.edges; await nextTick(); render() }
const nmap=computed(()=>Object.fromEntries(nodes.value.map(n=>[n.id,n])))
const shown=computed(()=>nodes.value.filter(n=>(layerFilter.value==='all'||n.layer===layerFilter.value)&&(!keyword.value||n.name.includes(keyword.value)||n.ip.includes(keyword.value))))

function render(){
  if(!chartEl.value) return
  if(!chart.value) chart.value=echarts.init(chartEl.value)
  chart.value.setOption({
    backgroundColor:'#f3f6fb', tooltip:{},
    series:[{type:'graph',layout:'none',roam:true,draggable:true,
      label:{show:true,position:'bottom',fontSize:12,color:'#1f2937'},
      edgeSymbol:['none','arrow'],edgeSymbolSize:8,
      data:shown.value.map(n=>({id:String(n.id),name:n.name,x:n.x,y:n.y,symbolSize:48,symbol:n.icon?'image://'+n.icon:`image:///topology-icons/${n.type}.png`,itemStyle:{}})),
      links:edges.value.filter(e=>nmap.value[e.source]&&nmap.value[e.target]).map(e=>({source:String(e.source),target:String(e.target),
        lineStyle:{color:e.status==='down'?'#ef4444':(e.utilization>70?'#f59e0b':'#22c55e'),width:2+(e.utilization||0)/15},
        label:{show:!!e.bandwidth,formatter:e.bandwidth,fontSize:10,color:'#475569'}}))
    }]
  })
  chart.value.off('click')
  chart.value.on('click',(p:any)=>{
    if(p.dataType==='node'){ selectedNode.value=nodes.value.find(n=>String(n.id)===p.data.id)||null; selectedEdge.value=null }
    if(p.dataType==='edge'){ selectedEdge.value=edges.value.find(e=>String(e.id)===p.data.id)||null; selectedNode.value=null }
  })
  chart.value.on('mouseup', async (p:any)=>{
    if(p.dataType==='node'){ const n=nodes.value.find(x=>String(x.id)===p.data.id); if(n) await putEnc('/topology/devices/'+n.id,{...n,x:p.data.x,y:p.data.y}) }
  })
}
async function layout(){ await postEnc('/topology/auto-layout',{}); ElMessage.success('已自动布局'); await load() }

const devDlg=ref(false), dev=reactive<Node>({id:0,name:'',ip:'',vendor:'h3c',type:'switch',layer:'access',status:'up',x:500,y:300,icon:'',remark:''})
function add(){ Object.assign(dev,{id:0,name:'',ip:'',vendor:'h3c',type:'switch',layer:'access',status:'up',x:500,y:300,icon:''}); devDlg.value=true }
async function save(){ if(!dev.name||!dev.ip) return ElMessage.warning('请填写名称和IP'); if(dev.id) await putEnc('/topology/devices/'+dev.id,dev); else await postEnc('/topology/devices',dev); devDlg.value=false; await load() }
function editNode(){ if(selectedNode.value){ Object.assign(dev,selectedNode.value); devDlg.value=true } }
async function removeNode(){ if(!selectedNode.value) return; await delEnc('/topology/devices/'+selectedNode.value.id); selectedNode.value=null; ElMessage.success('已删除'); await load() }
async function removeEdge(){ if(!selectedEdge.value) return; await delEnc('/topology/links/'+selectedEdge.value.id); selectedEdge.value=null; ElMessage.success('已删除'); await load() }

const linkDlg=ref(false), link=reactive<any>({id:0,source:0,target:0,name:'',bandwidth:'',utilization:0,status:'up',ips:''})
function addLink(){ Object.assign(link,{id:0,source:0,target:0,name:'',bandwidth:'',utilization:0,status:'up',ips:''}); linkDlg.value=true }
async function saveLink(){ if(!link.source||!link.target) return ElMessage.warning('请选择两端设备'); const body={...link,ips:link.ips.split(',').filter(Boolean)}; if(link.id) await putEnc('/topology/links/'+link.id,body); else await postEnc('/topology/links',body); linkDlg.value=false; await load() }
onMounted(load)
</script>

<template>
<div class="np-page topo-huawei">
  <div class="toolbar">
    <el-input v-model="keyword" placeholder="搜索设备名/IP" style="width:200px" @input="render"/>
    <el-radio-group v-model="layerFilter" size="small" @change="render">
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
    <div ref="chartEl" class="canvas"/>
    <div class="right">
      <h4>详情</h4>
      <div v-if="selectedNode">
        <p>名称：{{selectedNode.name}}</p><p>IP：{{selectedNode.ip}}</p><p>厂商：{{selectedNode.vendor}}</p><p>层级：{{selectedNode.layer}}</p><p>状态：{{selectedNode.status}}</p>
        <el-button size="small" type="primary" @click="editNode">修改</el-button>
        <el-button size="small" type="danger" @click="removeNode">删除</el-button>
      </div>
      <div v-else-if="selectedEdge">
        <p>源：{{nmap[selectedEdge.source]?.name}}</p><p>目标：{{nmap[selectedEdge.target]?.name}}</p><p>带宽：{{selectedEdge.bandwidth}}</p><p>利用率：{{selectedEdge.utilization}}%</p><p>IP：{{selectedEdge.ips.join(',')}}</p>
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
      <el-form-item label="带宽"><el-input v-model="link.bandwidth"/></el-form-item>
      <el-form-item label="利用率"><el-input-number v-model="link.utilization" :min="0" :max="100"/></el-form-item>
      <el-form-item label="IP"><el-input v-model="link.ips"/></el-form-item>
    </el-form>
    <template #footer><el-button @click="linkDlg=false">取消</el-button><el-button type="primary" @click="saveLink">保存</el-button></template>
  </el-dialog>
</div>
</template>

<style scoped>
.topo-huawei{display:flex;flex-direction:column;height:calc(100vh - 110px)}
.toolbar{display:flex;align-items:center;gap:10px;background:#fff;padding:10px;border-radius:8px;margin-bottom:10px}
.body{display:grid;grid-template-columns:180px 1fr 230px;gap:10px;flex:1}
.left,.right{background:#fff;border-radius:10px;padding:16px;color:#475569;font-size:13px;line-height:1.6}
.left h4,.right h4{font-size:14px;font-weight:600;color:#1e293b;margin:0 0 12px}
.stat-row{display:flex;justify-content:space-between;padding:8px 10px;border-radius:6px;background:#f8fafc;margin-bottom:6px}
.stat-row b{color:#2563eb}.stat-row.danger b{color:#ef4444}
.right p{margin:6px 0;color:#475569}
.canvas{background:#fff;border-radius:8px}
.legend{display:flex;align-items:center;gap:4px;font-size:12px}
.legend i{width:10px;height:10px;border-radius:50%;display:inline-block}
</style>
