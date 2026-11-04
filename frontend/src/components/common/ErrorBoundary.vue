<template>
  <div v-if="hasError" class="error-boundary-container">
    <div class="error-boundary-card">
      <div class="error-icon">⚠️</div>
      <h3 class="error-title">视图组件异常已自动隔离</h3>
      <p class="error-desc">
        当前视图因数据渲染异常触发保护性熔断，系统已阻止异常扩散，整体应用与核心数据未受影响。
      </p>
      
      <div class="error-actions">
        <button class="retry-btn" @click="resetError">
          🔄 重新加载视图
        </button>
        <button class="details-toggle-btn" @click="showDetails = !showDetails">
          {{ showDetails ? '收起技术详情' : '展开技术详情' }}
        </button>
      </div>

      <div v-if="showDetails" class="error-details">
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
const showDetails = ref(false)

onErrorCaptured((err: any, instance, info) => {
  console.error('[ErrorBoundary] Caught component error:', err, 'at:', info)
  hasError.value = true
  errorMessage.value = err?.message || String(err)
  errorStack.value = err?.stack || ''
  
  // 返回 false 阻止异常继续向外层冒泡，防止整页白屏崩溃 (P0-4)
  return false
})

function resetError() {
  hasError.value = false
  errorMessage.value = ''
  errorStack.value = ''
  showDetails.value = false
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
  max-width: 600px;
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
  margin: 0 0 20px 0;
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-bottom: 16px;
}

.retry-btn {
  background: #18a058;
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.retry-btn:hover {
  background: #36ad6a;
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

.error-details {
  margin-top: 16px;
  text-align: left;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 8px;
  padding: 12px;
  font-size: 12px;
  max-height: 200px;
  overflow-y: auto;
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
