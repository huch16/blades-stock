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
          <h2 class="hero-title">Blades 买方量化投研决策中枢</h2>
          <p class="hero-desc">
            从宏观大势择时到买方个股落子 · 五阶全时序量化闭环作业
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
          <!-- 阶段引导流程轴 -->
          <div class="pipeline-flow-guide">
            <div
              class="flow-step-node"
              :class="{ current: pipelineContext.currentStep === 1 }"
              @click="executeStep1"
              title="点击测算大盘择时与仓位上限"
            >
              <span class="step-num l1">1</span>
              <span class="step-name">看大势 (L1)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div
              class="flow-step-node"
              :class="{ current: pipelineContext.currentStep === 2 }"
              @click="executeStep2('etf')"
              title="点击计算核心行业ETF动量排名"
            >
              <span class="step-num l2">2</span>
              <span class="step-name">选赛道 (L2)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div
              class="flow-step-node"
              :class="{ current: pipelineContext.currentStep === 3 }"
              @click="executeStep3('breakout')"
              title="点击运行多因子策略量化选股"
            >
              <span class="step-num l3">3</span>
              <span class="step-name">筛标的 (L3)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div
              class="flow-step-node"
              :class="{ current: pipelineContext.currentStep === 4 }"
              @click="executeStep4('diag')"
              title="点击启动个股四维全景体检与排雷"
            >
              <span class="step-num l4">4</span>
              <span class="step-name">验体质 (L4)</span>
            </div>
            <div class="flow-arrow">➔</div>
            <div
              class="flow-step-node"
              :class="{ current: pipelineContext.currentStep === 5 }"
              @click="executeStep5()"
              title="点击形成买方5-Verdict裁决并存入认知账本"
            >
              <span class="step-num l5">5</span>
              <span class="step-name">定落子 (L5)</span>
            </div>
          </div>

          <!-- 全局流水线贯通标的与状态指示栏 (统一单入口，消除信息孤岛) -->
          <div class="pipeline-context-banner">
            <div class="banner-left">
              <span class="banner-icon">🎯</span>
              <span class="banner-label">全局贯通标的：</span>
              <div
                class="banner-stock-pill"
                @click="openStockActionModal('diag')"
                title="点击弹出标的库搜索或更换标的"
              >
                <span class="pill-name">{{ pipelineContext.stockName || '默认标的' }}</span>
                <span class="pill-code">{{ pipelineContext.stockCode }}</span>
              </div>
              <div class="banner-quick-input-wrap" title="直接输入6位股票代码按回车快速切换贯通标的">
                <n-input
                  v-model:value="pipelineContext.stockCode"
                  placeholder="代码/回车"
                  size="tiny"
                  class="banner-quick-input"
                  @update:value="onPipelineStockCodeChange"
                  @keydown.enter="onPipelineStockCodeEnter"
                />
              </div>
              <span v-if="pipelineContext.macroStance" class="banner-tag stance-tag">
                🚦 {{ pipelineContext.macroStance }}
              </span>
              <span v-if="pipelineContext.mainTrack" class="banner-tag track-tag">
                🏷️ {{ pipelineContext.mainTrack }}
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
                🔍 标的库
              </button>
            </div>
          </div>

          <!-- 五阶量化时序决策卡片矩阵 (严格等高、网格对称对齐) -->
          <div class="pipeline-steps-grid">
            <!-- Step 1: 🏛️ 看大势 (L1 Macro) -->
            <div class="pipeline-card step-card-1" :class="{ current: pipelineContext.currentStep === 1 }">
              <div class="step-card-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l1">L1 择时</span>
                  <span class="step-title">宏观大势与仓位上限</span>
                </div>
                <div class="step-tags">
                  <span class="mini-tag">核心宽基指数</span>
                  <span class="mini-tag">多空红绿灯</span>
                </div>
              </div>
              <p class="step-desc">
                评估大盘多空红绿灯与全市场赚钱效应，严禁外部搜索，精准测算机构总仓位上限。
              </p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep1">
                  🚀 测算大盘择时与仓位
                </button>
              </div>
            </div>

            <!-- Step 2: 🧭 选赛道 (L2 Sector & ETF) -->
            <div class="pipeline-card step-card-2" :class="{ current: pipelineContext.currentStep === 2 }">
              <div class="step-card-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l2">L2 主线</span>
                  <span class="step-title">赛道动量与主力席位</span>
                </div>
                <div class="step-tags">
                  <span class="mini-tag">21日加权动量</span>
                  <span class="mini-tag">龙虎榜机构</span>
                </div>
              </div>
              <p class="step-desc">
                运行纯 Go 本地 VectorBT 计算加权动量，捕捉场内领涨行业 ETF 与龙虎榜机构席位。
              </p>
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
            <div class="pipeline-card step-card-3" :class="{ current: pipelineContext.currentStep === 3 }">
              <div class="step-card-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l3">L3 选股</span>
                  <span class="step-title">量化多因子初筛池</span>
                </div>
                <div class="step-tags">
                  <span class="mini-tag">平台放量突破</span>
                  <span class="mini-tag">均线多头</span>
                </div>
              </div>
              <p class="step-desc">
                多因子量化模型初筛全市场高胜率标的，龙头标的自动无缝贯通至 Step 4 深度验质。
              </p>
              <div class="step-actions">
                <button class="step-btn primary" @click="executeStep3('breakout')">
                  🚀 放量突破选股
                </button>
                <div class="sub-actions-row">
                  <button class="step-btn sub mini" @click="executeStep3('trend')">
                    📈 均线多头
                  </button>
                  <button class="step-btn sub mini" @click="executeStep3('macd')">
                    ⚡ 金叉
                  </button>
                  <button class="step-btn sub mini" @click="executeStep3('dividend')">
                    🛡️ 高股息
                  </button>
                </div>
              </div>
            </div>

            <!-- Step 4: 🔬 验体质与极限排雷 (L4 Deep-Dive & Red Flags) -->
            <div class="pipeline-card step-card-4 highlight-step" :class="{ current: pipelineContext.currentStep === 4 }">
              <div class="step-card-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l4">L4 验质</span>
                  <span class="step-title">四维体检与极限排雷</span>
                </div>
                <div class="step-tags">
                  <span class="mini-tag">VectorBT胜率</span>
                  <span class="mini-tag">质押/解禁</span>
                </div>
              </div>
              <p class="step-desc">
                对贯通标的展开 VectorBT 胜率测算、均线量价及财务体检，排查股权质押与现金流暗雷。
              </p>
              <div class="step-actions">
                <button class="step-btn primary highlight" @click="executeStep4('diag')">
                  🔬 深度体检 ({{ pipelineContext.stockName || pipelineContext.stockCode }})
                </button>
                <div class="sub-actions-row">
                  <button class="step-btn warning mini" @click="executeStep4('risk')">
                    🛡️ 股权质押排雷
                  </button>
                  <button class="step-btn sub mini" @click="executeStep4('finance')">
                    📑 经营现金流
                  </button>
                </div>
              </div>
            </div>

            <!-- Step 5: 📑 定落子与立论 (L5 Verdict & Thesis Ledger) -->
            <div class="pipeline-card step-card-5 highlight-step" :class="{ current: pipelineContext.currentStep === 5 }">
              <div class="step-card-header">
                <div class="step-badge-wrap">
                  <span class="step-badge l5">L5 终局</span>
                  <span class="step-title">买方立论与 5-Verdict</span>
                </div>
                <div class="step-tags">
                  <span class="mini-tag">5-Verdict评级</span>
                  <span class="mini-tag">证伪底线入库</span>
                </div>
              </div>
              <p class="step-desc">
                严谨确立买方确定性评级与下行证伪防守底线，核心投资立论持久化写入认知账本。
              </p>
              <div class="step-actions">
                <button class="step-btn primary success" @click="executeStep5()">
                  🚀 5-Verdict 裁决入库 ({{ pipelineContext.stockName || pipelineContext.stockCode }})
                </button>
                <button class="step-btn sub" @click="viewVerdictStandards">
                  📖 5-Verdict 评级规范
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
                <div class="tool-input-row">
                  <n-input
                    v-model:value="directDiagCode"
                    placeholder="输入 6 位代码或名称 (如 300750)"
                    size="small"
                    class="tool-input"
                  />
                  <button
                    class="tool-fill-target-btn"
                    @click="directDiagCode = pipelineContext.stockCode"
                    title="填入当前贯通标的代码"
                  >
                    🎯 填入贯通标的
                  </button>
                </div>
                <div class="quick-hot-row">
                  <button
                    v-for="stk in hotStocks.slice(0, 5)"
                    :key="stk.code"
                    class="hot-chip"
                    :class="{ active: directDiagCode === stk.code }"
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
                <div class="tool-input-row">
                  <n-input
                    v-model:value="directRiskCode"
                    placeholder="输入代码或名称 (如 600519)"
                    size="small"
                    class="tool-input"
                  />
                  <button
                    class="tool-fill-target-btn"
                    @click="directRiskCode = pipelineContext.stockCode"
                    title="填入当前贯通标的代码"
                  >
                    🎯 填入贯通标的
                  </button>
                </div>
                <div class="quick-hot-row">
                  <button
                    v-for="stk in hotStocks.slice(0, 5)"
                    :key="stk.code"
                    class="hot-chip"
                    :class="{ active: directRiskCode === stk.code }"
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
          @continue-wait="continueWaiting"
          @stop="handleStop"
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
      <!-- 快捷指令横条 (芯片式工具条，对话进行时展现，欢迎屏时不与工作台重复冲突) -->
      <div class="input-quick-bar" v-if="!isCompact && messages.length > 0">
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
        <ChatSender
          ref="chatSenderRef"
          v-model="inputValue"
          :placeholder="configOptions.length ? '输入个股代码/名称、ETF轮动需求或选股策略... (Enter发送，Shift+Enter换行)' : '⚠️ 请先选择或在设置中添加 AI 模型配置'"
          :loading="isStreaming"
          :min-rows="1"
          :max-rows="6"
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
        </ChatSender>
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
import { ref, computed, onMounted, onBeforeMount, onBeforeUnmount, nextTick, provide } from 'vue'
import {
  NSelect,
  NButton,
  NTooltip,
  NDrawer,
  NDrawerContent,
  NModal,
  NSwitch,
  NInput,
  useMessage,
  useDialog,
} from 'naive-ui'
import {
  ArrowDownOutline as ArrowDownIcon,
  TrashOutline as DeleteIcon,
  ExpandOutline as FullscreenIcon,
  ContractOutline as FullscreenExitIcon,
  ChatbubbleEllipsesOutline as ChatIcon,
  AddOutline as AddIcon,
  ListOutline as ViewListIcon,
} from '@vicons/ionicons5'
import { ChatWithAgent, GetConfig, GetVersionInfo, GetSystemStateSnapshot, AbortChatWithAgent } from '../../wailsjs/go/main/App'
import { useAiConfigStore } from '@/stores/aiConfig'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { cleanFinalContent, stripAllAgentActionBlocks } from '@/utils/cleanText'
import { copyToClipboard } from '@/utils/clipboard'
import { consumePendingConsult, dispatchAgentAction } from '../utils/agentControlBus'
import {
  useMessageAnchor,
  type ChatMessage,
  type StreamChunk,
  genMsgId
} from './chat'
import ChatMessageComp from './chat/ChatMessage.vue'
import MessageAnchorNav from './chat/MessageAnchorNav.vue'

