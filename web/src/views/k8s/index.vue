<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'

interface Config { k8s_enable: boolean; k8s_interval: number }
interface K8sCluster { id: number; name: string; api_server: string; token: string; enabled: boolean; remark: string; online: boolean }

const config = reactive<Config>({ k8s_enable: false, k8s_interval: 60 })
const clusters = ref<K8sCluster[]>([])
const nodes = ref<any[]>([])
const loading = ref(false)
const dlgVisible = ref(false)
const editing = ref<any>({ id: 0, name: '', api_server: '', token: '', enabled: true, remark: '' })
const testing = ref(false)
const testResult = ref('')

async function load() {
  loading.value = true
  try {
    const c = await getEnc<Config>('/containermon/config')
    config.k8s_enable = c.k8s_enable; config.k8s_interval = c.k8s_interval
    clusters.value = await getEnc<K8sCluster[]>('/containermon/k8s/clusters')
    nodes.value = await getEnc('/containermon/k8s/nodes')
  } finally { loading.value = false }
}

function openAdd() { editing.value = { id: 0, name: '', api_server: '', token: '', enabled: true, remark: '' }; testResult.value = ''; dlgVisible.value = true }
function openEdit(k: K8sCluster) { editing.value = { ...k }; testResult.value = ''; dlgVisible.value = true }
async function save() {
  if (!editing.value.name) { ElMessage.warning('名称必填'); return }
  if (!editing.value.api_server) { ElMessage.warning('API Server必填'); return }
  if (editing.value.id) await putEnc(`/containermon/k8s/clusters/${editing.value.id}`, editing.value)
  else await postEnc('/containermon/k8s/clusters', editing.value)
  ElMessage.success('已保存'); dlgVisible.value = false; await load()
}
async function del(k: K8sCluster) { await ElMessageBox.confirm(`删除 ${k.name}？`, '提示', { type: 'warning' }); await delEnc(`/containermon/k8s/clusters/${k.id}`); await load() }

async function testConn() {
  if (!editing.value.api_server) { ElMessage.warning('请先填写API Server'); return }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ ok: boolean; version?: string; error?: string }>('/containermon/k8s/test', { api_server: editing.value.api_server, token: editing.value.token })
    testResult.value = r.ok ? `连接成功 v${r.version}` : `连接失败: ${r.error}`
  } catch (e: any) { testResult.value = '连接失败: ' + e.message } finally { testing.value = false }
}

async function saveConfig() {
  const full = await getEnc<any>('/containermon/config')
  await postEnc('/containermon/config', { ...full, k8s_enable: config.k8s_enable, k8s_interval: config.k8s_interval })
  ElMessage.success('已保存'); await load()
}
async function collect() { await postEnc('/containermon/k8s/collect', {}); ElMessage.success('完成'); await load() }
onMounted(load)
</script>

<template>
  <div class="np-page">
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>Kubernetes 监控</span>
        <el-tag :type="config.k8s_enable ? 'success' : 'info'" size="small">{{ config.k8s_enable ? '运行中' : '未启用' }}</el-tag>
      </div>
      <el-form inline>
        <el-form-item label="启用"><el-switch v-model="config.k8s_enable" /></el-form-item>
        <el-form-item label="间隔(秒)"><el-input-number v-model="config.k8s_interval" :min="10" :max="3600" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig">保存配置</el-button>
          <el-button @click="collect">立即采集</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Ship /></el-icon><span>K8s 集群</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>添加集群</el-button>
      </div>
      <el-table :data="clusters" stripe class="np-table">
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.online ? 'success' : 'danger'" size="small">{{ row.online ? '在线' : '离线' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="集群名称" min-width="140" />
        <el-table-column prop="api_server" label="API Server" min-width="200" />
        <el-table-column prop="remark" label="备注" min-width="150" />
        <el-table-column label="启用" width="70"><template #default="{ row }"><el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="del(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="np-card">
      <div class="np-card-title"><el-icon><Ship /></el-icon><span>节点列表</span><el-tag size="small">{{ nodes.length }}</el-tag></div>
      <el-table :data="nodes" v-loading="loading" stripe class="np-table">
        <el-table-column prop="cluster" label="集群" width="120" />
        <el-table-column prop="name" label="Node" min-width="160" />
        <el-table-column label="就绪" width="80">
          <template #default="{ row }">
            <el-tag :type="row.ready ? 'success' : 'danger'" size="small">{{ row.ready ? '就绪' : '未就绪' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="100"><template #default="{ row }">{{ (row.cpu||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column label="内存" width="100"><template #default="{ row }">{{ (row.mem||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column prop="role" label="角色" width="90" />
      </el-table>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? '编辑集群' : '添加集群'" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item label="名称" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item label="API Server" required><el-input v-model="editing.api_server" placeholder="https://10.0.0.1:6443" /></el-form-item>
        <el-form-item label="Token"><el-input v-model="editing.token" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button :loading="testing" @click="testConn">测试连接</el-button>
          <span v-if="testResult" :style="{ marginLeft:'12px', color: testResult.startsWith('连接成功') ? '#67C23A' : '#F56C6C', fontSize:'13px' }">{{ testResult }}</span>
        </el-form-item>
        <el-form-item label="启用"><el-switch v-model="editing.enabled" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="editing.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss"></style>
