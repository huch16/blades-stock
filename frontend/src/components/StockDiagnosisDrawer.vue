<script setup>
import { ref, watch } from 'vue'
import {
  NDrawer,
  NDrawerContent,
  NSpin,
  NCard,
  NFlex,
  NTag,
  NDivider,
  NGrid,
  NGridItem,
  NStatistic,
  NProgress,
  NAlert,
  NButton,
  useMessage
} from 'naive-ui'
import { DiagnoseStock } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  show: { type: Boolean, default: false },
  stockCode: { type: String, default: '' },
  stockName: { type: String, default: '' }
})

const emit = defineEmits(['update:show'])

const message = useMessage()
const loading = ref(false)
const diagnosisData = ref(null)

watch(
  () => props.show,
  async (visible) => {
    if (visible && props.stockCode) {
      await fetchDiagnosis()
    }
  }
)

async function fetchDiagnosis() {
  if (!props.stockCode) return
  loading.value = true
  try {
    const res = await DiagnoseStock(props.stockCode)
    if (res) {
      diagnosisData.value = res
    } else {
      message.error('量化诊断返回空数据')
    }
  } catch (err) {
    console.error('DiagnoseStock error:', err)
    message.error('量化诊断执行失败: ' + (err?.message || err))
  } finally {
    loading.value = false
  }
}

function handleClose() {
  emit('update:show', false)
}

function askAgentAssistant() {
  handleClose()
  const targetName = diagnosisData.value?.stockName || props.stockName || ''
  const targetCode = diagnosisData.value?.stockCode || props.stockCode || ''
  const score = diagnosisData.value?.overallScore || ''
  const suggestion = diagnosisData.value?.suggestion || ''
  const q = `请针对 ${targetName} (${targetCode}) 结合当前三维量化诊断（综合评分 ${score}分，建议 ${suggestion}），详细分析近期走势、买卖关键点位以及操作建议。`
  EventsEmit('openFloatingAgentWithQuestion', q)
}
</script>

