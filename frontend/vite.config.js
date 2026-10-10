import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite';
import Components from 'unplugin-vue-components/vite';
import { TDesignResolver } from '@tdesign-vue-next/auto-import-resolver';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
      vue(),
      AutoImport({
          resolvers: [TDesignResolver({
              library: 'chat'
          })],
      }),
      Components({
          resolvers: [TDesignResolver({
              library: 'chat'
          })],
      }),
  ],
  build: {
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          // 1. 图表可视化类 (按需分离)
          if (id.includes('/echarts/')) return 'vendor-echarts'
          if (id.includes('lightweight-charts')) return 'vendor-charts'
          
          // 2. Markdown 与 CodeMirror 编辑器 (统一打包消除循环引用)
          if (id.includes('md-editor-v3') || id.includes('@vavt') || id.includes('codemirror') || id.includes('@codemirror') || id.includes('@lezer')) {
            return 'vendor-markdown'
          }
          
          // 3. PDF 与截图导出工具
          if (id.includes('html2canvas') || id.includes('file-saver')) return 'vendor-export'
          
          // 4. 图标库 (独立分包，避免膨胀核心 UI)
          if (id.includes('@vicons')) return 'vendor-icons'
          if (id.includes('tdesign-icons')) return 'vendor-icons'
          
          // 5. 对话组件库
          if (id.includes('tdesign')) return 'vendor-tdesign'
          
          // 6. UI 与响应式底座 (Vue + Naive UI 统一绑定，消除宿主循环引用)
          if (id.includes('/naive-ui/') || id.includes('/vue') || id.includes('vue-router') || id.includes('pinia')) {
            return 'vendor-ui'
          }
          
          // 8. 基础高频工具库
          if (id.includes('lodash') || id.includes('date-fns')) {
            return 'vendor-utils'
          }
        }
      }
    }
  }
})
