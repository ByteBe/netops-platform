<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
interface AIConfig { id: number; name: string; provider: string; base_url: string; api_key: string; model: string; temperature: number; priority: number; enable: boolean; remark: string }
const list = ref<AIConfig[]>([])
const providers = ref<any[]>([])
const dlg = ref(false)
const form = reactive<AIConfig>({ id: 0, name: '', provider: 'openai', base_url: '', api_key: '', model: '', temperature: 0.7, priority: 0, enable: true, remark: '' })
async function load() {
  const [l, p] = await Promise.all([getEnc<AIConfig[]>('/system/ai/list'), getEnc<any[]>('/system/ai/providers')])
  list.value = l; providers.value = p
}
onMounted(load)
function open(a?: AIConfig) {
  reply.value = ''
  if (a) Object.assign(form, a)
  else Object.assign(form, { id: 0, name: '', provider: 'openai', base_url: 'https://api.openai.com/v1', api_key: '', model: '', temperature: 0.7, priority: list.value.length, enable: true, remark: '' })
  dlg.value = true
}
function onProvider() { const p = providers.value.find(x => x.value === form.provider); if (p?.base_url) form.base_url = p.base_url }
async function save() {
  if (!form.name || !form.base_url) return ElMessage.warning(t('common.tip'))
  if (form.id) await putEnc(`/system/ai/${form.id}`, form); else await postEnc('/system/ai', form)
  ElMessage.success(t('common.success')); dlg.value = false; await load()
}
async function remove(a: AIConfig) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/system/ai/${a.id}`); ElMessage.success(t('common.success')); await load()
}
const reply = ref('')
const replyVisible = ref(false)
async function test(a: AIConfig) { const r = await postEnc<any>(`/system/ai/${a.id}/test`, {}); reply.value = r.reply; replyVisible.value = true }
</script>
<template>
  <div>
    <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="open()">{{ t('system.addAi') }}</el-button></div>
    <div class="np-card">
      <el-table :data="list" stripe class="np-table">
        <el-table-column prop="name" :label="t('common.name')" min-width="120" />
        <el-table-column prop="provider" :label="t('system.provider')" width="120" />
        <el-table-column prop="model" :label="t('system.model')" min-width="140" />
        <el-table-column prop="priority" :label="t('system.priority')" width="80" />
        <el-table-column :label="t('common.enabled')" width="80">
          <template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?t('system.yes'):t('system.no') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="200">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="open(row)">{{ t('system.edit') }}</el-button>
            <el-button size="small" text type="success" @click="test(row)">{{ t('system.test') }}</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">{{ t('system.del') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="t('system.ai')" width="600px">
      <el-form :model="form" label-width="90px">
        <el-form-item :label="t('common.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('system.provider')">
          <el-select v-model="form.provider" @change="onProvider"><el-option v-for="p in providers" :key="p.value" :label="p.label" :value="p.value" /></el-select>
        </el-form-item>
        <el-form-item :label="t('system.apiBase')"><el-input v-model="form.base_url" /></el-form-item>
        <el-form-item :label="t('system.apiKey')"><el-input v-model="form.api_key" /></el-form-item>
        <el-form-item :label="t('system.model')"><el-input v-model="form.model" /></el-form-item>
        <el-form-item label="Temperature"><el-input-number v-model="form.temperature" :step="0.1" :min="0" :max="2" /></el-form-item>
        <el-form-item :label="t('system.priority')"><el-input-number v-model="form.priority" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="form.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="save">{{ t('system.save') }}</el-button></template>
    </el-dialog>
    <el-dialog v-model="replyVisible" :title="t('system.testReply')" width="500px"><pre style="white-space:pre-wrap">{{ reply }}</pre></el-dialog>
  </div>
</template>
