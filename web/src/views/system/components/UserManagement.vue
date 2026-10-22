<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit, Key, Delete } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
const { t } = useI18n()
interface SysUser { id: number; username: string; employee_no: string; role: string; node_scope: string; status: string }
interface Node { id: number; name: string; node_id: string }
const users = ref<SysUser[]>([])
const nodes = ref<Node[]>([])
const dlg = ref(false)
const form = reactive({ id: 0, username: '', employee_no: '', role: 'operator', node_scope: 'all', status: 'active', password: '' })
async function load() {
  const r = await getEnc<{list: SysUser[]}>('/user/users')
  users.value = r.list || []
  try { const nr = await getEnc<{list: Node[]}>('/distributed/nodes'); nodes.value = nr.list || nr || [] } catch {}
}
onMounted(load)
function open(u?: SysUser) {
  if (u) Object.assign(form, { id: u.id, username: u.username, employee_no: u.employee_no, role: u.role, node_scope: u.node_scope || 'all', status: u.status, password: '' })
  else Object.assign(form, { id: 0, username: '', employee_no: '', role: 'operator', node_scope: 'all', status: 'active', password: '' })
  dlg.value = true
}
async function save() {
  if (!form.username) return ElMessage.warning(t('common.tip'))
  if (form.id) await putEnc(`/user/users/${form.id}`, { employee_no: form.employee_no, role: form.role, node_scope: form.node_scope, status: form.status })
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
function roleLabel(r: string) {
  const map: Record<string, string> = { admin: t('system.admin'), operator: t('system.opRole'), viewer: t('system.viewer') }
  return map[r] || r
}
</script>
<template>
  <div>
    <div class="np-card">
      <div class="card-head-row">
        <div class="np-card-title">{{ t('system.users') }}</div>
        <el-button type="primary" @click="open()">{{ t('system.addUser') }}</el-button>
      </div>
      <el-table :data="users" stripe class="np-table user-table">
        <el-table-column prop="username" :label="t('system.username')" width="140" align="center" />
        <el-table-column :label="t('system.empNo')" width="120" align="center"><template #default="{row}">{{ row.employee_no || '—' }}</template></el-table-column>
        <el-table-column :label="t('system.roleName')" min-width="200" align="center"><template #default="{row}">{{ roleLabel(row.role) }}</template></el-table-column>
        <el-table-column :label="t('system.dataScope')" min-width="180" align="center"><template #default="{row}">{{ scopeLabel(row.node_scope) }}</template></el-table-column>
        <el-table-column :label="t('common.status')" width="100" align="center"><template #default="{row}"><el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">{{ row.status === 'active' ? t('common.enabled') : t('system.disabled') }}</el-tag></template></el-table-column>
        <el-table-column :label="t('common.actions')" width="140" fixed="right" align="center">
          <template #default="{ row }">
            <div class="op-cell">
              <el-tooltip :content="t('system.edit')"><el-button size="small" circle text type="primary" :icon="Edit" @click="open(row)" /></el-tooltip>
              <el-tooltip :content="t('system.resetPwd')"><el-button size="small" circle text type="warning" :icon="Key" @click="resetPwd(row)" /></el-tooltip>
              <el-tooltip :content="t('system.del')"><el-button size="small" circle text type="danger" :icon="Delete" :disabled="row.username==='admin'" @click="remove(row)" /></el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="t('system.users')" width="500px">
      <el-form :model="form" label-width="90px">
        <el-form-item :label="t('system.username')"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item :label="t('system.empNo')"><el-input v-model="form.employee_no" /></el-form-item>
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
<style scoped>
.card-head-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.card-head-row .np-card-title { margin-bottom: 0; }
.op-cell { display: flex; gap: 2px; align-items: center; justify-content: center; }
</style>
