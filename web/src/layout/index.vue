<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessageBox, ElMessage } from 'element-plus'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { navMeta } from '@/router/nav'

const route = useRoute()
const router = useRouter()
function goDashboard() { router.push('/dashboard').catch((e: any) => console.error('nav dashboard error', e)) }
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

// 功能路由分组（左侧导航）
const groups: { titleKey: string; items: string[] }[] = [
  { titleKey: 'group_monitor', items: ['topology', 'linkdetect', 'monitor', 'traffic', 'bgpmon'] },
  { titleKey: 'group_alert', items: ['alert'] },
  { titleKey: 'group_asset', items: ['resource', 'ipam', 'subnet'] },
  { titleKey: 'group_infra', items: ['container', 'k8s', 'dbmonitor', 'nginxmon'] },
  { titleKey: 'group_ops', items: ['report', 'scriptgen', 'devicelog', 'aiinspect'] },
  { titleKey: 'group_system', items: ['system', 'distributed', 'dbmigrate'] }
]
const menus = computed(() => {
  return groups.map(g => ({
    title: t('nav.' + g.titleKey),
    items: g.items
      .filter(n => navMeta[n] && !navMeta[n].hidden)
      .map(n => ({ name: n, title: t(navMeta[n].title), icon: navMeta[n].icon }))
  })).filter(g => g.items.length > 0)
})

function logout() {
  ElMessageBox.confirm(t('common.confirm') + '?', t('login.logout'), { type: 'warning' })
    .then(async () => {
      authStore.logout()
      ElMessage.success(t('common.success'))
      router.push('/login')
    })
    .catch(() => {})
}

const userTitle = computed(() => authStore.user?.username || '')

// 菜单图标主题色（彩色渐变底，每项独立配色）
const iconThemes: Record<string, [string, string]> = {
  topology: ['#4db8ff', '#1270e0'], // 网络拓扑 蓝
  linkdetect: ['#ffb84d', '#ef7b0a'], // 链路检测 橙
  monitor: ['#9f7bff', '#5b3ff0'], // 设备监控 紫
  traffic: ['#3fe0c0', '#0aa5a5'], // 流量监控 青
  bgpmon: ['#6ee06a', '#1e9e3a'], // BGP 绿
  alert: ['#ff6b6b', '#d92626'], // 告警中心 红
  resource: ['#5ab8ff', '#2b6fd4'], // 资源资产 蓝
  ipam: ['#d39aff', '#7a3fe0'], // IP 管理 紫
  subnet: ['#6ea8ff', '#2958c8'], // 子网 靛蓝
  container: ['#55c7f0', '#167fa8'], // 容器 青蓝
  k8s: ['#7ba7ff', '#3556c4'], // K8s 蓝
  dbmonitor: ['#ffa75a', '#e0601a'], // 数据库监控 橙
  nginxmon: ['#77dd7a', '#1f9c3a'], // Nginx 绿
  report: ['#7f9bff', '#3f52d8'], // 巡检报告 蓝
  scriptgen: ['#ff7ba9', '#d92e7a'], // 脚本生成 玫红
  devicelog: ['#ffcf5a', '#e09a1a'], // 设备日志 琥珀
  aiinspect: ['#9a7bff', '#4f37c8'], // AI 巡检 紫
  system: ['#8ab6ff', '#4a76c8'], // 系统管理 蓝灰
  distributed: ['#ffd25a', '#e09c1a'], // 节点证书 金
  dbmigrate: ['#38d0e0', '#0f8fa8'], // 数据库迁移 青
  alertcenter: ['#ff6b6b', '#d92626']
}
function miBg(name: string) {
  const [a, b] = iconThemes[name] || ['#9fb2c8', '#5a7ba8']
  return `linear-gradient(145deg, ${a} 0%, ${b} 100%)`
}
function miShadow(name: string) {
  const [, b] = iconThemes[name] || ['#9fb2c8', '#5a7ba8']
  return `0 2px 6px rgba(0,0,0,.18), 0 4px 10px ${b}55, inset 0 1px 0 rgba(255,255,255,.45)`
}
</script>

