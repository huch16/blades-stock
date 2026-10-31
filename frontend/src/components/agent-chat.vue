<template>
  <div class="chat-container">
    <!-- 顶部 HUD 控制中枢 -->
    <header class="chat-header">
      <div class="header-left">
        <div class="brand-title-wrap">
          <span class="brand-icon">🤖</span>
          <span class="brand-title">Blades Stock AI Agent 助手</span>
          <span class="brand-badge">L5自主量化</span>
        </div>
        <n-button
          type="primary"
          size="small"
          secondary
          class="new-chat-btn"
          @click="createNewSession"
          title="开启新投研对话"
        >
          <template #icon>
            <AddIcon />
          </template>
          ➕ 新对话
        </n-button>
        <n-button
          size="small"
          quaternary
          class="clear-chat-btn"
          @click="clearCurrentHistory"
          title="清空当前会话消息"
        >
          <template #icon>
            <DeleteIcon />
          </template>
          🗑️ 清空
        </n-button>
        <n-button
          quaternary
          size="small"
          class="session-drawer-btn"
          @click="showSessionDrawer = true"
          title="会话历史管理"
        >
          <template #icon>
            <ChatIcon />
          </template>
          <span class="session-title-text">{{ currentSessionTitle }}</span>
          <span class="session-badge">{{ messages.length }}</span>
        </n-button>
      </div>

      <div class="header-right">
        <!-- 思考模式与记忆模式 -->
        <div class="header-switch-item" title="是否开启大模型深度思考与慢思考链 (Thinking Mode)">
          <span class="header-switch-label">深度思考</span>
          <n-switch v-model:value="thinkingMode" size="small" />
        </div>
        <div class="header-switch-item" title="是否携带上下文历史多轮记忆">
          <span class="header-switch-label">记忆</span>
          <n-switch v-model:value="memoryMode" size="small" />
          <n-select
            v-if="memoryMode"
            v-model:value="memoryCount"
            :options="memoryCountOptions"
            size="small"
            class="header-memory-select"
            :consistent-menu-width="false"
          />
        </div>

        <!-- 模型选择 -->
        <n-select
          v-model:value="activeConfigId"
          :options="configOptions"
          :placeholder="configOptions.length ? '选择 AI 模型配置' : '请先在设置中添加 AI'"
          size="small"
          class="model-selector"
          clearable
          @update:value="onConfigChange"
        />

        <n-button
          quaternary
          size="small"
          class="scenario-btn"
          @click="showScenarioModal = true"
          title="投研场景库"
        >
          <template #icon>
            <ViewListIcon />
          </template>
          场景库
        </n-button>

        <n-tooltip trigger="hover" placement="bottom">
          <template #trigger>
            <n-button
              quaternary
              circle
              size="small"
              @click="isCompact = !isCompact"
            >
              <FullscreenExitIcon v-if="!isCompact" />
              <FullscreenIcon v-else />
            </n-button>
          </template>
          <span>{{ isCompact ? '标准视图' : '紧凑视图' }}</span>
        </n-tooltip>
      </div>
    </header>

    <!-- 全局宏观态势感知 HUD 仪表条 (桌面端与移动端统一呈现) -->
    <section
      v-if="perceptionSnapshot"
      class="macro-perception-hud"
      title="点击呼叫 Agent 深度解读当前宏观节律与持仓策略"
      @click="askMacroPerception"
    >
      <div class="hud-left">
        <span class="phase-pill">{{ perceptionSnapshot.Phase || '宏观节律感知' }}</span>
        <span
          class="traffic-beacon"
          :style="{ backgroundColor: getTrafficColor(perceptionSnapshot.TrafficLight) }"
        />
        <span
          class="traffic-label"
          :style="{ color: getTrafficColor(perceptionSnapshot.TrafficLight) }"
        >
          {{ getTrafficText(perceptionSnapshot.TrafficLight) }}
        </span>
        <span
          v-if="perceptionSnapshot.MarketBreadth"
          class="breadth-ratio"
        >
          <span class="up-count">涨 {{ perceptionSnapshot.MarketBreadth.UpCount }}</span>
          <span class="separator">/</span>
          <span class="down-count">跌 {{ perceptionSnapshot.MarketBreadth.DownCount }}</span>
        </span>
      </div>
      <div class="hud-right">
        <n-tag
          v-if="perceptionSnapshot.PortfolioRisk && perceptionSnapshot.PortfolioRisk.RiskLevel !== 'SAFE'"
          size="small"
          type="error"
          round
          class="risk-tag"
        >
          ⚠️ {{ perceptionSnapshot.PortfolioRisk.RiskLevel === 'DANGER' ? '高危持仓预警' : '持仓警示' }}
        </n-tag>
        <span class="action-hint">
          <span>宏观态势推演</span>
          <span class="arrow">➔</span>
        </span>
      </div>
    </section>

    <!-- 主体聊天消息滚动区域 (自然时间线：旧消息在上，新消息在下) -->
    <main
      class="chat-messages-viewport"
      ref="messagesContainer"
      @scroll="handleScroll"
    >
      <!-- 消息锚点导航（折叠式微型右侧指示器） -->
      <MessageAnchorNav
        :items="anchorNavItems"
        :show="messages.length > 2"
        @scroll="scrollToMessage"
      />

      <!-- 空白欢迎态：买方五阶投研决策流水线控制台 -->
      <section v-if="messages.length === 0" class="welcome-workspace">
        <div class="welcome-hero">
          <div class="hero-badge">BUIS QUANTITATIVE INTELLIGENCE LADDER</div>
          <h2 class="hero-title">Blades AI 智能投研决策中枢</h2>
          <p class="hero-desc">
            从宏观大势择时到买方个股落子，全流程标准化作业，实现高确定性投研闭环
          </p>

          <!-- 模式切换：标准五阶决策梯 vs 专项极速控制台 -->
          <div class="welcome-mode-toggle">
            <button
              class="toggle-tab-btn"
              :class="{ active: welcomeViewMode === 'pipeline' }"
              @click="welcomeViewMode = 'pipeline'"
            >
              <span class="tab-icon">🚀</span>
              <span class="tab-title">五阶量化时序决策梯 (L1~L5)</span>
            </button>
            <button
              class="toggle-tab-btn"
              :class="{ active: welcomeViewMode === 'direct' }"
              @click="welcomeViewMode = 'direct'"
            >
              <span class="tab-icon">⚡</span>
              <span class="tab-title">专项极速穿透台 (Direct Tools)</span>
            </button>
          </div>
        </div>

        <!-- 视图 1：标准五阶流水线 (时序导向) -->
        <div v-if="welcomeViewMode === 'pipeline'" class="pipeline-container">
          <div class="pipeline-flow-guide">
            <div class="flow-step-node" :class="{ current: pipelineContext.currentStep === 1 }" @click="executeStep1">
              <span class="step-num">1</span>
              <span class="step-name">看大势 (L1)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div class="flow-step-node" :class="{ current: pipelineContext.currentStep === 2 }" @click="executeStep2('etf')">
              <span class="step-num">2</span>
              <span class="step-name">选赛道 (L2)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div class="flow-step-node" :class="{ current: pipelineContext.currentStep === 3 }" @click="executeStep3('breakout')">
              <span class="step-num">3</span>
              <span class="step-name">筛标的 (L3)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div class="flow-step-node" :class="{ current: pipelineContext.currentStep === 4 }" @click="executeStep4('diag')">
              <span class="step-num">4</span>
              <span class="step-name">验体质 (L4)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div class="flow-step-node" :class="{ current: pipelineContext.currentStep === 5 }" @click="executeStep5()">
              <span class="step-num">5</span>
              <span class="step-name">定落子 (L5)</span>
            </div>
          </div>

          <!-- 全局流水线贯通标的与状态指示栏 (消除前后步骤脱节与信息孤岛) -->
          <div class="pipeline-context-banner">
            <div class="banner-left">
              <span class="banner-icon">🎯</span>
              <span class="banner-label">当前贯通标的：</span>
              <span class="banner-stock-pill" @click="openStockActionModal('diag')" title="点击更换标的">
                <span class="pill-name">{{ pipelineContext.stockName || '默认标的' }}</span>
                <span class="pill-code">{{ pipelineContext.stockCode }}</span>
              </span>
              <span v-if="pipelineContext.mainTrack" class="banner-tag track-tag">
                🏷️ {{ pipelineContext.mainTrack }}
              </span>
              <span v-if="pipelineContext.macroStance" class="banner-tag stance-tag">
                🚦 {{ pipelineContext.macroStance }}
              </span>
            </div>
            <div class="banner-right">
              <div class="banner-hot-chips">
                <span class="hot-label">快速点选:</span>
                <button
                  v-for="stk in hotStocks"
                  :key="stk.code"
                  class="hot-chip-btn"
                  :class="{ active: pipelineContext.stockCode === stk.code }"
                  @click="setPipelineStock(stk.code, stk.name)"
                >
                  {{ stk.name }}
                </button>
              </div>
              <button class="banner-switch-btn" @click="openStockActionModal('diag')">
                🔄 自选代码
              </button>
            </div>
          </div>

          <div class="pipeline-steps-grid">
            <!-- Step 1: 🏛️ 看大势 (L1 Macro) -->
            <div class="pipeline-card step-card-1">
              <div class="step-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l1">L1 择时</span>
                  <span class="step-title">宏观大势与仓位上限</span>
                </div>
              </div>
              <p class="step-desc">解决【大盘能不能做？给几成仓？】严禁外部搜索，调用 GetMarketData 评估大盘估值与多空红绿灯。</p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep1">
                  🚀 测算大盘择时与仓位
                </button>
              </div>
            </div>

            <!-- Step 2: 🧭 选赛道 (L2 Sector & ETF) -->
            <div class="pipeline-card step-card-2">
              <div class="step-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l2">L2 主线</span>
                  <span class="step-title">赛道动量与主力席位</span>
                </div>
              </div>
              <p class="step-desc">解决【资金流向哪里？】纯 Go 本地 VectorBT 运行21日加权动量，锁定场内领涨行业 ETF 与龙虎榜席位。</p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep2('etf')">
                  🚀 ETF 动量排名
                </button>
                <button class="step-btn sub" @click="executeStep2('seats')">
                  🏛️ 龙虎榜机构席位
                </button>
              </div>
            </div>

            <!-- Step 3: 🎯 筛标的 (L3 Alpha Screening) -->
            <div class="pipeline-card step-card-3">
              <div class="step-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l3">L3 选股</span>
                  <span class="step-title">量化多因子初筛池</span>
                </div>
              </div>
              <p class="step-desc">解决【优势赛道里买哪只？】调用 ScreenByStrategy 多因子选拔高胜率候选股票，首选标的自动贯通至 Step 4。</p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep3('breakout')">
                  🚀 放量突破
                </button>
                <button class="step-btn sub" @click="executeStep3('trend')">
                  📈 均线多头
                </button>
                <button class="step-btn sub" @click="executeStep3('macd')">
                  ⚡ MACD金叉
                </button>
                <button class="step-btn sub" @click="executeStep3('dividend')">
                  🛡️ 高股息
                </button>
              </div>
            </div>

            <!-- Step 4: 🔬 验体质与极限排雷 (L4 Deep-Dive & Red Flags) -->
            <div class="pipeline-card step-card-4 highlight-step">
              <div class="step-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l4">L4 验质</span>
                  <span class="step-title">四维体检与极限排雷</span>
                </div>
              </div>
              <p class="step-desc">解决【标的有无暗雷？盈亏比划算吗？】对贯通标的进行 VectorBT 胜率测算、60日均线、质押解禁与现金流排雷。</p>
              
              <!-- 内置标的快速填装器，彻底消灭占位符 -->
              <div class="inline-stock-picker">
                <n-input
                  v-model:value="step4StockInput"
                  placeholder="标的代码 (如 600519)"
                  size="small"
                  class="picker-input"
                  @update:value="onStep4StockInputChange"
                />
              </div>

              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep4('diag')">
                  🔬 四维深度体检
                </button>
                <button class="step-btn warning" @click="executeStep4('risk')">
                  🛡️ 股权质押排雷
                </button>
                <button class="step-btn sub" @click="executeStep4('finance')">
                  📑 经营现金流
                </button>
              </div>
            </div>

            <!-- Step 5: 📑 定落子与立论 (L5 Verdict & Thesis Ledger) -->
            <div class="pipeline-card step-card-5 highlight-step">
              <div class="step-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l5">L5 终局</span>
                  <span class="step-title">买方立论与 5-Verdict</span>
                </div>
              </div>
              <p class="step-desc">解决【知行合一与止损防守】对贯通标的作出买方确定性评级与下行证伪线，并强制写入 SQLite 认知账本数据库。</p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep5()">
                  🚀 5-Verdict 裁决入库
                </button>
                <button class="step-btn sub" @click="viewVerdictStandards">
                  📖 5-Verdict 规范
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 视图 2：专项极速控制台 (单点任务导向) -->
        <div v-else class="direct-tools-container">
          <div class="direct-tools-grid">
            <!-- 工具 1：个股全景深度体检与排雷 -->
            <div class="direct-tool-card">
              <div class="tool-head">
                <span class="tool-icon">🔬</span>
                <div class="tool-head-text">
                  <span class="tool-name">单股四维全景穿透体检</span>
                  <span class="tool-sub">VectorBT胜率 + 60日均线 + 财务真实性</span>
                </div>
              </div>
              <div class="tool-form">
                <n-input
                  v-model:value="directDiagCode"
                  placeholder="输入 6 位代码或名称 (如 300750)"
                  size="small"
                  class="tool-input"
                />
                <div class="quick-hot-row">
                  <button
                    v-for="stk in hotStocks.slice(0, 5)"
                    :key="stk.code"
                    class="hot-chip"
                    @click="directDiagCode = stk.code"
                  >
                    {{ stk.name }}
                  </button>
                </div>
                <button class="tool-submit-btn" @click="executeDirectDiag">
                  🚀 立即启动全景体检
                </button>
              </div>
            </div>

            <!-- 工具 2：同行业双龙头深度 PK -->
            <div class="direct-tool-card">
              <div class="tool-head">
                <span class="tool-icon">⚖️</span>
                <div class="tool-head-text">
                  <span class="tool-name">赛道同业双龙头深度 PK</span>
                  <span class="tool-sub">横向对比两只行业龙头的量价形态、估值与ROE</span>
                </div>
              </div>
              <div class="tool-form">
                <div class="pk-inputs-row">
                  <n-input
                    v-model:value="pkCodeA"
                    placeholder="标的 A (如 300750)"
                    size="small"
                  />
                  <span class="pk-vs">VS</span>
                  <n-input
                    v-model:value="pkCodeB"
                    placeholder="标的 B (如 002594)"
                    size="small"
                  />
                </div>
                <div class="quick-hot-row">
                  <button class="hot-chip" @click="applyPresetPK('300750', '002594')">
                    宁德时代 vs 比亚迪
                  </button>
                  <button class="hot-chip" @click="applyPresetPK('600519', '000858')">
                    贵州茅台 vs 五粮液
                  </button>
                  <button class="hot-chip" @click="applyPresetPK('688981', '002371')">
                    中芯国际 vs 北方华创
                  </button>
                </div>
                <button class="tool-submit-btn" @click="executeDirectPK">
                  ⚖️ 立即启动双股对比
                </button>
              </div>
            </div>

            <!-- 工具 3：筹码与风险极限排雷 -->
            <div class="direct-tool-card">
              <div class="tool-head">
                <span class="tool-icon">🛡️</span>
                <div class="tool-head-text">
                  <span class="tool-name">风险极限排雷与筹码穿透</span>
                  <span class="tool-sub">穿透大股东股权质押、解禁日程与股东户数</span>
                </div>
              </div>
              <div class="tool-form">
                <n-input
                  v-model:value="directRiskCode"
                  placeholder="输入代码或名称 (如 600519)"
                  size="small"
                  class="tool-input"
                />
                <div class="quick-hot-row">
                  <button
                    v-for="stk in hotStocks.slice(0, 5)"
                    :key="stk.code"
                    class="hot-chip"
                    @click="directRiskCode = stk.code"
                  >
                    {{ stk.name }}
                  </button>
                </div>
                <button class="tool-submit-btn warning" @click="executeDirectRisk">
                  🛡️ 立即启动专项排雷
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 消息列表渲染 (严格单次渲染，杜绝重复占位) -->
      <div v-else class="messages-flow">
        <ChatMessageComp
          v-for="msg in messages"
          :key="msg.id"
          :message="msg"
          :is-streaming="isStreaming && msg.id === streamingMsgId"
          :current-user-avatar="userAvatar"
          :current-user-name="userName"
          @copy="handleCopy"
          @retry="handleRetry"
          @reference="handleReference"
          @edit="handleEdit"
          @resend="handleResend"
          @save-thesis="handleSaveThesisFromMsg"
          @export-image="handleExportImage"
        />

        <!-- 下游时序推荐条 (消除断层与信息孤岛) -->
        <div v-if="!isStreaming && pipelineNextStep" class="pipeline-next-step-bar">
          <span class="next-step-icon">💡</span>
          <span class="next-step-text">流水线下一步推荐：</span>
          <button class="next-step-btn" @click="pipelineNextStep.action">
            {{ pipelineNextStep.label }} ➔
          </button>
        </div>
      </div>

      <!-- 滚动回底部悬浮球 -->
      <Transition name="fade">
        <button
          v-if="showScrollBottom"
          class="fab-scroll-bottom"
          @click="scrollToBottom(true)"
          title="回到底部最新消息"
        >
          <ArrowDownIcon />
          <span class="fab-text">最新</span>
        </button>
      </Transition>
    </main>

    <!-- 底部输入与操作中枢 -->
    <footer class="chat-input-cockpit" :class="{ compact: isCompact }">
      <!-- 快捷指令横条 (芯片式工具条) -->
      <div class="input-quick-bar" v-if="!isCompact">
        <div class="quick-chips">
          <button
            v-for="chip in quickChips"
            :key="chip.label"
            class="quick-chip-btn"
            @click="handleQuickChipClick(chip)"
            :title="chip.tip"
          >
            <span class="chip-icon">{{ chip.icon }}</span>
            <span class="chip-label">{{ chip.label }}</span>
          </button>
        </div>

        <div class="status-summary" v-if="isStreaming">
          <span class="live-dot"></span>
          <span class="live-text">{{ currentStreamStatus || '正在智能推理分析中...' }}</span>
        </div>
      </div>

      <!-- 输入框主体 -->
      <div class="input-sender-box">
        <t-chat-sender
          ref="chatSenderRef"
          v-model="inputValue"
          :textarea-props="{
            placeholder: configOptions.length ? '输入个股代码/名称、ETF轮动需求或选股策略... (Enter发送，Shift+Enter换行)' : '⚠️ 请先选择或在设置中添加 AI 模型配置',
            rows: 1,
            autosize: { minRows: 1, maxRows: 6 },
          }"
          :loading="isStreaming"
          :stop-disabled="false"
          @send="handleSend"
          @stop="handleStop"
        >
          <template #suffix>
            <div class="sender-actions">
              <n-button
                v-if="isStreaming"
                type="error"
                size="medium"
                class="stop-btn"
                @click="handleStop"
              >
                ⏹️ 停止生成
              </n-button>
              <n-button
                v-else
                type="primary"
                size="medium"
                :disabled="!inputValue.trim() || !activeConfigId"
                @click="handleSend"
              >
                发送
              </n-button>
            </div>
          </template>
        </t-chat-sender>
      </div>

      <!-- 缺少配置引导 -->
      <div v-if="!configOptions.length" class="config-alert-banner">
        ⚠️ 未检测到有效 AI 模型配置，请前往 <router-link to="/settings">设置中心</router-link> 选配 Ollama / DeepSeek / 硅基流动 等模型。
      </div>
    </footer>

    <!-- 左侧会话历史抽屉 -->
    <n-drawer
      v-model:show="showSessionDrawer"
      :width="drawerWidth"
      placement="left"
    >
      <n-drawer-content title="💬 投研会话记录" closable>
        <div class="session-drawer-content">
          <n-button
            type="primary"
            dashed
            block
            class="btn-new-chat"
            @click="createNewSession"
          >
            ➕ 新建投研对话
          </n-button>

          <div class="session-list">
            <div
              v-for="s in sessions"
              :key="s.id"
              class="session-item"
              :class="{ active: s.id === currentSessionId }"
              @click="switchSession(s.id)"
            >
              <div class="session-info">
                <span class="session-name" :title="s.title">{{ s.title }}</span>
                <span class="session-meta">
                  {{ formatSessionTime(s.updatedAt) }} · {{ s.messages.length }}条
                </span>
              </div>
              <div class="session-actions" @click.stop>
                <n-button
                  quaternary
                  circle
                  size="tiny"
                  @click="deleteSession(s.id)"
                  title="删除此会话"
                >
                  <DeleteIcon />
                </n-button>
              </div>
            </div>
          </div>
        </div>
      </n-drawer-content>
    </n-drawer>

    <!-- 投研场景库弹窗 -->
    <n-modal
      v-model:show="showScenarioModal"
      preset="card"
      title="💡 专业投研场景库"
      style="width: 720px; max-width: 90vw;"
    >
      <div class="modal-scenarios-list">
        <div
          v-for="(item, idx) in allScenarios"
          :key="idx"
          class="scenario-modal-item"
          @click="selectModalScenario(item.prompt)"
        >
          <div class="item-head">
            <span class="item-tag">{{ item.category }}</span>
            <span class="item-title">{{ item.title }}</span>
          </div>
          <p class="item-desc">{{ item.prompt }}</p>
        </div>
      </div>
    </n-modal>

    <!-- 标的快速填装与推演弹窗 (彻底消灭占位符) -->
    <n-modal
      v-model:show="showStockActionModal"
      preset="card"
      :title="stockActionModalTitle"
      style="width: 520px; max-width: 90vw;"
    >
      <div class="stock-action-modal-body">
        <div v-if="targetStockAction === 'pk'" class="pk-modal-inputs">
          <div class="modal-input-field">
            <label class="modal-field-label">标的 A 代码或名称：</label>
            <n-input
              v-model:value="modalStockInput"
              placeholder="如 300750 宁德时代"
              size="medium"
            />
          </div>
          <div class="modal-input-field">
            <label class="modal-field-label">标的 B 代码或名称：</label>
            <n-input
              v-model:value="modalStockInputB"
              placeholder="如 002594 比亚迪"
              size="medium"
            />
          </div>
          <div class="modal-preset-chips">
            <span class="preset-label">热门对比组合：</span>
            <span class="preset-chip" @click="setModalPK('300750', '002594')">宁德 vs 比亚迪</span>
            <span class="preset-chip" @click="setModalPK('600519', '000858')">茅台 vs 五粮液</span>
            <span class="preset-chip" @click="setModalPK('688981', '002371')">中芯 vs 北方华创</span>
          </div>
        </div>

        <div v-else class="single-modal-inputs">
          <label class="modal-field-label">目标标的代码或名称：</label>
          <n-input
            v-model:value="modalStockInput"
            placeholder="输入 6 位代码或名称 (如 600519 贵州茅台)"
            size="medium"
            clearable
          />
          <div class="modal-preset-chips">
            <span class="preset-label">核心龙头快捷选择：</span>
            <span
              v-for="stk in hotStocks"
              :key="stk.code"
              class="preset-chip"
              :class="{ active: modalStockInput === stk.code }"
              @click="modalStockInput = stk.code"
            >
              {{ stk.name }}
            </span>
          </div>
        </div>

        <div class="modal-footer-actions">
          <n-button @click="showStockActionModal = false">取消</n-button>
          <n-button type="primary" :disabled="!isModalStockActionValid" @click="submitModalStockAction">
            🚀 立即启动推演
          </n-button>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeMount, onBeforeUnmount, nextTick, provide } from 'vue'
