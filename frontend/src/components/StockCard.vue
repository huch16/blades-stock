<script setup>
import StockSparkLine from "./stockSparkLine.vue"
import { setCurrentStock } from "../utils/stockContextBus"

const props = defineProps({
  result: { type: Object, required: true },
  showSparkLine: { type: Boolean, default: false },
  groupId: { type: Number, default: null },
  openAiEnable: { type: Boolean, default: false },
  groupList: { type: Array, default: () => [] },
  thesis: { type: Object, default: null }
})

function getVerdictType(verdict) {
  switch (verdict) {
    case 'THESIS_STRENGTHENED':
      return 'success'
    case 'NEW_THESIS':
      return 'info'
    case 'THESIS_WEAKENED':
      return 'warning'
    case 'THESIS_BROKEN':
      return 'error'
    default:
      return 'default'
  }
}

function focusStock() {
  if (props.result && props.result['股票代码']) {
    setCurrentStock({
      code: props.result['股票代码'],
      name: props.result['股票名称']
    })
  }
}

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
</script>

<template>
  <n-card
    :id="result['股票代码']"
    :data-sort="result.sort"
    :data-code="result['股票代码']"
    :bordered="true"
    :title="result['股票名称']"
    :closable="false"
    @click="focusStock"
    @close="emit('remove-monitor', result['股票代码'], result['股票名称'], result.key)"
  >
    <n-grid
      v-if="!showSparkLine"
      :cols="1"
      :y-gap="6"
    >
      <n-gi>
        <n-text :type="result.type">
          <span v-if="!Number(result['当前价格']) || Number(result['当前价格']) <= 0" style="font-size: 1.1em; opacity: 0.6;">--</span>
          <n-number-animation
            v-else
            :duration="1000"
            :precision="2"
            :from="result['上次当前价格']"
            :to="Number(result['当前价格'])"
          />
          <n-tag
            v-if="result['盘前盘后']>0"
            size="small"
            :type="result.type"
            :bordered="false"
          >
            ({{ result['盘前盘后'] }} {{ result['盘前盘后涨跌幅'] }}%)
          </n-tag>
        </n-text>
        <n-text
          style="padding-left: 10px;"
          :type="result.type"
        >
          <span v-if="!Number(result['当前价格']) || Number(result['当前价格']) <= 0" style="opacity: 0.6;">--</span>
          <template v-else>
            <n-number-animation
              :duration="1000"
              :precision="3"
              :from="result.lastChangePercent"
              :to="result.changePercent"
            />
            %
          </template>
        </n-text>&nbsp;
        <n-text
          v-if="result.costVolume>0"
          size="small"
          :type="result.type"
        >
          <n-number-animation
            :duration="1000"
            :precision="2"
            :from="result.lastProfitAmountToday"
            :to="result.profitAmountToday"
          />
        </n-text>
      </n-gi>
    </n-grid>
    <n-grid
      v-else
      :cols="12"
      :y-gap="6"
    >
      <n-gi :span="6">
        <n-text :type="result.type">
          <span v-if="!Number(result['当前价格']) || Number(result['当前价格']) <= 0" style="font-size: 1.1em; opacity: 0.6;">--</span>
          <n-number-animation
            v-else
            :duration="1000"
            :precision="2"
            :from="result['上次当前价格']"
            :to="Number(result['当前价格'])"
          />
          <n-tag
            v-if="result['盘前盘后']>0"
            size="small"
            :type="result.type"
            :bordered="false"
          >
            ({{ result['盘前盘后'] }} {{ result['盘前盘后涨跌幅'] }}%)
          </n-tag>
        </n-text>
        <n-text
          style="padding-left: 10px;"
          :type="result.type"
        >
          <span v-if="!Number(result['当前价格']) || Number(result['当前价格']) <= 0" style="opacity: 0.6;">--</span>
          <template v-else>
            <n-number-animation
              :duration="1000"
              :precision="3"
              :from="result.lastChangePercent"
              :to="result.changePercent"
            />
            %
          </template>
        </n-text>&nbsp;
        <n-text
          v-if="result.costVolume>0"
          size="small"
          :type="result.type"
        >
          <n-number-animation
            :duration="1000"
            :precision="2"
            :from="result.lastProfitAmountToday"
            :to="result.profitAmountToday"
          />
        </n-text>
      </n-gi>
      <n-gi :span="6">
        <stock-spark-line
          :last-price="Number(result['当前价格'])"
          :open-price="Number(result['昨日收盘价'])"
          :stock-code="result['股票代码']"
          :stock-name="result['股票名称']"
        />
      </n-gi>
    </n-grid>
    <n-grid
      :cols="2"
      :y-gap="4"
      :x-gap="4"
    >
      <n-gi>
        <n-text :type="'info'">
          {{ "最高 " + result["今日最高价"] + " " + result.highRate }}%
        </n-text>
      </n-gi>
      <n-gi>
        <n-text :type="'info'">
          {{ "最低 " + result["今日最低价"] + " " + result.lowRate }}%
        </n-text>
      </n-gi>
      <n-gi>
        <n-text :type="'info'">
          {{ "昨收 " + result["昨日收盘价"] }}
        </n-text>
      </n-gi>
      <n-gi>
        <n-text :type="'info'">
          {{ "今开 " + result["今日开盘价"] }}
        </n-text>
      </n-gi>
    </n-grid>
    <n-collapse
      v-if="result['买一报价']>0"
      accordion
    >
      <n-collapse-item
        v-if="result['买一报价']>0"
        title="盘口"
        name="1"
      >
        <template #header-extra>
          <n-flex justify="space-between">
            <n-text :type="'info'">
              {{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}
            </n-text>
            <n-text :type="'info'">
              {{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}
            </n-text>
          </n-flex>
        </template>
        <n-grid
          :cols="2"
          :y-gap="4"
          :x-gap="4"
        >
          <n-gi v-if="result['买一报价']>0">
            <n-text :type="'info'">
              {{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}
            </n-text>
          </n-gi>
          <n-gi v-if="result['卖一报价']>0">
            <n-text :type="'info'">
              {{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}
            </n-text>
          </n-gi>

          <n-gi v-if="result['买二报价']>0">
            <n-text :type="'info'">
              {{ "买二 " + result["买二报价"] + '(' + result["买二申报"] + ")" }}
            </n-text>
          </n-gi>
          <n-gi v-if="result['卖二报价']>0">
            <n-text :type="'info'">
              {{ "卖二 " + result["卖二报价"] + '(' + result["卖二申报"] + ")" }}
            </n-text>
          </n-gi>

          <n-gi v-if="result['买三报价']>0">
            <n-text :type="'info'">
              {{ "买三 " + result["买三报价"] + '(' + result["买三申报"] + ")" }}
            </n-text>
          </n-gi>
          <n-gi v-if="result['卖三报价']>0">
            <n-text :type="'info'">
              {{ "买三 " + result["卖三报价"] + '(' + result["卖三申报"] + ")" }}
            </n-text>
          </n-gi>

          <n-gi v-if="result['买四报价']>0">
            <n-text :type="'info'">
              {{ "买四 " + result["买四报价"] + '(' + result["买四申报"] + ")" }}
            </n-text>
          </n-gi>
          <n-gi v-if="result['卖四报价']>0">
            <n-text :type="'info'">
              {{ "卖四 " + result["卖四报价"] + '(' + result["卖四申报"] + ")" }}
            </n-text>
          </n-gi>

          <n-gi v-if="result['买五报价']>0">
            <n-text :type="'info'">
              {{ "买五 " + result["买五报价"] + '(' + result["买五申报"] + ")" }}
            </n-text>
          </n-gi>
          <n-gi v-if="result['卖五报价']>0">
            <n-text :type="'info'">
              {{ "卖五 " + result["卖五报价"] + '(' + result["卖五申报"] + ")" }}
            </n-text>
          </n-gi>
        </n-grid>
      </n-collapse-item>
    </n-collapse>
    <template #header-extra>
      <div class="card-header-extra">
        <n-tooltip v-if="thesis && thesis.verdictText" trigger="hover">
          <template #trigger>
            <n-tag
              size="tiny"
              round
              :bordered="false"
              :type="getVerdictType(thesis.verdict)"
              style="cursor: pointer; font-size: 11px; font-weight: 500;"
              @click.stop="emit('ai-check-stock', result['股票名称'], result['股票代码'])"
            >
              {{ thesis.verdictText }}
            </n-tag>
          </template>
          <div style="max-width: 260px; font-size: 12px; line-height: 1.5; text-align: left;">
            <div style="font-weight: bold; margin-bottom: 4px; color: #18a058;">🎯 买方认知假说</div>
            <div>{{ thesis.coreThesis || '暂无详细假说描述' }}</div>
            <div v-if="thesis.keyCatalysts" style="margin-top: 4px; color: #f0a020;">
              <strong>催化：</strong>{{ thesis.keyCatalysts }}
            </div>
          </div>
        </n-tooltip>
        <n-tag
          size="small"
          :bordered="false"
        >
          {{ result['股票代码'] }}
        </n-tag>
        <n-button
          size="tiny"
          secondary
          type="primary"
          @click="emit('remove-monitor', result['股票代码'], result['股票名称'], result.key)"
        >
          取消关注
        </n-button>
        <n-button
          v-if="props.openAiEnable"
          size="tiny"
          secondary
          type="warning"
          @click="emit('ai-check-stock', result['股票名称'], result['股票代码'])"
        >
          AI分析
        </n-button>
        <n-button
          v-if="groupId != null"
          secondary
          type="error"
          size="tiny"
          @click="emit('del-stock-group', result['股票代码'], result['股票名称'], groupId)"
        >
          移出分组
        </n-button>
      </div>
    </template>
    <template #footer>
      <n-flex
        vertical
        :size="8"
      >
        <n-flex justify="center">
          <n-text :type="'info'">
            {{ result["日期"] + " " + result["时间"] }}
          </n-text>
          <n-tag
            v-if="result.volume>0"
            size="small"
            :type="result.profitType"
          >
            {{ result.volume + "股" }}
          </n-tag>
          <n-tag
            v-if="result.costPrice>0"
            size="small"
            :type="result.profitType"
          >
            {{
              "成本:" + result.costPrice + "*" + result.costVolume + " " + result.profit + "%" + " ( " + result.profitAmount + " ¥ )"
            }}
          </n-tag>
        </n-flex>
        <n-flex
          justify="center"
          :size="6"
        >
          <n-button
            size="tiny"
            type="primary"
            secondary
            @click="emit('show-lightweight-kline', result['股票代码'], result['股票名称'])"
          >
            多周期K线
          </n-button>
          <n-button
            size="tiny"
            type="warning"
            secondary
            @click="emit('diagnose-stock', result['股票代码'], result['股票名称'])"
          >
            ✨ 量化诊断
          </n-button>
        </n-flex>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="start" wrap :size="[6, 6]" class="card-action-flex">
        <n-button
          size="tiny"
          type="warning"
          @click="emit('set-cost', result['股票代码'], result['股票名称'])"
        >
          成本
        </n-button>
        <n-button
          size="tiny"
          type="error"
          @click="emit('show-fenshi', result['股票代码'], result['股票名称'], result.changePercent)"
        >
          分时
        </n-button>
        <n-button
          size="tiny"
          type="error"
          @click="emit('show-k', result['股票代码'], result['股票名称'])"
        >
          日K
        </n-button>
        <n-button
          v-if="result['买一报价']>0"
          size="tiny"
          type="error"
          @click="emit('show-money', result['股票代码'], result['股票名称'])"
        >
          资金
        </n-button>
        <n-button
          size="tiny"
          type="success"
          @click="emit('search-detail', result['股票代码'], result['股票名称'])"
        >
          详情
        </n-button>
        <n-button
          v-if="result['买一报价']>0"
          size="tiny"
          type="success"
          @click="emit('search-notice', result['股票代码'])"
        >
          公告
        </n-button>
        <n-button
          v-if="result['买一报价']>0"
          size="tiny"
          type="success"
          @click="emit('search-stock-report', result['股票代码'])"
        >
          研报
        </n-button>
        <n-dropdown
          trigger="click"
          :options="props.groupList"
          key-field="ID"
          label-field="name"
          @select="(gid) => emit('add-stock-group', gid, result['股票代码'], result['股票名称'])"
        >
          <n-button
            type="warning"
            size="tiny"
          >
            设置分组
          </n-button>
        </n-dropdown>
      </n-flex>
    </template>
  </n-card>
</template>

<style scoped>
.card-header-extra {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 4px;
}

.card-action-flex {
  width: 100%;
}

@media screen and (max-width: 768px) {
  :deep(.n-card-header) {
    padding: 10px 12px !important;
  }
  :deep(.n-card__content) {
    padding: 8px 12px !important;
  }
  :deep(.n-card__footer) {
    padding: 8px 12px !important;
  }
  :deep(.n-card__action) {
    padding: 8px 12px !important;
  }
  .card-action-flex :deep(.n-button) {
    padding: 0 6px !important;
    font-size: 11px !important;
    min-height: 24px;
  }
}
</style>
