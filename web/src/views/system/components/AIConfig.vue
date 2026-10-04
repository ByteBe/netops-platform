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
    <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="open()">添加 AI</el-button></div>
    <div class="np-card">
      <el-table :data="list" stripe class="np-table">
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="provider" label="提供商" width="120" />
        <el-table-column prop="model" label="模型" min-width="140" />
        <el-table-column prop="priority" label="优先级" width="80" />
        <el-table-column label="启用" width="80">
          <template #default="{ row }"><el-tag :type="row.enable?'success':'info'" size="small">{{ row.enable?'是':'否' }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="200">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="open(row)">编辑</el-button>
            <el-button size="small" text type="success" @click="test(row)">测试</el-button>
            <el-button size="small" text type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" title="AI 配置" width="600px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="提供商">
          <el-select v-model="form.provider" @change="onProvider"><el-option v-for="p in providers" :key="p.value" :label="p.label" :value="p.value" /></el-select>
        </el-form-item>
        <el-form-item label="Base URL"><el-input v-model="form.base_url" /></el-form-item>
        <el-form-item label="API Key"><el-input v-model="form.api_key" /></el-form-item>
        <el-form-item label="模型"><el-input v-model="form.model" /></el-form-item>
        <el-form-item label="Temperature"><el-input-number v-model="form.temperature" :step="0.1" :min="0" :max="2" /></el-form-item>
        <el-form-item label="优先级"><el-input-number v-model="form.priority" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enable" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="replyVisible" title="测试回复" width="500px"><pre style="white-space:pre-wrap">{{ reply }}</pre></el-dialog>
  </div>
</template>
