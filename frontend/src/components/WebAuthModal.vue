<template>
  <div>
    <!-- 1. 全屏沉浸式安全锁屏 (未授权或首次登录时绝对遮蔽工作站，杜绝无防范访问) -->
    <div v-if="isForced && showModal" class="web-auth-fullscreen-gate">
      <div class="auth-gate-card">
        <!-- 品牌头部 -->
        <div class="card-header">
          <div class="brand-badge">
            <span class="brand-icon">⚔️</span>
            <span class="brand-title">Blades Stock</span>
            <n-tag size="small" type="success" :bordered="false" round style="margin-left: 8px">
              Cloud-Native v1.8.0
            </n-tag>
          </div>
          <div class="card-subtitle">
            智能量化投研工作站 · 多 Agent 协同推演中台
          </div>
        </div>

        <!-- 场景 A：首次访问引导设置密码 -->
        <template v-if="needInit">
          <n-alert type="warning" :show-icon="true" class="auth-alert">
            <template #header>
              ✨ 首次使用初始化向导
            </template>
            检测到当前工作站尚未设定访问密码。为确保量化策略与资产数据安全，请立即设置管理员密码。
          </n-alert>

          <div class="form-body">
            <n-form-item label="设置初始访问密码" :feedback="errorMsg" :validation-status="errorMsg ? 'error' : undefined">
              <n-input
                v-model:value="initPwd"
                type="password"
                show-password-on="click"
                size="large"
                placeholder="请输入新密码 (不少于6位)"
                :disabled="loading"
                autofocus
                @keyup.enter="focusConfirmPwd"
              />
            </n-form-item>

            <n-form-item label="确认访问密码">
              <n-input
                ref="confirmInputRef"
                v-model:value="confirmPwd"
                type="password"
                show-password-on="click"
                size="large"
                placeholder="请再次输入以确认"
                :disabled="loading"
                @keyup.enter="handleInitPassword"
              />
            </n-form-item>

            <n-button
              type="primary"
              size="large"
              block
              :loading="loading"
              :disabled="!initPwd.trim() || !confirmPwd.trim()"
              class="auth-action-btn"
              @click="handleInitPassword"
            >
              🚀 完成初始化并解锁工作站
            </n-button>
          </div>
        </template>

        <!-- 场景 B：已有密码，强制口令解锁 -->
        <template v-else>
          <n-alert type="info" :show-icon="true" class="auth-alert">
            <template #header>
              🔐 云端安全防护已就绪
            </template>
            当前服务运行于云端安全授权模式。请输入访问口令以解锁量化投研驾驶舱。
          </n-alert>

          <div class="form-body">
            <n-form-item label="访问密码 / 授权口令" :feedback="errorMsg" :validation-status="errorMsg ? 'error' : undefined">
              <n-input
                v-model:value="tokenInput"
                type="password"
                show-password-on="click"
                size="large"
                placeholder="请输入访问口令"
                :disabled="loading"
                autofocus
                @keyup.enter="handleLogin"
              />
            </n-form-item>

            <n-button
              type="primary"
              size="large"
              block
              :loading="loading"
              :disabled="!tokenInput.trim()"
              class="auth-action-btn"
              @click="handleLogin"
            >
              🔓 验证密码并进入工作站
            </n-button>
          </div>
        </template>

        <!-- 底部网络与连通性指示 -->
        <div class="card-footer">
          <div class="health-indicator">
            <span class="indicator-dot" :class="healthStatus"></span>
            <span v-if="healthStatus === 'checking'">正在探测云端服务连通性...</span>
            <span v-else-if="healthStatus === 'healthy'">云端投研中台就绪 ({{ pingLatency }}ms)</span>
            <span v-else class="text-error">服务暂未连通，请检查服务器网络</span>
          </div>
          <n-button text size="tiny" type="primary" @click="checkServerHealth">
            刷新连通
          </n-button>
        </div>
      </div>
    </div>

    <!-- 2. 设置界面主动调起的凭据管理弹窗 (非强制模式) -->
    <n-modal
      v-else-if="!isForced && showModal"
      v-model:show="showModal"
      preset="card"
      title="🛡️ 访问凭证与中台安全管理"
      class="web-auth-manage-modal"
      :style="{ maxWidth: '460px', width: '92%' }"
      @close="handleClose"
    >
      <n-space vertical size="large">
        <n-alert type="info" :show-icon="true">
          您当前已成功建立安全连接。在此可测试服务端连通延迟、更新访问口令或注销当前浏览器凭证。
        </n-alert>

        <!-- 服务端连通状态与延迟 -->
        <div class="server-status-bar">
          <span class="status-label">云端中台连通状态：</span>
          <n-tag v-if="healthStatus === 'checking'" size="small" :bordered="false" type="info">
            🔄 探测连通性中...
          </n-tag>
          <n-tag v-else-if="healthStatus === 'healthy'" size="small" :bordered="false" type="success">
            🟢 连通正常 ({{ pingLatency }}ms)
          </n-tag>
          <n-tag v-else size="small" :bordered="false" type="error">
            🔴 连接异常 (请检查服务器运行状态)
          </n-tag>
          <n-button text size="tiny" type="primary" style="margin-left: 6px" @click="checkServerHealth">
            刷新
          </n-button>
        </div>

        <n-form-item
          label="更换新访问口令"
          :feedback="errorMsg"
          :validation-status="errorMsg ? 'error' : undefined"
        >
          <n-input
            v-model:value="tokenInput"
            type="password"
            show-password-on="click"
            placeholder="输入新口令以更换授权凭据 (留空则保持当前口令)"
            @keyup.enter="handleLogin"
            :disabled="loading"
          />
        </n-form-item>

        <n-space justify="space-between" align="center">
          <n-button
            v-if="currentSavedToken"
            quaternary
            type="error"
            size="small"
            :disabled="loading"
            @click="handleLogout"
          >
            退出登录 / 清除凭据
          </n-button>

          <n-space justify="end">
            <n-button quaternary size="small" :disabled="loading" @click="handleClose">
              取消
            </n-button>
            <n-button
              type="primary"
              :loading="loading"
              :disabled="!tokenInput.trim()"
              @click="handleLogin"
            >
              更新口令并重连
            </n-button>
          </n-space>
        </n-space>
      </n-space>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import {
  NModal,
  NSpace,
  NAlert,
  NFormItem,
  NInput,
  NButton,
  NTag,
  useMessage,
  useDialog
} from 'naive-ui'
import { isWails, getAuthToken, setAuthToken, clearAuthToken } from '../api/bridge'

