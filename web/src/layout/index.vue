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
const groups: { title: string; items: string[] }[] = [
  { title: '网络监控', items: ['linkdetect', 'monitor', 'topology', 'traffic'] },
  { title: '告警中心', items: ['alert'] },
  { title: '资源与资产', items: ['resource', 'ipam', 'subnet', 'configbackup'] },
  { title: '基础设施', items: ['container', 'k8s', 'dbmonitor'] },
  { title: '运维工具', items: ['report', 'scriptgen'] },
  { title: '系统管理', items: ['system', 'distributed', 'dbmigrate'] }
]
const menus = computed(() => {
  return groups.map(g => ({
    title: g.title,
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
</script>

<template>
  <div class="np-layout">
    <!-- 左侧导航栏 -->
    <aside class="np-aside" :class="{ collapsed: appStore.collapsed }">
      <div class="np-logo">
        <el-icon :size="22" color="#2f6bff"><Monitor /></el-icon>
        <span v-show="!appStore.collapsed">{{ t('app.shortName') }}</span>
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
            <el-icon><component :is="m.icon" /></el-icon>
            <template #title>{{ m.title }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <!-- 主区域 -->
    <div class="np-main">
      <header class="np-header">
        <div class="np-header-left">
          <el-icon class="np-collapse-btn" @click="appStore.collapsed = !appStore.collapsed">
            <Expand v-if="appStore.collapsed" />
            <Fold v-else />
          </el-icon>
          <span class="np-page-title">{{ t(navMeta[route.name as string]?.title ?? '') || (route.name as string) }}</span>
        </div>
        <div class="np-header-right">
          <!-- 数据大屏入口：固定在右上角，左侧导航不设入口 -->
          <el-button type="primary" round class="np-dashboard-entry" @click="goDashboard">
            <el-icon><DataBoard /></el-icon>
            <span>{{ t('dashboard.entry') }}</span>
          </el-button>

          <el-tooltip :content="appStore.lang === 'zh-CN' ? 'English' : '中文'" placement="bottom">
            <el-dropdown trigger="click" @command="(cmd: string) => appStore.setLang(cmd)">
              <el-icon class="np-header-icon"><Switch /></el-icon>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="zh-CN">中文</el-dropdown-item>
                  <el-dropdown-item command="en-US">English</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </el-tooltip>

          <el-tooltip :content="appStore.theme === 'light' ? t('theme.dark') : t('theme.light')" placement="bottom">
            <el-icon class="np-header-icon" @click="appStore.toggleTheme()">
              <Moon v-if="appStore.theme === 'light'" />
              <Sunny v-else />
            </el-icon>
          </el-tooltip>

          <el-dropdown trigger="click" @command="(cmd: string) => cmd === 'logout' ? logout() : router.push('/profile')">
            <span class="np-user">
              <el-avatar :size="28" class="np-avatar">{{ userTitle.slice(0, 1).toUpperCase() }}</el-avatar>
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
  background: var(--np-bg);
}

.np-aside {
  width: $sidebar-width;
  min-width: $sidebar-width;
  background: var(--np-sidebar-bg);
  border-right: 1px solid var(--np-border);
  display: flex;
  flex-direction: column;
  transition: width 0.2s, min-width 0.2s;
  &.collapsed {
    width: 64px;
    min-width: 64px;
  }
}

.np-logo {
  height: $header-height;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 17px;
  font-weight: 700;
  color: var(--np-text-1);
  border-bottom: 1px solid var(--np-border);
}

.np-menu {
  flex: 1;
  border-right: none;
  background: transparent;
  overflow-y: auto;
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
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--np-border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 16px;
  z-index: 10;

  &-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  &-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 16px;
  }
}

.np-collapse-btn {
  font-size: 20px;
  cursor: pointer;
  color: var(--np-text-2);
  &:hover {
    color: var(--np-primary);
  }
}

.np-page-title {
  font-size: 16px;
  font-weight: 600;
}

.np-dashboard-entry {
  background: linear-gradient(135deg, #2f6bff, #6d5bff);
  border: none;
  box-shadow: 0 4px 12px rgba(47, 107, 255, 0.35);
}

.np-header-icon {
  font-size: 18px;
  cursor: pointer;
  color: var(--np-text-2);
  &:hover {
    color: var(--np-primary);
  }
}

.np-user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  .np-avatar {
    background: var(--np-primary);
    color: #fff;
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
