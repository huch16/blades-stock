import {createRouter, createWebHashHistory} from 'vue-router'

import stockView from '../components/stock.vue'

// 非首页路由采用动态懒加载，大幅降低冷启动首屏 AST 解析时间与内存常驻
const fundView = () => import("../components/fund.vue")
const settingsView = () => import('../components/settings.vue')
const marketView = () => import("../components/market.vue")
const agentChat = () => import("../components/agent-chat.vue")
const research = () => import("../components/researchIndex.vue")
const cronTaskManager = () => import("../components/cron-task-manager.vue")
const mcpServerManager = () => import("../components/mcp-server-manager.vue")
const klineAnalysis = () => import("../components/kline-analysis.vue")
const vectorBTPanel = () => import("../components/VectorBTPanel.vue")
const auditView = () => import('../views/admin/Audit.vue')

const routes = [
    { path: '/', component: stockView, name: 'stock' },
    { path: '/fund', component: fundView, name: 'fund' },
    { path: '/settings', component: settingsView, name: 'settings' },
    { path: '/market', component: marketView, name: 'market' },
    { path: '/agent', component: agentChat, name: 'agent' },
    { path: '/research', component: research, name: 'research' },
    { path: '/cron-tasks', component: cronTaskManager, name: 'cronTasks' },
    { path: '/mcp-servers', component: mcpServerManager, name: 'mcpServers' },
    { path: '/kline-analysis', component: klineAnalysis, name: 'klineAnalysis' },
    { path: '/vectorbt', component: vectorBTPanel, name: 'vectorbt' },
    { path: '/audit', component: auditView, name: 'audit' },
]

const router = createRouter({
    //history: createWebHistory(),
    history: createWebHashHistory(),
    routes,
})

export default router