import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import * as Icons from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useAppStore } from './stores/app'
import '@/styles/index.scss'

const app = createApp(App)

// 全局注册图标
for (const [name, comp] of Object.entries(Icons)) {
  app.component(name, comp as never)
}

app.use(createPinia())

// 应用主题与语言
const appStore = useAppStore()
appStore.applyTheme()

app.use(i18n)
app.use(router)
app.use(ElementPlus, { locale: appStore.lang === 'zh-CN' ? zhCn : en })

app.mount('#app')
