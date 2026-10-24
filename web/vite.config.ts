import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import Icons from 'unplugin-icons/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import IconsResolver from 'unplugin-icons/resolver'

export default defineConfig({
  plugins: [
    vue(),
    // element-plus API 按需（ElMessage 等）
    AutoImport({
      resolvers: [ElementPlusResolver()],
      dts: 'src/auto-imports.d.ts'
    }),
    // element-plus 组件 + 图标按需（模板中 <el-*> / <Refresh /> 自动解析）
    Components({
      resolvers: [ElementPlusResolver(), IconsResolver({ enabledCollections: ['ep'] })],
      dts: 'src/components.d.ts'
    }),
    Icons({ autoInstall: true })
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@/styles/variables.scss" as *;`
      }
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        // Vite 8 (rolldown) 仅支持函数形式 manualChunks
        manualChunks(id) {
          if (id.includes('/node_modules/echarts/')) return 'echarts'
          if (id.includes('/node_modules/element-plus/') || id.includes('/node_modules/@element-plus/')) return 'element-plus'
          if (id.includes('/node_modules/')) return 'vendor'
          return undefined
        }
      }
    }
  },
  server: {
    host: '0.0.0.0',
    port: 30820,
    proxy: {
      '/api': { target: 'http://127.0.0.1:30821', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:30821', ws: true },
      '/mcp': { target: 'http://127.0.0.1:30821', changeOrigin: true }
    }
  }
})
