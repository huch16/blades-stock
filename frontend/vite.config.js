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
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('/echarts/')) return 'vendor-echarts'
          if (id.includes('md-editor-v3')) return 'vendor-md'
          if (id.includes('html2canvas')) return 'vendor-pdf'
          if (id.includes('lightweight-charts')) return 'vendor-charts'
          if (id.includes('/naive-ui/')) return 'vendor-ui'
          if (id.includes('/vue')) return 'vendor-ui'
          if (id.includes('pinia') || id.includes('lodash')) return 'vendor-ui'
          if (id.includes('tdesign')) return 'vendor-tdesign'
          if (id.includes('/@vavt/')) return 'vendor-cm'
        }
      }
    }
  }
})
