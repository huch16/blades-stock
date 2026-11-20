<template>
  <n-grid :x-gap="8" cols="1 s:1 m:2 l:3 xl:3" responsive="screen" :y-gap="8">
    <n-gi :id="result['股票代码']+'_gi'" v-for="result in results" :key="result.key" style="margin-left: 2px;">
      <StockCard :result="result" :show-spark-line="showSparkLine" :group-id="groupId" :open-ai-enable="openAiEnable" :group-list="groupList"
            :thesis="thesesMap[result['股票代码']] || thesesMap[result.key] || thesesMap[result.StockCode]"
            v-on="$attrs" />
    </n-gi>
  </n-grid>
</template>

<script setup lang="ts">
import StockCard from './StockCard.vue'

defineOptions({ inheritAttrs: false })

// 与 StockCard 同款运行时 props 写法，避免写出 any 类型触发 lint 警告
defineProps({
  results: { type: Array, required: true },
  showSparkLine: { type: Boolean, default: false },
  groupId: { type: Number, default: null },
  openAiEnable: { type: Boolean, default: false },
  groupList: { type: Array, default: () => [] },
  thesesMap: { type: Object, default: () => ({}) }
})
</script>
