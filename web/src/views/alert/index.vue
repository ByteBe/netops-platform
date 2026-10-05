<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
interface Rule { id: number; name: string; metric: string; threshold: number; duration: number; level: string; channels: string; enabled: boolean }
interface Event { id: number; metric: string; target: string; value: number; level: string; message: string; acked: boolean; created_at: string }
const rules = ref<Rule[]>([])
const events = ref<Event[]>([])
const tab = ref('events')
const dlg = ref(false)
const form = reactive({ id: 0, name: '', metric: 'cpu', threshold: 80, duration: 1, level: 'warning', channels: 'webhook', enabled: true })
async function loadRules() { rules.value = await getEnc<Rule[]>('/alert/rules') }
async function loadEvents() { const r = await getEnc<{list:Event[]}>('/alert/events?page=1&size=100'); events.value = r.list || [] }
onMounted(() => { loadRules(); loadEvents() })
function open(r?: Rule) {
  if (r) Object.assign(form, r); else Object.assign(form, { id: 0, name: '', metric: 'cpu', threshold: 80, duration: 1, level: 'warning', channels: 'webhook', enabled: true })
  dlg.value = true
}
async function save() {
  if (form.id) await putEnc(`/alert/rules/${form.id}`, form)
  else await postEnc('/alert/rules', form)
  ElMessage.success('保存成功'); dlg.value = false; loadRules()
}
async function remove(r: Rule) { await ElMessageBox.confirm('删除?', '提示', { type: 'warning' }); await delEnc(`/alert/rules/${r.id}`); loadRules() }
async function ack(e: Event) { await postEnc(`/alert/events/${e.id}/ack`, {}); loadEvents() }
function levelTag(l: string) { return l === 'critical' ? 'danger' : l === 'warning' ? 'warning' : 'info' }
</script>
<template>
  <div class="alert-page">
    <div class="np-toolbar">
      <el-tabs v-model="tab" class="alert-tabs">
        <el-tab-pane label="告警事件" name="events" />
        <el-tab-pane label="告警规则" name="rules" />
      </el-tabs>
      <div class="spacer"></div>
      <el-button v-if="tab==='rules'" type="primary" @click="open()">新建告警规则</el-button>
    </div>
    <div class="np-card">
      <el-table v-if="tab==='events'" :data="events" stripe>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column label="级别" width="100"><template #default="{row}"><el-tag :type="levelTag(row.level)">{{ row.level }}</el-tag></template></el-table-column>
        <el-table-column prop="target" label="目标" width="160" />
        <el-table-column prop="message" label="内容" min-width="300" />
        <el-table-column label="状态" width="100"><template #default="{row}">{{ row.acked ? '已确认' : '未确认' }}</template></el-table-column>
        <el-table-column label="操作" width="100"><template #default="{row}"><el-button v-if="!row.acked" size="small" text type="primary" @click="ack(row)">确认</el-button></template></el-table-column>
      </el-table>
      <el-table v-else-if="tab==='rules'" :data="rules" stripe>
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="metric" label="指标" width="100" />
        <el-table-column prop="threshold" label="阈值%" width="100" />
        <el-table-column prop="duration" label="持续分" width="80" />
        <el-table-column prop="level" label="级别" width="100" />
        <el-table-column prop="channels" label="渠道" width="120" />
        <el-table-column label="操作" width="150">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="open(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" title="告警规则" width="520px">
      <el-form :model="form" label-width="90px" class="alert-form">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="指标"><el-select v-model="form.metric"><el-option label="CPU" value="cpu" /><el-option label="内存" value="mem" /><el-option label="磁盘" value="disk" /><el-option label="链路" value="link" /><el-option label="节点离线" value="node" /></el-select></el-form-item>
        <el-form-item label="阈值%"><el-input-number v-model="form.threshold" /></el-form-item>
        <el-form-item label="持续分"><el-input-number v-model="form.duration" /></el-form-item>
        <el-form-item label="级别"><el-select v-model="form.level"><el-option label="信息" value="info" /><el-option label="警告" value="warning" /><el-option label="严重" value="critical" /></el-select></el-form-item>
        <el-form-item label="渠道"><el-input v-model="form.channels" placeholder="webhook,dingtalk,email" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
<style scoped>
.alert-page { padding: 16px; }
.alert-tabs :deep(.el-tabs__header) { margin: 0; }
.alert-form { margin-top: 10px; }
</style>
