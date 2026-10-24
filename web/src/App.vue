<script setup lang="ts">
import { computed, watch } from 'vue'
import { useAppStore } from '@/stores/app'
import { useI18n } from 'vue-i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const appStore = useAppStore()
const { locale } = useI18n()

// element-plus 组件库语言随应用语言切换
const elLocale = computed(() => (appStore.lang === 'zh-CN' ? zhCn : en))

// 语言切换时同步 vue-i18n locale
watch(
  () => appStore.lang,
  (v) => {
    locale.value = v
  }
)
</script>

<template>
  <el-config-provider :locale="elLocale">
    <router-view :key="appStore.lang" />
  </el-config-provider>
</template>

<style lang="scss">
#app {
  height: 100%;
}
</style>
