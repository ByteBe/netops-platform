// 应用全局状态：语言 / 主题 / 侧栏折叠
import { defineStore } from 'pinia'
import { i18n } from '@/i18n'

export const useAppStore = defineStore('app', {
  state: () => ({
    lang: localStorage.getItem('np-lang') || 'zh-CN',
    theme: (localStorage.getItem('np-theme') as 'light' | 'dark') || 'light',
    collapsed: false
  }),
  actions: {
    applyTheme() {
      const el = document.documentElement
      el.classList.toggle('dark', this.theme === 'dark')
      localStorage.setItem('np-theme', this.theme)
    },
    setTheme(theme: 'light' | 'dark') {
      this.theme = theme
      this.applyTheme()
    },
    toggleTheme() {
      this.setTheme(this.theme === 'light' ? 'dark' : 'light')
    },
    setLang(lang: string) {
      this.lang = lang
      localStorage.setItem('np-lang', lang)
      // 动态切换 vue-i18n 语言
      ;(i18n.global.locale as unknown as { value: string }).value = lang
      document.documentElement.lang = lang
    }
  }
})
