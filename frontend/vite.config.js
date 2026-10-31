import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite';
import Components from 'unplugin-vue-components/vite';
import { TDesignResolver } from '@tdesign-vue-next/auto-import-resolver';
import path from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
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
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          // 1. 图表可视化类 (按需分离)
          if (id.includes('/echarts/') || id.includes('/zrender/')) return 'vendor-echarts'
          if (id.includes('lightweight-charts')) return 'vendor-charts'
          
          // 2. 代码高亮与 CodeMirror 底层引擎
          if (id.includes('codemirror') || id.includes('@codemirror') || id.includes('@lezer')) {
            return 'vendor-codemirror'
          }

          // 3. Markdown 渲染与扩展库
          if (id.includes('md-editor-v3') || id.includes('@vavt')) {
            return 'vendor-markdown'
          }
          
          // 4. PDF 与截图导出工具
          if (id.includes('html2canvas') || id.includes('file-saver')) return 'vendor-export'
          
          // 5. 图标库 (独立分包，避免膨胀核心 UI)
          if (id.includes('@vicons') || id.includes('tdesign-icons')) return 'vendor-icons'
          
          // 6. 对话组件库
          if (id.includes('tdesign')) return 'vendor-tdesign'
          
          // 7. Naive UI 体系
          if (id.includes('/naive-ui/') || id.includes('seemly') || id.includes('vooks') || id.includes('evtd')) {
            return 'vendor-naive'
          }

          // 8. Vue 核心底座 (高频极速缓存)
          if (id.includes('/vue/') || id.includes('@vue/') || id.includes('vue-router') || id.includes('pinia')) {
            return 'vendor-vue-core'
          }
          
          // 9. 基础高频工具库
          if (id.includes('lodash') || id.includes('date-fns')) {
            return 'vendor-utils'
          }
        }
      }
    }
  }
})
