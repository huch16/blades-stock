// 前端 AI 组件回归守卫（C 组，2026-11-22）。
//
// 背景：本轮修掉的 AI 组件问题大多是「源码级静默错误」——字段名写错、
// 运算符优先级、永假的 v-if 门禁、死代码导出。Vue SFC 无法被 node:test
// 直接 import，因此这里用源码断言做回归守卫：改动回退时测试即失败。
// 若将来补上 Vue 组件测试基建，应把这些断言升级为真正的组件/行为测试。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const read = (rel) => fs.readFileSync(path.join(root, rel), 'utf8')
const exists = (rel) => fs.existsSync(path.join(root, rel))

test('ChatMessage 头像：assistant 头像 URL 不得被三元运算符优先级吞掉', () => {
  const src = read('src/components/chat/ChatMessage.vue')
  // 正确形态：`avatar || (cond ? '🏠' : '🤖')`；错误形态是
  // `avatar || cond ? '🏠' : '🤖'`（|| 高于 ?:，URL 非空时恒返回 '🏠'）。
  assert.match(src, /return props\.message\.avatar \|\| \(aiConfigStore\.activeConfig\?\.baseUrl\?\.includes\('localhost'\) \? '🏠' : '🤖'\)/)
  assert.doesNotMatch(src, /return props\.message\.avatar \|\| aiConfigStore\.activeConfig\?\.baseUrl\?\.includes\('localhost'\) \? '🏠' : '🤖'/)
})

test('AssistantMessage 看门狗提示：超时按钮在流式进行中就必须可见', () => {
  const src = read('src/components/chat/AssistantMessage.vue')
  // 旧条件 `message.streamNotice && !isStreaming` 把超时提示藏到流结束之后，
  // 而 finalizeStream 会清掉 streamTimeout，于是「继续等待/停止」按钮永远点不到。
  assert.match(src, /v-if="message\.streamNotice && \(!isStreaming \|\| message\.streamTimeout\)"/)
  // 空回复指引里提到的折叠区标题必须与实际标题一致（🧠 思考过程）。
  assert.doesNotMatch(src, /「💭 思考过程」/)
  assert.match(src, /「🧠 思考过程」/)
})

test('AssistantMessage：jsonMarkdown 结构化报告区已删除（无任何生产者）', () => {
  const assistant = read('src/components/chat/AssistantMessage.vue')
  const types = read('src/components/chat/types.ts')
  assert.doesNotMatch(assistant, /jsonMarkdown/)
  assert.doesNotMatch(types, /jsonMarkdown/)
})

test('chat/useChatStream 是死代码：文件与导出均已删除', () => {
  assert.equal(exists('src/components/chat/useChatStream.ts'), false)
  const index = read('src/components/chat/index.ts')
  assert.doesNotMatch(index, /useChatStream/)
})

test('useAgentChatThesis 导出图片：必须按真实 DOM id `msg-<id>` 查找', () => {
  const src = read('src/composables/useAgentChatThesis.ts')
  assert.match(src, /getElementById\('msg-' \+ msg\.id\)/)
  // 旧查找串只应出现在解释性注释里，不得再出现在可执行代码中。
  assert.doesNotMatch(src, /getElementById\('msg-anchor-/)
  assert.doesNotMatch(src, /getElementById\('msg-item-/)
  assert.doesNotMatch(src, /closest\('\.t-chat__message-item'\)/)
})

test('StockDiagnosisDrawer 发给 Agent 的上下文必须用后端真实字段名', () => {
  const src = read('src/components/StockDiagnosisDrawer.vue')
  // 后端 quant.StockDiagnosisResult 只有 overallScore/suggestion/
  // targetPrice/stopLossPrice，没有 totalScore/advice/supportPrice/resistancePrice。
  assert.match(src, /d\.overallScore/)
  assert.match(src, /d\.suggestion/)
  assert.match(src, /d\.targetPrice/)
  assert.match(src, /d\.stopLossPrice/)
  assert.doesNotMatch(src, /d\.totalScore/)
  assert.doesNotMatch(src, /d\.advice/)
  assert.doesNotMatch(src, /d\.supportPrice/)
  assert.doesNotMatch(src, /d\.resistancePrice/)
  // 诊断失败后必须能立即重试：时间戳只在成功后写。
  assert.match(src, /lastDiagnosisTime\.set\(code, Date\.now\(\)\)/)
})

test('ToolTimeline：参数/结果详情不得再被永假的 map 门禁锁死', () => {
  const src = read('src/components/chat/ToolTimeline.vue')
  assert.doesNotMatch(src, /showArgs\[/)
  assert.doesNotMatch(src, /showResult\[/)
  // 流式中后追加的 pending 调用也要有进度定时器（watch calls 变化）。
  assert.match(src, /watch\(\s*\(\) => props\.calls\.map/)
})

test('researchReport：不得再传后端不存在的 order 参数', () => {
  const src = read('src/components/researchReport.vue')
  assert.doesNotMatch(src, /order:\s*["']desc["']/)
  assert.doesNotMatch(src, /order = 'desc'/)
  // 删除必须二次确认；模板里不得再有未声明的 tipsRef。
  assert.match(src, /dialog\.warning\(\{\s*title: '确认删除'/)
  assert.doesNotMatch(src, /ref="tipsRef"/)
})

test('IndustryResearchReportList：loading 提示必须在 settled 后销毁', () => {
  const src = read('src/components/IndustryResearchReportList.vue')
  assert.match(src, /const destroyLoading = message\.loading\("正在刷新数据\.\.\."\)/)
  assert.match(src, /destroyLoading\(\)/)
  assert.match(src, /刷新行业研报失败/)
})

test('MarketIntelligenceDrawer：旧生成晚到时不得覆盖新一轮', () => {
  const src = read('src/components/MarketIntelligenceDrawer.vue')
  assert.match(src, /let generationSeq = 0/)
  assert.match(src, /const mySeq = \+\+generationSeq/)
  assert.match(src, /if \(mySeq !== generationSeq\) return/)
  assert.match(src, /startPollingForCompletion\(currentTab\.value, startTime, mySeq\)/)
})

test('agent-chat 看门狗注释不得再声称 300s 高于实测最坏 422.7s', () => {
  const src = read('src/components/agent-chat.vue')
  assert.doesNotMatch(src, /显著高于实测最坏/)
  assert.match(src, /仍\*\*低于\*\*实测最坏/)
  // ChatSender 的死 ref 已删。
  assert.doesNotMatch(src, /ref="chatSenderRef"/)
})

test('cron-task-manager：立即执行的失败文案不得再弹 success', () => {
  const src = read('src/components/cron-task-manager.vue')
  assert.match(src, /\/不存在\|失败\|错误\/\.test\(text\)/)
})