import {
  NSelect,
  NButton,
  NTooltip,
  NDrawer,
  NDrawerContent,
  NModal,
  NTag,
  NSwitch,
  NInput,
  useMessage,
  useDialog
} from 'naive-ui'
import {
  ArrowDownIcon,
  DeleteIcon,
  FullscreenIcon,
  FullscreenExitIcon,
  ChatIcon,
  AddIcon,
  ViewListIcon
} from 'tdesign-icons-vue-next'
import { ChatWithAgent, GetConfig, GetVersionInfo, GetSystemStateSnapshot, SaveStockThesis, AbortChatWithAgent, SaveImage } from '../../wailsjs/go/main/App'
import html2canvas from 'html2canvas'
import { useAiConfigStore } from '@/stores/aiConfig'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime'
import { cleanFinalContent } from '@/utils/cleanText'
import { copyToClipboard } from '@/utils/clipboard'
import { consumePendingConsult } from '../utils/agentControlBus'
import {
  useMessageAnchor,
  type ChatMessage,
  type StreamChunk,
  genMsgId
} from './chat'
import ChatMessageComp from './chat/ChatMessage.vue'
import MessageAnchorNav from './chat/MessageAnchorNav.vue'

// ========== 数据接口与持久化类型 ==========
interface ChatSession {
  id: string
  title: string
  updatedAt: number
  messages: ChatMessage[]
}

const STORAGE_KEY = 'blades_stock_chat_sessions_v2'
const ACTIVE_SESSION_KEY = 'blades_stock_active_session_id_v2'

// ========== 核心状态 ==========
const message = useMessage()
const dialog = useDialog()

function notifyWarning(msg: string) {
  if (message) message.warning(msg)
  else console.warn(msg)
}
function notifySuccess(msg: string) {
  if (message) message.success(msg)
  else console.log(msg)
}
function notifyError(msg: string) {
  if (message) message.error(msg)
  else console.error(msg)
}
function notifyInfo(msg: string) {
  if (message) message.info(msg)
  else console.log(msg)
}

const aiConfigStore = useAiConfigStore()
const sessions = ref<ChatSession[]>([])
const currentSessionId = ref<string>('')
const inputValue = ref('')
const isStreaming = ref(false)
const streamingMsgId = ref<string | null>(null)
const currentStreamStatus = ref<string>('')
const showScrollBottom = ref(false)
const isCompact = ref(false)
const showSessionDrawer = ref(false)
const showScenarioModal = ref(false)
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
const drawerWidth = computed(() => (isMobile.value ? '85vw' : 320))

// 思考模式与记忆模式
const thinkingMode = ref(true)
const memoryMode = ref(false)
const memoryCount = ref(3)
const memoryCountOptions = [
  { label: '1 轮', value: 1 },
  { label: '2 轮', value: 2 },
  { label: '3 轮', value: 3 },
  { label: '5 轮', value: 5 },
  { label: '10 轮', value: 10 }
]

function onWindowResize() {
  isMobile.value = typeof window !== 'undefined' ? window.innerWidth <= 768 : false
}
const agentMode = ref<'auto' | 'react' | 'plan_execute'>('auto')
const userAvatar = ref('https://tdesign.gtimg.com/site/avatar.jpg')
const userName = ref('用户')
const messagesContainer = ref<HTMLElement | null>(null)

// ========== BUIS 五阶流水线与交互控制中枢状态 ==========
const welcomeViewMode = ref<'pipeline' | 'direct'>('pipeline')
const step4StockInput = ref('600519')
const directDiagCode = ref('600519')
const directRiskCode = ref('600519')
const pkCodeA = ref('300750')
const pkCodeB = ref('002594')

const hotStocks = [
  { code: '600519', name: '贵州茅台' },
  { code: '300750', name: '宁德时代' },
  { code: '002594', name: '比亚迪' },
  { code: '688981', name: '中芯国际' },
  { code: '300059', name: '东方财富' },
  { code: '601318', name: '中国平安' },
  { code: '600900', name: '长江电力' },
]

// 交互式标的填装弹窗状态（彻底消灭占位符）
const showStockActionModal = ref(false)
const targetStockAction = ref<'diag' | 'risk' | 'finance' | 'pk'>('diag')
const modalStockInput = ref('')
const modalStockInputB = ref('')

const stockActionModalTitle = computed(() => {
  if (targetStockAction.value === 'diag') return '🔬 标的全维深度体检'
  if (targetStockAction.value === 'risk') return '🛡️ 股权质押与风险极限排雷'
  if (targetStockAction.value === 'finance') return '📑 经营现金流与真实ROE体检'
  if (targetStockAction.value === 'pk') return '⚖️ 赛道双龙头深度对比 PK'
  return '🎯 标的快速填装'
})

const isModalStockActionValid = computed(() => {
  if (targetStockAction.value === 'pk') {
    return Boolean(modalStockInput.value.trim() && modalStockInputB.value.trim())
  }
  return Boolean(modalStockInput.value.trim())
})

function setModalPK(a: string, b: string) {
  modalStockInput.value = a
  modalStockInputB.value = b
}

function openStockActionModal(action: 'diag' | 'risk' | 'finance' | 'pk') {
  targetStockAction.value = action
  // 若输入框已包含 6 位股票代码，直接复用
  const matched = inputValue.value.match(/\b\d{6}\b/)
  if (matched && action !== 'pk') {
    modalStockInput.value = matched[0]
  } else if (!modalStockInput.value) {
    modalStockInput.value = '600519'
  }
  if (action === 'pk' && !modalStockInputB.value) {
    modalStockInput.value = '300750'
    modalStockInputB.value = '002594'
  }
  showStockActionModal.value = true
}

