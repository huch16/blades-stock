<template>
  <div class="market-intelligence-page-view">
    <n-card class="intel-page-card" :bordered="false">
      <MarketIntelligenceDrawer
        :embedded="true"
        :report-type="currentReportType"
        :initial-serenity-target="serenityTarget"
        @change-tab="handleTabChange"
      />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NCard } from 'naive-ui'
import MarketIntelligenceDrawer from './MarketIntelligenceDrawer.vue'

const route = useRoute()
const router = useRouter()

const currentReportType = computed(() => {
  const t = route.query.tab as string
  if (t && ['pre_market_briefing', 'post_market_review', 'etf_rotation_decision', 'serenity_industry_analysis'].includes(t)) {
    return t
  }
  return 'pre_market_briefing'
})

const serenityTarget = computed(() => {
  return (route.query.target as string) || ''
})

function handleTabChange(tabKey: string) {
  if (route.query.tab !== tabKey) {
    router.replace({
      name: 'intelligence',
      query: {
        ...route.query,
        tab: tabKey
      }
    })
  }
}
</script>

<style scoped>
.market-intelligence-page-view {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 16px 24px;
  min-height: calc(100vh - 80px);
}

.intel-page-card {
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}

:deep(.n-card__content) {
  padding: 14px 18px 20px !important;
}

@media (max-width: 768px) {
  .market-intelligence-page-view {
    padding: 6px 4px 16px;
  }
  :deep(.n-card__content) {
    padding: 8px 6px 14px !important;
  }
}
</style>
