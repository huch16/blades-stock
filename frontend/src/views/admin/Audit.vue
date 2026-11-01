<template>
  <div class="audit-admin-page">
    <n-card :bordered="false" class="main-card">
      <div class="page-header">
        <div class="header-title">
          <span class="icon">🛡️</span>
          <h2>系统安全审计与中台治理</h2>
        </div>
        <p class="header-desc">
          记录所有 Agent 工具调用、MCP 服务通信与关键数据变更，提供实时可追溯的审计底账与技能变更控制。
        </p>
      </div>

      <n-tabs v-model:value="activeTab" type="line" animated>
        <!-- Tab 1: 审计日志 -->
        <n-tab-pane name="audit" tab="📋 运行与工具审计日志">
          <div class="toolbar-area">
            <n-space wrap align="center" :size="[12, 12]">
              <n-input
                v-model:value="auditQuery.toolName"
                placeholder="搜索工具名 (如 GetStockKLine)"
                clearable
                style="width: 200px"
                @keyup.enter="fetchAuditData"
              />
              <n-input
                v-model:value="auditQuery.serverName"
                placeholder="服务/来源 (如 tushare/adata)"
                clearable
                style="width: 180px"
                @keyup.enter="fetchAuditData"
              />
              <n-input
                v-model:value="auditQuery.keyword"
                placeholder="请求/响应关键词检索"
                clearable
                style="width: 200px"
                @keyup.enter="fetchAuditData"
              />
              <n-select
                v-model:value="auditQuery.successFilter"
                :options="successOptions"
                placeholder="执行结果"
                clearable
                style="width: 120px"
                @update:value="fetchAuditData"
              />
              <n-button type="primary" :loading="auditLoading" @click="fetchAuditData">
                查询
              </n-button>
              <n-button @click="resetAuditQuery">
                重置
              </n-button>
            </n-space>
          </div>

          <n-data-table
            :columns="auditColumns"
            :data="auditData"
            :loading="auditLoading"
            :bordered="true"
            :single-line="false"
            size="small"
            class="audit-table"
            :scroll-x="1100"
          />

          <div class="pagination-footer">
            <n-pagination
              v-model:page="auditQuery.page"
              v-model:page-size="auditQuery.pageSize"
              :item-count="auditTotal"
              :page-sizes="[10, 20, 50, 100]"
              show-size-picker
              show-quick-jumper
              @update:page="fetchAuditData"
              @update:page-size="onAuditPageSizeChange"
            />
          </div>
        </n-tab-pane>

        <!-- Tab 2: 技能变更审批 -->
        <n-tab-pane name="skill" tab="⚖️ 技能发布与变更审批">
          <div class="toolbar-area">
            <n-space wrap align="center" :size="[12, 12]">
              <n-select
                v-model:value="skillQuery.changeType"
                :options="changeTypeOptions"
                placeholder="变更动作"
                clearable
                style="width: 140px"
                @update:value="fetchSkillData"
              />
              <n-select
                v-model:value="skillQuery.status"
                :options="skillStatusOptions"
                placeholder="审批状态"
                clearable
                style="width: 140px"
                @update:value="fetchSkillData"
              />
              <n-button type="primary" :loading="skillLoading" @click="fetchSkillData">
                查询
              </n-button>
              <n-button @click="resetSkillQuery">
                重置
              </n-button>
              <n-button
                type="success"
                :disabled="checkedSkillIds.length === 0"
                @click="batchApprove"
              >
                批量通过 ({{ checkedSkillIds.length }})
              </n-button>
              <n-button
                type="error"
                :disabled="checkedSkillIds.length === 0"
                @click="batchReject"
              >
                批量驳回 ({{ checkedSkillIds.length }})
              </n-button>
            </n-space>
          </div>

          <n-data-table
            :columns="skillColumns"
            :data="skillDataList"
            :loading="skillLoading"
            :bordered="true"
            :single-line="false"
            size="small"
            class="audit-table"
            :row-key="(row) => row.id"
            v-model:checked-row-keys="checkedSkillIds"
            :scroll-x="1000"
          />

          <div class="pagination-footer">
            <n-pagination
              v-model:page="skillQuery.page"
              v-model:page-size="skillQuery.pageSize"
              :item-count="skillTotal"
              :page-sizes="[10, 20, 50]"
              show-size-picker
              @update:page="fetchSkillData"
              @update:page-size="onSkillPageSizeChange"
            />
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-card>

    <!-- 详情弹窗 -->
    <n-modal
      v-model:show="detailModalVisible"
      preset="card"
      :title="modalTitle"
      style="width: 800px; max-width: 90vw;"
    >
      <div class="detail-body">
        <pre class="code-preview">{{ modalContent }}</pre>
      </div>
      <template #footer>
        <n-space justify="end">
          <n-button @click="detailModalVisible = false">关闭</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  NCard, NTabs, NTabPane, NSpace, NInput, NSelect, NButton,
  NDataTable, NPagination, NTag, NEllipsis, NModal, useMessage, useDialog,
  type DataTableColumns
} from 'naive-ui'
import {
  FetchAuditLogs,
  FetchSkillChanges,
  ApproveSkillChange,
  RejectSkillChange,
  BatchApproveSkillChanges,
  BatchRejectSkillChanges
} from '../../../wailsjs/go/main/App'

