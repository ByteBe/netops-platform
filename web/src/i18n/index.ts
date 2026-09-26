import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import enUS from './en-US'

const saved = localStorage.getItem('np-lang') || 'zh-CN'

export const i18n = createI18n({
  legacy: false,
  locale: saved,
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, 'en-US': enUS }
})

export function setLang(lang: string) {
  i18n.global.locale.value = lang as 'zh-CN' | 'en-US'
  localStorage.setItem('np-lang', lang)
  document.documentElement.lang = lang
}
