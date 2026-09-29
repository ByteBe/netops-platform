import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
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
        manualChunks: {
          echarts: ['echarts'],
          three: ['three'],
          vendor: ['vue', 'vue-router', 'pinia', 'element-plus', 'axios', 'vue-i18n']
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