import ChatSender from './chat/ChatSender.vue'
import { useAgentChatSessions } from '../composables/useAgentChatSessions'
import { useAgentChatPipeline } from '../composables/useAgentChatPipeline'
import { useAgentChatThesis } from '../composables/useAgentChatThesis'




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
const agentMode = ref<'auto' | 'react' | 'plan_execute' | 'pipeline'>('auto')
const userAvatar = ref('https://tdesign.gtimg.com/site/avatar.jpg')
const userName = ref('用户')
const messagesContainer = ref<HTMLElement | null>(null)










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
  // 流式输出期间不刷新感知快照，避免抢占 RPC 带宽导致 AI 分析卡顿
  if (isStreaming.value) return
  try {
    const snap = await GetSystemStateSnapshot()
    if (snap) {
      perceptionSnapshot.value = snap
    }
  } catch (e) {
    console.warn('[agent-chat] refreshPerceptionSnapshot failed:', e)
  }
}

function getTrafficText(light?: string) {
  if (light === 'SAFE') return '积极做多'
  if (light === 'CAUTION') return '谨慎防守'
  if (light === 'DANGER') return '强风控空仓'
  return '中性观望'
}

// ========== 抽取的 composable：会话 / 流水线 / 认知账本（R-22c） ==========
const {
  sessions,
  currentSessionId,
  messages,
  currentSessionTitle,
  loadSessionsFromStorage,
  saveSessionsToStorage,
  createNewSession,
  switchSession,
  deleteSession,
  formatSessionTime,
} = useAgentChatSessions({ showSessionDrawer, scrollToBottom })
const {
  welcomeViewMode,
  directDiagCode,
  directRiskCode,
  pkCodeA,
  pkCodeB,
  hotStocks,
  showStockActionModal,
  targetStockAction,
  modalStockInput,
  modalStockInputB,
  stockActionModalTitle,
  isModalStockActionValid,
  setModalPK,
  openStockActionModal,
  submitModalStockAction,
  pipelineContext,
  setPipelineStock,
  onPipelineStockCodeChange,
  onPipelineStockCodeEnter,
  extractStockFromAssistantText,
  executeStep1,
  executeStep2,
  executeStep3,
  executeStep4,
  executeStep5,
  viewVerdictStandards,
  executeDirectDiag,
  executeDirectRisk,
  executeDirectPK,
  applyPresetPK,
  handleQuickChipClick,
  pipelineNextStep,
} = useAgentChatPipeline({
  applyScenario,
  fillInput,
  inputValue,
  messages,
  perceptionSnapshot,
  getTrafficText,
})
const {
  lastConsultStock,
  handleSaveThesisFromMsg,
  handleExportImage,
} = useAgentChatThesis({ messages, notifyWarning, notifySuccess, notifyError })

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
  { category: 'L1 大势与宽基', title: '核心宽基行情与量价均线穿透', prompt: '请调用系统内置 GetMarketData 获取上证指数(000001)、沪深300(000300)与创业板指(399006)的最新点位、涨跌幅与量价均线态势，结合全市场涨跌比，评估市场多空赚钱效应与总仓位上限。' },
  
  // L2 赛道主线与资金风向
  { category: 'L2 赛道与主线', title: 'ETF 加权动量轮动模型', prompt: '请调用系统内置 GetETFMomentumRanking 获取主流场内ETF的21日加权动量排名表，按动量评分排序并给出当前最具超额弹性的进攻主线与防御调仓建议。直接输出研报正文。' },
  { category: 'L2 赛道与主线', title: '上市公司法定公告与行业穿透', prompt: '请调用系统内置 StockNotice / GetTelegraphList 工具深入核验半导体及核心赛道上市公司的最新法定公告披露、订单备案与产业链快讯。严禁使用 WebSearch。直接输出客观证据链。' },
  { category: 'L2 赛道与主线', title: '龙虎榜机构专用席位追踪', prompt: '请调用系统内置 GetLongTigerList / GetStockBillboard 工具获取最近一个交易日的龙虎榜明细，重点筛选机构专用席位大额净买入的股票，分析机构资金的真实建仓意图。严禁使用 WebSearch。' },

  // L3 量化多因子选股池
  { category: 'L3 量化选股池', title: '强势平台放量突破选股', prompt: '请利用系统内置量化筛选工具（ScreenByStrategy / FilterStocks / HotStrategyTable），筛选突破近期整理平台且放量上涨、均线多头排列的强势标的，给出具体技术面评分与入选逻辑。严禁使用 WebSearch。' },
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


