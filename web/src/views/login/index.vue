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
  } catch {
    captchaImg.value = ''
    captchaFail.value = true
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
      ElMessage.success(t('login.success'))
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
    ? ['62,140,255', '77,184,255', '47,198,189', '111,168,255']
    : ['64,158,255', '77,184,255', '47,198,189', '125,224,255']
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
    <div class="np-login-wrap">
      <!-- 全屏科技背景：齿轮 / 电路走线 / 光效圆环（一比一还原参考图） -->
      <div class="np-tech-bg">
        <svg viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
          <defs>
            <linearGradient id="ntBG" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stop-color="#081a40" />
              <stop offset="55%" stop-color="#0d2f6b" />
              <stop offset="100%" stop-color="#124a9e" />
            </linearGradient>
            <radialGradient id="ntGlow" cx="38%" cy="46%" r="60%">
              <stop offset="0%" stop-color="rgba(64,158,255,.35)" />
              <stop offset="55%" stop-color="rgba(64,158,255,.08)" />
              <stop offset="100%" stop-color="rgba(64,158,255,0)" />
            </radialGradient>
            <linearGradient id="ntLine" x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stop-color="#7de0ff" />
              <stop offset="100%" stop-color="#2f7bd9" />
            </linearGradient>
            <filter id="ntF" x="-60%" y="-60%" width="220%" height="220%">
              <feGaussianBlur stdDeviation="4" result="b" />
              <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
            </filter>
            <!-- 立体化：金属渐变 / 齿面渐变 / 浮雕投影 -->
            <radialGradient id="npGearMetal" cx="35%" cy="35%" r="75%">
              <stop offset="0%" stop-color="rgba(205,235,255,.5)" />
              <stop offset="45%" stop-color="rgba(120,180,255,.18)" />
              <stop offset="100%" stop-color="rgba(48,98,200,.45)" />
            </radialGradient>
            <linearGradient id="npGearTooth" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="rgba(255,255,255,.42)" />
              <stop offset="100%" stop-color="rgba(150,205,255,.16)" />
            </linearGradient>
            <filter id="npGearShade" x="-30%" y="-30%" width="160%" height="160%">
              <feDropShadow dx="2.5" dy="4" stdDeviation="3.5" flood-color="rgba(2,10,30,.55)" />
            </filter>
          </defs>
          <rect width="1440" height="900" fill="url(#ntBG)" />
          <rect width="1440" height="900" fill="url(#ntGlow)" />

          <!-- 光效大圆环（左） -->
          <g fill="none" filter="url(#ntF)">
            <circle cx="300" cy="560" r="320" stroke="url(#ntLine)" stroke-opacity=".28" stroke-width="1.6" />
            <circle cx="300" cy="560" r="272" stroke="rgba(125,224,255,.3)" stroke-width="1.4" />
            <circle cx="300" cy="560" r="96" stroke="rgba(180,235,255,.7)" stroke-width="1.8" />
            <path d="M204 560 A96 96 0 0 1 300 464" stroke="rgba(190,245,255,.95)" stroke-width="2.8" />
            <circle cx="300" cy="560" r="6" fill="#7de0ff" />
          </g>
          <!-- 光效小环（右上/右下） -->
          <g fill="none" filter="url(#ntF)">
            <circle cx="1180" cy="180" r="120" stroke="rgba(125,224,255,.18)" stroke-width="1.2" />
            <circle cx="1180" cy="180" r="42" stroke="rgba(125,224,255,.45)" stroke-width="1.4" />
            <circle cx="1180" cy="180" r="5" fill="#7de0ff" />
            <circle cx="1310" cy="770" r="96" stroke="rgba(125,224,255,.16)" stroke-width="1.2" />
            <circle cx="1310" cy="770" r="34" stroke="rgba(125,224,255,.4)" stroke-width="1.3" />
            <circle cx="1310" cy="770" r="5" fill="#7de0ff" />
          </g>

                              <!-- 齿轮组（8 齿轮，错速旋转 + 金属立体质感） -->
          <g transform="translate(250 620)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="36s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(30.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(60.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(90.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(120.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(150.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(210.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(240.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(270.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(300.0)" />
              <rect x="-8" y="-46" width="16" height="15" rx="3" transform="rotate(330.0)" />
            </g>
            <circle r="46" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(0)" />
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(60)" />
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(120)" />
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(180)" />
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(240)" />
              <line x1="0" y1="-22" x2="0" y2="-32" transform="rotate(300)" />
            </g>
            <circle r="32" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-22" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-22" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-22" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-22" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-22" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-22" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="9" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(1060 230)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="28s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(30.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(60.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(90.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(120.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(150.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(210.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(240.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(270.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(300.0)" />
              <rect x="-8" y="-52" width="16" height="15" rx="3" transform="rotate(330.0)" />
            </g>
            <circle r="52" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(0)" />
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(60)" />
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(120)" />
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(180)" />
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(240)" />
              <line x1="0" y1="-25" x2="0" y2="-36" transform="rotate(300)" />
            </g>
            <circle r="36" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-25" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-25" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-25" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-25" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-25" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-25" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="11" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(1270 620)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="44s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(30.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(60.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(90.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(120.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(150.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(210.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(240.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(270.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(300.0)" />
              <rect x="-8" y="-60" width="16" height="15" rx="3" transform="rotate(330.0)" />
            </g>
            <circle r="60" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(0)" />
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(60)" />
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(120)" />
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(180)" />
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(240)" />
              <line x1="0" y1="-29" x2="0" y2="-42" transform="rotate(300)" />
            </g>
            <circle r="42" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-29" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-29" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-29" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-29" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-29" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-29" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="13" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(150 770)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="22s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(36.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(72.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(108.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(144.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(216.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(252.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(288.0)" />
              <rect x="-8" y="-30" width="16" height="15" rx="3" transform="rotate(324.0)" />
            </g>
            <circle r="30" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(0)" />
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(60)" />
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(120)" />
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(180)" />
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(240)" />
              <line x1="0" y1="-14" x2="0" y2="-20" transform="rotate(300)" />
            </g>
            <circle r="20" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-14" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-14" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-14" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-14" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-14" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-14" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="6" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(600 130)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="30s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(36.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(72.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(108.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(144.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(216.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(252.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(288.0)" />
              <rect x="-8" y="-27" width="16" height="15" rx="3" transform="rotate(324.0)" />
            </g>
            <circle r="27" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(0)" />
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(60)" />
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(120)" />
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(180)" />
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(240)" />
              <line x1="0" y1="-12" x2="0" y2="-18" transform="rotate(300)" />
            </g>
            <circle r="18" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-12" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-12" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-12" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-12" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-12" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-12" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="5" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(760 350)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="20s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(36.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(72.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(108.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(144.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(216.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(252.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(288.0)" />
              <rect x="-8" y="-24" width="16" height="15" rx="3" transform="rotate(324.0)" />
            </g>
            <circle r="24" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(0)" />
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(60)" />
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(120)" />
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(180)" />
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(240)" />
              <line x1="0" y1="-11" x2="0" y2="-16" transform="rotate(300)" />
            </g>
            <circle r="16" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-11" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-11" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-11" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-11" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-11" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-11" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="4" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(1010 500)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="32s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(36.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(72.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(108.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(144.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(216.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(252.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(288.0)" />
              <rect x="-8" y="-38" width="16" height="15" rx="3" transform="rotate(324.0)" />
            </g>
            <circle r="38" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(0)" />
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(60)" />
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(120)" />
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(180)" />
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(240)" />
              <line x1="0" y1="-18" x2="0" y2="-26" transform="rotate(300)" />
            </g>
            <circle r="26" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-18" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-18" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-18" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-18" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-18" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-18" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="8" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

          <g transform="translate(460 780)" class="np-gear" filter="url(#npGearShade)"><animateTransform attributeName="transform" type="rotate" from="0 0 0" to="360 0 0" dur="18s" repeatCount="indefinite" additive="sum"/>
            <g fill="url(#npGearTooth)">
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(0.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(36.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(72.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(108.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(144.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(180.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(216.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(252.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(288.0)" />
              <rect x="-8" y="-20" width="16" height="15" rx="3" transform="rotate(324.0)" />
            </g>
            <circle r="20" fill="url(#npGearMetal)" stroke="rgba(190,230,255,.45)" stroke-width="1.6" />
            <g stroke="rgba(160,215,255,.35)" stroke-width="1">
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(0)" />
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(60)" />
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(120)" />
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(180)" />
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(240)" />
              <line x1="0" y1="-9" x2="0" y2="-13" transform="rotate(300)" />
            </g>
            <circle r="13" fill="none" stroke="rgba(170,220,255,.4)" stroke-width="1" stroke-dasharray="4 5" />
            <g fill="rgba(190,230,255,.7)">
              <circle cx="0" cy="-9" r="1.8" transform="rotate(0)" />
              <circle cx="0" cy="-9" r="1.8" transform="rotate(60)" />
              <circle cx="0" cy="-9" r="1.8" transform="rotate(120)" />
              <circle cx="0" cy="-9" r="1.8" transform="rotate(180)" />
              <circle cx="0" cy="-9" r="1.8" transform="rotate(240)" />
              <circle cx="0" cy="-9" r="1.8" transform="rotate(300)" />
            </g>
            <circle r="4" fill="rgba(180,225,255,.75)" />
            <circle r="3.5" fill="#ffffff" opacity=".9" />
          </g>

<!-- 齿轮传动链（虚线连接） -->
          <g fill="none" stroke="rgba(125,224,255,.28)" stroke-width="1.2" stroke-dasharray="3 6">
            <path d="M250 620 L150 770" />
            <path d="M250 620 L460 780" />
            <path d="M1060 230 L1270 620" />
            <path d="M760 350 L1010 500" />
          </g>
<!-- 电路走线（折线 + 节点 + 端点块） -->
          <g fill="none" stroke="url(#ntLine)" stroke-width="1.6" opacity=".55">
            <path d="M60 520 L210 520 L210 600 L300 600" />
            <path d="M90 420 L190 420 L190 520" />
            <path d="M520 90 L520 190 L660 190" />
            <path d="M1120 420 L1120 520 L1000 520 L1000 640" />
            <path d="M1180 640 L1330 640 L1330 760" />
            <path d="M420 780 L560 780 L560 860" />
          </g>
          <g fill="#7de0ff" opacity=".8">
            <circle cx="210" cy="600" r="3.5" />
            <circle cx="300" cy="600" r="3.5" />
            <circle cx="190" cy="520" r="3.5" />
            <circle cx="660" cy="190" r="3.5" />
            <circle cx="1000" cy="640" r="3.5" />
            <circle cx="1330" cy="760" r="3.5" />
            <circle cx="560" cy="860" r="3.5" />
          </g>
          <g fill="rgba(125,224,255,.75)">
            <rect x="200" y="512" width="20" height="16" rx="3" />
            <rect x="350" y="412" width="20" height="16" rx="3" />
            <rect x="180" y="512" width="20" height="16" rx="3" />
            <rect x="990" y="632" width="20" height="16" rx="3" />
            <rect x="1320" y="752" width="20" height="16" rx="3" />
          </g>
          <!-- 星群散点：填补大空白，营造星空/数据节点感 -->
          <g fill="rgba(255,255,255,.35)">
            <circle cx="760" cy="700" r="2.2" />
            <circle cx="820" cy="740" r="1.6" />
            <circle cx="880" cy="780" r="2" />
            <circle cx="940" cy="700" r="1.8" />
            <circle cx="1000" cy="640" r="1.5" />
            <circle cx="700" cy="780" r="1.8" />
            <circle cx="76" cy="560" r="2.2" />
            <circle cx="160" cy="640" r="1.6" />
            <circle cx="90" cy="690" r="2" />
            <circle cx="420" cy="560" r="1.8" />
            <circle cx="520" cy="620" r="2.2" />
            <circle cx="560" cy="480" r="1.6" />
            <circle cx="640" cy="360" r="2" />
            <circle cx="700" cy="700" r="1.8" />
            <circle cx="860" cy="120" r="2.2" />
            <circle cx="960" cy="220" r="1.6" />
            <circle cx="900" cy="820" r="2" />
            <circle cx="1040" cy="760" r="1.8" />
          </g>
          <!-- 数据流弧线：向左汇聚，带虚线流动 -->
          <g fill="none" stroke="rgba(125,224,255,.5)" stroke-width="1.5" class="np-flow" opacity=".8">
            <path d="M120 220 C 260 300, 420 330, 560 400" />
            <path d="M90 470 C 250 500, 420 520, 560 540" />
            <path d="M140 700 C 280 660, 420 640, 560 620" />
          </g>
          <!-- 扫描脉冲环：大环中心扩散 -->
          <g fill="none" stroke="rgba(125,224,255,.6)" stroke-width="1.6">
            <circle cx="300" cy="560" r="60">
              <animate attributeName="r" values="40;110" dur="3.2s" repeatCount="indefinite" />
              <animate attributeName="opacity" values=".7;0" dur="3.2s" repeatCount="indefinite" />
            </circle>
            <circle cx="300" cy="560" r="60">
              <animate attributeName="r" values="40;110" dur="3.2s" begin="1.6s" repeatCount="indefinite" />
              <animate attributeName="opacity" values=".7;0" dur="3.2s" begin="1.6s" repeatCount="indefinite" />
            </circle>
          </g>
          <!-- 底部横线光带 -->
          <line x1="0" y1="880" x2="1440" y2="880" stroke="rgba(125,224,255,.3)" stroke-width="1" />
          <line x1="0" y1="870" x2="1440" y2="870" stroke="rgba(64,158,255,.18)" stroke-width="1" />
        </svg>
      </div>

      <!-- 左侧品牌区（宽屏展示） -->
      <div class="np-login-brand">
        <h2 class="np-brand-title">{{ t('login.introTitle') }}</h2>
        <p class="np-brand-desc">{{ t('login.introDesc') }}</p>
        <div class="np-brand-feats">
          <span class="np-feat"><el-icon><Connection /></el-icon>{{ t('login.feat1') }}</span>
          <span class="np-feat"><el-icon><Cpu /></el-icon>{{ t('login.feat2') }}</span>
          <span class="np-feat"><el-icon><Bell /></el-icon>{{ t('login.feat3') }}</span>
          <span class="np-feat"><el-icon><DataBoard /></el-icon>{{ t('login.feat4') }}</span>
        </div>
      </div>

      <!-- 右侧登录卡片 -->
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
          <div v-if="captchaFail" class="np-login-captcha-tip is-danger">
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
  </div>
</template>
