<template>
  <NFlex vertical gap="20" padding="20">
    <!-- 选项卡 -->
    <n-tabs v-model:value="activeTab" @update:value="onTabChange">
      <n-tab label="审计日志" name="audit" />
      <n-tab label="技能变更审批" name="skill" />
    </n-tabs>

    <!-- 审计日志内容 -->
    <n-stack v-show="activeTab === 'audit'" vertical gap="16">
      <!-- 查询表单 -->
      <NFlex vertical gap="12">
        <NSpace wrap>
          <n-form-item label="用户ID">
            <n-input v-model="auditQuery.userId" placeholder="请输入用户ID" clearable />
          </n-form-item>
          <n-form-item label="AgentID">
            <n-input v-model="auditQuery.agentId" placeholder="请输入AgentID" clearable />
          </n-form-item>
          <n-form-item label="SkillID">
            <n-input-number v-model="auditQuery.skillId" min="0" placeholder="请输入SkillID" clearable />
          </n-form-item>
          <n-form-item label="工具名称">
            <n-input v-model="auditQuery.toolName" placeholder="请输入工具名称" clearable />
          </n-form-item>
          <n-form-item label="服务器名称">
            <n-input v-model="auditQuery.serverName" placeholder="请输入服务器名称" clearable />
          </n-form-item>
          <n-form-item label="是否成功">
            <n-select v-model="auditQuery.success" placeholder="请选择" clearable>
              <n-option :label="''" :value="null" />
              <n-option label="成功" :value="true" />
              <n-option label="失败" :value="false" />
            </n-select>
          </n-form-item>
          <n-form-item label="开始日期">
            <n-date-picker v-model="auditQuery.startDate" placeholder="开始日期" clearable format="yyyy-MM-dd" />
          </n-form-item>
          <n-form-item label="结束日期">
            <n-date-picker v-model="auditQuery.endDate" placeholder="结束日期" clearable format="yyyy-MM-dd" />
          </n-form-item>
          <n-form-item label="关键字">
            <n-input v-model="auditQuery.keyword" placeholder="请输入关键字（在请求或响应中搜索)" clearable />
          </n-form-item>
          <NSpace justify="end">
            <n-button type="primary" @click="fetchAuditData">查询</n-button>
            <n-button ghost @click="resetAuditForm">重置</n-button>
          </NSpace>
        </NSpace>
      </NFlex>

      <!-- 数据表格 -->
      <NFlex vertical gap="12">
        <n-data-table
          :columns="auditColumns"
          :data="auditDataList"
          :loading="auditLoading"
          :pagination="false"
          :bordered="true"
          :show-header="true"
          :stripe="true"
          :height="500"
        >
          #default
            <template #default="{ row }">
              <template #default>
                <slot></slot>
              </template>
            </template>
        </n-data-table>
      </NFlex>

      <!-- 分页和导出 -->
      <NFlex justify="space-between" align="center">
        <n-pagination
          :page-count="Math.ceil(auditTotal / auditQuery.pageSize)"
          :show-less-pages="true"
          :show-size-changer="true"
          :show-total="total => `共 ${total} 条`"
          :current-page="auditQuery.page"
          :page-size="auditQuery.pageSize"
          @update:page-size="onAuditPageSizeChange"
          @update:page-number="onAuditPageChange"
        />
        <n-button type="primary" @click="exportAuditCSV">导出 CSV</n-button>
      </NFlex>
    </n-stack>

    <!-- 技能变更审批内容 -->
    <n-stack v-show="activeTab === 'skill'" vertical gap="16">
      <!-- 查询表单 -->
      <NFlex vertical gap="12">
        <NSpace wrap>
          <n-form-item label="技能ID">
            <n-input-number v-model="skillQuery.skillId" min="0" placeholder="请输入技能ID" clearable />
          </n-form-item>
          <n-form-item label="变更类型">
            <n-input v-model="skillQuery.changeType" placeholder="请输入变更类型 (create/update/delete)" clearable />
          </n-form-item>
          <n-form-item label="状态">
            <n-select v-model="skillQuery.status" placeholder="请选择状态" clearable>
              <n-option label="全部" :value="" />
              <n-option label="待审批" :value="pending" />
              <n-option label="已批准" :value="approved" />
              <n-option label="已拒绝" :value="rejected" />
            </n-select>
          </n-form-item>
          <NSpace justify="end">
            <n-button type="primary" @click="fetchSkillData">查询</n-button>
            <n-button ghost @click="resetSkillForm">重置</n-button>
          </NSpace>
        </NSpace>
      </NFlex>

      <!-- 数据表格 -->
      <NFlex vertical gap="12">
        <n-data-table
          :columns="skillColumns"
          :data="skillDataList"
          :loading="skillLoading"
          :pagination="false"
          :bordered="true"
          :show-header="true"
          :stripe="true"
          :height="500"
        >
          #default
            <template #default="{ row }">
              <template #default>
                <slot></slot>
              </template>
          </template>
        </n-data-table>
      </NFlex>

      <!-- 分页和操作 -->
      <NFlex justify="space-between" align="center">
        <n-pagination
          :page-count="Math.ceil(skillTotal / skillQuery.pageSize)"
          :show-less-pages="true"
          :show-size-changer="true"
          :show-total="total => `共 ${total} 条`"
          :current-page="skillQuery.page"
          :page-size="skillQuery.pageSize"
          @update:page-size="onSkillPageSizeChange"
          @update:page-number="onSkillPageChange"
        />
        <n-button type="primary" @click="batchApprove">批量批准选中项</n-button>
        <n-button type="error" @click="batchReject">批量拒绝选中项</n-button>
      </NFlex>
    </n-stack>
  </NFlex>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