const message = useMessage()
const dialog = useDialog()
const activeTab = ref('audit')

// ========== 1. 审计日志状态与查询 ==========
const auditQuery = reactive({
  toolName: '',
  serverName: '',
  keyword: '',
  successFilter: null as string | null,
  page: 1,
  pageSize: 20
})

const auditLoading = ref(false)
const auditTotal = ref(0)
const auditData = ref<any[]>([])

const successOptions = [
  { label: '全部结果', value: '' },
  { label: '成功', value: 'true' },
  { label: '失败/异常', value: 'false' }
]

const auditColumns: DataTableColumns<any> = [
  {
    title: 'ID',
    key: 'id',
    width: 70,
    align: 'center'
  },
  {
    title: '时间',
    key: 'createdAt',
    width: 160,
    render(row) {
      if (!row.createdAt) return '-'
      const d = new Date(row.createdAt)
      return d.toLocaleString('zh-CN', { hour12: false })
    }
  },
  {
    title: '工具名称',
    key: 'toolName',
    width: 170,
    render(row) {
      return h(
        'span',
        { style: { fontFamily: 'Consolas, monospace', fontWeight: 600, color: '#165dff' } },
        row.toolName || '-'
      )
    }
  },
  {
    title: '来源/MCP服务',
    key: 'serverName',
    width: 130,
    render(row) {
      return row.serverName ? h(NTag, { size: 'small', bordered: false }, { default: () => row.serverName }) : '-'
    }
  },
  {
    title: '状态',
    key: 'success',
    width: 90,
    align: 'center',
    render(row) {
      return h(
        NTag,
        { size: 'small', type: row.success ? 'success' : 'error', bordered: false },
        { default: () => (row.success ? '成功' : '失败') }
      )
    }
  },
  {
    title: '耗时',
    key: 'durationMs',
    width: 90,
    align: 'right',
    render(row) {
      const ms = row.durationMs || 0
      return h(
        'span',
        { style: { color: ms > 1500 ? '#f0a020' : '#888' } },
        `${ms}ms`
      )
    }
  },
  {
    title: '请求参数',
    key: 'requestBody',
    render(row) {
      if (!row.requestBody) return '-'
      return h(
        'div',
        {
          style: { cursor: 'pointer', color: '#165dff' },
          onClick: () => showDetail('请求参数详情', row.requestBody)
        },
        [
          h(NEllipsis, { lineClamp: 1, tooltip: false }, { default: () => row.requestBody })
        ]
      )
    }
  },
  {
    title: '响应预览 / 异常',
    key: 'response',
    render(row) {
      const text = row.errorMsg || row.response || '-'
      const isErr = !!row.errorMsg
      return h(
        'div',
        {
          style: { cursor: 'pointer', color: isErr ? '#d03050' : 'inherit' },
          onClick: () => showDetail(isErr ? '异常错误详情' : '响应数据详情', text)
        },
        [
          h(NEllipsis, { lineClamp: 1, tooltip: false }, { default: () => text })
        ]
      )
    }
  }
]

