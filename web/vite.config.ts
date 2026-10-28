import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [
    vue(),
    // Element Plus 按需引入：组件与 ElMessage 等工具样式随用随打，避免整包进 entry
    AutoImport({
      resolvers: [ElementPlusResolver()],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  base: '/',
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        // 仅固定框架层依赖：Element Plus 与 vditor 均按使用方自动懒加载，
        // 交由 Rollup 自动拆分，避免首页被迫加载全部组件与编辑器
        manualChunks(id: string) {
          if (!id.includes('node_modules')) return
          if (id.includes('/@vue/') || id.includes('/vue/') || id.includes('/vue-router/')) return 'vue'
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/media': 'http://127.0.0.1:8080',
    },
  },
})
