<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
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
  ElMessage.success(t('alert.saved')); dlg.value = false; loadRules()
}
async function remove(r: Rule) { await ElMessageBox.confirm(t('alert.deleteConfirm'), t('common.tip'), { type: 'warning' }); await delEnc(`/alert/rules/${r.id}`); loadRules() }
async function ack(e: Event) { await postEnc(`/alert/events/${e.id}/ack`, {}); loadEvents() }
function levelTag(l: string) { return l === 'critical' ? 'danger' : l === 'warning' ? 'warning' : 'info' }
</script>
<template>
  <div class="alert-page">
    <div class="np-toolbar">
      <el-tabs v-model="tab" class="alert-tabs">
        <el-tab-pane :label="t('alert.events')" name="events" />
        <el-tab-pane :label="t('alert.rules')" name="rules" />
      </el-tabs>
      <div class="spacer"></div>
      <el-button v-if="tab==='rules'" type="primary" @click="open()">{{ t('alert.newRule') }}</el-button>
    </div>
    <div class="np-card">
      <el-table v-if="tab==='events'" :data="events" stripe>
        <el-table-column prop="created_at" :label="t('alert.time')" width="180" />
        <el-table-column :label="t('alert.level')" width="100"><template #default="{row}"><el-tag :type="levelTag(row.level)">{{ row.level }}</el-tag></template></el-table-column>
        <el-table-column prop="target" :label="t('alert.target')" width="160" />
        <el-table-column prop="message" :label="t('alert.message')" min-width="300" />
        <el-table-column :label="t('common.status')" width="100"><template #default="{row}">{{ row.acked ? t('alert.acked') : t('alert.unacked') }}</template></el-table-column>
        <el-table-column :label="t('common.actions')" width="100"><template #default="{row}"><el-button v-if="!row.acked" size="small" text type="primary" @click="ack(row)">{{ t('alert.ack') }}</el-button></template></el-table-column>
      </el-table>
      <el-table v-else-if="tab==='rules'" :data="rules" stripe>
        <el-table-column prop="name" :label="t('alert.ruleName')" min-width="140" />
        <el-table-column prop="metric" :label="t('alert.metric')" width="100" />
        <el-table-column prop="threshold" :label="t('alert.thresholdPct')" width="100" />
        <el-table-column prop="duration" :label="t('alert.durationMin')" width="80" />
        <el-table-column prop="level" :label="t('alert.level')" width="100" />
        <el-table-column prop="channels" :label="t('alert.channel')" width="120" />
        <el-table-column :label="t('common.actions')" width="150">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="open(row)">{{ t('alert.edit') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">{{ t('alert.del') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="t('alert.rules')" width="520px">
      <el-form :model="form" label-width="90px" class="alert-form">
        <el-form-item :label="t('alert.ruleName')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('alert.metric')">
          <el-select v-model="form.metric">
            <el-option label="CPU" value="cpu" />
            <el-option :label="t('alert.memHigh')" value="mem" />
            <el-option :label="t('alert.diskHigh')" value="disk" />
            <el-option :label="t('alert.link')" value="link" />
            <el-option :label="t('alert.nodeOffline')" value="node" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('alert.thresholdPct')"><el-input-number v-model="form.threshold" /></el-form-item>
        <el-form-item :label="t('alert.durationMin')"><el-input-number v-model="form.duration" /></el-form-item>
        <el-form-item :label="t('alert.level')">
          <el-select v-model="form.level">
            <el-option :label="t('alert.info')" value="info" />
            <el-option :label="t('alert.warning')" value="warning" />
            <el-option :label="t('alert.critical')" value="critical" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('alert.channel')"><el-input v-model="form.channels" placeholder="webhook,dingtalk,email" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">{{ t('alert.cancel') }}</el-button><el-button type="primary" @click="save">{{ t('alert.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
<style scoped>
.alert-page { padding: 16px; }
.alert-tabs :deep(.el-tabs__header) { margin: 0; }
.alert-form { margin-top: 10px; }
</style>
