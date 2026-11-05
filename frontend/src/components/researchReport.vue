<script setup>
import { computed, h, onBeforeMount, onMounted, ref, reactive } from 'vue'
import { GetAIResponseResultList, GetConfig, SaveAsMarkdown, DeleteAIResponseResult } from "../../wailsjs/go/main/App"
import { NButton, NEllipsis, NText, useMessage } from "naive-ui"
import { MdPreview } from 'md-editor-v3'
import { sanitizeMarkdownHtml } from '../utils/sanitize'

const message = useMessage()
const mdPreviewRef = ref(null)

const editorDataRef = reactive({
  show: false,
  loading: false,
  darkTheme: false,
  chatId: "",
  modelName: "",
  CreatedAt: "",
  stockName: "",
  stockCode: "",
  question: "",
  content: "",
})

const dataRef = ref([])
const loadingRef = ref(true)

function formatDate(d) {
  if (!d) return ''
  const date = new Date(d)
  if (isNaN(date.getTime())) return ''
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  range: [
    new Date(Date.now() - 30 * 24 * 60 * 60 * 1000), // 默认最近30天
    new Date()
  ],
  prefix({ itemCount }) {
    return `${itemCount || 0} 条记录`
  }
})

function getDateRange() {
  if (Array.isArray(paginationReactive.range) && paginationReactive.range.length === 2 && paginationReactive.range[0] && paginationReactive.range[1]) {
    return {
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1])
    }
  }
  return { startDate: '', endDate: '' }
}

const columnsRef = ref([
  {
    title: '分析时间',
    key: 'CreatedAt',
    render(row) {
      if (!row || !row.CreatedAt) return '--'
      return String(row.CreatedAt).substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '模型名称',
    key: 'modelName',
    render(row) {
      return row?.modelName || '--'
    }
  },
  {
    title: '分析对象',
    key: 'stockName',
    render(row) {
      if (!row) return '--'
      if (row.stockName && row.stockCode) {
        return `${row.stockName} (${row.stockCode})`
      }
      return row.stockName || row.stockCode || '--'
    }
  },
  {
    title: '提示词',
    key: 'question',
    render(row) {
      const q = row?.question || '--'
      return h(
        NEllipsis,
        { tooltip: true, style: "max-width: 240px;" },
        { default: () => h(NText, { type: "info" }, { default: () => q }) }
      )
    }
  },
  {
    title: '操作',
    key: 'actions',
    render(row) {
      if (!row) return null
      return [
        h(
          NButton,
          {
            strong: true,
            tertiary: true,
            size: 'small',
            type: 'warning',
            style: 'font-size: 14px; padding: 0 10px; margin-right: 8px;',
            onClick: () => showReport(row)
          },
          { default: () => '查看分析报告' }
        ),
        h(
          NButton,
          {
            strong: true,
            tertiary: true,
            size: 'small',
            type: 'error',
            style: 'font-size: 14px; padding: 0 10px;',
            onClick: () => deleteAIResponseResult(row.ID)
          },
          { default: () => '删除' }
        )
      ]
    }
  }
])

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result && result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  }).catch(() => {})
})

onMounted(() => {
  const { startDate, endDate } = getDateRange()
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword || "",
    startDate,
    endDate
  }).then((data) => {
    dataRef.value = data?.data || []
    paginationReactive.page = 1
    paginationReactive.pageCount = data?.pageCount || 1
    paginationReactive.itemCount = data?.total || 0
  }).finally(() => {
    loadingRef.value = false
  })
})

const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})

function showReport(row) {
  if (!row) return
  editorDataRef.show = true
  editorDataRef.chatId = row.chatId || ""
  editorDataRef.modelName = row.modelName || ""
  editorDataRef.CreatedAt = row.CreatedAt ? String(row.CreatedAt).substring(0, 19).replace('T', ' ') : ""
  editorDataRef.stockName = row.stockName || "未知标的"
  editorDataRef.stockCode = row.stockCode || ""
  editorDataRef.question = row.question || ""
  editorDataRef.content = row.content || ""
  editorDataRef.loading = false
}