const message = useMessage()
const dialog = useDialog()

const showModal = ref(false)
const isForced = ref(false)
const needInit = ref(false)

const tokenInput = ref('')
const initPwd = ref('')
const confirmPwd = ref('')
const confirmInputRef = ref<any>(null)

const loading = ref(false)
const errorMsg = ref('')

const healthStatus = ref<'checking' | 'healthy' | 'error'>('checking')
const pingLatency = ref(0)
const currentSavedToken = ref('')

// Removed maskedToken display - security best practice: never show token fragments in UI

const checkServerHealth = async () => {
  healthStatus.value = 'checking'
  const start = Date.now()
  try {
    const res = await fetch('/health', { method: 'GET', cache: 'no-store' })
    if (res.ok) {
      pingLatency.value = Math.max(1, Date.now() - start)
      healthStatus.value = 'healthy'
    } else {
      healthStatus.value = 'error'
    }
  } catch {
    healthStatus.value = 'error'
  }
}

// 探测服务端是否需要初始化密码
const fetchAuthStatus = async (): Promise<boolean> => {
  try {
    const res = await fetch('/api/auth/status', { method: 'GET', cache: 'no-store' })
    if (res.ok) {
      const data = await res.json()
      if (data && data.code === 0 && data.data) {
        needInit.value = Boolean(data.data.needInit)
        return needInit.value
      }
    }
  } catch {
    // 降级使用现有状态
  }
  return false
}

const focusConfirmPwd = () => {
  nextTick(() => {
    confirmInputRef.value?.focus()
  })
}

const onUnauthorized = async () => {
  if (isWails()) return
  clearAuthToken()
  currentSavedToken.value = ''
  const wasForced = isForced.value && showModal.value
  isForced.value = true
  showModal.value = true
  errorMsg.value = '访问凭据未提供或已失效，请输入有效密码'
  if (!wasForced) {
    await fetchAuthStatus()
    checkServerHealth()
  }
}

