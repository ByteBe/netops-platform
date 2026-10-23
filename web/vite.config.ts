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
        // Vite 8 (rolldown) 仅支持函数形式 manualChunks
        manualChunks(id) {
          if (id.includes('/node_modules/echarts/')) return 'echarts'
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


