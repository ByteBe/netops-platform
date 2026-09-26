<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { validatePasswordStrength } from '@/utils'
import { useAppStore } from '@/stores/app'
import { login, changePassword, fetchPublicKey } from '@/api/login'
import type { LoginForm, ChangePwdForm } from '@/api/types'
import './style.scss'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()
const appStore = useAppStore()

const form = reactive<LoginForm>({ username: '', password: '' })
const loading = ref(false)
const forceChange = ref(route.query.force === '1')

const pwdForm = reactive<ChangePwdForm>({ oldPassword: '', newPassword: '', confirm: '' })
const pwdLoading = ref(false)

const step = computed(() => (forceChange.value ? 1 : 0))

async function onLogin() {
  if (!form.username || !form.password) {
    ElMessage.warning(t('common.tip'))
    return
  }
  loading.value = true
  try {
    const { result, sessionKey } = await login(form)
    auth.setLogin(result.token, sessionKey, localStorage.getItem('np-sm2-pub') || '')
    auth.setUser({
      id: 0,
      username: result.username,
      email: result.email,
      employee_no: result.employee_no,
      role: result.role,
      status: 'active',
      must_change_pwd: result.must_change_pwd,
      last_login_at: ''
    })
    if (result.must_change_pwd) {
      forceChange.value = true
    } else {
      ElMessage.success(t('common.success'))
      router.push('/linkdetect')
    }
  } catch (e) {
    ElMessage.error((e as Error).message || t('login.failed'))
  } finally {
    loading.value = false
  }
}

async function onChangePwd() {
  const err = validatePasswordStrength(pwdForm.newPassword)
  if (err) {
    ElMessage.warning(err)
    return
  }
  if (pwdForm.newPassword !== pwdForm.confirm) {
    ElMessage.warning(t('login.pwdNotMatch'))
    return
  }
  pwdLoading.value = true
  try {
    await changePassword({ old_password: pwdForm.oldPassword, new_password: pwdForm.newPassword })
    ElMessage.success(t('common.success'))
    auth.logout()
    forceChange.value = false
    pwdForm.oldPassword = ''
    pwdForm.newPassword = ''
    pwdForm.confirm = ''
    router.push('/login')
  } catch {
    /* 拦截器已提示 */
  } finally {
    pwdLoading.value = false
  }
}

onMounted(async () => {
  appStore.applyTheme()
  try {
    const pub = await fetchPublicKey()
    localStorage.setItem('np-sm2-pub', pub.public_key)
  } catch {
    /* 忽略 */
  }
})
</script>

<template>
  <div class="np-login">
    <div class="np-login-bg"></div>
    <div class="np-login-card">
      <div class="np-login-head">
        <div class="np-login-logo">
          <el-icon :size="34" color="#2f6bff"><Monitor /></el-icon>
        </div>
        <h1>{{ t('app.name') }}</h1>
        <p>{{ t('login.title') }}</p>
      </div>

      <el-form v-if="step === 0" :model="form" label-position="top" @keyup.enter="onLogin">
        <el-form-item :label="t('login.username')">
          <el-input v-model="form.username" size="large" :placeholder="t('login.username')" clearable>
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('login.password')">
          <el-input v-model="form.password" size="large" type="password" show-password :placeholder="t('login.password')">
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" class="np-login-btn" :loading="loading" @click="onLogin">
          {{ t('login.loginBtn') }}
        </el-button>
      </el-form>

      <div v-else class="np-force-pwd">
        <el-alert :title="t('login.changePwdTitle')" type="warning" :closable="false" show-icon />
        <p class="np-force-tip">{{ t('login.forceChangeTip') }}</p>
        <p class="np-pwd-rule">* {{ t('login.pwdRule') }}</p>
        <el-form :model="pwdForm" label-position="top">
          <el-form-item :label="t('login.oldPassword')">
            <el-input v-model="pwdForm.oldPassword" type="password" show-password size="large" />
          </el-form-item>
          <el-form-item :label="t('login.newPassword')">
            <el-input v-model="pwdForm.newPassword" type="password" show-password size="large" />
          </el-form-item>
          <el-form-item :label="t('login.confirmPassword')">
            <el-input v-model="pwdForm.confirm" type="password" show-password size="large" />
          </el-form-item>
          <el-button type="primary" size="large" class="np-login-btn" :loading="pwdLoading" @click="onChangePwd">
            {{ t('common.submit') }}
          </el-button>
        </el-form>
      </div>

      <div class="np-login-lang">
        <el-button text size="small" @click="appStore.setLang('zh-CN')">中文</el-button>
        <span>|</span>
        <el-button text size="small" @click="appStore.setLang('en-US')">English</el-button>
      </div>
    </div>
  </div>
</template>
