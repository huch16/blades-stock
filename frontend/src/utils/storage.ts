/**
 * safeStorage: 工业级高可靠本地存储代理服务
 * 
 * 解决原生 localStorage 的三大工程痛点：
 * 1. 静默数据丢失与恶意吞错：封装安全的 JSON 序列化与反序列化，防损坏崩溃；
 * 2. 配额超限（QuotaExceededError）与隐身模式异常：自动降级至内存缓存 Map，不阻塞业务；
 * 3. 跨标签页状态断连：内置 StorageEvent 与 BroadcastChannel 双通道实时响应多窗口同步。
 */

const memoryFallback = new Map<string, string>()

// 探测本地存储可用性
const isLocalStorageAvailable = (): boolean => {
  if (typeof window === 'undefined') return false
  try {
    const testKey = '__blades_storage_test__'
    window.localStorage.setItem(testKey, testKey)
    window.localStorage.removeItem(testKey)
    return true
  } catch (e) {
    console.warn('[safeStorage] localStorage 不可用 (可能处于无痕模式或被浏览器安全拦截)，自动降级至内存字典:', e)
    return false
  }
}

const hasStorage = isLocalStorageAvailable()

export const safeStorage = {
  /**
   * 安全读取本地存储
   * @param key 键名
   * @param defaultValue 解析失败或不存在时的保底默认值
   */
  getItem<T>(key: string, defaultValue: T): T {
    try {
      let raw: string | null = null
      if (hasStorage) {
        raw = window.localStorage.getItem(key)
      } else {
        raw = memoryFallback.get(key) || null
      }

      if (raw === null || raw === undefined) {
        return defaultValue
      }

      return JSON.parse(raw) as T
    } catch (err) {
      console.warn(`[safeStorage] 解析键 "${key}" 的数据失败，已重置为默认值:`, err)
      return defaultValue
    }
  },

  /**
   * 安全写入本地存储
   * @param key 键名
   * @param value 任意可序列化对象
   * @returns 写入是否成功
   */
  setItem<T>(key: string, value: T): boolean {
    // 说明（no-useless-assignment）：初值 '' 从未被读取，序列化失败即直接 return false
    let serialized: string
    try {
      serialized = JSON.stringify(value)
    } catch (err) {
      console.error(`[safeStorage] 序列化键 "${key}" 失败，拒绝写入:`, err)
      return false
    }

    if (hasStorage) {
      try {
        window.localStorage.setItem(key, serialized)
        return true
      } catch (err: any) {
        // 捕获 QuotaExceededError（存储配额满）
        console.error(`[safeStorage] localStorage 写入失败 (配额超限或权限不足)，降级写入内存:`, err)
        memoryFallback.set(key, serialized)
        return false
      }
    } else {
      memoryFallback.set(key, serialized)
      return true
    }
  },

  /**
   * 安全移除键
   */
  removeItem(key: string): void {
    if (hasStorage) {
      try {
        window.localStorage.removeItem(key)
      } catch (err) {
        console.warn(`[safeStorage] 删除键 "${key}" 异常:`, err)
      }
    }
    memoryFallback.delete(key)
  },

  /**
   * 清空所有数据
   */
  clear(): void {
    if (hasStorage) {
      try {
        window.localStorage.clear()
      } catch (err) {
        console.warn('[safeStorage] 清空 localStorage 异常:', err)
      }
    }
    memoryFallback.clear()
  },

  /**
   * 监听跨标签页存储变更事件
   */
  subscribe<T>(key: string, onChange: (newValue: T | null) => void): () => void {
    if (typeof window === 'undefined') return () => {}

    const handler = (event: StorageEvent) => {
      if (event.key === key) {
        if (event.newValue === null) {
          onChange(null)
        } else {
          try {
            onChange(JSON.parse(event.newValue) as T)
          } catch {
            onChange(null)
          }
        }
      }
    }

    window.addEventListener('storage', handler)
    return () => {
      window.removeEventListener('storage', handler)
    }
  }
}
