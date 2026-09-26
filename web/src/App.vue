<script setup lang="ts">
import { watch } from 'vue'
import { useAppStore } from '@/stores/app'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { locale } = useI18n()

// 语言切换时同步 ElementPlus locale（通过 key 强制重建根节点）
watch(
  () => appStore.lang,
  (v) => {
    locale.value = v
  }
)
</script>

<template>
  <router-view :key="appStore.lang" />
</template>

<style lang="scss">
#app {
  height: 100%;
}
</style>