const onOpenAuthModal = async (e: any) => {
  if (isWails()) return
  const mode = e?.detail?.mode || 'manage'
  currentSavedToken.value = getAuthToken()
  isForced.value = mode === 'login'
  errorMsg.value = ''
  tokenInput.value = currentSavedToken.value || ''
  initPwd.value = ''
  confirmPwd.value = ''
  showModal.value = true
  await fetchAuthStatus()
  checkServerHealth()
}

onMounted(async () => {
  // 原生桌面 Wails 环境直接放行
  if (isWails()) return

  window.addEventListener('blades:unauthorized', onUnauthorized)
  window.addEventListener('blades:open-auth-modal', onOpenAuthModal)

  currentSavedToken.value = getAuthToken()
  checkServerHealth()

  // 1. 若当前本地无 Token，则立即强制锁定并展示锁屏认证墙
  if (!currentSavedToken.value) {
    isForced.value = true
    showModal.value = true
    tokenInput.value = ''
    await fetchAuthStatus()
    return
  }

  // 2. 若已有 Token，向后端发送试探请求校验有效性
  try {
    const res = await fetch('/api/App/GetConfig', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${currentSavedToken.value}`
      },
      body: '[]'
    })
    if (res.status === 401) {
      // Token 无效或已变更
      clearAuthToken()
      currentSavedToken.value = ''
      isForced.value = true
      showModal.value = true
      tokenInput.value = ''
      await fetchAuthStatus()
    }
  } catch {
    // 网络波动时保留现有状态
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('blades:unauthorized', onUnauthorized)
  window.removeEventListener('blades:open-auth-modal', onOpenAuthModal)
})

const handleClose = () => {
  if (isForced.value) return
  showModal.value = false
  tokenInput.value = ''
  initPwd.value = ''
  confirmPwd.value = ''
  errorMsg.value = ''
}

const handleLogout = () => {
  dialog.warning({
    title: '确认退出登录？',
    content: '退出后将清除保存在当前浏览器的授权凭据，下次访问需重新输入访问口令。',
    positiveText: '确认退出',
    negativeText: '取消',
    onPositiveAction: () => {
      clearAuthToken()
      currentSavedToken.value = ''
      message.info('已清除授权凭证')
      showModal.value = false
      setTimeout(() => {
        window.location.reload()
      }, 300)
    }
  })
}

// 首次设置密码处理
const handleInitPassword = async () => {
  const pwd = initPwd.value.trim()
  const confirm = confirmPwd.value.trim()

  if (!pwd) {
    errorMsg.value = '请输入初始访问密码'
    return
  }
  if (pwd.length < 4) {
    errorMsg.value = '密码长度不能少于4位'
    return
  }
  if (pwd !== confirm) {
    errorMsg.value = '两次输入的密码不一致，请核对'
    return
  }

  loading.value = true
  errorMsg.value = ''

  try {
    const res = await fetch('/api/auth/init', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pwd })
    })

    const data = await res.json().catch(() => ({}))

    if (res.ok && data.code === 0) {
      setAuthToken(pwd)
      currentSavedToken.value = pwd
      needInit.value = false
      message.success('🎉 管理员密码初始化成功，欢迎进入工作站！')
      showModal.value = false
      initPwd.value = ''
      confirmPwd.value = ''
      window.dispatchEvent(new CustomEvent('blades:authenticated'))
      setTimeout(() => {
        window.location.reload()
      }, 400)
    } else {
      errorMsg.value = data.error || '初始化密码失败，请核对后重试'
    }
  } catch {
    errorMsg.value = '网络请求失败，请检查服务器网络与连通性'
  } finally {
    loading.value = false
  }
}

// 正常密码登录验证
const handleLogin = async () => {
  const token = tokenInput.value.trim()
  if (!token) return

  loading.value = true
  errorMsg.value = ''

  try {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token })
    })

    const data = await res.json().catch(() => ({}))

    if (res.ok && data.code === 0) {
      setAuthToken(token)
      currentSavedToken.value = token
      message.success('验证成功，已连接投研中台！')
      showModal.value = false
      tokenInput.value = ''
      window.dispatchEvent(new CustomEvent('blades:authenticated'))
      setTimeout(() => {
        window.location.reload()
      }, 400)
    } else {
      errorMsg.value = data.error || '访问口令错误，请核对后重试'
    }
  } catch {
    errorMsg.value = '网络请求失败，请检查服务器网络与连通性'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* 全屏高科技沉浸式锁屏遮罩 */
.web-auth-fullscreen-gate {
  position: fixed;
  inset: 0;
  z-index: 999999;
  background: radial-gradient(circle at 50% 30%, #161f30 0%, #0a0e17 100%);
  backdrop-filter: blur(28px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.auth-gate-card {
  width: 100%;
  max-width: 480px;
  background: rgba(18, 26, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 16px;
  box-shadow: 0 24px 64px -12px rgba(0, 0, 0, 0.75), 0 0 0 1px rgba(255, 255, 255, 0.05);
  padding: 32px 36px;
  color: #fff;
}

.card-header {
  text-align: center;
  margin-bottom: 24px;
}

.brand-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

.brand-icon {
  margin-right: 8px;
  font-size: 26px;
}

.card-subtitle {
  margin-top: 8px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.55);
  letter-spacing: 0.3px;
}

.auth-alert {
  margin-bottom: 20px;
  border-radius: 8px;
}

.form-body {
  margin-bottom: 16px;
}

.auth-action-btn {
  margin-top: 12px;
  font-weight: 600;
  letter-spacing: 0.5px;
  height: 44px;
  border-radius: 8px;
}

.card-footer {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.45);
}

.health-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.indicator-dot.checking {
  background: #f0a020;
}

.indicator-dot.healthy {
  background: #18a058;
  box-shadow: 0 0 8px rgba(24, 160, 88, 0.6);
}

.indicator-dot.error {
  background: #d03050;
}

.text-error {
  color: #d03050;
}

/* 管理模式弹窗 */
.web-auth-manage-modal {
  border-radius: 12px;
  backdrop-filter: blur(16px);
}

.server-status-bar {
  display: flex;
  align-items: center;
  font-size: 13px;
  color: var(--n-text-color-3);
  padding: 4px 0;
}

.status-label {
  margin-right: 6px;
}

/* ==========================================================================
   移动端响应式适配 (<= 480px 手机屏幕)
   ========================================================================== */
@media screen and (max-width: 480px) {
  .web-auth-fullscreen-gate {
    padding: 0;
    align-items: stretch;
    justify-content: flex-end;
  }

  .auth-gate-card {
    max-width: 100vw;
    width: 100vw;
    border-radius: 0;
    border-left: none;
    border-right: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    padding: 24px 20px;
    min-height: 100dvh;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    justify-content: center;
    box-shadow: none;
    background: linear-gradient(180deg, #0a0e17 0%, #161f30 100%);
  }

  .card-header {
    margin-bottom: 20px;
  }

  .brand-badge {
    font-size: 22px;
  }

  .brand-icon {
    font-size: 24px;
  }

  .card-subtitle {
    font-size: 12px;
  }

  .auth-alert {
    font-size: 13px;
    padding: 12px 14px;
  }

  .form-body {
    margin-bottom: 12px;
  }

  .form-body :deep(.n-form-item-label) {
    font-size: 13px;
  }

  .form-body :deep(.n-input) {
    font-size: 16px !important; /* 防止 iOS Safari 聚焦放大 */
  }

  .auth-action-btn {
    height: 52px;
    font-size: 16px;
    border-radius: 10px;
    margin-top: 16px;
  }

  .card-footer {
    margin-top: auto;
    padding-top: 20px;
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
    text-align: center;
  }

  .health-indicator {
    justify-content: center;
    font-size: 11px;
  }

  /* 管理模式弹窗移动端适配 */
  .web-auth-manage-modal {
    max-width: calc(100vw - 16px) !important;
    margin: 8px auto !important;
    border-radius: 16px 16px 0 0 !important;
  }

  .server-status-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .status-label {
    margin-right: 0;
    text-align: center;
  }
}

/* 平板横屏及小尺寸桌面 (<= 768px) */
@media screen and (min-width: 481px) and (max-width: 768px) {
  .web-auth-fullscreen-gate {
    padding: 24px;
  }

  .auth-gate-card {
    max-width: 420px;
    padding: 28px 24px;
  }

  .auth-action-btn {
    height: 48px;
  }
}

/* 安全区域适配 (iPhone X+ 刘海屏) */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  @media screen and (max-width: 480px) {
    .auth-gate-card {
      padding-bottom: max(24px, env(safe-area-inset-bottom));
    }
  }
}
</style>
