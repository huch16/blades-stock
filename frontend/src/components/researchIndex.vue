<script setup>
import { onBeforeMount, onBeforeUnmount, ref, watch } from 'vue'
import ResearchReport from "./researchReport.vue";
import CronTaskManager from "./cron-task-manager.vue";
import TradingRecordManager from "./TradingRecordManager.vue";
import StockChangesMonitor from "./stockChangesMonitor.vue";
import UplimitLadder from "./uplimitLadder.vue";
import SelectStock from "./SelectStock.vue";
import { EventsOff, EventsOn } from "../../wailsjs/runtime";
import { useRoute } from 'vue-router'

const nowTab = ref("AI研报与认知账本")
const route = useRoute()

// 规范化 Tab 名称映射，兼容旧版链接与新版专业命名
function normalizeTabName(name) {
  if (!name) return "AI研报与认知账本"
  if (name.includes("AI") || name.includes("研报")) return "AI研报与认知账本"
  if (name.includes("异动")) return "盘面大单异动监控"
  if (name.includes("涨停") || name.includes("梯队")) return "涨停连板梯队复盘"
  if (name.includes("选股") || name.includes("指标") || name.includes("形态")) return "量化选股策略库"
  if (name.includes("交易") || name.includes("日志") || name.includes("台账")) return "实盘交易台账"
  if (name.includes("定时") || name.includes("任务")) return "定时自动化任务"
  return name
}

onBeforeMount(() => {
  if (route.query.name) {
    nowTab.value = normalizeTabName(route.query.name)
  }
})

watch(() => route.query.name, (newName) => {
  if (newName) {
    updateTab(newName)
  }
})

onBeforeUnmount(() => {
  EventsOff("changeResearchTab")
})

EventsOn("changeResearchTab", async (msg) => {
  if (msg && msg.name) {
    updateTab(normalizeTabName(msg.name))
  }
})
function updateTab(name) {
  nowTab.value = normalizeTabName(name)
}
</script>

<template>
  <n-card>
    <!--
      display-directive="if"（懒挂载）—— 重要修复

      默认情况下 n-tab-pane 会**全部急切挂载**：6 个 pane 的子组件在页面打开时
      就统统 mount，于是：
        ① 任何一个子组件的渲染异常都会把整页拖进 ErrorBoundary
           （用户看到“研究中心视图组件异常已自动隔离”，其余 5 个完好的 tab 一并不可用）；
        ② 6 个组件同时发起各自的 RPC 拉数据，首屏变慢且并发请求翻倍。
      改为按需挂载后：故障被隔离在单个 tab 内，首屏只拉当前 tab 的数据。
      子组件均在 onMounted/onBeforeUnmount 中成对注册与注销 SSE 监听，
      因此切换 tab 时的卸载/重建不会造成监听泄漏或重复。
    -->
    <n-tabs
      type="line"
      animated
      display-directive="if"
      :value="nowTab"
      style="--wails-draggable:no-drag"
      @update-value="updateTab"
    >
      <n-tab-pane name="AI研报与认知账本" tab="📑 AI研报与认知账本">
        <ResearchReport />
      </n-tab-pane>
      <n-tab-pane name="盘面大单异动监控" tab="⚡ 盘面大单异动监控">
        <StockChangesMonitor />
      </n-tab-pane>
      <n-tab-pane name="涨停连板梯队复盘" tab="🪜 涨停连板梯队复盘">
        <UplimitLadder />
      </n-tab-pane>
      <n-tab-pane name="量化选股策略库" tab="🎯 量化选股策略库">
        <SelectStock />
      </n-tab-pane>
      <n-tab-pane name="实盘交易台账" tab="📝 实盘交易台账">
        <TradingRecordManager />
      </n-tab-pane>
      <n-tab-pane name="定时自动化任务" tab="⏰ 定时自动化任务">
        <CronTaskManager />
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>

<style scoped>
@media screen and (max-width: 768px) {
  :deep(.n-tabs-nav-scroll-content) {
    overflow-x: auto !important;
    -webkit-overflow-scrolling: touch;
  }
  :deep(.n-tabs-tab) {
    padding: 6px 10px !important;
    font-size: 13px !important;
    white-space: nowrap !important;
  }
  :deep(.n-card__content) {
    padding: 8px !important;
  }
}
</style>