// 导入 Wails 生成的 Go 绑定
import {
  GetAuditLogList,
  ExportAuditLogCSV,
  GetPendingSkillChanges,
  ApproveSkillChange,
  RejectSkillChange
} from '../../wailsjs/go/main/App'

// ==================== 审计日志 ====================
const auditQuery = reactive({
  page: 1,
  pageSize: 20,
  userId: '',
  agentId: '',
  skillId: 0,
  toolName: '',
  serverName: '',
  success: null,
  startDate: '',
  endDate: '',
  keyword: ''
})
const auditLoading = ref(false)
const auditDataList = ref([])
const auditTotal = ref(0)

const auditColumns = [
  {
    title: 'ID',
    key: 'id',
    width: 80
  },
  {
    title: '创建时间',
    key: 'createdAt',
    width: 160
  },
  {
    title: '用户ID',
    key: 'userId',
    width: 100
  },
  {
    title: 'AgentID',
    key: 'agentId',
    width: 100
  },
  {
    title: 'SkillID',
    key: 'skillId',
    width: 80
  },
  {
    title: '工具名称',
    key: 'toolName',
    width: 120
  },
  {
    title: '服务器名称',
    key: 'serverName',
    width: 120
  },
  {
    title: '请求体',
    key: 'requestBody',
    width: 200,
    renderSlot: (row) => {
      const text = row.requestBody || ''
      return h('div', { title: text, class: 'text-ellipsis' }, text.slice(0, 50) + (text.length > 50 ? '...' : ''))
    }
  },
  {
    title: '响应体',
    key: 'response',
    width: 200,
    renderSlot: (row) => {
      const text = row.response || ''
      return h('div', { title: text, class: 'text-ellipsis' }, text.slice(0, 50) + (text.length > 50 ? '...' : ''))
    }
  },
  {
    title: '是否成功',
    key: 'success',
    width: 80,
    renderSlot: (row) => {
      return h('span', { class: row.success ? 'text-success' : 'text-error' }, row.success ? '是' : '否')
    }
  },
  {
    title: '错误信息',
    key: 'errorMsg',
    width: 150,
    renderSlot: (row) => {
      const text = row.errorMsg || ''
      return h('div', { title: text, class: 'text-ellipsis' }, text.slice(0, 50) + (text.length > 50 ? '...' : ''))
    }
  },
  {
    title: '耗时(ms)',
    key: 'durationMs',
    width: 100
  }
]

