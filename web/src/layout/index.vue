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
</script>

<template>
  <div class="np-layout">
    <!-- 左侧导航栏 -->
    <aside class="np-aside" :class="{ collapsed: appStore.collapsed }">
      <div class="np-logo">
        <span class="np-logo-mark">
          <el-icon :size="18"><Monitor /></el-icon>
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
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    background: var(--np-primary);
    color: var(--np-primary-fg);
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
  transition: background-color 0.18s ease-out, color 0.18s ease-out;
  &:hover {
    color: var(--np-text-1);
    background: var(--np-hover);
  }
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
