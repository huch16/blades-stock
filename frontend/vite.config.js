import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite';
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
      // 本地组件自动注册（src/components 目录扫描）。
      // 注意：此处**不得**再挂载任何 UI 库 resolver —— TDesign 已按 UI 纯净度门禁
      // (scripts/check_ui_purity.js) 彻底移除，全站统一使用 Naive UI (app.use(naive) 全局注册)。
      Components({
          dts: 'components.d.ts',
      }),
  ],
  build: {
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      onwarn(warning, warn) {
        // 将未定义标识符、未导出变量等警告直接升级为编译失败 (Build Blocker)
        if (/is not defined|is not exported|unresolved import/i.test(warning.message)) {
          throw new Error(`[BUILD-BLOCKER] ${warning.message}`);
        }
        warn(warning);
      },
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
          // 3.1 轻量 Markdown 解析内核：Agent 对话流 (ChatMarkdown) 与 md-editor-v3 共用。
          //     必须独立成 chunk —— 否则 Rollup 会把它并入 vendor-markdown (404KB br)，
          //     导致只用轻量管线的 Agent 路由被迫加载完整编辑器依赖。
          if (
            id.includes('/markdown-it/') ||
            id.includes('/linkify-it/') ||
            id.includes('/mdurl/') ||
            id.includes('/uc.micro/') ||
            id.includes('/punycode/')
          ) {
            return 'vendor-md-core'
          }
          // 3.2 重量级编辑器及其扩展 (仅 md-editor-v3 路由需要)
          if (id.includes('md-editor-v3') || id.includes('@vavt')) {
            return 'vendor-markdown'
          }
          
          // 4. PDF 与截图导出工具
          if (id.includes('html2canvas') || id.includes('file-saver')) return 'vendor-export'
          
          // 5. 图标库 (独立分包，避免膨胀核心 UI)
          if (id.includes('@vicons') || id.includes('tdesign-icons')) return 'vendor-icons'

          // 6. [已移除] 曾用于隔离 vendor-tdesign 双 UI 库税。
          //    @tdesign-vue-next/chat + tdesign-vue-next 已彻底从依赖树移除
          //    (产物 raw 1.17MB / br 355KB)，此处不再需要分包规则。

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