// ========== Agent 流式通信监听 ==========
function handleAgentMessage(data: StreamChunk) {
  if (!isStreaming.value) return

  const curList = messages.value
  const targetMsg = curList.find(m => m.id === streamingMsgId.value)
  if (!targetMsg) return

  // 收到任何事件都重置看门狗：有动静就说明流还活着
  if (isStreaming.value) {
    armStreamWatchdog()
  }

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

    // 动作指令追踪与自动调度执行
    if (data.actions && data.actions.length > 0) {
      if (!targetMsg.actions) targetMsg.actions = []
      targetMsg.actions.push(...data.actions)
      for (const act of data.actions) {
        dispatchAgentAction(act)
      }
    }

    // 正文流式追加与结束判断
    if (data.content) {
      if (data.content === 'agent-DONE') {
        if (data.metrics) targetMsg.metrics = data.metrics
        finalizeStream(targetMsg)
        return
      } else {
        targetMsg.rawContent = (targetMsg.rawContent || '') + data.content
        // 流式过程中即时过滤未闭合/闭合的 agent-action，杜绝 JSON 代码块直接闪烁污染界面
        targetMsg.content = stripAllAgentActionBlocks(targetMsg.rawContent)
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

    // 结束处理：**任何终态** finish_reason 都必须收尾，而不只是 'stop'
    const finishReason = data.finish_reason || data.response_meta?.finish_reason
    if (finishReason && TERMINAL_FINISH_REASONS.has(finishReason)) {
      if (finishReason !== 'stop' && !targetMsg.streamNotice) {
        targetMsg.streamNotice =
          FINISH_REASON_NOTICE[finishReason] || `⚠️ 模型以「${finishReason}」结束本次输出，正文可能不完整。`
      }
      finalizeStream(targetMsg)
      return
    }
  }
}

/**
 * OpenAI 兼容协议中的**终态** finish_reason。
 *
 * 旧实现只认 'stop'，于是 'length' / 'content_filter' 会让界面永远停在「思考中」。
 * 注意：刻意**不**把 'tool_calls' / 'function_call' 列为终态 ——
 * 它们在 Agent 多轮工具循环中是合法的中间态，当成终态会截断正常的推演流程；
 * 这类情况由后端的终态护栏负责任地收尾。
 */
const TERMINAL_FINISH_REASONS = new Set(['stop', 'length', 'content_filter'])

const FINISH_REASON_NOTICE: Record<string, string> = {
  length: '⚠️ 输出达到长度上限被截断。可在「AI 模型注册表 → 高级调度参数」调大「最大输出 Tokens」，或把问题拆小后重试。',
  content_filter: '⚠️ 输出被模型服务商的内容安全策略拦截，正文可能不完整。可换个问法或更换模型重试。'
}

// ── 流式看门狗 ────────────────────────────────────────────────────────────
// 后端已用 defer 护栏保证「任何返回路径都会发送终态」，因此这里只是
// 「SSE 断开且没有任何终态到达」的兜底。
//
// ⚠️ 阈值必须大于**最慢一次真实运行**，否则会把仍在正常推演的任务掐死。
// 踩坑记录：初版取 180s，生产实测慢速模型（NVIDIA）单次流水线要
//   283.5s / 382.8s / 422.7s。现取 300s（5 分钟）：显著高于实测最坏
//   422.7s，又短于"真的该放弃"的尺度。若将来出现更慢的模型，请同步
//   上调本值。
//
// 设计原则：看门狗超时时**不直接终止流**，而是给用户选择"继续等待"
// 或"停止"。推理模型（如 DeepSeek-R1、Nemotron）单次思考可能需
// 2-4 分钟，直接掐断会导致用户白费一次完整的推理过程。
const STREAM_SILENCE_TIMEOUT_MS = 300_000
let streamWatchdogId: number | null = null

function clearStreamWatchdog() {
  if (streamWatchdogId !== null) {
    clearTimeout(streamWatchdogId)
    streamWatchdogId = null
  }
}

function armStreamWatchdog() {
  clearStreamWatchdog()
  streamWatchdogId = window.setTimeout(() => {
    streamWatchdogId = null
    if (!isStreaming.value) return
    const target = messages.value.find(m => m.id === streamingMsgId.value)
    if (target) {
      // 超时时不直接终止流，而是提示用户选择：继续等待或停止。
      // 注意：这里只写**纯文本** + 一个布尔标记，按钮由 AssistantMessage 用
      // 真实模板渲染并通过事件回传。曾把 `<button>` 拼进 streamNotice，
      // 而模板用 {{ }} 文本插值，导致标签被原样显示、点击无效。
      target.streamNotice =
        '⚠️ 已超过 5 分钟未收到模型任何输出，模型可能在深度思考或连接中断。'
      target.streamTimeout = true
    }
    // 注意：此处不立即 finalizeStream，保留流式状态，让用户可选择继续等待
  }, STREAM_SILENCE_TIMEOUT_MS)
}

function finalizeStream(targetMsg?: ChatMessage) {
  clearStreamWatchdog()
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''
  // 流已终结：撤下看门狗超时按钮（无论后续是否继续等待，都不应再显示）
  if (targetMsg) {
    targetMsg.streamTimeout = false
  }

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
  // 启动看门狗：若长时间无任何事件，自动收尾并告知（防止永久卡在流式状态）
  armStreamWatchdog()

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

function handleStop(msgId?: string) {
  try {
    AbortChatWithAgent()
  } catch (e) {
    console.warn('AbortChatWithAgent failed:', e)
  }
  // 撤下看门狗超时按钮（支持从具体消息发起）
  const target = msgId
    ? messages.value.find(m => m.id === msgId)
    : messages.value.find(m => m.id === streamingMsgId.value)
  if (target) {
    target.streamTimeout = false
  }
  clearStreamWatchdog()
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''
  saveSessionsToStorage()
  notifyInfo('已停止响应')
}

// 用户选择"继续等待"时：清除超时提示，重置看门狗，保持流式状态
function continueWaiting(msgId?: string) {
  const target = msgId
    ? messages.value.find(m => m.id === msgId)
    : messages.value.find(m => m.id === streamingMsgId.value)
  if (target) {
    target.streamNotice = ''
    target.streamTimeout = false
  }
  // 重置看门狗，继续等待模型响应
  armStreamWatchdog()
  notifyInfo('已继续等待模型响应，请稍候...')
}

// ========== 场景交互辅助 ==========
/**
 * 把一段提示词填入输入框（可选自动发送）。
 *
 * mode：显式指定本次运行的 Agent 模式。
 *   · 五阶流水线入口传 'pipeline' —— 后端据此启用流水线专用能力闸门
 *     （移除外网检索 WebSearch / FetchWebPage）。此前流水线与普通对话都传
 *     空 mode，后端无法区分，导致「严禁调用 WebSearch」只能靠提示词约束。
 *   · 不传则沿用用户当前选择的模式。
 */
function applyScenario(promptText: string, autoSend: boolean = false, mode?: 'auto' | 'react' | 'plan_execute' | 'pipeline') {
  if (mode) {
    agentMode.value = mode
  }
  inputValue.value = promptText
  nextTick(() => {
    const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
    if (textarea) {
      textarea.focus()
      // 自动计算并调整高度，确保多行内容完整可见
      textarea.style.height = 'auto'
      textarea.style.height = `${Math.min(Math.max(textarea.scrollHeight, 68), 240)}px`
      // 平滑滚动至输入框
      textarea.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
    if (autoSend) {
      handleSend()
    } else {
      notifyInfo('💡 已将投研指令载入输入框，您可按需调整修改后回车或点击发送 🚀')
    }
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
  if (mode && ['auto', 'react', 'plan_execute', 'direct', 'pipeline'].includes(mode)) {
    agentMode.value = mode as any
  }

  // 4. 组装输入框文本 (一手量化指标前置注入 + 深度分析推演任务)
  // 两个分支都必定赋值，故不设初值（no-useless-assignment）
  let finalInput: string
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

function onBeforeUnloadHandler() {
  if (isStreaming.value) {
    try {
      if (typeof navigator !== 'undefined' && navigator.sendBeacon) {
        navigator.sendBeacon('/api/App/AbortChatWithAgent', JSON.stringify([]))
      } else {
        AbortChatWithAgent().catch(() => {})
      }
    } catch {}
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
    window.addEventListener('beforeunload', onBeforeUnloadHandler)
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
  clearStreamWatchdog()
  // 核心风控：组件卸载或切出时，若正在流式推演，立即物理中止后端推理，严禁空转浪费 Token 与并发配额
  if (isStreaming.value) {
    AbortChatWithAgent().catch(() => {})
    isStreaming.value = false
  }

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
    window.removeEventListener('beforeunload', onBeforeUnloadHandler)
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
  background: var(--bs-bg-page, #f8f9fa);
  color: var(--bs-text-primary, #1d2129);
  position: relative;
  overflow: hidden;
}

/* 顶部 HUD 控制面板 */
.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--bs-bg-container, #ffffff);
  border-bottom: 1px solid var(--bs-border-1, #e5e6eb);
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
        color: var(--bs-text-primary, #1d2129);
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
        background: var(--bs-bg-secondary, #f2f3f5);
        color: var(--bs-text-secondary, #4e5969);
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
      background: var(--bs-bg-secondary, #f2f3f5);
      border: 1px solid var(--bs-border-1, #e5e6eb);
      white-space: nowrap;
      transition: all 0.2s ease;

      .header-switch-label {
        font-size: 12px;
        color: var(--bs-text-secondary, #4e5969);
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
    background: var(--bs-border-2, #c9cdd4);
    border-radius: 3px;
  }
}

/* 欢迎页买方量化工作台 (BUIS Quantitative Cockpit) */
.welcome-workspace {
  max-width: 1360px;
  width: 100%;
  margin: 12px auto 36px;
  display: flex;
  flex-direction: column;
  align-items: center;

  .welcome-hero {
    text-align: center;
    margin-bottom: 20px;

    .hero-badge {
      display: inline-block;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 1.5px;
      text-transform: uppercase;
      padding: 3px 12px;
      border-radius: 12px;
      background: linear-gradient(135deg, rgba(0, 82, 217, 0.12) 0%, rgba(43, 164, 113, 0.12) 100%);
      color: var(--bs-brand, #0052d9);
      margin-bottom: 8px;
      border: 1px solid rgba(0, 82, 217, 0.2);
    }

    .hero-title {
      font-size: 22px;
      font-weight: 800;
      margin: 0 0 6px;
      letter-spacing: 0.3px;
      background: linear-gradient(135deg, var(--bs-brand, #0052d9) 0%, #2ba471 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    .hero-desc {
      font-size: 13px;
      color: var(--bs-text-secondary, #86909c);
      max-width: 720px;
      line-height: 1.6;
      margin: 0 auto 16px;
    }

    /* 模式切换：标准五阶决策梯 vs 专项极速穿透台 */
    .welcome-mode-toggle {
      display: inline-flex;
      background: var(--bs-bg-component, #f2f3f5);
      padding: 4px;
      border-radius: 20px;
      gap: 6px;
      box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.05);

      .toggle-tab-btn {
        border: none;
        background: transparent;
        font-size: 13px;
        font-weight: 500;
        padding: 6px 16px;
        border-radius: 16px;
        cursor: pointer;
        color: var(--bs-text-secondary, #4e5969);
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 6px;

        &:hover {
          color: var(--bs-text-primary, #1d2129);
        }

        &.active {
          background: var(--bs-bg-container, #ffffff);
          color: var(--bs-brand, #0052d9);
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
    gap: 14px;

    /* 阶段引导流程轴 */
    .pipeline-flow-guide {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      padding: 8px 16px;
      background: var(--bs-bg-container, #ffffff);
      border: 1px solid var(--bs-border-1, #e5e6eb);
      border-radius: 10px;
      overflow-x: auto;
      white-space: nowrap;

      .flow-step-node {
        font-size: 12px;
        font-weight: 600;
        color: var(--bs-text-secondary, #4e5969);
        padding: 5px 12px;
        background: var(--bs-bg-component, #f2f3f5);
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
          font-size: 11px;
          font-weight: 700;
          color: #ffffff;

          &.l1 { background: #0052d9; }
          &.l2 { background: #fa8c16; }
          &.l3 { background: #722ed1; }
          &.l4 { background: #e11d48; }
          &.l5 { background: #059669; }
        }

        &:hover {
          background: var(--bs-brand-light, #ecf2fe);
          color: var(--bs-brand, #0052d9);
        }

        &.current {
          background: var(--bs-brand, #0052d9);
          color: #ffffff;

          .step-num {
            background: rgba(255, 255, 255, 0.3);
          }
        }
      }

      .flow-arrow {
        color: var(--bs-text-placeholder, #c9cdd4);
        font-size: 12px;
      }
    }

    /* 全局流水线贯通指示栏 (单入口闭环设计) */
    .pipeline-context-banner {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 16px;
      background: linear-gradient(135deg, rgba(0, 82, 217, 0.05) 0%, rgba(102, 126, 234, 0.07) 100%);
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
          font-weight: 700;
          color: var(--bs-text-primary, #1d2129);
        }

        .banner-stock-pill {
          font-size: 13px;
          font-weight: 700;
          color: #0052d9;
          background: var(--bs-bg-container, #ffffff);
          padding: 3px 10px;
          border-radius: 14px;
          border: 1px solid rgba(0, 82, 217, 0.3);
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
          cursor: pointer;
          display: inline-flex;
          align-items: center;
          gap: 5px;
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

        .banner-quick-input-wrap {
          width: 90px;
          .banner-quick-input {
            font-size: 12px;
            font-family: monospace;
          }
        }

        .banner-tag {
          font-size: 12px;
          padding: 2px 8px;
          border-radius: 12px;
          background: rgba(0, 0, 0, 0.04);
          color: var(--bs-text-secondary, #4e5969);

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
            color: var(--bs-text-placeholder, #86909c);
          }

          .hot-chip-btn {
            font-size: 11px;
            padding: 2px 8px;
            border-radius: 10px;
            background: var(--bs-bg-container, #ffffff);
            border: 1px solid var(--bs-border-1, #e5e6eb);
            color: var(--bs-text-secondary, #4e5969);
            cursor: pointer;
            transition: all 0.15s ease;

            &:hover {
              color: var(--bs-brand, #0052d9);
              border-color: var(--bs-brand, #0052d9);
            }

            &.active {
              background: var(--bs-brand, #0052d9);
              color: #ffffff;
              border-color: var(--bs-brand, #0052d9);
            }
          }
        }

        .banner-switch-btn {
          border: 1px solid var(--bs-border-2, #dcdcdc);
          background: var(--bs-bg-container, #ffffff);
          color: var(--bs-text-primary, #1d2129);
          padding: 3px 10px;
          border-radius: 6px;
          font-size: 12px;
          font-weight: 500;
          cursor: pointer;
          transition: all 0.2s ease;

          &:hover {
            border-color: var(--bs-brand, #0052d9);
            color: var(--bs-brand, #0052d9);
          }
        }
      }
    }

    /* 五阶时序决策卡片矩阵：严格等高、网格对称对齐 */
    .pipeline-steps-grid {
      display: grid;
      grid-template-columns: repeat(5, minmax(0, 1fr));
      gap: 12px;

      @media (max-width: 1200px) {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }

      @media (max-width: 768px) {
        grid-template-columns: 1fr;
      }
    }

    .pipeline-card {
      background: var(--bs-bg-container, #ffffff);
      border: 1px solid var(--bs-border-1, #e5e6eb);
      border-radius: 12px;
      padding: 14px 12px;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      transition: all 0.25s ease;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);
      min-height: 220px;

      &:hover {
        border-color: var(--bs-brand, #0052d9);
        transform: translateY(-2px);
        box-shadow: 0 6px 16px rgba(0, 82, 217, 0.08);
      }

      &.current {
        border-color: var(--bs-brand, #0052d9);
        box-shadow: 0 0 0 1px var(--bs-brand, #0052d9);
      }

      &.highlight-step {
        border-color: rgba(0, 82, 217, 0.35);
        background: linear-gradient(180deg, var(--bs-bg-container, #ffffff) 0%, rgba(0, 82, 217, 0.02) 100%);
      }

      .step-card-header {
        margin-bottom: 8px;

        .step-badge-wrap {
          display: flex;
          align-items: center;
          gap: 6px;
          margin-bottom: 6px;

          .step-badge {
            font-size: 11px;
            padding: 2px 6px;
            border-radius: 4px;
            font-weight: 700;
            letter-spacing: 0.3px;
            white-space: nowrap;

            &.l1 { background: rgba(0, 82, 217, 0.12); color: #0052d9; }
            &.l2 { background: rgba(240, 160, 32, 0.14); color: #d46b08; }
            &.l3 { background: rgba(114, 46, 209, 0.14); color: #722ed1; }
            &.l4 { background: rgba(225, 29, 72, 0.12); color: #e11d48; }
            &.l5 { background: rgba(5, 150, 105, 0.14); color: #059669; }
          }

          .step-title {
            font-size: 13px;
            font-weight: 700;
            color: var(--bs-text-primary, #1d2129);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
          }
        }

        .step-tags {
          display: flex;
          align-items: center;
          gap: 4px;
          flex-wrap: wrap;

          .mini-tag {
            font-size: 10px;
            padding: 1px 5px;
            border-radius: 4px;
            background: var(--bs-bg-component, #f2f3f5);
            color: var(--bs-text-secondary, #4e5969);
            border: 1px solid var(--bs-border-1, #e5e6eb);
          }
        }
      }

      .step-desc {
        font-size: 12px;
        color: var(--bs-text-secondary, #4e5969);
        line-height: 1.5;
        margin: 0 0 12px;
        min-height: 48px;
        max-height: 48px;
        overflow: hidden;
        display: -webkit-box;
        -webkit-line-clamp: 2;
        -webkit-box-orient: vertical;
      }

      .step-actions {
        display: flex;
        flex-direction: column;
        gap: 6px;
        margin-top: auto;

        .sub-actions-row {
          display: flex;
          align-items: center;
          gap: 4px;
          flex-wrap: wrap;
        }

        .step-btn {
          border: none;
          padding: 6px 10px;
          border-radius: 6px;
          font-size: 12px;
          font-weight: 500;
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 4px;
          transition: all 0.2s ease;
          width: 100%;

          &.mini {
            flex: 1;
            padding: 4px 6px;
            font-size: 11px;
            white-space: nowrap;
          }

          &.primary {
            background: var(--bs-brand, #0052d9);
            color: #ffffff;
            font-weight: 600;

            &:hover {
              background: #0044b8;
              box-shadow: 0 2px 8px rgba(0, 82, 217, 0.25);
            }

            &.highlight {
              background: linear-gradient(135deg, #0052d9 0%, #722ed1 100%);
              &:hover {
                box-shadow: 0 2px 8px rgba(114, 46, 209, 0.3);
              }
            }

            &.success {
              background: #059669;
              &:hover {
                background: #047857;
                box-shadow: 0 2px 8px rgba(5, 150, 105, 0.3);
              }
            }
          }

          &.warning {
            background: rgba(225, 29, 72, 0.1);
            color: #e11d48;
            border: 1px solid rgba(225, 29, 72, 0.25);

            &:hover {
              background: #e11d48;
              color: #ffffff;
            }
          }

          &.sub {
            background: var(--bs-bg-component, #f2f3f5);
            color: var(--bs-text-primary, #1d2129);
            border: 1px solid var(--bs-border-1, #e5e6eb);

            &:hover {
              background: var(--bs-brand-light, #ecf2fe);
              color: var(--bs-brand, #0052d9);
              border-color: var(--bs-brand, #0052d9);
            }
          }
        }
      }
    }
  }

  /* 视图 2：专项极速穿透台 */
  .direct-tools-container {
    width: 100%;

    .direct-tools-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 16px;

      @media (max-width: 900px) {
        grid-template-columns: 1fr;
      }
    }

    .direct-tool-card {
      background: var(--bs-bg-container, #ffffff);
      border: 1px solid var(--bs-border-1, #e5e6eb);
      border-radius: 12px;
      padding: 16px 18px;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);
      transition: all 0.25s ease;

      &:hover {
        border-color: var(--bs-brand, #0052d9);
        transform: translateY(-2px);
        box-shadow: 0 6px 16px rgba(0, 82, 217, 0.06);
      }

      .tool-head {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 12px;

        .tool-icon {
          font-size: 22px;
        }

        .tool-head-text {
          display: flex;
          flex-direction: column;
          gap: 2px;

          .tool-name {
            font-size: 14px;
            font-weight: 700;
            color: var(--bs-text-primary, #1d2129);
          }

          .tool-sub {
            font-size: 11px;
            color: var(--bs-text-secondary, #86909c);
          }
        }
      }

      .tool-form {
        display: flex;
        flex-direction: column;
        gap: 10px;

        .tool-input-row {
          display: flex;
          align-items: center;
          gap: 6px;

          .tool-input {
            flex: 1;
          }

          .tool-fill-target-btn {
            border: 1px solid var(--bs-border-2, #dcdcdc);
            background: var(--bs-bg-component, #f2f3f5);
            font-size: 11px;
            padding: 4px 8px;
            border-radius: 4px;
            cursor: pointer;
            white-space: nowrap;
            color: var(--bs-brand, #0052d9);
            font-weight: 500;
            transition: all 0.2s ease;

            &:hover {
              background: var(--bs-brand-light, #ecf2fe);
              border-color: var(--bs-brand, #0052d9);
            }
          }
        }

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
          gap: 5px;
          flex-wrap: wrap;

          .hot-chip {
            font-size: 11px;
            padding: 2px 7px;
            border-radius: 4px;
            border: 1px solid var(--bs-border-1, #e5e6eb);
            background: var(--bs-bg-component, #f2f3f5);
            color: var(--bs-text-secondary, #4e5969);
            cursor: pointer;
            transition: all 0.15s ease;

            &:hover,
            &.active {
              background: var(--bs-brand-light, #ecf2fe);
              border-color: var(--bs-brand, #0052d9);
              color: var(--bs-brand, #0052d9);
            }
          }
        }

        .tool-submit-btn {
          border: none;
          padding: 8px 14px;
          border-radius: 6px;
          font-size: 12px;
          font-weight: 600;
          cursor: pointer;
          background: var(--bs-brand, #0052d9);
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
    color: var(--bs-text-primary, #1d2129);
  }

  .next-step-btn {
    border: none;
    background: var(--bs-brand, #0052d9);
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
    color: var(--bs-text-primary, #1d2129);
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
      color: var(--bs-text-secondary, #86909c);
    }

    .preset-chip {
      font-size: 12px;
      padding: 3px 8px;
      border-radius: 4px;
      background: var(--bs-bg-component, #f2f3f5);
      color: var(--bs-text-secondary, #4e5969);
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover,
      &.active {
        background: var(--bs-brand-light, #ecf2fe);
        color: var(--bs-brand, #0052d9);
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
  background: var(--bs-bg-container, #ffffff);
  border: 1px solid var(--bs-border-1, #e5e6eb);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.12);
  border-radius: 20px;
  padding: 6px 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: var(--bs-brand, #0052d9);
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
  background: var(--bs-bg-container, #ffffff);
  border-top: 1px solid var(--bs-border-1, #e5e6eb);
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
      background: var(--bs-bg-secondary, #f2f3f5);
      border: 1px solid transparent;
      padding: 3px 10px;
      border-radius: 12px;
      cursor: pointer;
      font-size: 12px;
      color: var(--bs-text-secondary, #4e5969);
      white-space: nowrap;
      flex-shrink: 0;
      transition: all 0.2s;

      &:hover {
        background: var(--bs-brand-light, #ecf2fe);
        color: var(--bs-brand, #0052d9);
      }
    }

    .status-summary {
      display: flex;
      align-items: center;
      gap: 6px;
      color: var(--bs-brand, #0052d9);
      font-size: 12px;
      white-space: nowrap;
      flex-shrink: 0;
      margin-left: 10px;

      .live-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--bs-brand, #0052d9);
        animation: pulse-dot 1.5s infinite;
      }
    }
  }

  .input-sender-box {
    max-width: 1240px;
    width: 100%;
    margin: 0 auto;

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
      color: var(--bs-brand, #0052d9);
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
      background: var(--bs-bg-secondary, #f2f3f5);
      transition: all 0.2s;

      &:hover {
        background: var(--bs-bg-container, #ffffff);
        border-color: var(--bs-border-2, #dcdcdc);
      }

      &.active {
        background: var(--bs-brand-light, #ecf2fe);
        border-color: var(--bs-brand, #0052d9);

        .session-name {
          color: var(--bs-brand, #0052d9);
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
          color: var(--bs-text-placeholder, #86909c);
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
    background: var(--bs-bg-secondary, #f8f9fa);
    border: 1px solid var(--bs-border-1, #e5e6eb);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      border-color: var(--bs-brand, #0052d9);
      background: var(--bs-brand-light, #ecf2fe);
    }

    .item-head {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 6px;

      .item-tag {
        font-size: 11px;
        background: var(--bs-brand, #0052d9);
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
      color: var(--bs-text-secondary, #4e5969);
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

/* ---------------------------------------------------------------------------
   深色模式：改由 style.css 的 --bs-* 主题变量统一驱动。

   这里原先有一段 :global([theme-mode="dark"]) 包裹的 263 行暗色覆盖，
   但该写法在 scoped LESS 下**未被编译进产物**（实测 dist CSS 中不存在任何
   theme-mode="dark" 选择器），属于静默失效的死代码。
   配色已通过 --bs-* 变量（见 style.css）在组件样式内部自动切换，
   此处不再重复声明，避免两套来源互相冲突。
   （若将来确实需要某些暗色专属微调，请写在**非 scoped** 的 <style> 块中，
     并使用 html[theme-mode="dark"] 前缀，不要用 :global() 包裹嵌套规则。）
   --------------------------------------------------------------------------- */
</style>