<template>
  <div class="np-layout">
    <!-- 左侧导航栏 -->
    <aside class="np-aside" :class="{ collapsed: appStore.collapsed }">
      <div class="np-logo">
        <span class="np-logo-mark">
          <svg viewBox="0 0 32 32" width="26" height="26" aria-hidden="true">
            <defs>
              <linearGradient id="npN1" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0%" stop-color="#6bdcff" />
                <stop offset="48%" stop-color="#3f8bff" />
                <stop offset="100%" stop-color="#8a4bff" />
              </linearGradient>
              <linearGradient id="npN2" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="#2f55e0" />
                <stop offset="100%" stop-color="#341b96" />
              </linearGradient>
              <radialGradient id="npB1" cx="0.35" cy="0.3" r="0.95">
                <stop offset="0%" stop-color="#ffd48a" />
                <stop offset="55%" stop-color="#ff9a3d" />
                <stop offset="100%" stop-color="#e26a0a" />
              </radialGradient>
              <radialGradient id="npB2" cx="0.35" cy="0.3" r="0.95">
                <stop offset="0%" stop-color="#8ff0e8" />
                <stop offset="55%" stop-color="#2fc6bd" />
                <stop offset="100%" stop-color="#0a8f96" />
              </radialGradient>
              <radialGradient id="npB3" cx="0.35" cy="0.3" r="0.95">
                <stop offset="0%" stop-color="#9ff0a8" />
                <stop offset="55%" stop-color="#4cc45f" />
                <stop offset="100%" stop-color="#1d8f33" />
              </radialGradient>
            </defs>
            <!-- 底部投影 -->
            <ellipse cx="16.5" cy="28.2" rx="13.5" ry="2" fill="rgba(30,50,90,.16)" />
            <!-- 立体 N：厚度层 -->
            <path d="M9.5 25.5 L9.5 8.5 L24.5 25.5 L24.5 8.5" stroke="url(#npN2)" stroke-width="7.6" fill="none" stroke-linecap="round" stroke-linejoin="round" transform="translate(0 3)" />
            <!-- 立体 N：主体渐变 -->
            <path d="M9.5 25.5 L9.5 8.5 L24.5 25.5 L24.5 8.5" stroke="url(#npN1)" stroke-width="7.6" fill="none" stroke-linecap="round" stroke-linejoin="round" />
            <!-- 顶面高光 -->
            <path d="M9.5 25.5 L9.5 8.5 L24.5 25.5 L24.5 8.5" stroke="rgba(255,255,255,.6)" stroke-width="1.8" fill="none" stroke-linecap="round" stroke-linejoin="round" transform="translate(0 -1.7)" />
            <!-- 网络节点球（橙/青/绿） -->
            <circle cx="27.5" cy="6.8" r="3" fill="url(#npB1)" />
            <circle cx="26.8" cy="6.1" r="1" fill="rgba(255,255,255,.85)" />
            <circle cx="4.6" cy="7.4" r="2.6" fill="url(#npB2)" />
            <circle cx="4" cy="6.8" r="0.9" fill="rgba(255,255,255,.85)" />
            <circle cx="27.5" cy="20.5" r="2.4" fill="url(#npB3)" />
            <circle cx="27" cy="19.9" r="0.8" fill="rgba(255,255,255,.85)" />
          </svg>
        </span>
        <span v-show="!appStore.collapsed" class="np-logo-text">{{ t('app.shortName') }}</span>
      </div>
      <el-menu
        class="np-menu"
        :default-active="route.path"
        :collapse="appStore.collapsed"
        :collapse-transition="false"
        router
      >
        <el-sub-menu v-for="g in menus" :key="g.title" :index="g.title">
          <template #title>
            <span>{{ g.title }}</span>
          </template>
          <el-menu-item v-for="m in g.items" :key="m.name" :index="'/' + m.name">
            <span class="np-mi" :style="{ background: miBg(m.name), boxShadow: miShadow(m.name) }">
              <el-icon :size="16"><component :is="m.icon" /></el-icon>
            </span>
            <template #title>{{ m.title }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <!-- 主区域 -->
    <div class="np-main">
      <header class="np-header">
        <div class="np-header-left">
          <el-icon class="np-collapse-btn" @click="appStore.toggleCollapsed()">
            <Expand v-if="appStore.collapsed" />
            <Fold v-else />
          </el-icon>
          <span class="np-page-title">{{ t(navMeta[route.name as string]?.title ?? '') || (route.name as string) }}</span>
        </div>
        <div class="np-header-right">
          <!-- 数据大屏入口：固定在右上角，左侧导航不设入口 -->
          <el-button type="primary" class="np-dashboard-entry" @click="goDashboard">
            <el-icon><DataBoard /></el-icon>
            <span>{{ t('dashboard.entry') }}</span>
          </el-button>

          <el-dropdown trigger="click" @command="(cmd: string) => appStore.setLang(cmd)">
            <el-icon class="np-header-icon np-hdr-lang"><Switch /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="zh-CN">中文</el-dropdown-item>
                <el-dropdown-item command="en-US">English</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-icon class="np-header-icon np-hdr-theme" @click="appStore.toggleTheme()">
            <Moon v-if="appStore.theme === 'light'" />
            <Sunny v-else />
          </el-icon>

          <el-dropdown trigger="click" @command="(cmd: string) => cmd === 'logout' ? logout() : router.push('/profile')">
            <span class="np-user">
              <el-avatar :size="26" class="np-avatar">{{ userTitle.slice(0, 1).toUpperCase() }}</el-avatar>
              <span class="np-username">{{ userTitle }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">{{ t('nav.profile') }}</el-dropdown-item>
                <el-dropdown-item command="logout" divided>{{ t('login.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="np-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style lang="scss">
.np-layout {
  display: flex;
  height: 100%;
  width: 100%;
}

.np-aside {
  width: $sidebar-width;
  min-width: $sidebar-width;
  background: var(--np-sidebar-bg);
  border-right: 1px solid var(--np-border);
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease-out, min-width 0.2s ease-out;
  &.collapsed {
    width: 64px;
    min-width: 64px;
  }
}

.np-logo {
  height: $header-height;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  padding: 0 16px;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: var(--np-text-1);
  border-bottom: 1px solid var(--np-border);

  .np-logo-mark {
    flex: none;
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 9px;
    background: var(--np-surface-2);
    border: 1px solid var(--np-border-strong);
    box-shadow: var(--np-shadow-sm);
  }

  .np-logo-text {
    @include ellipsis;
  }
}

.np-aside.collapsed .np-logo {
  justify-content: center;
  padding: 0;
}

.np-menu {
  flex: 1;
  border-right: none;
  background: transparent;
  overflow-y: auto;
  padding: 10px 10px 16px;

  .el-menu-item,
  .el-sub-menu__title {
    border-radius: $radius-md;
    height: 42px;
    margin: 2px 0;
  }

  .el-menu-item.is-active {
    box-shadow: inset 3px 0 0 var(--np-primary);
  }

  // 彩色立体图标
  .np-mi {
    width: 26px;
    height: 26px;
    border-radius: 7px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    flex: none;
    margin-right: 8px;
    transition: transform 0.18s ease-out, box-shadow 0.18s ease-out;
  }
  .el-menu-item:hover .np-mi {
    transform: translateY(-1px) scale(1.06);
  }
  .el-menu-item.is-active .np-mi {
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.5), 0 0 0 2px rgba(255, 255, 255, 0.35);
  }
}

.np-aside.collapsed .np-menu .np-mi {
  margin-right: 0;
}

.np-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.np-header {
  height: $header-height;
  background: var(--np-header-bg);
  border-bottom: 1px solid var(--np-border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  z-index: 10;

  &-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  &-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 10px;
  }
}

.np-collapse-btn,
.np-header-icon {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: $radius-md;
  font-size: 19px;
  cursor: pointer;
  color: var(--np-text-2);
  transition: background-color 0.18s ease-out, color 0.18s ease-out, filter 0.18s ease-out;
  &:hover {
    color: var(--np-text-1);
    background: var(--np-hover);
  }
}

/* 中英文切换与亮暗切换：彩色渐变立体球形图标 */
.np-header-icon.np-hdr-lang {
  border-radius: 50%;
  background: linear-gradient(145deg, #4db8ff 0%, #1270e0 100%);
  color: #fff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, .18), 0 4px 10px rgba(18, 112, 224, .45), inset 0 1px 0 rgba(255, 255, 255, .45);
  &:hover { color: #fff; background: linear-gradient(145deg, #57c0ff 0%, #1a7cf0 100%); }
}
.np-header-icon.np-hdr-theme {
  border-radius: 50%;
  background: linear-gradient(145deg, #9f7bff 0%, #5b3ff0 100%);
  color: #fff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, .18), 0 4px 10px rgba(91, 63, 240, .45), inset 0 1px 0 rgba(255, 255, 255, .45);
  &:hover { color: #fff; background: linear-gradient(145deg, #ac8bff 0%, #6a4cf5 100%); }
}

.np-page-title {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.3px;
  color: var(--np-text-1);
}

.np-dashboard-entry {
  font-weight: 600;
}

.np-user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 12px 4px 5px;
  border-radius: $radius-md;
  background: var(--np-surface-1);
  border: 1px solid var(--np-border);
  transition: background-color 0.18s ease-out, border-color 0.18s ease-out;
  &:hover {
    background: var(--np-hover);
    border-color: var(--np-border-strong);
  }
  .np-avatar {
    background: var(--np-primary);
    color: var(--np-primary-fg);
    font-weight: 600;
    flex: none;
  }
  .np-username {
    font-size: 14px;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.np-content {
  flex: 1;
  overflow-y: auto;
}
</style>
