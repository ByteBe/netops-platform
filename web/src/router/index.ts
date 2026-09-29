// 路由自动注册：views/<模块>/index.vue 即生成 /<模块> 路由
// 新增页面只需在 views 下新建文件夹并放置 index.vue，无需修改路由表
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { navMeta } from './nav'

// 自动扫描所有视图（vite 约定式路由）
const modules = import.meta.glob('../views/**/index.vue')

const rootRoutes: RouteRecordRaw[] = []

for (const path in modules) {
  // ../views/dashboard/index.vue → dashboard
  const rel = path.replace('../views/', '').replace('/index.vue', '')
  const segments = rel.split('/')
  const name = segments[segments.length - 1]
  const meta = navMeta[name]
  if (name === 'login' || name === 'setup') {
    rootRoutes.push({
      path: `/${name}`,
      name: name === 'setup' ? 'setup' : 'login',
      component: modules[path],
      meta: { title: name === 'setup' ? '系统初始化' : '登录', hidden: true, requiresAuth: false }
    })
    continue
  }
  rootRoutes.push({
    path: `/${name}`,
    name,
    component: modules[path],
    meta: { title: meta?.title ?? name, icon: meta?.icon, hidden: meta?.hidden, requiresAuth: true }
  })
}

// 布局（含侧栏/顶栏），子路由为各功能模块（大屏独立，不在此布局内）
const Layout = () => import('@/layout/index.vue')
const featureRoutes = rootRoutes.filter((r) => r.meta?.requiresAuth && r.path !== '/dashboard')

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/login/index.vue'), meta: { requiresAuth: false } },
  { path: '/setup', name: 'setup', component: () => import('@/views/setup/index.vue'), meta: { requiresAuth: false } },
  {
    path: '/',
    component: Layout,
    redirect: '/linkdetect',
    children: featureRoutes
  },
  // 数据大屏：独立全屏路由（无侧边栏/顶栏），右上角入口进入
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/dashboard/index.vue'),
    meta: { requiresAuth: true, title: 'nav.dashboard', hidden: true }
  },
  { path: '/:pathMatch(.*)*', redirect: '/linkdetect' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 全局守卫：登录态 / 初始化态
let _setupChecked = false
let _initialized = true
router.beforeEach(async (to) => {
  const { useAuthStore } = await import('@/stores/auth')
  const auth = useAuthStore()
  const { getEnc } = await import('@/utils/request')

  // 检查系统是否已初始化（只查一次，用原生 fetch 避免拦截器干扰）
  if (!_setupChecked) {
    try {
      const r = await fetch('/api/v1/setup/status', {
        headers: {
          'X-Timestamp': String(Math.floor(Date.now() / 1000)),
          'X-Nonce': Math.random().toString(36).substring(2, 15) + Date.now().toString(36)
        }
      })
      const j = await r.json()
      _initialized = !!(j.data && j.data.initialized)
    } catch { _initialized = true }
    _setupChecked = true
  }
  if (!_initialized && to.path !== '/setup') {
    return { path: '/setup' }
  }
  if (_initialized && to.path === '/setup') {
    return { path: '/login' }
  }

  if (to.path === '/login' || to.path === '/setup') {
    return true
  }
  if (!auth.token) {
    return { path: '/login' }
  }
  // 拉取用户信息（含强制改密状态）
  if (!auth.user) {
    try {
      const me = await getEnc<Record<string, unknown>>('/auth/me')
      auth.setUser(me as never)
    } catch {
      return { path: '/login' }
    }
  }
  if (auth.user?.must_change_pwd && to.path !== '/login') {
    return { path: '/login', query: { force: '1' } }
  }
  return true
})

export default router