function query({
  page = 1,
  pageSize = 12,
  order = 'desc',
  keyword = "",
  startDate = "",
  endDate = ""
}) {
  return new Promise((resolve) => {
    GetAIResponseResultList({
      page: page,
      pageSize: pageSize,
      // 修复：order 参数此前被声明却从未使用，请求体中硬编码为 "desc"，
      // 导致调用方传入的排序方向被静默忽略。现按签名契约如实透传（当前所有调用方
      // 均传 "desc"，因此对外行为不变，仅消除潜在陷阱）。
      order: order,
      modelName: keyword,
      question: keyword,
      stockName: keyword,
      stockCode: keyword,
      startDate: startDate,
      endDate: endDate
    }).then((res) => {
      const pagedData = res?.list || []
      const total = typeof res?.total === 'number' ? res.total : (pagedData.length || 0)
      const pageCount = res?.totalPages || Math.ceil(total / pageSize) || 1
      resolve({
        pageCount,
        data: pagedData,
        total,
        page: res?.page || page
      })
    }).catch((err) => {
      console.error('[ResearchReport] GetAIResponseResultList failed:', err)
      message.error('获取分析报告列表失败: ' + (err?.message || err))
      resolve({
        pageCount: 1,
        data: [],
        total: 0,
        page: 1
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    const { startDate, endDate } = getDateRange()
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword || "",
      startDate,
      endDate
    }).then((data) => {
      dataRef.value = data?.data || []
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data?.pageCount || 1
      paginationReactive.itemCount = data?.total || 0
    }).finally(() => {
      loadingRef.value = false
    })
  }
}

function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    const { startDate, endDate } = getDateRange()
    query({
      page: 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword || "",
      startDate,
      endDate
    }).then((data) => {
      dataRef.value = data?.data || []
      paginationReactive.page = 1
      paginationReactive.pageCount = data?.pageCount || 1
      paginationReactive.itemCount = data?.total || 0
    }).finally(() => {
      loadingRef.value = false
    })
  }
}

function saveAsMarkdown(code, name) {
  SaveAsMarkdown(code, name).then(result => {
    if (!result || result === '取消保存') {
      message.info(result || '已取消保存')
      return
    }
    if (result.startsWith('保存失败') || result.startsWith('分析结果异常')) {
      message.error(result)
      return
    }
    if (result.startsWith('已保存至：')) {
      message.success(result)
      return
    }
    // Web 模式下后端返回 Markdown 正文，触发浏览器下载
    const blob = new Blob([result], { type: 'text/markdown;charset=utf-8' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `${name || '个股'}[${code || ''}]-AI研报.md`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(link.href)
    message.success('已导出为 Markdown 文件并开始下载')
  }).catch(err => {
    message.error('导出 Markdown 失败: ' + (err?.message || err))
  })
}

async function copyToClipboard() {
  try {
    if (!editorDataRef.content) {
      message.warning('暂无内容可复制')
      return
    }
    await navigator.clipboard.writeText(editorDataRef.content)
    message.success('分析结果已复制到剪切板')
  } catch (err) {
    message.error('复制失败: ' + (err?.message || err))
  }
}

function deleteAIResponseResult(id) {
  if (!id) return
  DeleteAIResponseResult(id).then(result => {
    if (result) {
      message.success(result)
    }
    handleSearch()
  }).catch(err => {
    message.error('删除失败: ' + (err?.message || err))
  })
}
</script>

<template>
  <n-input-group>
    <n-date-picker
      v-model:value="paginationReactive.range"
      type="daterange"
      clearable
      style="width: 50%"
      @update:value="handleSearch"
    />
    <n-input
      v-model:value="paginationReactive.keyword"
      clearable
      placeholder="输入关键词、股票代码/名称或模型搜索"
      @keyup.enter="handleSearch"
    />
    <n-button
      type="primary"
      ghost
      :loading="loadingRef"
      @click="handleSearch"
    >
      搜索
    </n-button>
  </n-input-group>
  <n-data-table
    remote
    size="small"
    :columns="columnsRef"
    :data="dataRef"
    :loading="loadingRef"
    :pagination="paginationReactive"
    :row-key="(rowData) => rowData?.ID || Math.random()"
    flex-height
    style="height: calc(100vh - 210px);margin-top: 10px"
    @update:page="handlePageChange"
  />

  <n-modal
    v-model:show="editorDataRef.show"
    transform-origin="center"
    preset="card"
    style="width: 800px;max-width: calc(100vw - 32px);"
    :title="'[' + (editorDataRef.stockName || '分析') + '] AI分析报告'"
  >
    <n-spin
      size="small"
      :show="editorDataRef.loading"
    >
      <MdPreview
        ref="mdPreviewRef"
        style="height: 540px;max-height: 60vh;text-align: left;overflow-y: auto;"
        :model-value="editorDataRef.content"
        :theme="theme"
        :sanitize="sanitizeMarkdownHtml"
      />
    </n-spin>
    <template #footer>
      <n-flex
        ref="tipsRef"
        justify="space-between"
      >
        <n-text
          v-if="editorDataRef.chatId"
          type="info"
        >
          <n-tag
            v-if="editorDataRef.modelName"
            type="warning"
            round
            :title="editorDataRef.chatId"
            :bordered="false"
          >
            {{ editorDataRef.modelName }}
          </n-tag>
          {{ editorDataRef.CreatedAt }}
        </n-text>
        <n-text type="error">
          *AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。
        </n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="right">
        <n-button
          size="tiny"
          type="success"
          @click="copyToClipboard"
        >
          复制到剪切板
        </n-button>
        <n-button
          size="tiny"
          type="primary"
          @click="saveAsMarkdown(editorDataRef.stockCode, editorDataRef.stockName)"
        >
          保存为Markdown文件
        </n-button>
      </n-flex>
    </template>
  </n-modal>
</template>

<style scoped>
</style>
