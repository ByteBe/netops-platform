import { createApp } from 'vue'
import { createPinia } from 'pinia'
// ElementPlus 按需引入：组件/样式/API 由 unplugin 自动导入，无需全量注册
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useAppStore } from './stores/app'
import '@/styles/index.scss'

// 页面模板中直接使用的 Element Plus 图标（裸标签 <Bell /> 等）。
// 按需模式下 unplugin IconsResolver 仅解析 i-ep- 前缀，裸标签需显式注册
// 此处仅注册实际用到的图标（tree-shake 按需打包，避免全量引入）。
import {
  Bell,
  BellFilled,
  Box,
  ChatDotRound,
  CircleCheck,
  CircleClose,
  Coin,
  Collection,
  Connection,
  Cpu,
  DataBoard,
  Document,
  Download,
  Expand,
  Fold,
  Folder,
  FolderOpened,
  Grid,
  InfoFilled,
  Key,
  Link,
  Lock,
  MagicStick,
  MapLocation,
  Monitor,
  Moon,
  Picture,
  Plus,
  Pointer,
  Position,
  Promotion,
  Refresh,
  Search,
  SetUp,
  Share,
  Ship,
  Sunny,
  Switch,
  TrendCharts,
  Upload,
  User,
  Warning,
  WarningFilled
} from '@element-plus/icons-vue'

const app = createApp(App)

app.use(createPinia())

// 应用主题与语言
const appStore = useAppStore()
appStore.applyTheme()

// 注册裸标签图标组件（供模板 <X /> 与动态字符串组件解析）
const usedIcons = {
  Bell, BellFilled, Box, ChatDotRound, CircleCheck, CircleClose,
  Coin, Collection, Connection, Cpu, DataBoard, Document, Download, Expand,
  Fold, Folder, FolderOpened, Grid, InfoFilled, Key, Link, Lock, MagicStick,
  MapLocation, Monitor, Moon, Picture, Plus, Pointer, Position,
  Promotion, Refresh, Search, SetUp, Share, Ship, Sunny, Switch, TrendCharts,
  Upload, User, Warning, WarningFilled
}
for (const [name, comp] of Object.entries(usedIcons)) {
  app.component(name, comp)
}

app.use(i18n)
app.use(router)

app.mount('#app')
