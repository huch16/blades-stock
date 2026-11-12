<script setup lang="ts">
import { RefreshCircleSharp } from "@vicons/ionicons5";
import { onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  headerTitle: {
    type: String,
    default: '市场资讯'
  },
  newsList: {
    type: Array,
    default: () => []
  },
})

const emits = defineEmits(['update:message'])

const refreshing = ref(false)

const updateMessage = () => {
  refreshing.value = true
  emits('update:message', props.headerTitle)
  setTimeout(() => {
    refreshing.value = false
  }, 1000)
}

// 使用 ref 创建响应式时间数据
const time = ref(new Date())

// 更新时间的函数
const updateTime = () => {
  time.value = new Date()
}

let timer = null

// 组件挂载时启动定时器
onMounted(() => {
  if (props.headerTitle === '财联社电报') {
    timer = setInterval(updateTime, 1000)
  }
})

// 组件卸载时清除定时器
onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
  }
})

function getDateStr(dt) {
  if (!dt) return ''
  if (typeof dt === 'string') {
    return dt.length >= 10 ? dt.substring(0, 10) : dt
  }
  return ''
}

function shouldShowDateDivider(idx) {
  if (idx === 0) return false
  const list = props.newsList || []
  const prevDate = getDateStr(list[idx - 1]?.dataTime)
  const currDate = getDateStr(list[idx]?.dataTime)
  return Boolean(currDate && prevDate && currDate !== prevDate)
}
</script>

<template>
  <n-list bordered class="news-list-box">
    <template #header>
      <n-flex justify="space-between" align="center">
        <n-tag
          :bordered="false"
          size="large"
          type="success"
        >
          {{ headerTitle }}
        </n-tag>
        <n-tag
          v-if="headerTitle === '财联社电报'"
          :bordered="false"
          size="large"
          type="info"
        >
          <n-time :time="time" />
        </n-tag>
        <n-button
          :bordered="false"
          :loading="refreshing"
          @click="updateMessage"
        >
          <n-icon
            color="#409EFF"
            size="22"
            :component="RefreshCircleSharp"
          />
        </n-button>
      </n-flex>
    </template>

    <n-empty
      v-if="!newsList || newsList.length === 0"
      description="暂无快讯数据，正在为您采集最新动态..."
      style="padding: 40px 0;"
    />

    <n-list-item
      v-for="(item, idx) in newsList"
      :key="item.ID || idx"
    >
      <n-space
        v-if="shouldShowDateDivider(idx)"
        justify="center"
      >
        <n-divider>
          {{ getDateStr(item.dataTime) }}
        </n-divider>
      </n-space>
      <n-space justify="start">
        <n-collapse
          v-if="item.title"
          arrow-placement="right"
        >
          <n-collapse-item :name="item.title">
            <template #header>
              <n-tag
                size="small"
                :type="item.isRed ? 'error' : 'warning'"
                :bordered="false"
              >
                {{ item.time || (item.dataTime ? item.dataTime.slice(11, 19) : '') }}
              </n-tag>
              <n-text
                size="small"
                :type="item.isRed ? 'error' : 'info'"
                :bordered="false"
              >
                {{ item.title }}
              </n-text>
            </template>
            <n-text
              justify="start"
              :bordered="false"
              :type="item.isRed ? 'error' : 'info'"
            >
              {{ item.content }}
            </n-text>
          </n-collapse-item>
        </n-collapse>
        <n-text
          v-if="!item.title"
          justify="start"
          :bordered="false"
          :type="item.isRed ? 'error' : 'info'"
        >
          <n-tag
            size="small"
            :type="item.isRed ? 'error' : 'warning'"
            :bordered="false"
          >
            {{ item.time || (item.dataTime ? item.dataTime.slice(11, 19) : '') }}
          </n-tag>
          {{ item.content }}
        </n-text>
      </n-space>
      <n-space
        v-if="item.subjects && item.subjects.length > 0"
        style="margin-top: 2px"
      >
        <n-tag
          v-for="sub in item.subjects"
          :key="sub"
          :bordered="false"
          type="success"
          size="small"
        >
          {{ sub }}
        </n-tag>
        <n-space v-if="item.stocks && item.stocks.length > 0">
          <n-tag
            v-for="sub in item.stocks"
            :key="sub"
            :bordered="false"
            type="warning"
            size="small"
          >
            {{ sub }}
          </n-tag>
        </n-space>
        <n-tag
          v-if="item.url"
          :bordered="false"
          type="warning"
          size="small"
        >
          <a
            :href="item.url"
            target="_blank"
          >
            <n-text type="warning">查看原文</n-text>
          </a>
        </n-tag>
        <n-tag
          v-if="item.sentimentResult"
          :bordered="false"
          :type="item.sentimentResult === '看涨' ? 'error' : item.sentimentResult === '看跌' ? 'success' : 'info'"
          size="small"
        >
          {{ item.sentimentResult }}
        </n-tag>
      </n-space>
    </n-list-item>
  </n-list>
</template>

<style scoped>
.news-list-box {
  min-height: 300px;
}
</style>
