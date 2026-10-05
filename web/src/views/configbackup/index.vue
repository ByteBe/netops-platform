<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, delEnc } from '@/utils/request'
interface Backup { id: number; device_ip: string; device_name: string; vendor: string; version: number; hash: string; source: string; created_at: string }
const list = ref<Backup[]>([])
const total = ref(0)
const page = ref(1)
const size = 20
const dlg = ref(false)
const detail = ref<any>(null)
const diffDlg = ref(false)
const diffData = ref<any>(null)
const sel = ref<number[]>([])
const form = reactive({ device_ip: '', device_name: '', vendor: 'huawei', content: '' })
const schedDlg = ref(false)
const sched = reactive({ enabled: false, cron: '0 2 * * *', interval_hours: 24, devices: 'all', keep: 30 })
const schedPreset = ref('')
async function loadSched() {
  try { const r = await getEnc<any>('/configbackup/schedule'); Object.assign(sched, r) } catch {}
}
async function saveSched() {
  await postEnc('/configbackup/schedule', sched)
  ElMessage.success('已保存'); schedDlg.value = false
}
async function load() {
  const r = await getEnc<{total:number;list:Backup[]}>(`/configbackup/config-backups?page=${page.value}&size=${size}`)
  list.value = r.list || []; total.value = r.total || 0
}
onMounted(() => { load(); loadSched() })
function openNew() { dlg.value = true }
async function save() {
  await postEnc('/configbackup/config-backups', form)
  ElMessage.success('备份成功'); dlg.value = false; await load()
}
async function view(row: Backup) {
  const r = await getEnc<any>(`/configbackup/config-backups/${row.id}`)
  detail.value = r
}
async function diff(row: Backup) {
  const others = list.value.filter(b => b.device_ip === row.device_ip && b.id !== row.id)
  if (!others.length) return ElMessage.warning('该设备暂无其他版本可对比')
  const { value } = await ElMessageBox.prompt('输入要对比的版本ID', '版本对比', { inputPlaceholder: others.map(b=>`v${b.version}#${b.id}`).join(', ') })
  const r = await getEnc<any>(`/configbackup/config-backups/diff?id1=${row.id}&id2=${value}`)
  diffData.value = r
  diffDlg.value = true
}
async function rollback(row: Backup) {
  await ElMessageBox.confirm(`确认回滚到 ${row.device_ip} v${row.version}?`, '回滚', { type: 'warning' })
  const r = await postEnc(`/configbackup/config-backups/${row.id}/rollback`, {})
  ElMessage.success('回滚包已生成')
}
async function remove(row: Backup) {
  await ElMessageBox.confirm('确认删除该备份?', '删除', { type: 'warning' })
  await delEnc(`/configbackup/config-backups/${row.id}`); ElMessage.success('已删除'); await load()
}
</script>
<template>
  <div>
    <div class="np-toolbar">
      <div class="spacer"></div>
      <el-button @click="schedDlg = true">定时备份</el-button>
      <el-button type="primary" @click="openNew()">手动备份</el-button>
    </div>
    <div class="np-card">
      <el-table :data="list" stripe class="np-table">
        <el-table-column prop="device_ip" label="设备IP" width="140" />
        <el-table-column prop="device_name" label="设备名" min-width="140" />
        <el-table-column prop="vendor" label="厂商" width="100" />
        <el-table-column prop="version" label="版本" width="80" />
        <el-table-column prop="source" label="来源" width="80" />
        <el-table-column prop="created_at" label="备份时间" width="180" />
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="view(row)">查看</el-button>
            <el-button size="small" text type="success" @click="diff(row)">对比</el-button>
            <el-button size="small" text type="warning" @click="rollback(row)">回滚</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" title="手动备份" width="600px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="设备IP"><el-input v-model="form.device_ip" /></el-form-item>
        <el-form-item label="设备名"><el-input v-model="form.device_name" /></el-form-item>
        <el-form-item label="厂商">
          <el-select v-model="form.vendor"><el-option label="华为" value="huawei" /><el-option label="思科" value="cisco" /><el-option label="H3C" value="h3c" /></el-select>
        </el-form-item>
        <el-form-item label="配置内容"><el-input v-model="form.content" type="textarea" :rows="10" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="detail" title="备份内容" width="700px">
      <pre style="max-height:500px;overflow:auto;background:#f5f5f5;padding:10px">{{ detail?.content }}</pre>
    </el-dialog>
    <el-dialog v-model="diffDlg" title="版本对比" width="900px">
      <el-row :gutter="16">
        <el-col :span="12"><h4>左</h4><pre style="max-height:400px;overflow:auto;background:#f5f5f5;padding:10px">{{ diffData?.left?.content }}</pre></el-col>
        <el-col :span="12"><h4>右</h4><pre style="max-height:400px;overflow:auto;background:#f5f5f5;padding:10px">{{ diffData?.right?.content }}</pre></el-col>
      </el-row>
      <el-alert :title="diffData?.same ? '两版本完全一致' : '两版本有差异'" :type="diffData?.same ? 'success' : 'warning'" />
    </el-dialog>
    <el-dialog v-model="schedDlg" title="定时备份" width="520px">
      <el-form :model="sched" label-width="110px">
        <el-form-item label="启用"><el-switch v-model="sched.enabled" /></el-form-item>
        <el-form-item label="执行间隔(小时)"><el-input-number v-model="sched.interval_hours" :min="1" /></el-form-item>
        <el-form-item label="Cron">
          <el-input v-model="sched.cron" />
          <div style="color:#999;font-size:12px;margin-top:4px">分 时 日 月 周，如 0 2 * * * 表示每天凌晨2点</div>
        </el-form-item>
        <el-form-item label="快捷选择">
          <el-radio-group v-model="schedPreset" @change="(v:any)=>sched.cron=v">
            <el-radio value="0 2 * * *">每天凌晨2点</el-radio>
            <el-radio value="0 2 * * 0">每周日</el-radio>
            <el-radio value="0 2 1 * *">每月1号</el-radio>
            <el-radio value="0 */6 * * *">每6小时</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备份范围">
          <el-radio-group v-model="sched.devices">
            <el-radio value="all">全部设备</el-radio>
            <el-radio value="group">按分组</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="保留天数"><el-input-number v-model="sched.keep" :min="1" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="schedDlg=false">取消</el-button><el-button type="primary" @click="saveSched">保存</el-button></template>
    </el-dialog>
  </div>
</template>