function submitModalStockAction() {
  if (!isModalStockActionValid.value) return
  const codeA = modalStockInput.value.trim()
  const codeB = modalStockInputB.value.trim()
  showStockActionModal.value = false

  if (targetStockAction.value === 'diag') {
    applyScenario(`【L4 个股全景四维体检】请对股票 ${codeA} 进行全维度量化深度诊断：
1. 优先调用系统内置 DiagnoseStock(${codeA}) 获取 VectorBT 向量化回测置信胜率、量化综合评分与关键支撑阻力位；
2. 调用 GetEastMoneyKLineWithMA(${codeA}, "day", 60) 查看日K线均线系统与量价趋势；
3. 调用 GetStockFinancialInfo(${codeA}) 与 GetStockLatestFinance(${codeA}) 穿透营收利润与真实ROE；
4. 调用 GetStockHolderTrend(${codeA}) 穿透近几个季度股东户数增减与筹码集中度。
【执行纪律】：严禁调用 WebSearch 从外网抓取陈旧碎片。直接输出结构化量化研报，严禁在正文输出"下一步需要调用..."等机械规划文字。`)
  } else if (targetStockAction.value === 'risk') {
    applyScenario(`【L4 筹码与风险极限排雷】请对股票 ${codeA} 进行深度排雷诊断：
1. 调用 GetStockHolderTrend(${codeA}) 穿透股东户数变动趋势，判断主力吸筹或散户接盘；
2. 调用 GetStockMarginTrading(${codeA}) 分析两融杠杆资金多空动向；
3. 核查控股股东股权质押比例、未来一年限售解禁日程、商誉减值与资产负债率风险。
【执行纪律】：严禁调用 WebSearch。直接输出专业排雷研报与风控评级。`)
  } else if (targetStockAction.value === 'finance') {
    applyScenario(`【L4 财报体检与真实ROE验证】请调用系统内置 GetStockFinancialInfo(${codeA}) 与 GetStockLatestFinance(${codeA}) 深度体检财务质量：
1. 对比近3年营业收入与扣非净利润增长趋势；
2. 穿透销售毛利率、真实ROE与净利率水平；
3. 核验经营活动现金流净额与净利润比值，验证其盈利真实性，排查应收账款与坏账风险。
【执行纪律】：严禁调用 WebSearch。直接输出财务健康度体检结论。`)
  } else if (targetStockAction.value === 'pk') {
    applyScenario(`【L4 赛道双龙头深度PK】请调用系统内置量化与行情工具，对比分析股票 ${codeA} 与 ${codeB}：
1. 分别调用 GetEastMoneyKLineWithMA(${codeA}, "day", 60) 与 GetEastMoneyKLineWithMA(${codeB}, "day", 60) 对比量价强弱与均线排列；
2. 分别调用 GetStockLatestFinance 对比两者的市盈率估值、真实ROE与营收成长质量；
3. 分别调用 GetStockHolderTrend 对比两者的筹码集中度变动。
【执行纪律】：严禁调用 WebSearch。直接输出双龙头横向对比报告与配置优选建议。`)
  }
}

// ========== 五阶时序流水线贯通上下文（确保 L1 -> L2 -> L3 -> L4 -> L5 实体数据无缝流转） ==========
const pipelineContext = reactive({
  currentStep: 1,
  stockCode: '600519',
  stockName: '贵州茅台',
  macroStance: '',
  positionCap: '',
  mainTrack: '',
  lastVerdict: '',
})

function setPipelineStock(code: string, name?: string) {
  const trimmed = (code || '').trim()
  if (!trimmed) return
  pipelineContext.stockCode = trimmed
  step4StockInput.value = trimmed
  if (name) {
    pipelineContext.stockName = name.trim()
  } else {
    const found = hotStocks.find(s => s.code === trimmed)
    if (found) pipelineContext.stockName = found.name
  }
}

function onStep4StockInputChange(val: string) {
  const trimmed = (val || '').trim()
  pipelineContext.stockCode = trimmed
  const found = hotStocks.find(s => s.code === trimmed)
  if (found) {
    pipelineContext.stockName = found.name
  }
}

// 从助手回答中智能抽取选股/推荐标的代码与名称，自动贯通到下游体检与立论
function extractStockFromAssistantText(text: string) {
  if (!text) return
  // 1. 优先匹配 Markdown 表格数据行：| 1 | 002371 | 北方华创 |
  const tableMatch = text.match(/\|\s*\d+\s*\|\s*(\d{6})\s*\|\s*([^\|\r\n]+)\s*\|/)
  if (tableMatch && tableMatch[1]) {
    const code = tableMatch[1].trim()
    const name = tableMatch[2].trim().replace(/\s+/g, '')
    setPipelineStock(code, name)
    return
  }
  // 2. 匹配 "北方华创 (002371)"
  const parenMatch = text.match(/([\u4e00-\u9fa5A-Za-z0-9]{2,8})\s*[(（](\d{6})[)）]/)
  if (parenMatch && parenMatch[2]) {
    setPipelineStock(parenMatch[2].trim(), parenMatch[1].trim())
    return
  }
  // 3. 匹配 "002371 北方华创"
  const codeNameMatch = text.match(/(?:\b|\D)(\d{6})(?:\b|\D)\s*([\u4e00-\u9fa5]{2,6})/)
  if (codeNameMatch && codeNameMatch[1]) {
    setPipelineStock(codeNameMatch[1].trim(), codeNameMatch[2].trim())
    return
  }
}

// ========== 五阶时序流水线快速执行方法（全链路绑定 Blades 内置工具与权威数据） ==========
function executeStep1() {
  pipelineContext.currentStep = 1
  const snap = perceptionSnapshot.value
  let realTimeFacts = ''
  if (snap) {
    const lightText = getTrafficText(snap.TrafficLight || snap.trafficLight)
    const phase = snap.Phase || snap.phaseDesc || snap.clockPhase || '正常交易时钟'
    const up = snap.UpCount || snap.upCount || 0
    const down = snap.DownCount || snap.downCount || 0
    const limitUp = snap.LimitUpCount || snap.limitUpCount || 0
    const limitDown = snap.LimitDownCount || snap.limitDownCount || 0
    pipelineContext.macroStance = lightText
    realTimeFacts = `
【系统实时态势感知一手事实（由 blades 引擎刚刚捕获）】：
- 盘面交易节律：${phase}
- 多空红绿灯状态：${lightText} (${snap.TrafficLight || snap.trafficLight})
- 全市场涨跌家数比：上涨 ${up} 家 vs 下跌 ${down} 家
- 涨跌停极端对比：涨停 ${limitUp} 家 vs 跌停 ${limitDown} 家`
  }

  applyScenario(`【L1 宏观大势与仓位择时】${realTimeFacts}
【执行纪律与工具调用要求】：
1. 优先调用系统内置 GetMarketData 获取核心宽基指数（上证指数 000001、沪深300 000300、创业板指 399006）的最新行情截面，严禁调用 WebSearch 从外网抓取过时新闻碎片！
2. 严禁在回答中输出任何"下一步需要调用..."、"我将查询..."等机械思考或规划句子。
3. 直接输出结构化宏观研报：
   - 一、大盘核心指数行情与量价均线态势（调用 GetMarketData）
   - 二、全市场微观多空博弈与红绿灯节律诊断（结合上述系统实盘数据）
   - 三、宏观择时评级与总仓位上限指导（必须给出明确的最高持仓上限百分比，例如 ≤30% 或 ≤70%）。`)
}

function executeStep2(type: 'etf' | 'seats') {
  pipelineContext.currentStep = 2
  if (type === 'etf') {
    applyScenario(`【L2 赛道主线轮动】请调用系统内置 GetETFMomentumRanking 工具获取场内核心 ETF 21日加权动量排名：
1. 严禁调用 WebSearch 从外网搜索。
2. 完整列出动量评分 >0 的正动量进攻候选池，并根据动量衰减度指出当前资金流入最强的主线板块；
3. 对动量 <=0 的防守/规避板块给出调仓规避建议。
4. 直接输出研报正文，严禁输出任何机械调用步骤或规划文字。`)
  } else {
    applyScenario(`【L2 主力风向追踪】请调用系统内置 GetLongTigerList / GetStockBillboard 工具获取最新龙虎榜明细：
1. 严禁使用 WebSearch。
2. 重点筛选机构专用席位大额净买入的股票及所属板块，分析机构资金的真实建仓方向；
3. 排查游资席位是否存在"一日游"打板出逃迹象。
4. 直接输出研报正文，严禁输出机械调用步骤。`)
  }
}

function executeStep3(type: 'breakout' | 'trend' | 'macd' | 'dividend') {
  pipelineContext.currentStep = 3
  if (type === 'breakout') {
    applyScenario(`【L3 量化多因子初筛池 · 放量突破】请调用系统内置 ScreenByStrategy(strategyName="放量突破", limit=10) 工具：
1. 筛选全市场呈现成交量放量突破近期整理平台的强势标的；
2. 严禁调用 WebSearch 搜外网概念股；
3. 输出入选股票的具体代码、名称、技术面评分、现价、涨跌幅、量比与突破确认点位；
4. 综合评选出最优的前3只标的，系统将自动继承首选标的供后续 Step 4 四维深度体检。直接输出选股清单。`)
  } else if (type === 'trend') {
    applyScenario(`【L3 量化多因子初筛池 · 均线多头】请调用系统内置 ScreenByStrategy(strategyName="趋势跟随", limit=10) 工具：
1. 筛选均线多头排列（MA5>MA10>MA20）且趋势持续向上的强势标的；
2. 严禁调用 WebSearch；
3. 输出入选股票的具体代码、名称、技术面评分与均线支撑位，直接输出选股清单。`)
  } else if (type === 'macd') {
    applyScenario(`【L3 量化多因子初筛池 · MACD金叉】请调用系统内置 ScreenByStrategy(strategyName="macd_golden_cross", limit=10) 工具：
1. 筛选近期在零轴上方形成二次金叉放量共振的个股；
2. 严禁调用 WebSearch；
3. 输出入选股票的具体代码、名称与金叉确认理由，直接输出选股清单。`)
  } else {
    applyScenario(`【L3 价值防守选股池】请利用系统内置量化筛选工具（SearchStockByIndicators / FilterStocks）：
1. 筛选市盈率PE<15、近三年平均股息率>3.5%、ROE>12%的高股息稳健型股票；
2. 严禁调用 WebSearch；
3. 评估入选标的的安全边际与抗跌属性，给出防御性底仓配置建议，直接输出选股清单。`)
  }
}

function executeStep4(action: 'diag' | 'risk' | 'finance') {
  pipelineContext.currentStep = 4
  const code = (step4StockInput.value || pipelineContext.stockCode || '600519').trim()
  const name = pipelineContext.stockName || ''
  const targetDesc = name ? `${name} (${code})` : `股票 ${code}`

  if (action === 'diag') {
    applyScenario(`【L4 个股全景深度体检】请对 ${targetDesc} 进行全维度深度量化诊断：
1. 优先调用系统内置 DiagnoseStock(${code}) 获取 VectorBT 向量化回测置信胜率、量化综合评分与第一目标价/止损位；
2. 调用 GetEastMoneyKLineWithMA(${code}, "day", 60) 查看日K线均线系统与关键支撑阻力位；
3. 调用 GetStockFinancialInfo(${code}) 与 GetStockLatestFinance(${code}) 穿透营收利润与真实ROE；
4. 调用 GetStockHolderTrend(${code}) 穿透股东户数集中度与主力资金动向。
【执行纪律】：严禁调用 WebSearch。直接输出专业研报，给出买方 5-Verdict 确定性评级与明确的下行证伪止损线，严禁输出机械思考文字。`)
  } else if (action === 'risk') {
    applyScenario(`【L4 筹码与风险极限排雷】请对 ${targetDesc} 进行深度排雷：
1. 调用 GetStockHolderTrend(${code}) 穿透股东户数集中度趋势；
2. 调用 GetStockMarginTrading(${code}) 分析两融杠杆资金多空动向；
3. 核查控股股东股权质押比例、未来一年限售解禁日程、商誉减值与资产负债率风险。
【执行纪律】：严禁调用 WebSearch。直接输出风控评级与排雷研报。`)
  } else {
    applyScenario(`【L4 财报体检与真实ROE验证】请调用系统内置 GetStockFinancialInfo(${code}) 与 GetStockLatestFinance(${code}) 深度体检 ${targetDesc} 的财务质量：
1. 对比近3年营业收入与扣非净利润增长趋势；
2. 穿透销售毛利率、真实ROE与销售净利率；
3. 核验经营活动现金流净额与净利润比值，验证其盈利真实性，排查应收账款异常。
【执行纪律】：严禁调用 WebSearch。直接输出财务健康度结论。`)
  }
}

function executeStep5(customCode?: string) {
  pipelineContext.currentStep = 5
  const code = (customCode || step4StockInput.value || pipelineContext.stockCode || '600519').trim()
  const name = pipelineContext.stockName || ''
  const targetDesc = name ? `${name} (${code})` : `股票 ${code}`

  applyScenario(`【L5 买方立论与 5-Verdict 裁决】请根据前期宏观大势、赛道动量、量化体检与风控排雷结论，对 ${targetDesc} 作出最终买方投资决断：
1. 明确给出严肃的 5-Verdict 买方评级（【强力买入】、【逢低布局】、【观望等待】、【逢高减持】、【规避离场】）；
2. 提炼核心买入立论假说（Core Thesis，1-2句话高度概括其商业壁垒与成长逻辑）；
3. 明确列出下一阶段重点跟踪的验证催化事件（Key Catalysts）；
4. 设定严格的下行证伪止损底线价位（Risk Factors / Invalidation Price）；
5. 【核心纪律与工具调用】：必须直接调用系统内置 SaveStockThesis 工具，将上述结构化投资假说正式持久化存入系统认知账本数据库！并在回答末尾输出认知账本保存凭据。`)
}

function viewVerdictStandards() {
  applyScenario(`请说明 Blades 买方认知账本与 5-Verdict 评级规范：包括【强力买入、逢低布局、观望等待、逢高减持、规避离场】的评定边界、核心买入立论提取、催化剂假设提取以及下行证伪止损底线的设定方法。`)
}

function executeDirectDiag() {
  const code = (directDiagCode.value || '600519').trim()
  setPipelineStock(code)
  applyScenario(`【L4 极速全维体检】请对股票 ${code} 进行全维度深度诊断：优先调用系统内置 DiagnoseStock(${code}) 获取 VectorBT 量化评分与置信胜率，调用 GetEastMoneyKLineWithMA(${code}, "day", 60) 查看均线量价，调用 GetStockLatestFinance 查看财务质量。严禁调用 WebSearch。直接给出买方 5-Verdict 裁决与止损位。`)
}

