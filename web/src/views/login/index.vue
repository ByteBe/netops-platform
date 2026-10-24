<script setup lang="ts">
import { reactive, ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { validatePasswordStrength } from '@/utils'
import { useAppStore } from '@/stores/app'
import { login, changePassword, fetchPublicKey, fetchCaptcha } from '@/api/login'
import type { LoginForm, ChangePwdForm } from '@/api/types'
import '@/styles/login.scss'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()
const appStore = useAppStore()

const form = reactive<LoginForm & { captchaCode?: string }>({ username: '', password: '', captchaCode: '' })
const loading = ref(false)
const forceChange = ref(route.query.force === '1')
const captchaId = ref('')
const captchaImg = ref('')
const captchaFail = ref(false)
const captchaLeft = ref(0) // 剩余秒数
let captchaExpireAt = 0
let captchaTimer = 0

// 验证码倒计时：显示剩余时间，最后 10 秒自动刷新并提示
function startCaptchaCountdown() {
  if (captchaTimer) clearInterval(captchaTimer)
  captchaExpireAt = Date.now() + 5 * 60 * 1000
  captchaTimer = window.setInterval(() => {
    const left = Math.max(0, Math.round((captchaExpireAt - Date.now()) / 1000))
    captchaLeft.value = left
    if (left > 0 && left <= 10 && captchaImg.value) {
      refreshCaptcha()
      ElMessage.info(t('login.captchaRefreshed'))
    }
  }, 1000)
}

const pwdForm = reactive<ChangePwdForm>({ oldPassword: '', newPassword: '', confirm: '' })
const pwdLoading = ref(false)

const step = computed(() => (forceChange.value ? 1 : 0))

async function refreshCaptcha() {
  try {
    const cap = await fetchCaptcha()
    captchaId.value = cap.captcha_id
    captchaImg.value = 'data:image/png;base64,' + cap.image_base64
    captchaFail.value = false
    form.captchaCode = ''
    startCaptchaCountdown()
  } catch {
    captchaImg.value = ''
    captchaFail.value = true
    captchaLeft.value = 0
  }
}

async function onLogin() {
  // 防重入：回车键 + 点击按钮并发时，只允许一个登录请求
  // （一次性验证码被第一个请求消费后，第二个请求会报"验证码错误或已过期"并覆盖成功提示）
  if (loading.value) return
  if (!form.username || !form.password) {
    ElMessage.warning(t('common.tip'))
    return
  }
  if (!captchaId.value || !form.captchaCode) {
    ElMessage.warning(t('login.captchaPlaceholder'))
    return
  }
  if (captchaFail.value || !captchaImg.value) {
    ElMessage.warning(t('login.captchaLoadFail'))
    refreshCaptcha()
    return
  }
  loading.value = true
  try {
    const { result, sessionKey } = await login({ ...form, captcha_id: captchaId.value, captcha_code: form.captchaCode })
    auth.setLogin(result.token, sessionKey, sessionStorage.getItem('np-sm2-pub') || '')
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
    form.captchaCode = ''
    refreshCaptcha()
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
    sessionStorage.setItem('np-sm2-pub', pub.public_key)
  } catch {
    /* 忽略 */
  }
  refreshCaptcha()
  initParticles()
})
onBeforeUnmount(() => {
  if (captchaTimer) clearInterval(captchaTimer)
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', onBgResize)
})

// ===== 背景网络粒子动画（节点漂移 + 近距连线，呼应网络拓扑主题） =====
const bgCanvas = ref<HTMLCanvasElement>()
let bgCtx: CanvasRenderingContext2D | null = null
let raf = 0
let parts: { x: number; y: number; vx: number; vy: number; r: number; c: string; a: number }[] = []

function initParticles() {
  onBgResize()
  window.addEventListener('resize', onBgResize)
}
function onBgResize() {
  const cv = bgCanvas.value
  if (!cv) return
  const dpr = window.devicePixelRatio || 1
  cv.width = window.innerWidth * dpr
  cv.height = window.innerHeight * dpr
  cv.style.width = window.innerWidth + 'px'
  cv.style.height = window.innerHeight + 'px'
  bgCtx = cv.getContext('2d')
  const dark = appStore.theme === 'dark'
  const palette = dark
    ? ['62,140,255', '138,99,255', '255,154,61', '61,210,192']
    : ['64,158,255', '138,75,255', '255,154,61', '47,198,189']
  const count = Math.min(80, Math.floor(window.innerWidth / 16))
  parts = []
  for (let i = 0; i < count; i++) {
    parts.push({
      x: Math.random() * cv.width,
      y: Math.random() * cv.height,
      vx: (Math.random() - 0.5) * 0.4 * dpr,
      vy: (Math.random() - 0.5) * 0.4 * dpr,
      r: (1 + Math.random() * 1.7) * dpr,
      c: palette[Math.floor(Math.random() * palette.length)],
      a: 0.25 + Math.random() * 0.4
    })
  }
  cancelAnimationFrame(raf)
  drawLoop()
}
function drawLoop() {
  const cv = bgCanvas.value
  const ctx = bgCtx
  if (!cv || !ctx) return
  ctx.clearRect(0, 0, cv.width, cv.height)
  const dpr = window.devicePixelRatio || 1
  const linkDist = 130 * dpr
  for (let i = 0; i < parts.length; i++) {
    const p = parts[i]
    for (let j = i + 1; j < parts.length; j++) {
      const q = parts[j]
      const dx = p.x - q.x
      const dy = p.y - q.y
      const d = Math.hypot(dx, dy)
      if (d < linkDist) {
        ctx.strokeStyle = `rgba(${p.c},${(1 - d / linkDist) * 0.22})`
        ctx.lineWidth = 1 * dpr
        ctx.beginPath()
        ctx.moveTo(p.x, p.y)
        ctx.lineTo(q.x, q.y)
        ctx.stroke()
      }
    }
  }
  for (const p of parts) {
    p.x += p.vx
    p.y += p.vy
    if (p.x < 0 || p.x > cv.width) p.vx *= -1
    if (p.y < 0 || p.y > cv.height) p.vy *= -1
    ctx.beginPath()
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
    ctx.fillStyle = `rgba(${p.c},${p.a})`
    ctx.fill()
  }
  raf = requestAnimationFrame(drawLoop)
}
</script>

<template>
  <div class="np-login">
    <canvas ref="bgCanvas" class="np-login-bg"></canvas>
    <div class="np-login-card">
      <div class="np-login-head">
        <div class="np-login-logo">
          <span class="np-login-mark">
            <el-icon :size="24"><Monitor /></el-icon>
          </span>
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
        <el-form-item :label="t('login.captcha')">
          <div class="np-login-captcha">
            <el-input v-model="form.captchaCode" size="large" :placeholder="t('login.captchaPlaceholder')" maxlength="4" @keyup.enter="onLogin" />
            <img v-if="captchaImg" class="np-login-captcha-img" :src="captchaImg" alt="captcha" :title="t('login.captchaRefresh')" @click="refreshCaptcha" />
            <div v-else class="np-login-captcha-fallback" @click="refreshCaptcha">
              {{ captchaFail ? t('login.captchaLoadFail') : t('login.captchaLoading') }}
            </div>
          </div>
          <div v-if="captchaLeft > 0" class="np-login-captcha-tip">
            {{ t('login.captchaExpireTip') }} {{ captchaLeft }}s
          </div>
          <div v-else-if="captchaFail" class="np-login-captcha-tip is-danger">
            {{ t('login.captchaLoadFail') }}，{{ t('login.captchaClickRetry') }}
          </div>
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
