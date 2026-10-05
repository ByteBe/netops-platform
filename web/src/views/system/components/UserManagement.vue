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
  const { value } = await ElMessageBox.prompt('输入新密码（>12位）', '重置密码', { inputType: 'password' })
  await postEnc(`/user/users/${u.id}/reset-password`, { password: value }); ElMessage.success(t('common.success'))
}
function scopeLabel(s: string) { return s === 'all' ? '全部节点' : (nodes.value.find(n => n.node_id === s)?.name || s) }
</script>
<template>
  <div>
    <div class="np-toolbar"><div class="spacer"></div><el-button type="primary" @click="open()">添加用户</el-button></div>
    <div class="np-card">
      <el-table :data="users" stripe class="np-table">
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="employee_no" label="工号" width="120" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="role" label="角色" width="90" />
        <el-table-column label="数据范围" min-width="140"><template #default="{row}">{{ scopeLabel(row.node_scope) }}</template></el-table-column>
        <el-table-column prop="status" label="状态" width="90" />
        <el-table-column :label="t('common.actions')" width="230" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="open(row)">编辑</el-button>
            <el-button size="small" text type="warning" @click="resetPwd(row)">重置密码</el-button>
            <el-button size="small" text type="danger" :disabled="row.username==='admin'" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" title="用户" width="500px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item label="工号"><el-input v-model="form.employee_no" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role"><el-option label="管理员" value="admin" /><el-option label="操作员" value="operator" /></el-select>
        </el-form-item>
        <el-form-item label="数据范围">
          <el-select v-model="form.node_scope" placeholder="选择节点">
            <el-option label="全部节点（总部）" value="all" />
            <el-option v-for="n in nodes" :key="n.node_id" :label="n.name" :value="n.node_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status"><el-option label="启用" value="active" /><el-option label="禁用" value="disabled" /></el-select>
        </el-form-item>
        <el-form-item v-if="!form.id" label="密码"><el-input v-model="form.password" type="password" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
