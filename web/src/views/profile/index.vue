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
  <div class="np-page np-profile">
    <!-- 统一页头 -->
    <div class="np-page-head">
      <div class="np-page-head-info">
        <div class="np-page-head-title"><el-icon><User /></el-icon>{{ t('profile.info') }}</div>
        <div class="np-page-head-desc">{{ t('profile.pageDesc') }}</div>
      </div>
    </div>

    <!-- 个人信息 -->
    <div class="np-card">
      <div class="np-profile-hero">
        <el-avatar :size="56" class="np-profile-avatar">
          {{ (auth.user?.username || '?').slice(0, 1).toUpperCase() }}
        </el-avatar>
        <div class="np-profile-hero-info">
          <div class="np-profile-name">{{ auth.user?.username }}</div>
          <el-tag size="small" type="primary" class="np-profile-role">
            {{ auth.user?.role === 'admin' ? t('profile.admin') : t('profile.user') }}
          </el-tag>
        </div>
      </div>
      <el-descriptions :column="2" border class="np-profile-desc">
        <el-descriptions-item :label="t('profile.username')">{{ auth.user?.username }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.employeeNo')">{{ auth.user?.employee_no || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.email')">{{ auth.user?.email || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.role')">{{ auth.user?.role }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.lastLogin')">{{ auth.user?.last_login_at || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('profile.status')">{{ auth.user?.status }}</el-descriptions-item>
      </el-descriptions>
    </div>

    <!-- 修改密码 -->
    <div class="np-card">
      <div class="np-card-title">
        <el-icon><Lock /></el-icon>
        <span>{{ t('profile.changePwd') }}</span>
      </div>
      <el-alert type="info" :closable="false" :title="t('profile.pwdStrength')" class="np-pwd-alert" />
      <el-form :model="pwdForm" label-width="120px" class="np-pwd-form">
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

<style lang="scss" scoped>
.np-profile {
  max-width: 720px;
}

.np-profile-hero {
  display: flex;
  align-items: center;
  gap: 16px;
  padding-bottom: 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--np-border);
}

.np-profile-avatar {
  flex: none;
  background: var(--np-primary);
  color: var(--np-primary-fg);
  font-size: 22px;
  font-weight: 700;
  border: 1px solid var(--np-primary);
}

.np-profile-hero-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.np-profile-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--np-text-1);
}

.np-profile-desc {
  margin-top: 4px;
}

.np-pwd-alert {
  margin-bottom: 16px;
}

.np-pwd-form {
  max-width: 460px;
}
</style>
