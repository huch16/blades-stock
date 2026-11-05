import {createRouter, createWebHashHistory} from 'vue-router'
import { handleStaleChunkFailure } from '../utils/staleChunkGuard'

import stockView from '../components/stock.vue'

// 非首页路由采用动态懒加载，大幅降低冷启动首屏 AST 解析时间与内存常驻
const fundView = () => import("../components/fund.vue")
const settingsView = () => import('../components/settings.vue')
const marketView = () => import("../components/market.vue")
const marketIntelligenceView = () => import("../components/MarketIntelligenceView.vue")
const agentChat = () => import("../components/agent-chat.vue")
const research = () => import("../components/researchIndex.vue")
const cronTaskManager = () => import("../components/cron-task-manager.vue")
const klineAnalysis = () => import("../components/kline-analysis.vue")
const vectorBTPanel = () => import("../components/VectorBTPanel.vue")
const auditView = () => import('../views/admin/Audit.vue')

const routes = [
    { path: '/', component: stockView, name: 'stock' },
    { path: '/fund', component: fundView, name: 'fund' },
    { path: '/settings', component: settingsView, name: 'settings' },
    { path: '/market', component: marketView, name: 'market' },
    { path: '/intelligence', component: marketIntelligenceView, name: 'intelligence' },
    { path: '/agent', component: agentChat, name: 'agent' },
    { path: '/research', component: research, name: 'research' },
    { path: '/cron-tasks', component: cronTaskManager, name: 'cronTasks' },
    { path: '/kline-analysis', component: klineAnalysis, name: 'klineAnalysis' },
    { path: '/vectorbt', component: vectorBTPanel, name: 'vectorbt' },
    { path: '/audit', component: auditView, name: 'audit' },
]

const router = createRouter({
    //history: createWebHistory(),
    history: createWebHashHistory(),
    routes,
})

// ── 发布韧性兜底：路由级懒加载 chunk 失效 ────────────────────────────────────
// 部署新版本后旧哈希 chunk 会被整体删除，在线用户点击路由时会拿到 404。
// `vite:preloadError` 已覆盖绝大多数场景，此处为「非 Vite 预加载路径 / 浏览器
// 差异」再补一层保险：统一交给 staleChunkGuard 做一次性自动刷新或降级提示。
router.onError((error, to) => {
    const msg = (error && error.message) || String(error || '')
    if (/dynamically imported module|Importing a module script failed|Unable to preload CSS|Load failed/i.test(msg)) {
        console.warn('[Router] 路由懒加载模块失效，交由发布韧性守卫接管:', to && to.fullPath, msg)
        handleStaleChunkFailure(error)
        return
    }
    console.error('[Router] 路由切换发生异常:', to && to.fullPath, error)
})

export default router