async function fetchAuditData() {
  auditLoading.value = true
  try {
    const resp = await GetAuditLogList(auditQuery)
    if (resp.code === 0) {
      auditDataList.value = resp.data.list
      auditTotal.value = resp.data.total
    } else {
      ElMessage.error(resp.message || '获取数据失败')
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('请求异常')
  } finally {
    auditLoading.value = false
  }
}

async function exportAuditCSV() {
  try {
    const csv = await ExportAuditLogCSV(auditQuery)
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit_log_${new Date().toISOString().slice(0,10)}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e) {
    console.error(e)
    ElMessage.error('导出失败')
  }
}

function resetAuditForm() {
  auditQuery.userId = ''
  auditQuery.agentId = ''
  auditQuery.skillId = 0
  auditQuery.toolName = ''
  auditQuery.serverName = ''
  auditQuery.success = null
  auditQuery.startDate = ''
  auditQuery.endDate = ''
  auditQuery.keyword = ''
  auditQuery.page = 1
  fetchAuditData()
}

function onAuditPageChange(page) {
  auditQuery.page = page
  fetchAuditData()
}

function onAuditPageSizeChange(pageSize) {
  auditQuery.pageSize = pageSize
  auditQuery.page = 1
  fetchAuditData()
}

// ==================== 技能变更审批 ====================
const skillQuery = reactive({
  page: 1,
  pageSize: 20,
  skillId: 0,
  changeType: '',
  status: '' // 默认显示全部
})
const skillLoading = ref(false)
const skillDataList = ref([])
const skillTotal = ref(0)

const skillColumns = [
  {
    title: 'ID',
    key: 'id',
    width: 80
  },
  {
    title: '创建时间',
    key: 'createdAt',
    width: 160
  },
  {
    title: '技能ID',
    key: 'skillId',
    width: 80
  },
  {
    title: '变更类型',
    key: 'changeType',
    width: 100
  },
  {
    title: '新技能数据',
    key: 'newSkill',
    width: 250,
    renderSlot: (row) => {
      const text = row.newSkill || ''
      return h('div', { title: text, class: 'text-ellipsis' }, text.slice(0, 100) + (text.length > 100 ? '...' : ''))
    }
  },
  {
    title: '旧技能数据',
    key: 'oldSkill',
    width: 250,
    renderSlot: (row) => {
      const text = row.oldSkill || ''
      return h('div', { title: text, class: 'text-ellipsis' }, text.slice(0, 100) + (text.length > 100 ? '...' : ''))
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    renderSlot: (row) => {
      let color = 'text-info'
      if (row.status === 'approved') color = 'text-success'
      else if (row.status === 'rejected') color = 'text-error'
      else if (row.status === 'pending') color = 'text-warning'
      return h('span', { class: color }, row.status)
    }
  },
  {
    title: '审批人',
    key: 'reviewerId',
    width: 100
  },
  {
    title: '审批时间',
    key: 'reviewedAt',
    width: 160
  }
]

async function fetchSkillData() {
  skillLoading.value = true
  try {
    const resp = await GetPendingSkillChanges(skillQuery)
    if (resp.code === 0) {
      skillDataList.value = resp.data.list
      skillTotal.value = resp.data.total
    } else {
      ElMessage.error(resp.message || '获取数据失败')
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('请求异常')
  } finally {
    skillLoading.value = false
  }
}

function resetSkillForm() {
  skillQuery.skillId = 0
  skillQuery.changeType = ''
  skillQuery.status = '' // 显示全部
  skillQuery.page = 1
  fetchSkillData()
}

function onSkillPageChange(page) {
  skillQuery.page = page
  fetchSkillData()
}

function onSkillPageSizeChange(pageSize) {
  skillQuery.pageSize = pageSize
  skillQuery.page = 1
  fetchSkillData()
}

// 批量批准选中项（这里简化为批量处理当前页的所有项，实际应需要勾选框）
async function batchApprove() {
  if (skillDataList.value.length === 0) {
    ElMessage.warning('没有数据可操作')
    return
  }
  try {
    for (const item of skillDataList.value) {
      if (item.status === 'pending') {
        await ApproveSkillChange(item.id)
      }
    }
    ElMessage.success('批量批准成功')
    fetchSkillData()
  } catch (e) {
    console.error(e)
    ElMessage.error('批量批准失败')
  }
}

// 批量拒绝选中项
async function batchReject() {
  if (skillDataList.value.length === 0) {
    ElMessage.warning('没有数据可操作')
    return
  }
  try {
    for (const item of skillDataList.value) {
      if (item.status === 'pending') {
        await RejectSkillChange(item.id)
      }
    }
    ElMessage.success('批量拒绝成功')
    fetchSkillData()
  } catch (e) {
    console.error(e)
    ElMessage.error('批量拒绝失败')
  }
}

// 初始化
onMounted(() => {
  fetchAuditData()
  fetchSkillData()
})
</script>
</component>