async function fetchAuditData() {
  auditLoading.value = true
  try {
    let successVal: boolean | undefined = undefined
    if (auditQuery.successFilter === 'true') successVal = true
    if (auditQuery.successFilter === 'false') successVal = false

    const resp: any = await FetchAuditLogs({
      page: auditQuery.page,
      pageSize: auditQuery.pageSize,
      toolName: auditQuery.toolName || undefined,
      serverName: auditQuery.serverName || undefined,
      keyword: auditQuery.keyword || undefined,
      success: successVal
    })

    if (resp && resp.data) {
      auditData.value = resp.data.list || []
      auditTotal.value = resp.data.total || 0
    } else if (Array.isArray(resp)) {
      auditData.value = resp
      auditTotal.value = resp.length
    } else {
      auditData.value = []
      auditTotal.value = 0
    }
  } catch (err: any) {
    message.error(`获取审计日志失败: ${err?.message || err}`)
  } finally {
    auditLoading.value = false
  }
}

function resetAuditQuery() {
  auditQuery.toolName = ''
  auditQuery.serverName = ''
  auditQuery.keyword = ''
  auditQuery.successFilter = null
  auditQuery.page = 1
  fetchAuditData()
}

function onAuditPageSizeChange(val: number) {
  auditQuery.pageSize = val
  auditQuery.page = 1
  fetchAuditData()
}

// ========== 2. 技能审批状态与查询 ==========
const skillQuery = reactive({
  changeType: null as string | null,
  status: null as string | null,
  page: 1,
  pageSize: 20
})

const skillLoading = ref(false)
const skillTotal = ref(0)
const skillDataList = ref<any[]>([])
const checkedSkillIds = ref<(string | number)[]>([])

const changeTypeOptions = [
  { label: '全部动作', value: '' },
  { label: '新建 (Create)', value: 'create' },
  { label: '更新 (Update)', value: 'update' },
  { label: '删除 (Delete)', value: 'delete' }
]

const skillStatusOptions = [
  { label: '全部状态', value: '' },
  { label: '待审批 (Pending)', value: 'pending' },
  { label: '已通过 (Approved)', value: 'approved' },
  { label: '已驳回 (Rejected)', value: 'rejected' }
]