function executeDirectRisk() {
  const code = (directRiskCode.value || '600519').trim()
  setPipelineStock(code)
  applyScenario(`【L4 专项极限排雷】请对股票 ${code} 进行深度排雷：调用 GetStockHolderTrend(${code}) 查看股东户数集中度，调用 GetStockMarginTrading(${code}) 查看两融杠杆，核查股权质押与限售解禁日程。严禁调用 WebSearch。直接输出综合排雷评级。`)
}

function executeDirectPK() {
  const a = (pkCodeA.value || '300750').trim()
  const b = (pkCodeB.value || '002594').trim()
  applyScenario(`【L4 双龙头横向对比】请调用系统内置量化工具对比 ${a} 与 ${b}：分别对比两者的日K量价走势（GetEastMoneyKLineWithMA）、最新财务ROE与估值（GetStockLatestFinance）、以及筹码集中度（GetStockHolderTrend）。严禁调用 WebSearch。直接输出双龙对比结论。`)
}

function applyPresetPK(a: string, b: string) {
  pkCodeA.value = a
  pkCodeB.value = b
}

function handleQuickChipClick(chip: any) {
  if (typeof chip.action === 'function') {
    chip.action()
  } else if (chip.text) {
    fillInput(chip.text)
  }
}

// 下游时序推荐计算：根据最近一条助手的输出推演内容，智能推荐下一步动作（自动继承贯通标的）
const pipelineNextStep = computed(() => {
  if (messages.value.length === 0) return null
  const lastMsg = messages.value[messages.value.length - 1]
  if (lastMsg.role !== 'assistant' || !lastMsg.content) return null
  const text = lastMsg.content

  const currentCode = pipelineContext.stockCode || '600519'
  const currentName = pipelineContext.stockName || ''
  const displayTarget = currentName ? `${currentName} (${currentCode})` : currentCode

  // 1. 如果是宏观大势或 ETF 动量分析 -> 推荐进入 Step 3 进行多因子选股
  if (text.includes('动量') || text.includes('ETF') || text.includes('大势') || text.includes('择时') || text.includes('宏观')) {
    return {
      label: '👉 进入 Step 3 · 运行多因子放量突破量化选股',
      action: () => executeStep3('breakout'),
    }
  }
  // 2. 如果是选股初筛池 -> 推荐进入 Step 4 对筛选出的标的进行全维体检
  if (text.includes('选股') || text.includes('突破') || text.includes('初筛') || text.includes('高股息') || text.includes('多头排列')) {
    return {
      label: `👉 进入 Step 4 · 对标的「${displayTarget}」启动四维体检与极限排雷`,
      action: () => executeStep4('diag'),
    }
  }
  // 3. 如果是个股体检、排雷或财报分析 -> 推荐进入 Step 5 做出终局买方立论并写入账本
  if (text.includes('诊断') || text.includes('体检') || text.includes('排雷') || text.includes('5-Verdict') || text.includes('VectorBT')) {
    return {
      label: `👉 进入 Step 5 · 对「${displayTarget}」做出买方 5-Verdict 裁决并存入认知账本`,
      action: () => executeStep5(),
    }
  }
  return null
})

// 响应式消息列表：直接绑定当前激活会话的 messages
const messages = computed<ChatMessage[]>({
  get() {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    return s ? s.messages : []
  },
  set(val: ChatMessage[]) {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    if (s) {
      s.messages = val
      s.updatedAt = Date.now()
      saveSessionsToStorage()
    }
  }
})

const currentSessionTitle = computed(() => {
  const s = sessions.value.find(item => item.id === currentSessionId.value)
  return s ? s.title : '默认对话'
})

// ========== AI 配置计算 ==========
const activeConfigId = computed({
  get: () => aiConfigStore.activeConfigId,
  set: (val) => { aiConfigStore.setActiveConfigId(val) }
})

const configOptions = computed(() => aiConfigStore.configs.map(c => ({
  label: `${c.name || '配置' + c.ID} (${c.modelName || '默认模型'})`,
  value: c.ID,
})))

// ========== 全局宏观态势感知 ==========
const perceptionSnapshot = ref<any>(null)
let perceptionTimer: any = null

async function refreshPerceptionSnapshot() {
  try {
    const snap = await GetSystemStateSnapshot()
    if (snap) {
      perceptionSnapshot.value = snap
    }
  } catch (e) {
    console.warn('[agent-chat] refreshPerceptionSnapshot failed:', e)
  }
}

function getTrafficColor(light?: string) {
  if (light === 'SAFE') return '#18a058'
  if (light === 'CAUTION') return '#f0a020'
  if (light === 'DANGER') return '#d03050'
  return '#888'
}

function getTrafficText(light?: string) {
  if (light === 'SAFE') return '积极做多'
  if (light === 'CAUTION') return '谨慎防守'
  if (light === 'DANGER') return '强风控空仓'
  return '中性观望'
}

function askMacroPerception() {
  if (!perceptionSnapshot.value) return
  const phase = perceptionSnapshot.value.Phase || ''
  const traffic = perceptionSnapshot.value.TrafficLight || ''
  inputValue.value = `请结合当前市场【${phase}】节律与情绪【${traffic}】状态，对全天大局观、主线机会和实盘持仓风控进行综合推演。`
  agentMode.value = 'plan_execute'
  nextTick(() => {
    handleSend()
  })
}

const agentModeOptions = [
  { label: '🤖 智能自适应', value: 'auto' as const, description: '根据问题复杂度自动选择单步或多步执行策略' },
  { label: '⚡ 极速诊断', value: 'react' as const, description: 'ReAct 即时单步调用，秒级响应，适合快速行情问答' },
  { label: '🧠 深度规划', value: 'plan_execute' as const, description: 'Plan-and-Execute 深度多步拆解，适合全维度复杂分析' },
  { label: '📑 买方直出', value: 'direct' as const, description: '原生直连生成机构深度研报，零外部工具开销' },
]

// ========== 投研场景库数据（按 BUIS 投研决策五阶天梯规范化） ==========
const scenarioCards = [
  {
    icon: '🧭',
    badge: 'L2 主线轮动',
    title: 'ETF 加权动量轮动',
    desc: '计算全市场核心宽基与行业 ETF 21日加权动量评分，捕捉进攻主线。',
    example: '计算当前 ETF 动量排名',
    prompt: '请调用系统内置 GetETFMomentumRanking 工具获取当前场内核心 ETF 21日加权动量排名，严禁使用 WebSearch。根据动量绝对值（>0进攻，<=0防守）全面分析最强进攻赛道与防御调仓建议。直接输出研报正文，严禁输出机械调用步骤。',
  },
  {
    icon: '🎯',
    badge: 'L3 量化选股',
    title: '多因子策略选股',
    desc: '基于均线多头排列、放量突破或高股息防守因子，全市场量化初筛。',
    example: '筛选均线多头放量突破个股',
    prompt: '请利用系统内置量化筛选工具（SearchStockByIndicators / FilterStocks / HotStrategyTable），筛选市场上呈现均线多头排列且成交量放量突破近期整理平台的强势标的，严禁使用 WebSearch。给出具体技术面评分与入选逻辑。直接输出研报正文。',
  },
  {
    icon: '🔬',
    badge: 'L4 全维诊断',
    title: '个股全景深度体检',
    desc: '四维穿透财务基本面、量价均线形态、大资金动向与估值弹性。',
    example: '深度分析 600519 贵州茅台',
    prompt: '请对 600519 贵州茅台 进行全维度深度诊断：优先调用系统内置 DiagnoseStock(600519) 获取 VectorBT 向量化回测置信胜率与量化评分，调用 GetEastMoneyKLineWithMA(600519, "day", 60) 查看量价均线，调用 GetStockFinancialInfo 穿透真实财务。严禁使用 WebSearch。直接输出专业量化研报并给出买方 5-Verdict 裁决与证伪底线。',
  },
  {
    icon: '🛡️',
    badge: 'L4 微观排雷',
    title: '筹码与风控极限穿透',
    desc: '深度穿透大股东质押率、解禁日程、股东户数集中度与财务真实性。',
    example: '深度排雷 600519 贵州茅台',
    prompt: '请对 600519 贵州茅台 进行深度排雷：调用 GetStockHolderTrend(600519) 穿透股东户数集中度，调用 GetStockMarginTrading 分析两融杠杆，核查控股股东质押比例、未来一年解禁日程及主力资金动向，严禁使用 WebSearch。给出综合风控评级。',
  },
]

const quickKeywords = [
  '计算 场内ETF动量排名',
  '筛选 均线多头放量突破股',
  '筛选 低估值高股息稳健股',
  '深度分析 600519 贵州茅台',
  '深度量化排雷 000001',
  '财报真实体检 300750',
  '宁德时代 vs 比亚迪',
  '半导体算力产业链景气度',
]

const quickChips = [
  { icon: '🏛️', label: 'L1 大势择时', action: () => executeStep1(), tip: '评估大盘择时、估值水位与仓位上限' },
  { icon: '🧭', label: 'L2 ETF动量', action: () => executeStep2('etf'), tip: '计算场内主流 ETF 21日动量排名' },
  { icon: '🎯', label: 'L3 突破选股', action: () => executeStep3('breakout'), tip: '量化筛选均线多头放量突破股' },
  { icon: '🔬', label: 'L4 个股体检', action: () => openStockActionModal('diag'), tip: '四维穿透基本面、量价与资金' },
  { icon: '🛡️', label: 'L4 深度排雷', action: () => openStockActionModal('risk'), tip: '穿透大股东质押、解禁与股东户数' },
  { icon: '📑', label: 'L4 财报体检', action: () => openStockActionModal('finance'), tip: '验证扣非净利润与经营现金流真实性' },
  { icon: '⚖️', label: 'L4 双龙头PK', action: () => openStockActionModal('pk'), tip: '对比同行业两只核心龙头' },
  { icon: '🏛️', label: 'L2 机构席位', action: () => executeStep2('seats'), tip: '查询龙虎榜机构专用席位大额建仓' },
]

const allScenarios = [
  // L1 宏观大势与环境感知
  { category: 'L1 大势与宽基', title: '核心宽基行情与量价均线穿透', prompt: '请调用系统内置 GetMarketData 获取上证指数(000001)、沪深300(000300)与创业板指(399006)的最新点位、涨跌幅与量价均线态势，结合全市场涨跌比，评估市场多空赚钱效应与总仓位上限。严禁调用 WebSearch。' },
  
  // L2 赛道主线与资金风向
  { category: 'L2 赛道与主线', title: 'ETF 加权动量轮动模型', prompt: '请调用系统内置 GetETFMomentumRanking 获取主流场内ETF的21日加权动量排名表，按动量评分排序并给出当前最具超额弹性的进攻主线与防御调仓建议。严禁调用 WebSearch。直接输出研报正文。' },
  { category: 'L2 赛道与主线', title: '重点产业链知识图谱研判', prompt: '请调用系统内置 GetIndustrySupplyGraph / QuerySupplyChainCascade 工具深入分析半导体及算力产业链的上下游供需格局、核心受益梯队与暴露度。严禁使用 WebSearch。' },
  { category: 'L2 赛道与主线', title: '龙虎榜机构专用席位追踪', prompt: '请调用系统内置 GetLongTigerList / GetStockBillboard 工具获取最近一个交易日的龙虎榜明细，重点筛选机构专用席位大额净买入的股票，分析机构资金的真实建仓意图。严禁使用 WebSearch。' },

  // L3 量化多因子选股池
  { category: 'L3 量化选股池', title: '强势平台放量突破选股', prompt: '请利用系统内置量化筛选工具（SearchStockByIndicators / FilterStocks / HotStrategyTable），筛选突破近期整理平台且放量上涨、均线多头排列的强势标的，给出具体技术面评分与入选逻辑。严禁使用 WebSearch。' },
  { category: 'L3 量化选股池', title: '低估值高股息价值防守', prompt: '请利用系统内置指标工具筛选市盈率PE<15、近三年股息率>3.5%、ROE>12%的高股息稳健型股票，评估其安全边际与抗跌属性。严禁使用 WebSearch。' },
  { category: 'L3 量化选股池', title: '业绩预告超预期挖掘', prompt: '请检索近期披露业绩预告中净利润同比大幅预增的公司，结合当前估值与成长确定性给出高性价比标的分析。严禁使用 WebSearch。' },

  // L4 微观解构与极限制约
  { category: 'L4 个股深度解构', title: '全景四维深度体检', prompt: '请对 [输入代码] 进行全方位深度诊断：优先调用 DiagnoseStock 获取 VectorBT 回测胜率与量化评分，调用 GetEastMoneyKLineWithMA 查看量价均线，调用 GetStockFinancialInfo 穿透财务真实性，给出买方 5-Verdict 确定性评级与止损线。严禁使用 WebSearch。' },
  { category: 'L4 个股深度解构', title: '深度排雷与筹码穿透', prompt: '请对 [输入代码] 进行深度排雷：调用 GetStockHolderTrend 穿透股东户数集中度，调用 GetStockMarginTrading 分析两融多空动向，核查控股股东质押比例与限售解禁日程。严禁使用 WebSearch。' },
  { category: 'L4 个股深度解构', title: '财报体检与真实ROE验证', prompt: '请调用系统内置 GetStockFinancialInfo 与 GetStockLatestFinance 深入体检 [输入代码] 的财务报表：对比近3年营业收入与扣非净利润、ROE与销售净利率、分析经营现金流净额比值，验证其盈利真实性。严禁使用 WebSearch。' },
  { category: 'L4 个股深度解构', title: '关键支撑阻力位测算', prompt: '请调用 GetEastMoneyKLineWithMA 分析 [输入代码] 的近期日K线走势与成交密集区，精确测算关键支撑位、压力位、止损参考点位以及当前买卖盈亏比。严禁使用 WebSearch。' },
  { category: 'L4 个股深度解构', title: '同业龙头双股深度PK', prompt: '请调用内置量化与行情工具对比分析 [代码A] 与 [代码B]：横向比较两者的日K走势强弱（GetEastMoneyKLineWithMA）、最新财务ROE与成长质量（GetStockLatestFinance）以及筹码集中度（GetStockHolderTrend）。严禁使用 WebSearch。' },

  // L5 买方立论与风控
  { category: 'L5 买方决策风控', title: '标的 5-Verdict 裁决与证伪线', prompt: '请对 [输入代码] 给出买方确定性评级（强力买入/逢低布局/观望等待/逢高减持/规避离场），提炼核心买入立论、催化剂假设、以及明确的下行证伪止损底线，并说明如何调用 SaveStockThesis 存入认知账本。' },
]

