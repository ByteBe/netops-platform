<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
interface SysUser { id: number; username: string; employee_no: string; email: string; role: string; node_scope: string; status: string }
interface Node { id: number; name: string; node_id: string }
const users = ref<SysUser[]>([])
const nodes = ref<Node[]>([])
const dlg = ref(false)
const form = reactive({ id: 0, username: '', employee_no: '', email: '', role: 'operator', node_scope: 'all', status: 'active', password: '' })
async function load() {
  const r = await getEnc<{list: SysUser[]}>('/user/users')
  users.value = r.list || []
  try { const nr = await getEnc<{list: Node[]}>('/distributed/nodes'); nodes.value = nr.list || nr || [] } catch {}
}
onMounted(load)
function open(u?: SysUser) {
  if (u) Object.assign(form, { id: u.id, username: u.username, employee_no: u.employee_no, email: u.email, role: u.role, node_scope: u.node_scope || 'all', status: u.status, password: '' })
  else Object.assign(form, { id: 0, username: '', employee_no: '', email: '', role: 'operator', node_scope: 'all', status: 'active', password: '' })
  dlg.value = true
}
async function save() {
  if (!form.username || !form.email) return ElMessage.warning(t('common.tip'))
  if (form.id) await putEnc(`/user/users/${form.id}`, { employee_no: form.employee_no, email: form.email, role: form.role, node_scope: form.node_scope, status: form.status })
  else await postEnc('/user/users', form)
  ElMessage.success(t('common.success')); dlg.value = false; await load()
}
async function remove(u: SysUser) {
  await ElMessageBox.confirm(t('common.confirmDelete'), t('common.tip'), { type: 'warning' })
  await delEnc(`/user/users/${u.id}`); ElMessage.success(t('common.success')); await load()
}
async function resetPwd(u: SysUser) {
  const { value } = await ElMessageBox.prompt(t('system.pwdPrompt'), t('system.resetPwdTitle'), { inputType: 'password' })
  await postEnc(`/user/users/${u.id}/reset-password`, { password: value }); ElMessage.success(t('common.success'))
}
function scopeLabel(s: string) { return s === 'all' ? t('system.allNodes') : (nodes.value.find(n => n.node_id === s)?.name || s) }
</script>
<template>
  <div>
    <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="open()">{{ t('system.addUser') }}</el-button></div>
    <div class="np-card">
      <el-table :data="users" stripe class="np-table">
        <el-table-column prop="username" :label="t('system.username')" min-width="120" />
        <el-table-column prop="employee_no" :label="t('system.empNo')" width="120" />
        <el-table-column prop="email" :label="t('system.emailCol')" min-width="180" />
        <el-table-column prop="role" :label="t('system.roleName')" width="90" />
        <el-table-column :label="t('system.dataScope')" min-width="140"><template #default="{row}">{{ scopeLabel(row.node_scope) }}</template></el-table-column>
        <el-table-column prop="status" :label="t('common.status')" width="90" />
        <el-table-column :label="t('common.actions')" width="230" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="open(row)">{{ t('system.edit') }}</el-button>
            <el-button size="small" text type="warning" @click="resetPwd(row)">{{ t('system.resetPwd') }}</el-button>
            <el-button size="small" text type="danger" :disabled="row.username==='admin'" @click="remove(row)">{{ t('system.del') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="t('system.users')" width="500px">
      <el-form :model="form" label-width="90px">
        <el-form-item :label="t('system.username')"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item :label="t('system.empNo')"><el-input v-model="form.employee_no" /></el-form-item>
        <el-form-item :label="t('system.emailCol')"><el-input v-model="form.email" /></el-form-item>
        <el-form-item :label="t('system.roleName')">
          <el-select v-model="form.role"><el-option :label="t('system.admin')" value="admin" /><el-option :label="t('system.opRole')" value="operator" /></el-select>
        </el-form-item>
        <el-form-item :label="t('system.dataScope')">
          <el-select v-model="form.node_scope" :placeholder="t('system.selectNode')">
            <el-option :label="t('system.allNodes')" value="all" />
            <el-option v-for="n in nodes" :key="n.node_id" :label="n.name" :value="n.node_id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-select v-model="form.status"><el-option :label="t('common.enabled')" value="active" /><el-option :label="t('system.disabled')" value="disabled" /></el-select>
        </el-form-item>
        <el-form-item v-if="!form.id" :label="t('system.password')"><el-input v-model="form.password" type="password" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">{{ t('system.cancel') }}</el-button><el-button type="primary" @click="save">{{ t('system.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
