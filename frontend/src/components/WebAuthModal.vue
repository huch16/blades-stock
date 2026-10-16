<template>
  <n-modal
    v-model:show="showModal"
    :mask-closable="!isForced"
    :close-on-esc="!isForced"
    :closable="!isForced"
    preset="card"
    :title="isForced ? '🔐 投研工作站授权访问' : '🛡️ 访问凭证与中台安全管理'"
    class="web-auth-modal"
    :style="{ maxWidth: '460px', width: '92%' }"
    @close="handleClose"
  >
    <n-space vertical size="large">
      <n-alert :type="isForced ? 'warning' : 'info'" :show-icon="true">
        <template v-if="isForced">
          当前服务运行于云端安全防护模式，请输入专属访问口令（BLADES_AUTH_TOKEN）以解锁量化模型与投研驾驶舱。
        </template>
        <template v-else>
          您当前已成功建立安全连接。在此可测试服务端连通延迟、更新访问口令或注销当前浏览器凭证。
        </template>
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

      <!-- 当前口令状态提示 (仅在管理模式且已有 Token 时展示) -->
      <div v-if="!isForced && currentSavedToken" class="current-token-info">
        <span class="info-label">当前生效口令：</span>
        <span class="info-masked">{{ maskedToken }}</span>
      </div>

      <n-form-item
        :label="isForced ? '访问口令 (Access Token)' : '新访问口令 (如需修改请输入)'"
        :feedback="errorMsg"
        :validation-status="errorMsg ? 'error' : undefined"
      >
        <n-input
          v-model:value="tokenInput"
          type="password"
          show-password-on="click"
          :placeholder="isForced ? '请输入环境变量 BLADES_AUTH_TOKEN 设定的口令' : '输入新口令以更换授权凭据'"
          @keyup.enter="handleLogin"
          :disabled="loading"
          autofocus
        />
      </n-form-item>

      <n-space justify="space-between" align="center">
        <div>
          <n-button
            v-if="!isForced && currentSavedToken"
            quaternary
            type="error"
            size="small"
            :disabled="loading"
            @click="handleLogout"
          >
            退出登录 / 清除凭证
          </n-button>
        </div>

        <n-space justify="end">
          <n-button
            v-if="!isForced"
            quaternary
            size="small"
            :disabled="loading"
            @click="handleClose"
          >
            取消
          </n-button>
          <n-button
            type="primary"
            :loading="loading"
            :disabled="!tokenInput.trim()"
            @click="handleLogin"
          >
            {{ isForced ? '验证并进入工作站' : '更新口令并重连' }}
          </n-button>
        </n-space>
      </n-space>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
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
const isForced = ref(true)
const tokenInput = ref('')
const loading = ref(false)
const errorMsg = ref('')

const healthStatus = ref<'checking' | 'healthy' | 'error'>('checking')
const pingLatency = ref(0)
const currentSavedToken = ref('')

const maskedToken = computed(() => {
  if (!currentSavedToken.value) return '未设置'
  const len = currentSavedToken.value.length
  if (len <= 4) return '••••'
  return currentSavedToken.value.slice(0, 2) + '••••••••' + currentSavedToken.value.slice(-2)
})

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

const onUnauthorized = () => {
  if (isWails()) return
  currentSavedToken.value = ''
  isForced.value = true
  showModal.value = true
  errorMsg.value = '访问口令未提供或已失效，请输入有效口令'
  checkServerHealth()
}

const onOpenAuthModal = (e: any) => {
  if (isWails()) return
  const mode = e?.detail?.mode || 'manage'
  currentSavedToken.value = getAuthToken()
  isForced.value = mode === 'login'
  errorMsg.value = ''
  tokenInput.value = ''
  showModal.value = true
  checkServerHealth()
}

onMounted(() => {
  if (isWails()) return

  window.addEventListener('blades:unauthorized', onUnauthorized)
  window.addEventListener('blades:open-auth-modal', onOpenAuthModal)

  currentSavedToken.value = getAuthToken()

  // 首次打开检查是否已有 Token
  if (!currentSavedToken.value) {
    checkServerHealth()
    fetch('/api/App/GetConfig', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '[]'
    })
      .then((res) => {
        if (res.status === 401) {
          isForced.value = true
          showModal.value = true
        }
      })
      .catch(() => {})
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
      // 刷新页面重新建立 SSE 连接与加载初始数据
      setTimeout(() => {
        window.location.reload()
      }, 400)
    } else {
      errorMsg.value = data.error || '访问口令错误，请核对后重试'
    }
  } catch (err: any) {
    errorMsg.value = '网络请求失败，请检查服务器网络与连通性'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.web-auth-modal {
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
.current-token-info {
  display: flex;
  align-items: center;
  font-size: 13px;
  background: rgba(128, 128, 128, 0.08);
  padding: 6px 12px;
  border-radius: 6px;
}
.info-label {
  color: var(--n-text-color-3);
  margin-right: 6px;
}
.info-masked {
  font-family: monospace;
  font-weight: 600;
  color: #18a058;
}
</style>