const skillColumns: DataTableColumns<any> = [
  {
    type: 'selection'
  },
  {
    title: 'ID',
    key: 'id',
    width: 70,
    align: 'center'
  },
  {
    title: '申请时间',
    key: 'createdAt',
    width: 160,
    render(row) {
      if (!row.createdAt) return '-'
      const d = new Date(row.createdAt)
      return d.toLocaleString('zh-CN', { hour12: false })
    }
  },
  {
    title: '技能ID',
    key: 'skillId',
    width: 90,
    align: 'center',
    render(row) {
      return row.skillId ? `#${row.skillId}` : '新技能'
    }
  },
  {
    title: '变更动作',
    key: 'changeType',
    width: 100,
    align: 'center',
    render(row) {
      const map: Record<string, { label: string; type: any }> = {
        create: { label: '新建', type: 'info' },
        update: { label: '更新', type: 'warning' },
        delete: { label: '删除', type: 'error' }
      }
      const item = map[row.changeType] || { label: row.changeType || '-', type: 'default' }
      return h(NTag, { size: 'small', type: item.type, bordered: false }, { default: () => item.label })
    }
  },
  {
    title: '变更内容详情',
    key: 'newSkill',
    render(row) {
      return h(
        NButton,
        {
          size: 'tiny',
          quaternary: true,
          type: 'primary',
          onClick: () => showDetail(`技能变更详情 (#${row.id})`, row.newSkill || row.oldSkill || '无内容')
        },
        { default: () => '查看技能规范 JSON' }
      )
    }
  },
  {
    title: '当前状态',
    key: 'status',
    width: 110,
    align: 'center',
    render(row) {
      const map: Record<string, { label: string; type: any }> = {
        pending: { label: '⏳ 待审批', type: 'warning' },
        approved: { label: '✅ 已通过', type: 'success' },
        rejected: { label: '❌ 已驳回', type: 'error' }
      }
      const item = map[row.status] || { label: row.status, type: 'default' }
      return h(NTag, { size: 'small', type: item.type }, { default: () => item.label })
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    align: 'center',
    render(row) {
      if (row.status !== 'pending') {
        return h('span', { style: { color: '#888', fontSize: '12px' } }, '已归档')
      }
      return h(NSpace, { justify: 'center', size: 6 }, {
        default: () => [
          h(
            NButton,
            {
              size: 'tiny',
              type: 'success',
              onClick: () => handleApprove(row.id)
            },
            { default: () => '通过' }
          ),
          h(
            NButton,
            {
              size: 'tiny',
              type: 'error',
              onClick: () => handleReject(row.id)
            },
            { default: () => '驳回' }
          )
        ]
      })
    }
  }
]

async function fetchSkillData() {
  skillLoading.value = true
  try {
    const resp: any = await FetchSkillChanges({
      page: skillQuery.page,
      pageSize: skillQuery.pageSize,
      changeType: skillQuery.changeType || undefined,
      status: skillQuery.status || undefined
    })

    if (resp && resp.data) {
      skillDataList.value = resp.data.list || []
      skillTotal.value = resp.data.total || 0
    } else if (Array.isArray(resp)) {
      skillDataList.value = resp
      skillTotal.value = resp.length
    } else {
      skillDataList.value = []
      skillTotal.value = 0
    }
  } catch (err: any) {
    message.error(`获取技能变更审批失败: ${err?.message || err}`)
  } finally {
    skillLoading.value = false
  }
}

function resetSkillQuery() {
  skillQuery.changeType = null
  skillQuery.status = null
  skillQuery.page = 1
  checkedSkillIds.value = []
  fetchSkillData()
}

function onSkillPageSizeChange(val: number) {
  skillQuery.pageSize = val
  skillQuery.page = 1
  fetchSkillData()
}

async function handleApprove(id: number) {
  try {
    await ApproveSkillChange(id)
    message.success('已批准该技能变更')
    fetchSkillData()
  } catch (err: any) {
    message.error(`批准失败: ${err?.message || err}`)
  }
}

async function handleReject(id: number) {
  try {
    await RejectSkillChange(id)
    message.warning('已驳回该技能变更')
    fetchSkillData()
  } catch (err: any) {
    message.error(`驳回失败: ${err?.message || err}`)
  }
}

function batchApprove() {
  if (checkedSkillIds.value.length === 0) return
  dialog.warning({
    title: '批量批准确认',
    content: `确认批量批准选中的 ${checkedSkillIds.value.length} 条技能变更？`,
    positiveText: '确认批准',
    negativeText: '取消',
    onPositiveAction: async () => {
      try {
        await BatchApproveSkillChanges(checkedSkillIds.value)
        message.success('批量批准成功')
        checkedSkillIds.value = []
        fetchSkillData()
      } catch (err: any) {
        message.error(`批量批准失败: ${err?.message || err}`)
      }
    }
  })
}

function batchReject() {
  if (checkedSkillIds.value.length === 0) return
  dialog.warning({
    title: '批量驳回确认',
    content: `确认批量驳回选中的 ${checkedSkillIds.value.length} 条技能变更？`,
    positiveText: '确认驳回',
    negativeText: '取消',
    onPositiveAction: async () => {
      try {
        await BatchRejectSkillChanges(checkedSkillIds.value)
        message.warning('批量驳回完成')
        checkedSkillIds.value = []
        fetchSkillData()
      } catch (err: any) {
        message.error(`批量驳回失败: ${err?.message || err}`)
      }
    }
  })
}

// ========== 3. 详情弹窗逻辑 ==========
const detailModalVisible = ref(false)
const modalTitle = ref('')
const modalContent = ref('')

function showDetail(title: string, content: string) {
  modalTitle.value = title
  try {
    const parsed = JSON.parse(content)
    modalContent.value = JSON.stringify(parsed, null, 2)
  } catch {
    modalContent.value = content || '（内容为空）'
  }
  detailModalVisible.value = true
}

onMounted(() => {
  fetchAuditData()
  fetchSkillData()
})
</script>

<style scoped>
.audit-admin-page {
  padding: 16px;
  max-width: 1600px;
  margin: 0 auto;
}

.main-card {
  border-radius: 8px;
}

.page-header {
  margin-bottom: 20px;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-title .icon {
  font-size: 24px;
}

.header-title h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.header-desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: #888;
}

.toolbar-area {
  margin-bottom: 16px;
  padding: 12px 16px;
  background: rgba(125, 125, 125, 0.04);
  border-radius: 6px;
}

.audit-table {
  background: transparent;
}

.pagination-footer {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.detail-body {
  max-height: 60vh;
  overflow-y: auto;
}

.code-preview {
  margin: 0;
  padding: 12px;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 6px;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
