<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {GetAIResponseResultList, GetConfig, SaveAsMarkdown, DeleteAIResponseResult} from "../../wailsjs/go/main/App";
import {NAvatar, NButton, NEllipsis, NText, useMessage} from "naive-ui";
import {MdEditor, MdPreview} from 'md-editor-v3';



onBeforeMount(()=> {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })
})
onMounted(() => {
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword,
    startDate: paginationReactive.range[0],
    endDate: paginationReactive.range[1]
  }).then((data) => {
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.pageCount
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
})
const message = useMessage()
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
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
const columnsRef = ref([
  {
    title: '分析时间',
    key: 'CreatedAt',
    render(row, index) {
      //2026-01-14T22:13:27.2693252+08:00 格式化为常用时间格式
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '模型名称',
    key: 'modelName'
  },
  {
    title: '分析对象',
    key: 'stockName'
  },
  {
    title: '提示词',
    key: 'question',
    render(row, index) {
      return h(NEllipsis, { tooltip: true ,style: "max-width: 240px;"}, {default: () => h(NText,{type: "info"},{default: () => row.question}),})
    }
  },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NButton,
          {
            strong: true,
            tertiary: true,
            size: 'small',
            type: 'warning', // 橙色按钮
            style: 'font-size: 14px; padding: 0 10px;', // 稍微大一点的按钮
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
            type: 'error', // 橙色按钮
            style: 'font-size: 14px; padding: 0 10px;', // 稍微大一点的按钮
            onClick: () => deleteAIResponseResult(row.ID)
          },
          { default: () => '删除' }
      ),
      ]
    }
  },
])
const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  startDate:"",
  range: [
    new Date(new Date().getTime() - 3 * 24 * 60 * 60 * 1000), // 前3天
    new Date() // 当天
  ],
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})
const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})
function showReport(row) {

  editorDataRef.show = true
  editorDataRef.chatId = row.chatId
  editorDataRef.modelName = row.modelName
  editorDataRef.CreatedAt = row.CreatedAt.substring(0, 19).replace('T', ' ')
  editorDataRef.stockName = row.stockName
  editorDataRef.stockCode = row.stockCode
  editorDataRef.question = row.question
  editorDataRef.content = row.content
  editorDataRef.loading = false
}

function query({
                 page,
                 pageSize = 10,
                 order = 'desc',
                 keyword = "",
                 startDate = "",
                 endDate = ""
               }) {
  return new Promise((resolve) => {

    GetAIResponseResultList({
      "page": page,
      "pageSize": pageSize,
      "modelName":keyword,
      "question":keyword,
      "stockName":keyword,
      "stockCode":keyword,
      "startDate":startDate,
      "endDate":endDate
    }).then((res) => {
      const pagedData =res.list
      const total = res.total
      const pageCount =res.totalPages
      resolve({
        pageCount,
        data: pagedData,
        total
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1])
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: paginationReactive?.page ?? 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1])
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = data.page
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
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
    await navigator.clipboard.writeText(editorDataRef.content);
    message.success('分析结果已复制到剪切板');
  } catch (err) {
    message.error('复制失败: ' + err);
  }
}
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  // const hours = String(date.getHours()).padStart(2, '0')
  // const minutes = String(date.getMinutes()).padStart(2, '0')
  // const seconds = String(date.getSeconds()).padStart(2, '0')
  //return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  return `${year}-${month}-${day}`
}

function deleteAIResponseResult(id){
  DeleteAIResponseResult(id).then(result => {
    if(result !== ""){
      message.success(result)
    }
    handleSearch()
  })
}
</script>

<template>
  <n-input-group>
    <n-date-picker
      v-model:value="paginationReactive.range"
      type="daterange"
      style="width: 50%"
    />
    <n-input
      v-model:value="paginationReactive.keyword"
      clearable
      placeholder="输入关键词搜索"
    />
    <n-button
      type="primary"
      ghost
      @click="handleSearch"
      @input="handleSearch"
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
    :row-key="(rowData)=>rowData.ID"
    flex-height
    style="height: calc(100vh - 210px);margin-top: 10px"
    @update:page="handlePageChange"
  />



  <n-modal
    v-model:show="editorDataRef.show"
    transform-origin="center"
    preset="card"
    style="width: 800px;max-width: calc(100vw - 32px);"
    :title="'['+editorDataRef.stockName+']AI分析'"
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
          @click="saveAsMarkdown(editorDataRef.stockCode,editorDataRef.stockName)"
        >
          保存为Markdown文件
        </n-button>
      </n-flex>
    </template>
  </n-modal>
</template>

<style scoped>

</style>