<template>
  <NDrawer
    :show="show"
    @update:show="handleClose"
    :width="540"
    placement="right"
    :trap-focus="false"
  >
    <NDrawerContent
      :title="`✨ ${diagnosisData?.stockName || stockName} (${diagnosisData?.stockCode || stockCode}) 量化三维诊断`"
      closable
    >
      <div v-if="loading" style="display: flex; justify-content: center; align-items: center; height: 300px;">
        <NSpin size="large" description="正在调取 VectorBT 回测引擎、资金筹码及技术形态..." />
      </div>

      <div v-else-if="diagnosisData" class="diagnosis-container">
        <!-- 核心评分卡片 -->
        <NCard size="small" embedded class="score-card">
          <NFlex justify="space-between" align="center">
            <div>
              <div class="score-label">综合量化评分</div>
              <div class="score-value" :class="diagnosisData.overallScore >= 70 ? 'bull' : (diagnosisData.overallScore <= 40 ? 'bear' : 'neutral')">
                {{ diagnosisData.overallScore }} <span class="score-max">/ 100</span>
              </div>
            </div>
            <div style="text-align: right">
              <NTag
                :type="diagnosisData.overallScore >= 70 ? 'success' : (diagnosisData.overallScore <= 40 ? 'error' : 'warning')"
                size="large"
                round
                style="font-weight: 700; font-size: 14px;"
              >
                {{ diagnosisData.suggestion }}
              </NTag>
              <div style="margin-top: 6px; font-size: 12px; color: #888;">
                历史回测置信度: <strong style="color: #4098fc">{{ diagnosisData.confidence }}%</strong>
              </div>
            </div>
          </NFlex>

          <NDivider style="margin: 10px 0 8px 0;" />

          <!-- 关键点位建议 -->
          <NGrid :cols="4" :x-gap="8">
            <NGridItem>
              <NStatistic label="当前价格" :value="diagnosisData.currentPrice.toFixed(2)" />
            </NGridItem>
            <NGridItem>
              <NStatistic label="第一目标价" :value="diagnosisData.targetPrice.toFixed(2)">
                <template #suffix>
                  <span style="font-size: 10px; color: #18a058;">
                    (+{{ (((diagnosisData.targetPrice - diagnosisData.currentPrice) / diagnosisData.currentPrice) * 100).toFixed(1) }}%)
                  </span>
                </template>
              </NStatistic>
            </NGridItem>
            <NGridItem>
              <NStatistic label="建议止损价" :value="diagnosisData.stopLossPrice.toFixed(2)">
                <template #suffix>
                  <span style="font-size: 10px; color: #d03050;">
                    ({{ (((diagnosisData.stopLossPrice - diagnosisData.currentPrice) / diagnosisData.currentPrice) * 100).toFixed(1) }}%)
                  </span>
                </template>
              </NStatistic>
            </NGridItem>
            <NGridItem>
              <NStatistic label="盈亏比" :value="diagnosisData.riskRewardRatio.toFixed(2)" />
            </NGridItem>
          </NGrid>
        </NCard>

        <!-- 维度1: 量化回测维度 -->
        <NCard size="small" class="dim-card" style="margin-top: 10px">
          <template #header>
            <NFlex justify="space-between" align="center">
              <span>📊 量化回测验证 (VectorBT)</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.quantDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.quantDim.score"
            :color="diagnosisData.quantDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <div style="font-size: 12px; color: #666; margin-bottom: 6px;">
            策略: {{ diagnosisData.quantDim.strategyName }} (交易样本: {{ diagnosisData.quantDim.totalTrades }} 次)
          </div>
          <NGrid :cols="3" :x-gap="6" style="margin-bottom: 8px">
            <NGridItem>
              <div class="mini-stat-label">历史胜率</div>
              <div class="mini-stat-value">{{ diagnosisData.quantDim.winRatePct }}%</div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">夏普比率</div>
              <div class="mini-stat-value">{{ diagnosisData.quantDim.sharpeRatio }}</div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">最大回撤</div>
              <div class="mini-stat-value" style="color: #d03050">{{ diagnosisData.quantDim.maxDrawdownPct }}%</div>
            </NGridItem>
          </NGrid>
          <div class="dim-evaluation">{{ diagnosisData.quantDim.evaluation }}</div>
        </NCard>

        <!-- 维度2: 资金筹码维度 -->
        <NCard size="small" class="dim-card" style="margin-top: 10px">
          <template #header>
            <NFlex justify="space-between" align="center">
              <span>💰 资金筹码博弈</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.moneyDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.moneyDim.score"
            :color="diagnosisData.moneyDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <NGrid :cols="2" :x-gap="6" style="margin-bottom: 8px">
            <NGridItem>
              <div class="mini-stat-label">近5日主力净买入</div>
              <div class="mini-stat-value" :class="diagnosisData.moneyDim.mainNetInflow5D >= 0 ? 'bull' : 'bear'">
                {{ diagnosisData.moneyDim.mainNetInflow5D }} 万元
              </div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">超大单净占比</div>
              <div class="mini-stat-value">
                {{ diagnosisData.moneyDim.superLargeNetPct }}%
              </div>
            </NGridItem>
          </NGrid>
          <div style="font-size: 12px; margin-bottom: 4px;">
            <NTag size="tiny" type="info">{{ diagnosisData.moneyDim.recentInflowTrend }}</NTag>
            <NTag size="tiny" type="default" style="margin-left: 6px;">{{ diagnosisData.moneyDim.chipConcentration }}</NTag>
          </div>
          <div class="dim-evaluation">{{ diagnosisData.moneyDim.evaluation }}</div>
        </NCard>

        <!-- 维度3: 技术形态维度 -->
        <NCard size="small" class="dim-card" style="margin-top: 10px">
          <template #header>
            <NFlex justify="space-between" align="center">
              <span>📈 技术与波动形态</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.technicalDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.technicalDim.score"
            :color="diagnosisData.technicalDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <div style="font-size: 12px; color: #666; margin-bottom: 6px;">
            均线形态: <strong>{{ diagnosisData.technicalDim.trend }}</strong> | ATR(14): {{ diagnosisData.technicalDim.atr }}
          </div>
          <div class="dim-evaluation">{{ diagnosisData.technicalDim.evaluation }}</div>
        </NCard>

        <!-- 历史战绩与反思记录 -->
        <NCard size="small" class="dim-card" style="margin-top: 10px; background: rgba(32, 128, 240, 0.04);">
          <template #header>
            <span style="font-size: 13px;">🧠 AI 历史预测复盘闭环</span>
          </template>
          <div style="font-size: 12px; line-height: 1.6;">
            {{ diagnosisData.historyTrackRecord }}
          </div>
        </NCard>

        <!-- 风险提示 -->
        <div style="margin-top: 10px" v-if="diagnosisData.risks && diagnosisData.risks.length > 0">
          <NAlert title="关键风险排查" type="warning" :show-icon="true">
            <ul style="margin: 0; padding-left: 16px; font-size: 12px;">
              <li v-for="(r, idx) in diagnosisData.risks" :key="idx">{{ r }}</li>
            </ul>
          </NAlert>
        </div>

        <!-- 底部操作按钮 -->
        <div style="margin-top: 16px; text-align: center;">
          <NButton type="primary" block size="medium" @click="askAgentAssistant">
            💬 与 AI 投研助手深度探讨
          </NButton>
        </div>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.diagnosis-container {
  padding: 4px;
}
.score-card {
  border-radius: 8px;
}
.score-label {
  font-size: 13px;
  color: #888;
}
.score-value {
  font-size: 28px;
  font-weight: 800;
  line-height: 1.2;
}
.score-max {
  font-size: 13px;
  font-weight: 400;
  color: #999;
}
.bull {
  color: #18a058;
}
.bear {
  color: #d03050;
}
.neutral {
  color: #f0a020;
}
.mini-stat-label {
  font-size: 11px;
  color: #888;
}
.mini-stat-value {
  font-size: 14px;
  font-weight: 600;
}
.dim-card {
  border-radius: 8px;
}
.dim-evaluation {
  font-size: 12px;
  color: #555;
  background: rgba(0, 0, 0, 0.03);
  padding: 6px 8px;
  border-radius: 4px;
  line-height: 1.5;
}
</style>
