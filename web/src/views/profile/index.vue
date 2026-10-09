<script setup lang="ts">
// 个人中心：个人信息 + 修改密码（密码强度 >12位：大写+小写+数字+符号）
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { postEnc } from '@/utils/request'
import { validatePasswordStrength } from '@/utils'

const { t } = useI18n()
const auth = useAuthStore()

const pwdForm = reactive({ old_password: '', new_password: '', confirm: '' })
const saving = ref(false)

async function changePwd() {
  if (!pwdForm.old_password || !pwdForm.new_password) {
    ElMessage.warning(t('common.tip'))
    return
  }
  const reason = validatePasswordStrength(pwdForm.new_password)
  if (reason) {
    ElMessage.warning(reason)
    return
  }
  if (pwdForm.new_password !== pwdForm.confirm) {
    ElMessage.warning(t('profile.pwdMismatch'))
    return
  }
  saving.value = true
  try {
    await postEnc('/auth/change-password', pwdForm)
    ElMessage.success(t('common.success'))
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm = ''
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="np-page" style="max-width: 720px">
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><User /></el-icon>
        <span>{{ t('profile.info') }}</span>
      </div>
      <el-descriptions :column="2" border>
        <el-descriptions-item :label="t('profile.username')">{{ auth.user?.username }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.employeeNo')">{{ auth.user?.employee_no || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.email')">{{ auth.user?.email || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.role')">{{ auth.user?.role }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.lastLogin')">{{ auth.user?.last_login_at || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.status')">{{ auth.user?.status }}</el-descriptions-item>
      </el-descriptions>
    </div>

    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Lock /></el-icon>
        <span>{{ t('profile.changePwd') }}</span>
      </div>
      <el-alert type="info" :closable="false" :title="t('profile.pwdStrength')" style="margin-bottom: 14px" />
      <el-form :model="pwdForm" label-width="120px" style="max-width: 460px">
        <el-form-item :label="t('profile.oldPwd')">
          <el-input v-model="pwdForm.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('profile.newPwd')">
          <el-input v-model="pwdForm.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('profile.confirmPwd')">
          <el-input v-model="pwdForm.confirm" type="password" show-password @keyup.enter="changePwd" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="changePwd">{{ t('common.save') }}</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<style lang="scss"></style>
