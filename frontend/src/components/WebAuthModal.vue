<template>
  <n-modal
    v-model:show="showModal"
    :mask-closable="false"
    :close-on-esc="false"
    preset="card"
    title="🔐 投研工作站授权访问"
    class="web-auth-modal"
    :style="{ maxWidth: '440px', width: '90%' }"
  >
    <n-space vertical size="large">
      <n-alert type="info" :show-icon="true">
        当前服务运行于云端安全模式，请输入专属访问口令以解锁量化模型与投研驾驶舱。
      </n-alert>

      <n-form-item label="访问口令 (Access Token)" :feedback="errorMsg" :validation-status="errorMsg ? 'error' : undefined">
        <n-input
          v-model:value="tokenInput"
          type="password"
          show-password-on="click"
          placeholder="请输入环境变量 BLADES_AUTH_TOKEN 设定的口令"
          @keyup.enter="handleLogin"
          :disabled="loading"
          autofocus
        />
      </n-form-item>

      <n-space justify="end">
        <n-button
          type="primary"
          :loading="loading"
          :disabled="!tokenInput.trim()"
          @click="handleLogin"
        >
          验证并进入工作站
        </n-button>
      </n-space>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import {
  NModal,
  NSpace,
  NAlert,
  NFormItem,
  NInput,
  NButton,
  useMessage
} from 'naive-ui'
import { isWails, getAuthToken, setAuthToken } from '../api/bridge'

const message = useMessage()
const showModal = ref(false)
const tokenInput = ref('')
const loading = ref(false)
const errorMsg = ref('')

const onUnauthorized = () => {
  // 如果是桌面端，不触发 Web 口令弹窗
  if (isWails()) return
  showModal.value = true
  errorMsg.value = '访问口令未提供或已失效，请重新输入'
}

onMounted(() => {
  if (isWails()) return

  // 监听 Universal Bridge 发出的 401 事件
  window.addEventListener('blades:unauthorized', onUnauthorized)

  // 首次打开检查是否已有 Token
  const currentToken = getAuthToken()
  if (!currentToken) {
    // 探测服务端是否开启了鉴权
    fetch('/health')
      .then(() => {
        // 请求一个需要鉴权的简单接口探测
        return fetch('/api/App/GetConfig', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: '[]'
        })
      })
      .then((res) => {
        if (res.status === 401) {
          showModal.value = true
        }
      })
      .catch(() => {})
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('blades:unauthorized', onUnauthorized)
})

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

    const data = await res.json()

    if (res.ok && data.code === 0) {
      setAuthToken(token)
      message.success('验证成功，已连接投研中台！')
      showModal.value = false
      tokenInput.value = ''
      // 刷新页面重新建立 SSE 与数据加载
      setTimeout(() => {
        window.location.reload()
      }, 500)
    } else {
      errorMsg.value = data.error || '访问口令错误，请核对后重试'
    }
  } catch (err: any) {
    errorMsg.value = '网络请求失败，请检查服务连通性'
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
</style>