// ========== 锚点导航 ==========
const {
  anchorNavItems,
  registerElement,
  unregisterElement,
  scrollToMessage,
} = useMessageAnchor(messages)

provide('registerMessageElement', registerElement)
provide('unregisterMessageElement', unregisterElement)

// ========== 本地会话持久化逻辑 ==========
function loadSessionsFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      sessions.value = JSON.parse(raw)
    }
  } catch (e) {
    console.error('Failed to parse sessions from localStorage', e)
  }

  if (!sessions.value || sessions.value.length === 0) {
    const defaultSession: ChatSession = {
      id: genMsgId('session'),
      title: '新对话',
      updatedAt: Date.now(),
      messages: [],
    }
    sessions.value = [defaultSession]
    currentSessionId.value = defaultSession.id
  } else {
    const savedActive = localStorage.getItem(ACTIVE_SESSION_KEY)
    const exists = sessions.value.find(s => s.id === savedActive)
    currentSessionId.value = exists ? exists.id : sessions.value[0].id
  }
}

function saveSessionsToStorage() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions.value))
    localStorage.setItem(ACTIVE_SESSION_KEY, currentSessionId.value)
  } catch (e) {
    console.error('Failed to save sessions to localStorage', e)
  }
}

function createNewSession() {
  const newSession: ChatSession = {
    id: genMsgId('session'),
    title: '新对话',
    updatedAt: Date.now(),
    messages: [],
  }
  sessions.value.unshift(newSession)
  currentSessionId.value = newSession.id
  saveSessionsToStorage()
  showSessionDrawer.value = false
}

function switchSession(id: string) {
  currentSessionId.value = id
  saveSessionsToStorage()
  showSessionDrawer.value = false
  nextTick(() => {
    scrollToBottom(false)
  })
}

function deleteSession(id: string) {
  if (sessions.value.length <= 1) {
    const s = sessions.value[0]
    s.title = '新对话'
    s.messages = []
    s.updatedAt = Date.now()
    saveSessionsToStorage()
    return
  }
  sessions.value = sessions.value.filter(s => s.id !== id)
  if (currentSessionId.value === id) {
    currentSessionId.value = sessions.value[0].id
  }
  saveSessionsToStorage()
}

function formatSessionTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

// ========== Agent 流式通信监听 ==========
function handleAgentMessage(data: StreamChunk) {
  if (!isStreaming.value) return

  const curList = messages.value
  const targetMsg = curList.find(m => m.id === streamingMsgId.value)
  if (!targetMsg) return

  if (data.role === 'assistant') {
    if ((data as any).model_name || (data as any).modelName) {
      targetMsg.modelName = ((data as any).model_name || (data as any).modelName) + ' (容灾降级)'
    }
    // 状态更新
    if (data.reasoning_content) {
      const rc = data.reasoning_content
      if (rc.startsWith('[STATUS]')) {
        const st = rc.replace(/^\[STATUS\]/, '').trim()
        targetMsg.statusText = st
        currentStreamStatus.value = st
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!targetMsg.steps) targetMsg.steps = []
          targetMsg.steps.push(stepText)
        }
      } else {
        targetMsg.rawReasoning = (targetMsg.rawReasoning || '') + rc
        targetMsg.reasoning = targetMsg.rawReasoning
      }
    }

    // 正文流式追加
    if (data.content) {
      if (data.content === 'agent-DONE') {
        if (data.metrics) targetMsg.metrics = data.metrics
      } else {
        targetMsg.rawContent = (targetMsg.rawContent || '') + data.content
        targetMsg.content = targetMsg.rawContent
      }
    }

    // 工具调用追踪
    if (data.tool_calls && data.tool_calls.length > 0) {
      if (!targetMsg.toolCalls) targetMsg.toolCalls = []
      targetMsg.toolCalls.push(...data.tool_calls)
    }

    if (data.metrics) {
      targetMsg.metrics = data.metrics
    }

    // 智能滚动吸底
    autoScrollIfNearBottom()

    // 结束处理
    if (data.response_meta?.finish_reason === 'stop') {
      finalizeStream(targetMsg)
    }
  }
}

function finalizeStream(targetMsg?: ChatMessage) {
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''

  if (targetMsg) {
    if (targetMsg.rawContent) {
      targetMsg.content = cleanFinalContent(targetMsg.rawContent)
    }
    // 智能抽取分析中产出的个股实体（代码与名称），自动贯通流水线上下文供后续阶段使用
    if (targetMsg.content) {
      extractStockFromAssistantText(targetMsg.content)
    }
  }
  saveSessionsToStorage()
}

// ========== 滚动控制 ==========
let userIsScrolledUp = false
let scrollRafId: number | null = null

function handleScroll() {
  const el = messagesContainer.value
  if (!el) return
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  userIsScrolledUp = distanceFromBottom > 150
  showScrollBottom.value = userIsScrolledUp
}

function autoScrollIfNearBottom() {
  if (userIsScrolledUp) return
  if (scrollRafId !== null) return
  // 使用 requestAnimationFrame 节流，与浏览器刷新率 (60fps/120fps) 严格同步，彻底消除流式高频触发的掉帧与抖动
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = null
    const el = messagesContainer.value
    if (el && !userIsScrolledUp) {
      // 流式生成中必须使用瞬时赋值，严禁 smooth 滚动（smooth 与高频新 Token 会产生持续打断震荡）
      el.scrollTop = el.scrollHeight
    }
  })
}

function scrollToBottom(smooth = false) {
  nextTick(() => {
    const el = messagesContainer.value
    if (el) {
      if (smooth) {
        el.scrollTo({
          top: el.scrollHeight,
          behavior: 'smooth',
        })
      } else {
        el.scrollTop = el.scrollHeight
      }
    }
  })
}

// ========== 发送与停止 ==========
function handleSend() {
  if (isStreaming.value) {
    return
  }

  const text = inputValue.value.trim()
  if (!text) return

  const configId = activeConfigId.value
  if (!configId || configId <= 0) {
    notifyWarning('请先选择有效的 AI 模型配置')
    return
  }

  // 1. 用户消息放入列表末尾 (正序)
  const userMsg: ChatMessage = {
    id: genMsgId('user'),
    avatar: userAvatar.value,
    name: userName.value,
    datetime: new Date().toISOString(),
    content: text,
    role: 'user',
  }

  // 2. AI 占位消息放入列表末尾
  const assistantId = genMsgId('assistant')
  const assistantMsg: ChatMessage = {
    id: assistantId,
    avatar: '',
    name: 'Blades 投研 Agent',
    datetime: new Date().toISOString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    role: 'assistant',
    steps: [],
    statusText: '思考中...',
    toolCalls: [],
  }

  const s = sessions.value.find(item => item.id === currentSessionId.value)
  if (s) {
    s.messages.push(userMsg, assistantMsg)
    // 自动重命名默认会话标题
    if (s.title === '新对话' && text.length > 0) {
      s.title = text.slice(0, 18) + (text.length > 18 ? '…' : '')
    }
    s.updatedAt = Date.now()
    saveSessionsToStorage()
  }

  streamingMsgId.value = assistantId
  isStreaming.value = true
  currentStreamStatus.value = '正在思考并解析意图...'
  userIsScrolledUp = false

  inputValue.value = ''
  // 发送消息后瞬时定位到底部，禁止平滑动画（平滑动画尚未结束首包 Token 就已到达，会导致持续打断震荡）
  scrollToBottom(false)

  ChatWithAgent(
    text,
    configId,
    null as any,
    !!memoryMode.value,
    Number(memoryCount.value) || 0,
    !!thinkingMode.value,
    agentMode.value === 'auto' ? '' : agentMode.value,
    currentSessionId.value || ''
  ).catch((err: any) => {
    console.error('ChatWithAgent error', err)
    assistantMsg.content = `❌ 服务调用失败：${err?.message || err}`
    finalizeStream(assistantMsg)
  })
}

function handleStop() {
  try {
    AbortChatWithAgent()
  } catch (e) {
    console.warn('AbortChatWithAgent failed:', e)
  }
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''
  saveSessionsToStorage()
  notifyInfo('已停止响应')
}

// ========== 场景交互辅助 ==========
function applyScenario(promptText: string) {
  inputValue.value = promptText
  nextTick(() => {
    handleSend()
  })
}

function fillInput(text: string) {
  inputValue.value = text
  const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
  if (textarea) textarea.focus()
}

function selectModalScenario(prompt: string) {
  inputValue.value = prompt
  showScenarioModal.value = false
  const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
  if (textarea) textarea.focus()
}

async function handleCopy(content: string) {
  const text = (content || '').trim()
  if (!text) {
    notifyWarning('暂无可复制的内容')
    return
  }
  const ok = await copyToClipboard(text)
  if (ok) notifySuccess('内容已复制到剪贴板')
  else notifyError('复制失败，请手动选取文本')
}

function handleRetry(msgId: string) {
  const curList = messages.value
  const idx = curList.findIndex(m => m.id === msgId)
  if (idx === -1) return
  const msg = curList[idx]
  let userText = ''
  let sliceIdx = idx

  if (msg.role === 'assistant') {
    // 寻找前面最近一条对应的用户提问
    for (let i = idx - 1; i >= 0; i--) {
      if (curList[i].role === 'user') {
        userText = curList[i].content
        sliceIdx = i
        break
      }
    }
  } else if (msg.role === 'user') {
    userText = msg.content
    sliceIdx = idx
  }

  if (!userText) {
    notifyWarning('未找到可重试的提问内容')
    return
  }

  inputValue.value = userText
  messages.value = curList.slice(0, sliceIdx)
  saveSessionsToStorage()
  nextTick(() => {
    handleSend()
  })
}

function handleReference(msgId: string) {
  const msg = messages.value.find(m => m.id === msgId)
  if (!msg) return
  const raw = (msg.content || msg.rawContent || '').trim()
  if (!raw) {
    notifyWarning('暂无内容可引用')
    return
  }
  const quote = raw.slice(0, 180).replace(/\n+/g, ' ')
  const author = msg.role === 'assistant' ? (msg.name || 'AI 智能体') : '我'
  const prefix = inputValue.value.trim() ? inputValue.value.trim() + '\n\n' : ''
  inputValue.value = `${prefix}> 引用【${author}】：${quote}...\n\n`
  notifySuccess('已引用到输入框')
  nextTick(() => {
    const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
    if (textarea) {
      textarea.focus()
      textarea.scrollTop = textarea.scrollHeight
    }
  })
}

function handleEdit(msg: ChatMessage) {
  if (msg.role === 'user') {
    inputValue.value = msg.content
    messages.value = messages.value.filter(m => m.id !== msg.id)
  }
}

function handleResend(msg: ChatMessage) {
  handleEdit(msg)
  handleSend()
}

function onConfigChange(val: number) {
  activeConfigId.value = val
  aiConfigStore.setActiveConfigId(val)
}

function clearCurrentHistory() {
  if (dialog) {
    dialog.warning({
      title: '清空当前对话',
      content: '确定要清空当前会话的全部消息历史吗？此操作不可恢复。',
      positiveText: '确认清空',
      negativeText: '取消',
      onPositiveClick: () => {
        messages.value = []
        saveSessionsToStorage()
        notifySuccess('会话已清空')
      }
    })
  } else {
    messages.value = []
    saveSessionsToStorage()
    notifySuccess('会话已清空')
  }
}

// ========== 跨组件咨询接入与买方立论记忆入库 ==========
const lastConsultStock = ref<{ code: string; name: string } | null>(null)
const processedConsultIds = new Set<string>()
let lastConsultSignature = ''
let lastConsultTimestamp = 0

async function handleAgentConsult(payload: any) {
  if (!payload) return
  const { id: consultId, question, stockCode, stockName, context, mode, autoSend } = payload
  if (!question && !context) return

  // 1. 去重守卫：防止路由跳转与自定义事件并发导致的重复消费与竞态
  if (consultId && processedConsultIds.has(consultId)) {
    return
  }
  const currentSig = `${stockCode || ''}_${stockName || ''}_${(question || '').slice(0, 50)}`
  const now = Date.now()
  if (currentSig === lastConsultSignature && now - lastConsultTimestamp < 1500) {
    return
  }
  if (consultId) {
    processedConsultIds.add(consultId)
  }
  lastConsultSignature = currentSig
  lastConsultTimestamp = now

  // 2. 记录最近一次咨询的标的上下文，供后续买方立论归档提取兜底
  if (stockCode || stockName) {
    lastConsultStock.value = {
      code: stockCode || '',
      name: stockName || ''
    }
  }

  // 3. 设置执行模式
  if (mode && ['auto', 'react', 'plan_execute', 'direct'].includes(mode)) {
    agentMode.value = mode as any
  }

  // 4. 组装输入框文本 (一手量化指标前置注入 + 深度分析推演任务)
  let finalInput = ''
  if (context) {
    finalInput = `【一手量化与买方参考数据】：\n${context}\n\n【分析推演任务】：\n${question || '请结合上述一手量化参考数据，给出全维度买方深度研报与买方立论。'}`
  } else {
    finalInput = question || ''
  }

  inputValue.value = finalInput

  // 5. 自动触发分析
  if (autoSend !== false) {
    // 若 AI 配置正在异步拉取中，等待最多 500ms 确保配置就绪
    if (!activeConfigId.value || activeConfigId.value <= 0) {
      for (let i = 0; i < 5; i++) {
        await new Promise(resolve => setTimeout(resolve, 100))
        if (activeConfigId.value && activeConfigId.value > 0) break
      }
    }
    nextTick(() => {
      handleSend()
    })
  }
}

function onWindowAgentConsult(e: any) {
  if (e?.detail) {
    handleAgentConsult(e.detail)
  }
}

