<template>
  <n-grid :x-gap="8" cols="1 s:1 m:2 l:3 xl:3" responsive="screen" :y-gap="8">
    <n-gi :id="result['股票代码']+'_gi'" v-for="result in results" :key="result.key" style="margin-left: 2px;">
      <StockCard :result="result" :show-spark-line="showSparkLine" :group-id="groupId" :open-ai-enable="openAiEnable" :group-list="groupList"
            :thesis="thesesMap[result['股票代码']] || thesesMap[result.key] || thesesMap[result.StockCode]"
            @remove-monitor="(code, name, key) => emit('remove-monitor', code, name, key)"
            @ai-check-stock="(name, code) => emit('ai-check-stock', name, code)"
            @set-cost="(code, name) => emit('set-cost', code, name)"
            @show-fenshi="(code, name, changePercent) => emit('show-fenshi', code, name, changePercent)"
            @show-k="(code, name) => emit('show-k', code, name)"
            @show-money="(code, name) => emit('show-money', code, name)"
            @search-detail="(code, name) => emit('search-detail', code, name)"
            @search-notice="(code) => emit('search-notice', code)"
            @search-stock-report="(code) => emit('search-stock-report', code)"
            @add-stock-group="(gid, code, name) => emit('add-stock-group', gid, code, name)"
            @del-stock-group="(code, name, groupId) => emit('del-stock-group', code, name, groupId)"
            @show-lightweight-kline="(code, name) => emit('show-lightweight-kline', code, name)"
            @diagnose-stock="(code, name) => emit('diagnose-stock', code, name)" />
    </n-gi>
  </n-grid>
</template>

<script setup lang="ts">
import StockCard from './StockCard.vue'

defineOptions({ inheritAttrs: false })

// 卡片上的 13 个按钮全部以事件发往父组件（stock.vue）。
// 此前用 v-on="$attrs" 隐式透传，但实测 6 个按钮全部无反应（本地盘口折叠正常，
// 说明点击本身能到达卡片，只是事件没走到父组件），故改为逐个显式声明并转发，
// 不再依赖 $attrs 的隐式透传语义。
const emit = defineEmits([
  'remove-monitor',
  'ai-check-stock',
  'set-cost',
  'show-fenshi',
  'show-k',
  'show-money',
  'search-detail',
  'search-notice',
  'search-stock-report',
  'add-stock-group',
  'del-stock-group',
  'show-lightweight-kline',
  'diagnose-stock'
])

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
