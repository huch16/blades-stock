<template>
  <div v-if="hasError" class="error-boundary-container">
    <div class="error-boundary-card">
      <div class="error-icon">⚠️</div>
      <h3 class="error-title">视图组件异常已自动隔离</h3>
      <p class="error-desc">
        当前视图因数据渲染异常触发保护性熔断，系统已阻止异常扩散，整体应用与核心数据未受影响。
      </p>

      <div v-if="isDeterministicError" class="deterministic-warning">
        <span class="warning-icon">⛔</span>
        <div class="warning-text">
          <strong>检测到确定性代码缺陷：</strong>
          该视图连续重试多次均抛出相同异常，重试已自动熔断以防死循环。请一键复制诊断信息联系维护人员。
        </div>
      </div>
      
      <div class="error-actions">
        <button 
          class="retry-btn" 
          :class="{ 'btn-disabled': isDeterministicError }" 
          :disabled="isDeterministicError"
          @click="handleRetry"
        >
          <span v-if="isDeterministicError">⛔ 自动重试已熔断</span>
          <span v-else>🔄 重新加载视图 ({{ retryCount }}/2)</span>
        </button>

        <button class="copy-diag-btn" @click="copyDiagnostics">
          {{ copied ? '✅ 诊断信息已复制' : '📋 复制技术诊断' }}
        </button>

        <button class="details-toggle-btn" @click="showDetails = !showDetails">
          {{ showDetails ? '收起技术详情' : '展开技术详情' }}
        </button>

        <button class="home-btn" @click="goHome">
          🏠 返回首页
        </button>
      </div>

      <div v-if="showDetails" class="error-details">
        <div class="error-meta">
          <div><strong>触发时间:</strong> {{ errorTime }}</div>
          <div><strong>路由:</strong> {{ currentRoute }}</div>
          <div><strong>重试次数:</strong> {{ retryCount }}</div>
        </div>
        <div class="error-msg"><strong>Error:</strong> {{ errorMessage }}</div>
        <pre v-if="errorStack" class="error-stack"><code>{{ errorStack }}</code></pre>
      </div>
    </div>
  </div>
  <slot v-else></slot>
</template>

<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'

const hasError = ref(false)
const errorMessage = ref('')
const errorStack = ref('')
const errorTime = ref('')
const currentRoute = ref('')
const showDetails = ref(false)
const copied = ref(false)

const retryCount = ref(0)
const lastErrorFingerprint = ref('')
const isDeterministicError = ref(false)

onErrorCaptured((err: any, instance, info) => {
  console.error('[ErrorBoundary] Caught component error:', err, 'at:', info)
  hasError.value = true
  errorMessage.value = err?.message || String(err)
  errorStack.value = err?.stack || ''
  errorTime.value = new Date().toLocaleString()
  currentRoute.value = window.location.hash || window.location.pathname

  const fingerprint = `${err?.name || 'Error'}:${errorMessage.value}`
  if (lastErrorFingerprint.value === fingerprint) {
    retryCount.value++
    if (retryCount.value >= 2) {
      isDeterministicError.value = true
    }
  } else {
    lastErrorFingerprint.value = fingerprint
    retryCount.value = 0
    isDeterministicError.value = false
  }
  
  // 返回 false 阻止异常继续向外层冒泡，防止整页白屏崩溃
  return false
})

function handleRetry() {
  if (isDeterministicError.value) return
  hasError.value = false
  errorMessage.value = ''
  errorStack.value = ''
  showDetails.value = false
  copied.value = false
}

function goHome() {
  hasError.value = false
  window.location.hash = '#/'
}

async function copyDiagnostics() {
  const diagData = [
    `=== Blades Stock 技术诊断报告 (ErrorBoundary) ===`,
    `时间: ${errorTime.value || new Date().toISOString()}`,
    `路由: ${currentRoute.value || window.location.href}`,
    `重试次数: ${retryCount.value}`,
    `确定性故障: ${isDeterministicError.value ? '是 (Deterministic Bug)' : '否 (Transient)'}`,
    `错误信息: ${errorMessage.value}`,
    `调用栈:`,
    errorStack.value || '无调用栈信息',
    `User-Agent: ${navigator.userAgent}`,
    `==============================================`
  ].join('\n')

  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(diagData)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = diagData
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    copied.value = true
    setTimeout(() => { copied.value = false }, 3000)
  } catch (e) {
    console.error('Failed to copy diagnostics:', e)
  }
}
</script>

<style scoped>
.error-boundary-container {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  min-height: 280px;
  width: 100%;
  box-sizing: border-box;
}

.error-boundary-card {
  background: var(--n-color, rgba(255, 255, 255, 0.05));
  border: 1px solid rgba(255, 160, 0, 0.3);
  border-radius: 12px;
  padding: 32px;
  max-width: 640px;
  width: 100%;
  text-align: center;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
}

.error-icon {
  font-size: 40px;
  margin-bottom: 12px;
}

.error-title {
  font-size: 18px;
  font-weight: 600;
  color: #f5a623;
  margin: 0 0 10px 0;
}

.error-desc {
  font-size: 13px;
  line-height: 1.6;
  color: rgba(255, 255, 255, 0.7);
  margin: 0 0 16px 0;
}

.deterministic-warning {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  text-align: left;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.4);
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 18px;
  color: #fca5a5;
  font-size: 12px;
  line-height: 1.5;
}

.warning-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.error-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-bottom: 16px;
}

.retry-btn {
  background: #18a058;
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.retry-btn:hover:not(:disabled) {
  background: #36ad6a;
}

.retry-btn.btn-disabled {
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.4);
  cursor: not-allowed;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.copy-diag-btn {
  background: rgba(32, 128, 240, 0.15);
  color: #70c0e8;
  border: 1px solid rgba(32, 128, 240, 0.3);
  border-radius: 6px;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.copy-diag-btn:hover {
  background: rgba(32, 128, 240, 0.25);
  color: #a0d8f5;
}

.details-toggle-btn {
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 6px;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
}

.details-toggle-btn:hover {
  color: rgba(255, 255, 255, 0.9);
  border-color: rgba(255, 255, 255, 0.4);
}

.home-btn {
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 6px;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
}

.home-btn:hover {
  color: rgba(255, 255, 255, 0.9);
  border-color: rgba(255, 255, 255, 0.4);
}

.error-details {
  margin-top: 16px;
  text-align: left;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 8px;
  padding: 12px;
  font-size: 12px;
  max-height: 220px;
  overflow-y: auto;
}

.error-meta {
  color: rgba(255, 255, 255, 0.5);
  font-size: 11px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  padding-bottom: 8px;
  margin-bottom: 8px;
  line-height: 1.5;
}

.error-msg {
  color: #ff6b6b;
  margin-bottom: 8px;
  word-break: break-all;
}

.error-stack {
  margin: 0;
  color: rgba(255, 255, 255, 0.5);
  font-family: monospace;
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
