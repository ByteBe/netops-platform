<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
interface Rule { id: number; name: string; metric: string; threshold: number; duration: number; level: string; channels: any; enabled: boolean }
interface Event { id: number; metric: string; target: string; value: number; level: string; message: string; acked: boolean; count?: number; suppressed?: boolean; escalated?: boolean; created_at: string }
const rules = ref<Rule[]>([])
const events = ref<Event[]>([])
const tab = ref('events')
const dlg = ref(false)
const form = reactive({ id: 0, name: '', metric: 'cpu', threshold: 80, duration: 1, durationUnit: 'm', level: 'warning', channels: '', enabled: true })
async function loadRules() { rules.value = await getEnc<Rule[]>('/alert/rules') }
async function loadEvents() { const r = await getEnc<{list:Event[]}>('/alert/events?page=1&size=100'); events.value = r.list || [] }
let autoTimer: number | undefined
onMounted(() => { loadRules(); loadEvents(); autoTimer = window.setInterval(() => { loadRules(); loadEvents() }, 30000) })
onUnmounted(() => { if (autoTimer) clearInterval(autoTimer) })
function open(r?: Partial<Rule>) {
  if (r) Object.assign(form, r)
  else Object.assign(form, { id: 0, name: '', metric: 'cpu', threshold: 80, duration: 1, durationUnit: 'm', level: 'warning', channels: '', enabled: true })
  dlg.value = true
}
async function save() {
  try {
    const payload = { ...form }
    if (form.id) await putEnc(`/alert/rules/${form.id}`, payload)
    else await postEnc('/alert/rules', payload)
    ElMessage.success(t('alert.saved')); dlg.value = false; loadRules()
  } catch (e: any) {
    ElMessage.error(e?.message || t('common.failed'))
  }
}
async function remove(r: Partial<Rule>) { await ElMessageBox.confirm(t('alert.deleteConfirm'), t('common.tip'), { type: 'warning' }); await delEnc(`/alert/rules/${r.id}`); loadRules() }
async function ack(e: Partial<Event>) { await postEnc(`/alert/events/${e.id}/ack`, {}); loadEvents() }
function levelTag(l: string) { return l === 'critical' ? 'danger' : l === 'warning' ? 'warning' : 'info' }
function levelLabel(l: string) { return l === 'critical' ? t('alert.critical') : l === 'warning' ? t('alert.warning') : t('alert.info') }
</script>
<template>
  <div class="np-page">
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title">{{ t('alert.title') }}</div>
        <div class="np-page-head-desc">{{ t('alert.desc') }}</div>
      </div>
      <div class="np-page-head-actions">
        <el-button v-if="tab==='rules'" type="primary" @click="open()">{{ t('alert.newRule') }}</el-button>
      </div>
    </div>

    <div class="np-kpi-grid">
      <div class="np-kpi">
        <div class="np-kpi-icon"><el-icon><Bell /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ events.length }}</div>
          <div class="np-kpi-label">{{ t('alert.events') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-warning"><el-icon><BellRing /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ events.filter(e => !e.acked).length }}</div>
          <div class="np-kpi-label">{{ t('alert.unacked') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-danger"><el-icon><WarningFilled /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ events.filter(e => e.level === 'critical').length }}</div>
          <div class="np-kpi-label">{{ t('alert.critical') }}</div>
        </div>
      </div>
      <div class="np-kpi">
        <div class="np-kpi-icon is-info"><el-icon><SetUp /></el-icon></div>
        <div class="np-kpi-body">
          <div class="np-kpi-value">{{ rules.length }}</div>
          <div class="np-kpi-label">{{ t('alert.rules') }}</div>
        </div>
      </div>
    </div>

    <div class="np-toolbar alert-tabs-bar">
      <el-tabs v-model="tab" class="alert-tabs">
        <el-tab-pane :label="t('alert.events')" name="events" />
        <el-tab-pane :label="t('alert.rules')" name="rules" />
      </el-tabs>
    </div>

    <div class="np-card">
      <el-table v-if="tab==='events'" :data="events" stripe>
        <el-table-column prop="created_at" :label="t('alert.time')" width="180" />
        <el-table-column :label="t('alert.level')" width="100"><template #default="{row}"><el-tag :type="levelTag(row.level)">{{ levelLabel(row.level) }}</el-tag></template></el-table-column>
        <el-table-column prop="target" :label="t('alert.target')" width="160" />
        <el-table-column prop="message" :label="t('alert.message')" min-width="300" />
        <el-table-column :label="t('common.status')" width="100"><template #default="{row}">{{ row.acked ? t('alert.acked') : t('alert.unacked') }}</template></el-table-column>
        <el-table-column :label="t('common.actions')" width="100"><template #default="{row}"><el-button v-if="!row.acked" size="small" text type="primary" @click="ack(row)">{{ t('alert.ack') }}</el-button></template></el-table-column>
      </el-table>
      <el-table v-else-if="tab==='rules'" :data="rules" stripe>
        <el-table-column prop="name" :label="t('alert.ruleName')" min-width="140" />
        <el-table-column prop="metric" :label="t('alert.metric')" width="100" />
        <el-table-column :label="t('alert.thresholdPct')" width="100">
          <template #default="{row}">{{ row.metric==='link'||row.metric==='node' ? '-' : row.threshold }}</template>
        </el-table-column>
        <el-table-column prop="duration" :label="t('alert.durationMin')" width="80" />
        <el-table-column :label="t('alert.level')" width="100"><template #default="{row}">{{ levelLabel(row.level) }}</template></el-table-column>
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
        <el-form-item v-if="form.metric!=='link' && form.metric!=='node'" :label="t('alert.threshold')">
          <el-input v-model="form.threshold" placeholder="如 80 或 80%" />
        </el-form-item>
        <el-form-item :label="t('alert.duration')">
          <el-input v-model="form.duration" placeholder="如 60" style="width:140px" />
          <el-select v-model="form.durationUnit" style="width:90px;margin-left:8px">
            <el-option :label="t('alert.unitSec')" value="s" />
            <el-option :label="t('alert.unitMin')" value="m" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('alert.level')">
          <el-select v-model="form.level">
            <el-option :label="t('alert.info')" value="info" />
            <el-option :label="t('alert.warning')" value="warning" />
            <el-option :label="t('alert.critical')" value="critical" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">{{ t('alert.cancel') }}</el-button><el-button type="primary" @click="save">{{ t('alert.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
<style scoped>
.alert-tabs :deep(.el-tabs__header) { margin: 0; }
.alert-form { margin-top: 10px; }
</style>