// 智能提炼买方 5-Verdict 认知结构 (立论、假说、催化、证伪风险)
function extractThesisFromText(text: string, defaultCode?: string, defaultName?: string) {
  if (!text) return null

  // 1. 识别股票代码 (标准 6 位 A 股/ETF 代码，支持兼容 sh/sz 前缀、.SH/.SZ 后缀以及 Markdown 标记)
  let code = defaultCode || ''
  if (!code) {
    // 优先匹配标准 6 位代码
    const codeMatch = text.match(/(?:sh|sz|bj)?\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b(?:\.(?:SH|SZ|BJ))?/i)
    if (codeMatch) {
      code = codeMatch[1]
    } else if (lastConsultStock.value?.code) {
      code = lastConsultStock.value.code
    } else {
      // 从会话历史反向追溯最近提及的标准股票代码
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const histText = messages.value[i].content || messages.value[i].rawContent || ''
        const histMatch = histText.match(/(?:sh|sz|bj)?\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b(?:\.(?:SH|SZ|BJ))?/i)
        if (histMatch) {
          code = histMatch[1]
          break
        }
      }
    }
  }
  if (!code) return null

  // 2. 识别股票名称 (支持多种常见 Markdown、括号与标的引导词格式)
  let name = defaultName || ''
  if (!name) {
    const namePatterns = [
      /[【(（]([A-Za-z\u4e00-\u9fa5\d]+)[)）】]\s*(?:[(（]?(?:sh|sz|bj)?\b\d{6}\b(?:\.(?:SH|SZ|BJ))?[)）]?)/,
      /【([A-Za-z\u4e00-\u9fa5\d]+)\s*[0-9]{6}】/,
      /(?:标的(?:名称|代码)?|股票(?:名称)?|分析标的)[：:\s*]+(?:\*\*)?([A-Za-z\u4e00-\u9fa5\d]+)(?:\*\*)?/,
      /(?:\*\*|__)([A-Za-z\u4e00-\u9fa5\d]+)(?:\*\*|__)\s*[(（]\s*\d{6}\s*[)）]/,
      /([A-Za-z\u4e00-\u9fa5\d]+)\s*[(（]\s*(?:sh|sz|bj)?\d{6}(?:\.(?:SH|SZ|BJ))?\s*[)）]/
    ]
    for (const pat of namePatterns) {
      const match = text.match(pat)
      if (match && match[1] && match[1].length >= 2 && match[1].length <= 12) {
        name = match[1].trim()
        break
      }
    }
    if (!name && lastConsultStock.value?.name) {
      name = lastConsultStock.value.name
    }
    if (!name && code) {
      // 从会话历史反向追溯股票名称
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const histText = messages.value[i].content || messages.value[i].rawContent || ''
        for (const pat of namePatterns) {
          const match = histText.match(pat)
          if (match && match[1] && match[1].length >= 2 && match[1].length <= 12) {
            name = match[1].trim()
            break
          }
        }
        if (name) break
      }
    }
  }

  // 3. 识别买方五维定性判定 (多维金融自然语言模糊匹配)
  let verdict = 'NO_CHANGE'
  let verdictText = '维持现状'
  if (
    text.includes('逻辑强化') ||
    text.includes('THESIS_STRENGTHENED') ||
    text.includes('强烈买入') ||
    text.includes('积极做多') ||
    text.includes('超预期') ||
    text.includes('突破确认') ||
    text.includes('多头共振')
  ) {
    verdict = 'THESIS_STRENGTHENED'
    verdictText = '逻辑强化'
  } else if (
    text.includes('逻辑证伪') ||
    text.includes('THESIS_BROKEN') ||
    text.includes('清仓止损') ||
    text.includes('破位止损') ||
    text.includes('强风控空仓') ||
    text.includes('假说破灭')
  ) {
    verdict = 'THESIS_BROKEN'
    verdictText = '逻辑证伪'
  } else if (
    text.includes('逻辑削弱') ||
    text.includes('THESIS_WEAKENED') ||
    text.includes('谨慎防守') ||
    text.includes('逢高减仓') ||
    text.includes('不及预期') ||
    text.includes('动能衰减') ||
    text.includes('冲高回落')
  ) {
    verdict = 'THESIS_WEAKENED'
    verdictText = '逻辑削弱'
  } else if (
    text.includes('首次建档') ||
    text.includes('新论点') ||
    text.includes('首次立论') ||
    text.includes('NEW_THESIS') ||
    text.includes('首次覆盖') ||
    text.includes('新建仓')
  ) {
    verdict = 'NEW_THESIS'
    verdictText = '首次立论'
  }

  // 4. 提取核心投资假说 (过滤 Markdown 符号)
  let coreThesis = ''
  const thesisMatch = text.match(/(?:核心投资逻辑|核心假说|核心立论|投资逻辑|核心论点|买方论据)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (thesisMatch) {
    coreThesis = thesisMatch[1].replace(/[#*`]/g, '').trim()
  } else {
    // 提取正文第一段有实质意义的内容
    const cleanLines = text.split('\n')
      .map(l => l.replace(/[#*`]/g, '').trim())
      .filter(l => l.length > 15 && !l.startsWith('好的') && !l.startsWith('收到') && !l.includes('免责声明'))
    coreThesis = cleanLines.length > 0 ? cleanLines[0].slice(0, 160) : text.slice(0, 160).replace(/[#*`]/g, '').trim()
  }

  // 5. 提取重点跟踪催化剂
  let keyCatalysts = ''
  const catMatch = text.match(/(?:催化剂|跟踪催化剂|核心催化|催化事件|关键催化|关键事件)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (catMatch) {
    keyCatalysts = catMatch[1].replace(/[#*`]/g, '').trim()
  }

  // 6. 提取证伪条件与防守止损
  let riskFactors = ''
  const riskMatch = text.match(/(?:证伪条件|风险因素|反转条件|止损条件|防守位|黑天鹅风险|下行风险)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (riskMatch) {
    riskFactors = riskMatch[1].replace(/[#*`]/g, '').trim()
  }

  return {
    code,
    name: name || code,
    verdict,
    verdictText,
    coreThesis,
    keyCatalysts,
    riskFactors,
  }
}

// 消息卡片点击「存入买方认知账本」
async function handleSaveThesisFromMsg(msg: ChatMessage) {
  const content = msg.content || msg.rawContent || ''
  const thesis = extractThesisFromText(content)
  if (!thesis || !thesis.code) {
    notifyWarning('未在分析结果中识别到明确的股票代码或买方论点')
    return
  }

  try {
    const activeCfg = aiConfigStore.activeConfig
    const modelName = activeCfg?.modelName || 'Blades-Agent'
    await SaveStockThesis(
      thesis.code,
      thesis.name || thesis.code,
      thesis.verdict,
      thesis.verdictText,
      thesis.coreThesis,
      thesis.keyCatalysts,
      thesis.riskFactors,
      modelName
    )
    notifySuccess(`🎉 已将【${thesis.name || thesis.code}】买方认知(${thesis.verdictText})归档入库！`)
    // 广播标的认知更新事件，驱动自选股卡片与诊断抽屉实时刷新
    EventsEmit('stockThesisUpdated', { stockCode: thesis.code })
  } catch (err: any) {
    console.error('SaveStockThesis error:', err)
    notifyError(`存入买方认知账本失败: ${err?.message || err}`)
  }
}

// 导出单条 AI 投研回答为高清长图
async function handleExportImage(msg: ChatMessage) {
  const anchorId = 'msg-anchor-' + msg.id
  const el = document.getElementById(anchorId) || document.getElementById('msg-item-' + msg.id)
  if (!el) {
    notifyWarning('未找到对应消息 DOM 节点，请重试')
    return
  }

  // 查找最近的 .t-chat__message-item 或 .assistant-msg 容器
  const target = (el.closest('.t-chat__message-item') || el.closest('.assistant-msg') || el) as HTMLElement

  const savedStyles: Array<{ el: HTMLElement; overflow: string; overflowY: string; height: string; maxHeight: string }> = []
  let p = target.parentElement
  while (p && p !== document.body) {
    const style = getComputedStyle(p)
    if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
      savedStyles.push({
        el: p,
        overflow: p.style.overflow,
        overflowY: p.style.overflowY,
        height: p.style.height,
        maxHeight: p.style.maxHeight
      })
      p.style.overflow = 'visible'
      p.style.overflowY = 'visible'
      p.style.height = 'auto'
      p.style.maxHeight = 'none'
    }
    p = p.parentElement
  }

  const isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
  try {
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: isDark ? '#18181c' : '#ffffff'
    })
    const dataUrl = canvas.toDataURL('image/png')
    const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')

    try {
      const result = await SaveImage(`blades-stock-agent-${safeTime}`, base64)
      if (result && !result.includes('异常') && !result.includes('无法')) {
        notifySuccess(`已导出为 PNG 图片：${result}`)
        return
      }
    } catch (e) {
      // fallback to browser file download
    }

    const link = document.createElement('a')
    link.href = dataUrl
    link.download = `blades-stock-agent-${safeTime}.png`
    link.click()
    notifySuccess('已导出下载 PNG 投研长图')
  } catch (err: any) {
    console.error('handleExportImage failed:', err)
    notifyError(`导出图片失败: ${err?.message || err}`)
  } finally {
    savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
      el.style.overflow = overflow
      el.style.overflowY = overflowY
      el.style.height = height
      el.style.maxHeight = maxHeight
    })
  }
}

// ========== 生命周期 ==========
onBeforeMount(() => {
  EventsOff('agent-message', handleAgentMessage)
  EventsOn('agent-message', handleAgentMessage)
})

async function onChatSettingsUpdated() {
  await aiConfigStore.loadConfigs()
}

onMounted(async () => {
  window.addEventListener('resize', onWindowResize)
  loadSessionsFromStorage()
  await aiConfigStore.loadConfigs()
  EventsOn('updateSettings', onChatSettingsUpdated)
  EventsOn('aiConfigsUpdated', onChatSettingsUpdated)

  refreshPerceptionSnapshot()
  perceptionTimer = setInterval(refreshPerceptionSnapshot, 60000)

  EventsOn('agent:consult', handleAgentConsult)
  if (typeof window !== 'undefined') {
    window.addEventListener('agent:consult', onWindowAgentConsult)
  }

  // 消费挂起的跨组件咨询任务 (消除异步路由加载延迟导致丢消息的问题)
  const pending = consumePendingConsult()
  if (pending) {
    nextTick(() => {
      handleAgentConsult(pending)
    })
  }

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute('theme-mode', 'dark')
    } else {
      document.documentElement.removeAttribute('theme-mode')
    }
  }).catch(() => {})

  GetVersionInfo().then((res) => {
    if (res.icon) {
      messages.value.forEach(m => {
        if (m.role === 'assistant') m.avatar = res.icon
      })
    }
  }).catch(() => {})

  nextTick(() => {
    scrollToBottom(false)
  })
})

onBeforeUnmount(() => {
  if (perceptionTimer) {
    clearInterval(perceptionTimer)
    perceptionTimer = null
  }
  EventsOff('agent:consult', handleAgentConsult)
  if (scrollRafId !== null) {
    cancelAnimationFrame(scrollRafId)
    scrollRafId = null
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('agent:consult', onWindowAgentConsult)
  }
  window.removeEventListener('resize', onWindowResize)
  EventsOff('agent-message', handleAgentMessage)
  EventsOff('updateSettings', onChatSettingsUpdated)
  EventsOff('aiConfigsUpdated', onChatSettingsUpdated)
})
</script>

<style scoped lang="less">
.chat-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  background: var(--td-bg-color-page, #f8f9fa);
  color: var(--td-text-color-primary, #1d2129);
  position: relative;
  overflow: hidden;
}

/* 顶部 HUD 控制面板 */
.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--td-bg-color-container, #ffffff);
  border-bottom: 1px solid var(--td-border-level-1-color, #e5e6eb);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
  z-index: 10;
  gap: 12px;

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;

    .brand-title-wrap {
      display: flex;
      align-items: center;
      gap: 6px;
      margin-right: 4px;
      user-select: none;

      .brand-icon {
        font-size: 18px;
        line-height: 1;
      }

      .brand-title {
        font-size: 15px;
        font-weight: 700;
        color: var(--td-text-color-primary, #1d2129);
        letter-spacing: 0.2px;
        white-space: nowrap;
      }

      .brand-badge {
        font-size: 10px;
        font-weight: 600;
        padding: 1px 6px;
        border-radius: 4px;
        background: linear-gradient(135deg, rgba(102, 126, 234, 0.15) 0%, rgba(118, 75, 162, 0.15) 100%);
        color: #667eea;
        border: 1px solid rgba(102, 126, 234, 0.3);
        white-space: nowrap;
      }
    }

    .session-drawer-btn {
      font-weight: 600;
      font-size: 14px;
      display: flex;
      align-items: center;
      gap: 6px;
      max-width: 200px;

      .session-title-text {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .session-badge {
        font-size: 11px;
        background: var(--td-bg-color-secondarycontainer, #f2f3f5);
        color: var(--td-text-color-secondary, #4e5969);
        padding: 1px 6px;
        border-radius: 10px;
      }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;

    .header-switch-item {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 3px 8px;
      border-radius: 6px;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      border: 1px solid var(--td-border-level-1-color, #e5e6eb);
      white-space: nowrap;
      transition: all 0.2s ease;

      .header-switch-label {
        font-size: 12px;
        color: var(--td-text-color-secondary, #4e5969);
        user-select: none;
      }

      .header-memory-select {
        width: 76px;
        margin-left: 2px;
      }
    }

    .model-selector {
      width: 240px;
    }
  }
}

/* 宏观态势感知 HUD 仪表条 (双端统一呈现) */
.macro-perception-hud {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--td-bg-color-secondarycontainer, rgba(24, 160, 88, 0.06));
  border-bottom: 1px solid var(--td-border-level-1-color, rgba(255, 255, 255, 0.08));
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  font-size: 12px;
  flex-shrink: 0;

  &:hover {
    background: var(--td-bg-color-component-hover, rgba(24, 160, 88, 0.12));

    .action-hint .arrow {
      transform: translateX(3px);
    }
  }

  .hud-left {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;

    .phase-pill {
      font-weight: 600;
      padding: 2px 8px;
      border-radius: 4px;
      background: rgba(255, 255, 255, 0.08);
      color: var(--td-text-color-primary, #e5e5e5);
    }

    .traffic-beacon {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      display: inline-block;
      box-shadow: 0 0 6px currentColor;
    }

    .traffic-label {
      font-weight: 600;
      letter-spacing: 0.5px;
    }

    .breadth-ratio {
      font-size: 12px;
      margin-left: 4px;

      .up-count {
        color: #f67979;
        font-weight: 600;
      }
      .separator {
        margin: 0 4px;
        opacity: 0.4;
      }
      .down-count {
        color: #18a058;
        font-weight: 600;
      }
    }
  }

  .hud-right {
    display: flex;
    align-items: center;
    gap: 10px;

    .risk-tag {
      font-size: 11px;
    }

    .action-hint {
      display: flex;
      align-items: center;
      gap: 4px;
      color: #18a058;
      font-weight: 500;

      .arrow {
        transition: transform 0.2s ease;
      }
    }
  }
}

/* 主体消息滚动视图 (正序) */
.chat-messages-viewport {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px 30px;
  position: relative;
  /* 禁用默认平滑滚动（平滑滚动会与高频流式 SSE 渲染产生冲突导致上下抽搐），改由特定动作显式触发 */
  /* 禁用浏览器默认锚定，防止动态 Markdown 渲染时与 JS scrollTop 吸底竞争导致剧烈上下抖动 */
  overflow-anchor: none;

  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-thumb {
    background: var(--td-border-level-2-color, #c9cdd4);
    border-radius: 3px;
  }
}

/* 欢迎页卡片矩阵 */
.welcome-workspace {
  max-width: 1200px;
  width: 100%;
  margin: 20px auto 40px;
  display: flex;
  flex-direction: column;
  align-items: center;

  .welcome-hero {
    text-align: center;
    margin-bottom: 24px;

    .hero-badge {
      display: inline-block;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 1.5px;
      text-transform: uppercase;
      padding: 3px 10px;
      border-radius: 12px;
      background: linear-gradient(135deg, rgba(0, 82, 217, 0.12) 0%, rgba(43, 164, 113, 0.12) 100%);
      color: var(--td-brand-color, #0052d9);
      margin-bottom: 10px;
      border: 1px solid rgba(0, 82, 217, 0.2);
    }

    .hero-title {
      font-size: 22px;
      font-weight: 800;
      margin: 0 0 8px;
      letter-spacing: 0.3px;
      background: linear-gradient(135deg, var(--td-brand-color, #0052d9) 0%, #2ba471 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    .hero-desc {
      font-size: 13px;
      color: var(--td-text-color-secondary, #86909c);
      max-width: 680px;
      line-height: 1.6;
      margin: 0 auto 18px;
    }

    /* 模式切换：标准五阶流水线 vs 专项极速控制台 */
    .welcome-mode-toggle {
      display: inline-flex;
      background: var(--td-bg-color-component, #f2f3f5);
      padding: 4px;
      border-radius: 20px;
      gap: 6px;
      box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.05);

      .toggle-tab-btn {
        border: none;
        background: transparent;
        font-size: 13px;
        font-weight: 500;
        padding: 7px 18px;
        border-radius: 16px;
        cursor: pointer;
        color: var(--td-text-color-secondary, #4e5969);
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 6px;

        &:hover {
          color: var(--td-text-color-primary, #1d2129);
        }

        &.active {
          background: var(--td-bg-color-container, #ffffff);
          color: var(--td-brand-color, #0052d9);
          font-weight: 600;
          box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08);
        }
      }
    }
  }

  /* 视图 1：标准五阶流水线 */
  .pipeline-container {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 16px;

    .pipeline-flow-guide {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 10px;
      padding: 10px 16px;
      background: var(--td-bg-color-container, #ffffff);
      border: 1px solid var(--td-border-level-1-color, #e5e6eb);
      border-radius: 10px;
      overflow-x: auto;
      white-space: nowrap;

      .flow-step-node {
        font-size: 13px;
        font-weight: 600;
        color: var(--td-text-color-secondary, #4e5969);
        padding: 5px 12px;
        background: var(--td-bg-color-component, #f2f3f5);
        border-radius: 8px;
        cursor: pointer;
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 6px;

        .step-num {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          width: 18px;
          height: 18px;
          border-radius: 50%;
          background: rgba(0, 0, 0, 0.1);
          font-size: 11px;
        }

        &:hover {
          background: var(--td-brand-color-light, #ecf2fe);
          color: var(--td-brand-color, #0052d9);
        }

        &.current {
          background: var(--td-brand-color, #0052d9);
          color: #ffffff;

          .step-num {
            background: rgba(255, 255, 255, 0.25);
          }
        }
      }

      .flow-arrow {
        color: var(--td-text-color-placeholder, #c9cdd4);
        font-size: 13px;
      }
    }

    /* 全局流水线贯通指示栏 */
    .pipeline-context-banner {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 16px;
      background: linear-gradient(135deg, rgba(0, 82, 217, 0.05) 0%, rgba(102, 126, 234, 0.08) 100%);
      border: 1px solid rgba(0, 82, 217, 0.18);
      border-radius: 10px;
      flex-wrap: wrap;

      .banner-left {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;

        .banner-icon {
          font-size: 16px;
        }

        .banner-label {
          font-size: 13px;
          font-weight: 600;
          color: var(--td-text-color-primary, #1d2129);
        }

        .banner-stock-pill {
          font-size: 13px;
          font-weight: 700;
          color: #0052d9;
          background: var(--td-bg-color-container, #ffffff);
          padding: 3px 10px;
          border-radius: 14px;
          border: 1px solid rgba(0, 82, 217, 0.3);
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
          cursor: pointer;
          display: inline-flex;
          align-items: center;
          gap: 4px;
          transition: all 0.2s ease;

          &:hover {
            border-color: #0052d9;
            box-shadow: 0 2px 6px rgba(0, 82, 217, 0.2);
          }

          .pill-name {
            font-weight: 700;
          }

          .pill-code {
            font-size: 12px;
            opacity: 0.85;
            font-family: monospace;
          }
        }

        .banner-tag {
          font-size: 12px;
          padding: 2px 8px;
          border-radius: 12px;
          background: rgba(0, 0, 0, 0.04);
          color: var(--td-text-color-secondary, #4e5969);

          &.track-tag {
            background: rgba(240, 160, 32, 0.12);
            color: #d46b08;
            font-weight: 500;
          }

          &.stance-tag {
            background: rgba(24, 160, 88, 0.12);
            color: #18a058;
            font-weight: 500;
          }
        }
      }

      .banner-right {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-wrap: wrap;

        .banner-hot-chips {
          display: flex;
          align-items: center;
          gap: 5px;

          .hot-label {
            font-size: 11px;
            color: var(--td-text-color-placeholder, #86909c);
          }

          .hot-chip-btn {
            font-size: 11px;
            padding: 2px 7px;
            border-radius: 10px;
            background: var(--td-bg-color-container, #ffffff);
            border: 1px solid var(--td-border-level-1-color, #e5e6eb);
            color: var(--td-text-color-secondary, #4e5969);
            cursor: pointer;
            transition: all 0.15s ease;

            &:hover {
              color: var(--td-brand-color, #0052d9);
              border-color: var(--td-brand-color, #0052d9);
            }

            &.active {
              background: var(--td-brand-color, #0052d9);
              color: #ffffff;
              border-color: var(--td-brand-color, #0052d9);
            }
          }
        }

        .banner-switch-btn {
          border: 1px solid var(--td-border-level-2-color, #dcdcdc);
          background: var(--td-bg-color-container, #ffffff);
          color: var(--td-text-color-primary, #1d2129);
          padding: 4px 10px;
          border-radius: 6px;
          font-size: 12px;
          font-weight: 500;
          cursor: pointer;
          transition: all 0.2s ease;

          &:hover {
            border-color: var(--td-brand-color, #0052d9);
            color: var(--td-brand-color, #0052d9);
          }
        }
      }
    }

    .pipeline-steps-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;

      @media (max-width: 768px) {
        grid-template-columns: 1fr;
      }
    }

    .pipeline-card {
      background: var(--td-bg-color-container, #ffffff);
      border: 1px solid var(--td-border-level-1-color, #e5e6eb);
      border-radius: 12px;
      padding: 16px;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      transition: all 0.25s ease;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);

      &:hover {
        border-color: var(--td-brand-color, #0052d9);
        transform: translateY(-2px);
        box-shadow: 0 6px 16px rgba(0, 82, 217, 0.08);
      }

      &.highlight-step {
        border-color: rgba(0, 82, 217, 0.35);
        background: linear-gradient(180deg, var(--td-bg-color-container, #ffffff) 0%, rgba(0, 82, 217, 0.02) 100%);
      }

      .step-header {
        margin-bottom: 8px;

        .step-badge-wrap {
          display: flex;
          align-items: center;
          gap: 8px;

          .step-badge {
            font-size: 11px;
            padding: 2px 7px;
            border-radius: 4px;
            font-weight: 700;
            letter-spacing: 0.3px;

            &.l1 { background: rgba(0, 82, 217, 0.12); color: #0052d9; }
            &.l2 { background: rgba(240, 160, 32, 0.14); color: #d46b08; }
            &.l3 { background: rgba(102, 126, 234, 0.14); color: #5c6ac4; }
            &.l4 { background: rgba(220, 53, 69, 0.12); color: #dc3545; }
            &.l5 { background: rgba(43, 164, 113, 0.14); color: #2ba471; }
          }

          .step-title {
            font-size: 14px;
            font-weight: 700;
            color: var(--td-text-color-primary, #1d2129);
          }
        }
      }

      .step-desc {
        font-size: 12px;
        color: var(--td-text-color-secondary, #4e5969);
        line-height: 1.5;
        margin: 0 0 14px;
        flex: 1;
      }

      .inline-stock-picker {
        background: var(--td-bg-color-component, #f7f8fa);
        padding: 10px;
        border-radius: 8px;
        margin-bottom: 12px;

        .picker-input-row {
          margin-bottom: 8px;
        }

        .picker-hot-chips {
          display: flex;
          align-items: center;
          gap: 6px;
          flex-wrap: wrap;

          .hot-label {
            font-size: 11px;
            color: var(--td-text-color-placeholder, #86909c);
          }

          .hot-chip {
            font-size: 11px;
            padding: 2px 6px;
            border-radius: 4px;
            background: var(--td-bg-color-container, #ffffff);
            border: 1px solid var(--td-border-level-1-color, #e5e6eb);
            cursor: pointer;
            transition: all 0.2s ease;

            &:hover,
            &.active {
              color: var(--td-brand-color, #0052d9);
              border-color: var(--td-brand-color, #0052d9);
              background: var(--td-brand-color-light, #ecf2fe);
            }
          }
        }
      }

      .ledger-guide-box {
        display: flex;
        align-items: flex-start;
        gap: 6px;
        background: rgba(43, 164, 113, 0.08);
        border: 1px dashed rgba(43, 164, 113, 0.3);
        padding: 8px 10px;
        border-radius: 6px;
        margin-bottom: 12px;
        font-size: 11px;
        color: #198754;
        line-height: 1.4;

        .guide-icon {
          font-size: 13px;
        }
      }

      .step-actions {
        display: flex;
        flex-direction: column;
        gap: 8px;

        .step-btn {
          border: none;
          padding: 7px 12px;
          border-radius: 6px;
          font-size: 12px;
          font-weight: 500;
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 4px;
          transition: all 0.2s ease;

          &.primary {
            background: var(--td-brand-color, #0052d9);
            color: #ffffff;

            &:hover {
              background: #0044b8;
              box-shadow: 0 2px 8px rgba(0, 82, 217, 0.25);
            }
          }

          &.warning {
            background: #fa8c16;
            color: #ffffff;

            &:hover {
              background: #d46b08;
              box-shadow: 0 2px 8px rgba(250, 140, 22, 0.25);
            }
          }

          &.sub {
            background: var(--td-bg-color-component, #f2f3f5);
            color: var(--td-text-color-primary, #1d2129);

            &:hover {
              background: var(--td-brand-color-light, #ecf2fe);
              color: var(--td-brand-color, #0052d9);
            }
          }
        }
      }
    }
  }

  /* 视图 2：专项极速控制台 */
  .direct-tools-container {
    width: 100%;

    .direct-tools-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 16px;

      @media (max-width: 768px) {
        grid-template-columns: 1fr;
      }
    }

    .direct-tool-card {
      background: var(--td-bg-color-container, #ffffff);
      border: 1px solid var(--td-border-level-1-color, #e5e6eb);
      border-radius: 12px;
      padding: 18px 20px;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);
      transition: all 0.25s ease;

      &:hover {
        border-color: var(--td-brand-color, #0052d9);
        transform: translateY(-2px);
        box-shadow: 0 6px 16px rgba(0, 82, 217, 0.06);
      }

      .tool-head {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 10px;

        .tool-icon {
          font-size: 24px;
        }

        .tool-head-text {
          display: flex;
          flex-direction: column;
          gap: 2px;

          .tool-name {
            font-size: 15px;
            font-weight: 700;
            color: var(--td-text-color-primary, #1d2129);
          }

          .tool-sub {
            font-size: 11px;
            color: var(--td-text-color-secondary, #86909c);
          }
        }
      }

      .tool-desc {
        font-size: 12px;
        color: var(--td-text-color-secondary, #4e5969);
        line-height: 1.5;
        margin: 0 0 16px;
      }

      .tool-form {
        display: flex;
        flex-direction: column;
        gap: 10px;

        .pk-inputs-row {
          display: flex;
          align-items: center;
          gap: 8px;

          .pk-vs {
            font-size: 12px;
            font-weight: 700;
            color: #d03050;
            padding: 0 4px;
          }
        }

        .quick-hot-row {
          display: flex;
          align-items: center;
          gap: 6px;
          flex-wrap: wrap;

          .hot-chip {
            font-size: 11px;
            padding: 3px 8px;
            border-radius: 4px;
            border: 1px solid var(--td-border-level-1-color, #e5e6eb);
            background: var(--td-bg-color-component, #f2f3f5);
            color: var(--td-text-color-secondary, #4e5969);
            cursor: pointer;
            transition: all 0.2s ease;

            &:hover {
              background: var(--td-brand-color-light, #ecf2fe);
              border-color: var(--td-brand-color, #0052d9);
              color: var(--td-brand-color, #0052d9);
            }
          }
        }

        .tool-submit-btn {
          border: none;
          padding: 8px 14px;
          border-radius: 6px;
          font-size: 13px;
          font-weight: 600;
          cursor: pointer;
          background: var(--td-brand-color, #0052d9);
          color: #ffffff;
          transition: all 0.2s ease;
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 6px;
          margin-top: 4px;

          &:hover {
            background: #0044b8;
            box-shadow: 0 2px 8px rgba(0, 82, 217, 0.25);
          }

          &.warning {
            background: #fa8c16;

            &:hover {
              background: #d46b08;
              box-shadow: 0 2px 8px rgba(250, 140, 22, 0.25);
            }
          }
        }
      }
    }
  }
}

/* 消息流布局 */
.messages-flow {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 1240px;
  width: 100%;
  margin: 0 auto;
}

/* 投研流水线下一步推荐条 */
.pipeline-next-step-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  background: linear-gradient(90deg, rgba(0, 82, 217, 0.06) 0%, rgba(43, 164, 113, 0.06) 100%);
  border: 1px dashed rgba(0, 82, 217, 0.3);
  padding: 10px 16px;
  border-radius: 10px;
  margin-top: -6px;
  animation: fadeIn 0.3s ease;

  .next-step-icon {
    font-size: 16px;
  }

  .next-step-text {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary, #1d2129);
  }

  .next-step-btn {
    border: none;
    background: var(--td-brand-color, #0052d9);
    color: #ffffff;
    font-size: 12px;
    font-weight: 600;
    padding: 5px 14px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
    margin-left: auto;

    &:hover {
      background: #0044b8;
      box-shadow: 0 2px 8px rgba(0, 82, 217, 0.25);
      transform: translateY(-1px);
    }
  }
}

/* 标的快速填装弹窗样式 */
.stock-action-modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;

  .modal-field-label {
    font-size: 13px;
    font-weight: 600;
    color: var(--td-text-color-primary, #1d2129);
    margin-bottom: 6px;
    display: block;
  }

  .single-modal-inputs,
  .pk-modal-inputs {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .modal-preset-chips {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;

    .preset-label {
      font-size: 12px;
      color: var(--td-text-color-secondary, #86909c);
    }

    .preset-chip {
      font-size: 12px;
      padding: 3px 8px;
      border-radius: 4px;
      background: var(--td-bg-color-component, #f2f3f5);
      color: var(--td-text-color-secondary, #4e5969);
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover,
      &.active {
        background: var(--td-brand-color-light, #ecf2fe);
        color: var(--td-brand-color, #0052d9);
      }
    }
  }

  .modal-footer-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    margin-top: 10px;
  }
}

/* 回到底部悬浮球 */
.fab-scroll-bottom {
  position: fixed;
  right: 28px;
  bottom: 120px;
  background: var(--td-bg-color-container, #ffffff);
  border: 1px solid var(--td-border-level-1-color, #e5e6eb);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.12);
  border-radius: 20px;
  padding: 6px 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: var(--td-brand-color, #0052d9);
  font-size: 12px;
  font-weight: 600;
  z-index: 50;
  transition: all 0.2s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(0, 82, 217, 0.2);
  }
}

/* 底部输入中枢 */
.chat-input-cockpit {
  padding: 10px 20px 16px;
  background: var(--td-bg-color-container, #ffffff);
  border-top: 1px solid var(--td-border-level-1-color, #e5e6eb);
  flex-shrink: 0;
  z-index: 20;

  &.compact {
    padding: 8px 16px 10px;
  }

  .input-quick-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    font-size: 12px;
    min-height: 28px;
    line-height: 28px;

    .quick-chips {
      display: flex;
      gap: 6px;
      overflow-x: auto;
      flex: 1;
      min-width: 0;
      flex-wrap: nowrap;
      padding-bottom: 2px;

      &::-webkit-scrollbar {
        height: 0;
      }
    }

    .quick-chip-btn {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      border: 1px solid transparent;
      padding: 3px 10px;
      border-radius: 12px;
      cursor: pointer;
      font-size: 12px;
      color: var(--td-text-color-secondary, #4e5969);
      white-space: nowrap;
      flex-shrink: 0;
      transition: all 0.2s;

      &:hover {
        background: var(--td-brand-color-light, #ecf2fe);
        color: var(--td-brand-color, #0052d9);
      }
    }

    .status-summary {
      display: flex;
      align-items: center;
      gap: 6px;
      color: var(--td-brand-color, #0052d9);
      font-size: 12px;
      white-space: nowrap;
      flex-shrink: 0;
      margin-left: 10px;

      .live-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--td-brand-color, #0052d9);
        animation: pulse-dot 1.5s infinite;
      }
    }
  }

  .input-sender-box {
    max-width: 1240px;
    width: 100%;
    margin: 0 auto;

    :deep(.t-chat-sender) {
      .t-chat-sender__textarea {
        border-radius: 10px !important;
        border: 1px solid var(--td-border-level-2-color, #dcdcdc) !important;
        background: var(--td-bg-color-container, #ffffff) !important;
        font-size: 14px !important;
        line-height: 1.6 !important;
        padding: 8px 12px !important;
        transition: border-color 0.2s;

        &:focus {
          border-color: var(--td-brand-color, #0052d9) !important;
          box-shadow: 0 0 0 2px var(--td-brand-color-focus, rgba(0, 82, 217, 0.15)) !important;
        }
      }
    }

    .sender-actions {
      display: flex;
      align-items: center;
      gap: 8px;

      .stop-btn {
        font-weight: 600;
        animation: pulse-warning 2s infinite;
      }
    }
  }

  .config-alert-banner {
    max-width: 980px;
    margin: 8px auto 0;
    padding: 6px 12px;
    background: #fffbe6;
    border: 1px solid #ffe58f;
    border-radius: 6px;
    font-size: 12px;
    color: #ad6800;

    a {
      color: var(--td-brand-color, #0052d9);
      font-weight: bold;
    }
  }
}

/* 抽屉样式 */
.session-drawer-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 12px;

  .btn-new-chat {
    border-radius: 8px;
    font-weight: 600;
  }

  .session-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;

    .session-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 12px;
      border-radius: 8px;
      cursor: pointer;
      border: 1px solid transparent;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      transition: all 0.2s;

      &:hover {
        background: var(--td-bg-color-container, #ffffff);
        border-color: var(--td-border-level-2-color, #dcdcdc);
      }

      &.active {
        background: var(--td-brand-color-light, #ecf2fe);
        border-color: var(--td-brand-color, #0052d9);

        .session-name {
          color: var(--td-brand-color, #0052d9);
          font-weight: 600;
        }
      }

      .session-info {
        display: flex;
        flex-direction: column;
        gap: 2px;
        overflow: hidden;

        .session-name {
          font-size: 13px;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }

        .session-meta {
          font-size: 11px;
          color: var(--td-text-color-placeholder, #86909c);
        }
      }
    }
  }
}

/* 弹窗场景列表 */
.modal-scenarios-list {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-height: 480px;
  overflow-y: auto;

  .scenario-modal-item {
    padding: 12px 14px;
    background: var(--td-bg-color-secondarycontainer, #f8f9fa);
    border: 1px solid var(--td-border-level-1-color, #e5e6eb);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      border-color: var(--td-brand-color, #0052d9);
      background: var(--td-brand-color-light, #ecf2fe);
    }

    .item-head {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 6px;

      .item-tag {
        font-size: 11px;
        background: var(--td-brand-color, #0052d9);
        color: #ffffff;
        padding: 1px 6px;
        border-radius: 4px;
      }

      .item-title {
        font-size: 14px;
        font-weight: 600;
      }
    }

    .item-desc {
      font-size: 12px;
      color: var(--td-text-color-secondary, #4e5969);
      margin: 0;
      line-height: 1.5;
    }
  }
}

/* 动画定义 */
@keyframes pulse-dot {
  0% { transform: scale(0.95); opacity: 0.6; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.95); opacity: 0.6; }
}

@keyframes float-glow {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}

@keyframes pulse-warning {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.75; }
}

/* -------------------------------------------------------------------------
   移动端 (Screen <= 768px) 深度触控与全景响应式适配
   ------------------------------------------------------------------------- */
@media screen and (max-width: 768px) {
  .chat-header {
    flex-wrap: wrap !important;
    gap: 6px !important;
    padding: 6px 10px !important;

    .header-left {
      order: 1;
      flex-wrap: wrap;
      gap: 6px;

      .brand-title-wrap {
        margin-right: 2px;
        .brand-title {
          font-size: 13px !important;
        }
        .brand-badge {
          display: none;
        }
      }

      .session-drawer-btn {
        max-width: 120px !important;
        font-size: 12px !important;
      }
    }

    .header-right {
      order: 2;
      margin-left: auto;
      flex-wrap: wrap;
      gap: 4px !important;

      .header-switch-item {
        padding: 2px 6px;
        gap: 4px;
        .header-switch-label {
          font-size: 11px;
        }
        .header-memory-select {
          width: 60px;
        }
      }

      .model-selector {
        width: 130px !important;
        max-width: 150px !important;
      }
    }
  }

  .macro-perception-hud {
    padding: 6px 10px !important;
    gap: 6px !important;

    .hud-left {
      gap: 6px !important;
      font-size: 11px !important;

      .phase-pill {
        padding: 1px 6px !important;
        font-size: 11px !important;
      }
      .breadth-ratio {
        font-size: 11px !important;
      }
    }

    .hud-right {
      gap: 6px !important;
      .risk-tag {
        display: none !important;
      }
      .action-hint {
        font-size: 11px !important;
      }
    }
  }

  .chat-messages-viewport {
    padding: 10px 8px 20px !important;
  }

  .welcome-workspace {
    margin: 10px auto 20px !important;
    .welcome-hero {
      margin-bottom: 16px !important;
      .hero-title {
        font-size: 18px !important;
      }
    }
  }

  .fab-scroll-bottom {
    right: 14px !important;
    bottom: 90px !important;
    padding: 4px 10px !important;
    font-size: 11px !important;
  }

  .chat-input-cockpit {
    padding: 6px 10px max(10px, var(--sab, 0px)) !important;

    .input-quick-bar {
      margin-bottom: 6px !important;
      .quick-chips {
        gap: 4px !important;
      }
      .quick-chip-btn {
        padding: 2px 8px !important;
        font-size: 11px !important;
      }
    }
  }
}

/* -------------------------------------------------------------------------
   深色模式 (Theme Mode: Dark) 高对比度与层次质感增强
   ------------------------------------------------------------------------- */
:global([theme-mode="dark"]) {
  .chat-header {
    background: #18181c !important;
    border-bottom-color: #2e2e33 !important;

    .brand-title-wrap {
      .brand-title {
        color: #f2f3f5 !important;
      }
      .brand-badge {
        background: rgba(102, 126, 234, 0.25) !important;
        color: #a5b4fc !important;
        border-color: rgba(102, 126, 234, 0.45) !important;
      }
    }

    .header-switch-item {
      background: rgba(255, 255, 255, 0.06) !important;
      border-color: rgba(255, 255, 255, 0.12) !important;

      .header-switch-label {
        color: #c9cdd4 !important;
      }
    }
  }

  .macro-perception-hud {
    background: #1e1e24 !important;
    border-bottom-color: #2e2e33 !important;
  }

  .chat-input-cockpit {
    background: #18181c !important;
    border-top-color: #2e2e33 !important;
  }

  /* 欢迎页与五阶流水线控制台深色适配 */
  .welcome-workspace {
    .welcome-mode-toggle {
      background: #242429 !important;

      .toggle-tab-btn {
        color: #86909c !important;

        &:hover {
          color: #f2f3f5 !important;
        }

        &.active {
          background: #323239 !important;
          color: #a5b4fc !important;
        }
      }
    }

    .pipeline-container {
      .pipeline-flow-guide {
        background: #1e1e24 !important;
        border-color: #2e2e33 !important;

        .flow-step-node {
          background: #2b2b32 !important;
          color: #c9cdd4 !important;

          .step-num {
            background: rgba(255, 255, 255, 0.1) !important;
          }

          &:hover {
            background: rgba(102, 126, 234, 0.25) !important;
            color: #a5b4fc !important;
          }

          &.current {
            background: #4f46e5 !important;
            color: #ffffff !important;

            .step-num {
              background: rgba(255, 255, 255, 0.3) !important;
            }
          }
        }

        .flow-arrow {
          color: #4e5969 !important;
        }
      }

      .pipeline-context-banner {
        background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(30, 30, 36, 0.9) 100%) !important;
        border-color: rgba(102, 126, 234, 0.3) !important;

        .banner-left {
          .banner-label {
            color: #f2f3f5 !important;
          }

          .banner-stock-pill {
            background: #2b2b32 !important;
            color: #a5b4fc !important;
            border-color: rgba(102, 126, 234, 0.5) !important;

            .pill-name {
              color: #a5b4fc !important;
            }

            .pill-code {
              color: #818cf8 !important;
            }
          }

          .banner-tag {
            &.track-tag {
              background: rgba(240, 160, 32, 0.2) !important;
              color: #f59e0b !important;
            }

            &.stance-tag {
              background: rgba(24, 160, 88, 0.2) !important;
              color: #10b981 !important;
            }
          }
        }

        .banner-right {
          .banner-hot-chips {
            .hot-label {
              color: #86909c !important;
            }

            .hot-chip-btn {
              background: #2b2b32 !important;
              border-color: #3e3e48 !important;
              color: #c9cdd4 !important;

              &:hover {
                border-color: #818cf8 !important;
                color: #a5b4fc !important;
              }

              &.active {
                background: #4f46e5 !important;
                border-color: #4f46e5 !important;
                color: #ffffff !important;
              }
            }
          }

          .banner-switch-btn {
            background: #2b2b32 !important;
            border-color: #4e5969 !important;
            color: #e5e6eb !important;

            &:hover {
              border-color: #a5b4fc !important;
              color: #a5b4fc !important;
            }
          }
        }
      }

      .pipeline-card {
        background: #1e1e24 !important;
        border-color: #2e2e33 !important;

        &:hover {
          border-color: #667eea !important;
          box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4) !important;
        }

        &.highlight-step {
          background: linear-gradient(180deg, #1e1e24 0%, rgba(102, 126, 234, 0.08) 100%) !important;
          border-color: rgba(102, 126, 234, 0.4) !important;
        }

        .step-header {
          .step-badge-wrap {
            .step-badge {
              &.l1 { background: rgba(99, 102, 241, 0.2) !important; color: #a5b4fc !important; }
              &.l2 { background: rgba(245, 158, 11, 0.2) !important; color: #fbbf24 !important; }
              &.l3 { background: rgba(139, 92, 246, 0.2) !important; color: #c4b5fd !important; }
              &.l4 { background: rgba(239, 68, 68, 0.2) !important; color: #f87171 !important; }
              &.l5 { background: rgba(16, 185, 129, 0.2) !important; color: #34d399 !important; }
            }

            .step-title {
              color: #f2f3f5 !important;
            }
          }
        }

        .step-desc {
          color: #a4abb5 !important;
        }

        .inline-stock-picker {
          background: #26262c !important;

          .picker-hot-chips {
            .hot-chip {
              background: #1e1e24 !important;
              border-color: #383842 !important;
              color: #c9cdd4 !important;

              &:hover,
              &.active {
                background: rgba(102, 126, 234, 0.25) !important;
                color: #a5b4fc !important;
                border-color: #667eea !important;
              }
            }
          }
        }

        .step-actions {
          .step-btn.sub {
            background: #2b2b32 !important;
            color: #f2f3f5 !important;

            &:hover {
              background: rgba(102, 126, 234, 0.25) !important;
              color: #a5b4fc !important;
            }
          }
        }
      }
    }

    .direct-tools-container {
      .direct-tool-card {
        background: #1e1e24 !important;
        border-color: #2e2e33 !important;

        &:hover {
          border-color: #667eea !important;
          box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4) !important;
        }

        .tool-head .tool-name {
          color: #f2f3f5 !important;
        }

        .tool-desc {
          color: #a4abb5 !important;
        }

        .quick-hot-row .hot-chip {
          background: #2b2b32 !important;
          color: #c9cdd4 !important;

          &:hover {
            background: rgba(102, 126, 234, 0.25) !important;
            color: #a5b4fc !important;
          }
        }
      }
    }
  }

  .pipeline-next-step-bar {
    background: linear-gradient(90deg, rgba(102, 126, 234, 0.12) 0%, rgba(43, 164, 113, 0.12) 100%) !important;
    border-color: rgba(102, 126, 234, 0.4) !important;

    .next-step-text {
      color: #f2f3f5 !important;
    }
  }
}
</style>
