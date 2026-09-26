<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'

interface Config { docker_enable: boolean; docker_interval: number }
interface DockerHost { id: number; name: string; address: string; enabled: boolean; remark: string; online: boolean }
interface ContainerInfo { id: string; names: string; image: string; state: string; cpu: number; mem_pct: number; host: string }

const config = reactive<Config>({ docker_enable: false, docker_interval: 30 })
const hosts = ref<DockerHost[]>([])
const containers = ref<ContainerInfo[]>([])
const loading = ref(false)
const dlgVisible = ref(false)
const editing = ref<any>({ id: 0, name: '', address: '', enabled: true, remark: '' })
const testing = ref(false)
const testResult = ref('')

async function load() {
  loading.value = true
  try {
    const c = await getEnc<Config>('/containermon/config')
    config.docker_enable = c.docker_enable; config.docker_interval = c.docker_interval
    hosts.value = await getEnc<DockerHost[]>('/containermon/docker/hosts')
    containers.value = await getEnc<ContainerInfo[]>('/containermon/docker/containers')
  } finally { loading.value = false }
}

function openAdd() { editing.value = { id: 0, name: '', address: '', enabled: true, remark: '' }; testResult.value = ''; dlgVisible.value = true }
function openEdit(h: DockerHost) { editing.value = { ...h }; testResult.value = ''; dlgVisible.value = true }
async function save() {
  if (!editing.value.name) { ElMessage.warning('名称必填'); return }
  editing.value.id ? await putEnc(`/containermon/docker/hosts/${editing.value.id}`, editing.value) : await postEnc('/containermon/docker/hosts', editing.value)
  ElMessage.success('已保存'); dlgVisible.value = false; await load()
}
async function del(h: DockerHost) { await ElMessageBox.confirm(`删除 ${h.name}？`, '提示', { type: 'warning' }); await delEnc(`/containermon/docker/hosts/${h.id}`); await load() }

async function testConn() {
  if (!editing.value.address) { ElMessage.warning('地址为空时测试本机'); }
  testing.value = true; testResult.value = ''
  try {
    const r = await postEnc<{ ok: boolean; version?: string; error?: string }>('/containermon/docker/test', { host: editing.value.address || '' })
    testResult.value = r.ok ? `连接成功 Docker v${r.version}` : `连接失败: ${r.error}`
  } catch (e: any) { testResult.value = '连接失败: ' + e.message } finally { testing.value = false }
}
async function saveConfig() {
  const full = await getEnc<any>('/containermon/config')
  await postEnc('/containermon/config', { ...full, docker_enable: config.docker_enable, docker_interval: config.docker_interval })
  ElMessage.success('已保存'); await load()
}
async function collect() { await postEnc('/containermon/docker/collect', {}); ElMessage.success('完成'); await load() }
function hostName(addr: string): string {
  const h = hosts.value.find(x => x.address === addr)
  return h ? h.name : addr
}
onMounted(load)
</script>

<template>
  <div class="np-page">
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Box /></el-icon><span>Docker 监控</span>
        <el-tag :type="config.docker_enable ? 'success' : 'info'" size="small">{{ config.docker_enable ? '运行中' : '未启用' }}</el-tag>
      </div>
      <el-form inline>
        <el-form-item label="启用"><el-switch v-model="config.docker_enable" /></el-form-item>
        <el-form-item label="间隔(秒)"><el-input-number v-model="config.docker_interval" :min="10" :max="3600" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig">保存配置</el-button>
          <el-button @click="collect">立即采集</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Monitor /></el-icon><span>Docker 主机</span>
        <div style="flex:1"></div>
        <el-button type="primary" size="small" @click="openAdd"><el-icon><Plus /></el-icon>添加主机</el-button>
      </div>
      <el-table :data="hosts" stripe class="np-table">
        <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.online ? 'success' : 'danger'" size="small">{{ row.online ? '在线' : '离线' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="主机名称" min-width="140" />
        <el-table-column label="地址" min-width="200"><template #default="{ row }">{{ row.address || '本机' }}</template></el-table-column>
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
      <div class="np-card-title"><el-icon><Box /></el-icon><span>容器列表</span><el-tag size="small">{{ containers.length }}</el-tag></div>
      <el-table :data="containers" v-loading="loading" stripe class="np-table">
        <el-table-column label="主机" width="140">
          <template #default="{ row }">{{ hostName(row.host) }}</template>
        </el-table-column>
        <el-table-column prop="names" label="名称" min-width="140" />
        <el-table-column prop="image" label="镜像" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.state === 'running' ? 'success' : 'info'" size="small">{{ row.state || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="100"><template #default="{ row }">{{ (row.cpu||0).toFixed(1) }}%</template></el-table-column>
        <el-table-column label="内存" width="100"><template #default="{ row }">{{ (row.mem_pct||0).toFixed(1) }}%</template></el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dlgVisible" :title="editing.id ? '编辑主机' : '添加主机'" width="520px">
      <el-form :model="editing" label-width="100px">
        <el-form-item label="名称" required><el-input v-model="editing.name" /></el-form-item>
        <el-form-item label="地址">
          <el-input v-model="editing.address" placeholder="空=本机；远程 tcp://IP:2375" />
        </el-form-item>
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
