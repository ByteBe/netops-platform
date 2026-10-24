import { createApp } from 'vue'
import { createPinia } from 'pinia'
// ElementPlus 按需引入：组件/样式/API 由 unplugin 自动导入，无需全量注册
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useAppStore } from './stores/app'
import '@/styles/index.scss'

const app = createApp(App)

app.use(createPinia())

// 应用主题与语言
const appStore = useAppStore()
appStore.applyTheme()

app.use(i18n)
app.use(router)

app.mount('#app')
