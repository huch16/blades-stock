import {createApp} from 'vue'
import {createPinia} from 'pinia'
import naive from 'naive-ui'
import App from './App.vue'
import router from './router/router'
import { initUniversalBridge } from './api/bridge'
import { installStaleChunkGuard } from './utils/staleChunkGuard'

// 部署热更新韧性守卫：必须在任何动态 import() 发生之前注册，
// 否则用户点击懒加载路由时旧 chunk 404 将直接白屏（早于 ErrorBoundary 挂载）。
installStaleChunkGuard();

// 初始化通用通信适配层（抹平 Wails 桌面与 Web 浏览器运行环境）
initUniversalBridge();

// 抑制无害的 ResizeObserver 警告（WebView2 在窗口创建时的常见产物）
;(function() {
  const origError = console.error
  console.error = function(...args) {
    if (args.some(a => typeof a === 'string' && a.includes('ResizeObserver'))) return
    return origError.apply(this, args)
  }
})()

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)

app.config.errorHandler = (err) => {
  if (err && err.message && err.message.includes('ResizeObserver')) {
    return
  }
  console.error(err)
}

window.addEventListener('error', (event) => {
  if (event.message && event.message.includes('ResizeObserver')) {
    event.preventDefault()
    return true
  }
})

window.addEventListener('unhandledrejection', (event) => {
  if (event.reason && event.reason.message && event.reason.message.includes('ResizeObserver')) {
    event.preventDefault()
    return true
  }
})

app.use(router)
app.use(naive)
app.mount('#app')