<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {GetAIResponseResultList} from "../../wailsjs/go/main/App";
import {NButton, NEllipsis, NText} from "naive-ui";
import ResearchReport from "./researchReport.vue";
import AllStockList from "./allStockList.vue";
import CronTaskManager from "./cron-task-manager.vue";
import TradingRecordManager from "./TradingRecordManager.vue";
import StockChangesMonitor from "./stockChangesMonitor.vue";
import UplimitLadder from "./uplimitLadder.vue";
import SelectStock from "./SelectStock.vue";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import {useRoute} from 'vue-router'


const nowTab = ref("AI分析报告")
const route = useRoute()
onBeforeMount(() => {
  nowTab.value = route.query.name
})

onBeforeUnmount(() => {
  EventsOff("changeResearchTab")
})

onUnmounted(() => {

});

EventsOn("changeResearchTab", async (msg) => {
  updateTab(msg.name)
})
function updateTab(name) {
  nowTab.value = name
}
</script>

<template>
  <n-card>
    <n-tabs
      type="line"
      animated
      :value="nowTab"
      style="--wails-draggable:no-drag"
      @update-value="updateTab"
    >
      <n-tab-pane name="AI分析报告">
        <ResearchReport />
      </n-tab-pane>
      <n-tab-pane name="异动监控">
        <StockChangesMonitor />
      </n-tab-pane>
      <n-tab-pane name="涨停梯队">
        <UplimitLadder />
      </n-tab-pane>
      <n-tab-pane name="形态选股">
        <AllStockList />
      </n-tab-pane>
      <n-tab-pane name="指标选股">
        <SelectStock />
      </n-tab-pane>
      <n-tab-pane name="定时任务">
        <CronTaskManager />
      </n-tab-pane>
      <n-tab-pane name="交易日志">
        <TradingRecordManager />
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
