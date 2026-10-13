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
            <n-input v-model="auditQuery.userId" placeholder="请输入用户ID" />
          </n-form-item>
          <n-form-item label="操作类型">
            <n-select v-model="auditQuery.actionType" placeholder="请选择操作类型">
              <n-option label="全部" value="" />
              <n-option label="创建" value="create" />
              <n-option label="更新" value="update" />
              <n-option label="删除" value="delete" />
            </n-select>
          </n-form-item>
          <n-form-item label="开始时间">
            <n-date-picker v-model="auditQuery.startTime" type="datetime" placeholder="开始时间" />
          </n-form-item>
          <n-form-item label="结束时间">
            <n-date-picker v-model="auditQuery.endTime" type="datetime" placeholder="结束时间" />
          </n-form-item>
          <n-button type="primary" @click="fetchAuditData">查询</n-button>
          <n-button @click="resetAuditQuery">重置</n-button>
        </NSpace>
      </NFlex>

      <!-- 数据表格 -->
      <n-table :data="auditData" :loading="auditLoading" border>
        <n-column label="ID" :width="80" property="id" />
        <n-column label="用户ID" property="userId" />
        <n-column label="操作类型" property="actionType" />
        <n-column label="操作对象" property="target" />
        <n-column label="操作前值" property="beforeValue" show-tooltip>
          <template #default="{ row }">
            <el-tooltip class="item" :content="row.beforeValue">
              <div class="tooltip-content">{{ row.beforeValue }}</div>
            </el-tooltip>
          </template>
        </n-column>
        <n-column label="操作后值" property="afterValue" show-tooltip>
          <template #default="{ row }">
            <el-tooltip class="item" :content="row.afterValue">
              <div class="tooltip-content">{{ row.afterValue }}</div>
            </el-tooltip>
          </template>
        </n-column>
        <n-column label="操作时间" property="createdAt" />
        <n-column label="操作人" property="operator" />
        <n-column label="备注" property="remark" />
      </n-table>

      <!-- 分页 -->
      <n-pagination
        :page-size="auditQuery.pageSize"
        show-sizer
        :total="auditTotal"
        @update:page-size="onAuditPageSizeChange"
        @update:current-page="onAuditPageChange"
        style="margin-top: 12px; text-align: right"
      />
    </n-stack>

    <!-- 技能变更审批内容 -->
    <n-stack v-show="activeTab === 'skill'" vertical gap="16">
      <!-- 查询表单 -->
      <NFlex vertical gap="12">
        <NSpace wrap>
          <n-form-item label="技能名称">
            <n-input v-model="skillQuery.name" placeholder="请输入技能名称" />
          </n-form-item>
          <n-form-item label="申请人">
            <n-input v-model="skillQuery.applicant" placeholder="请输入申请人" />
          </n-form-item>
          <n-form-item label="状态">
            <n-select v-model="skillQuery.status" placeholder="请选择状态">
              <n-option label="全部" value="" />
              <n-option label="待审批" value="pending" />
              <n-option label="已通过" value="approved" />
              <n-option label="已拒绝" value="rejected" />
            </n-select>
          </n-form-item>
          <n-button type="primary" @click="fetchSkillData">查询</n-button>
          <n-button @click="resetSkillQuery">重置</n-button>
          <n-button type="danger" @click="batchApprove">批量通过</n-button>
          <n-button type="error" @click="batchReject">批量拒绝</n-button>
        </NSpace>
      </NFlex>

      <!-- 技能变更表格 -->
      <n-table :data="skillDataList" :loading="skillLoading" border>
        <n-column label="ID" :width="80" property="id" />
        <n-column label="技能名称" property="name" />
        <n-column label="申请人" property="applicant" />
        <n-column label="申请时间" property="createdAt" />
        <n-column label="变更类型" property="changeType" />
        <n-column label="变更内容" property="changeContent" show-tooltip>
          <template #default="{ row }">
            <el-tooltip class="item" :content="row.changeContent">
              <div class="tooltip-content">{{ row.changeContent }}</div>
            </el-tooltip>
          </template>
        </n-column>
        <n-column label="审批状态" :width="100" property="status">
          <template #default="{ row }">
            <n-tag :type="row.status === 'approved' ? 'success' : row.status === 'rejected' ? 'error' : 'warning'">
              {{ row.status === 'pending' ? '待审批' : row.status === 'approved' ? '已通过' : '已拒绝' }}
            </n-tag>
          </template>
        </n-column>
        <n-column label="操作" :width="120" fixed="right">
          <template #default="{ row }">
            <n-button-group>
              <n-button
                v-if="row.status === 'pending'"
                type="primary"
                size="small"
                @click="handleApprove(row.id)"
                >通过
              </n-button>
              <n-button
                v-if="row.status === 'pending'"
                type="error"
                size="small"
                @click="handleReject(row.id)"
                >拒绝
              </n-button>
              <n-button
                v-if="row.status !== 'pending'"
                type="default"
                size="small"
                :disabled="true"
              >
                {{ row.status === 'approved' ? '已通过' : '已拒绝' }}
              </n-button>
            </n-button-group>
          </template>
        </n-column>
      </n-table>

      <!-- 分页 -->
      <n-pagination
        :page-size="skillQuery.pageSize"
        show-sizer
        :total="skillTotal"
        @update:page-size="onSkillPageSizeChange"
        @update:current-page="onSkillPageChange"
        style="margin-top: 12px; text-align: right"
      />
    </n-stack>
  </NFlex>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import {
  FetchAuditLogs,
  FetchSkillChanges,
  ApproveSkillChange,
  RejectSkillChange,
  BatchApproveSkillChanges,
  BatchRejectSkillChanges
} from '../../../wailsjs/go/main/App'

// 消息提示
const message = useMessage()

// 查询表单状态
const auditQuery = reactive({
  userId: '',
  actionType: '',
  startTime: '',
  endTime: '',
  pageSize: 20,
  currentPage: 1
})

const skillQuery = reactive({
  name: '',
  applicant: '',
  status: '',
  pageSize: 20,
  currentPage: 1
})

// 数据状态
const auditData = ref<any[]>([])
const auditLoading = ref(false)
const auditTotal = ref(0)

const skillDataList = ref<any[]>([])
const skillLoading = ref(false)
const skillTotal = ref(0)

// 获取审计日志
async function fetchAuditData() {
  auditLoading.value = true
  try {
    const resp = await FetchAuditLogs({
      userId: auditQuery.userId,
      actionType: auditQuery.actionType,
      startTime: auditQuery.startTime,
      endTime: auditQuery.endTime,
      page: auditQuery.currentPage,
      pageSize: auditQuery.pageSize
    })
    if (resp.code === 1) {
      auditData.value = resp.data.list || []
      auditTotal.value = resp.data.total || 0
    } else {
      message.error(resp.msg || '获取审计日志失败')
    }
  } catch (err) {
    message.error(`获取审计日志异常: ${err}`)
  } finally {
    auditLoading.value = false
  }
}

// 重置审计查询条件
function resetAuditQuery() {
  auditQuery.userId = ''
  auditQuery.actionType = ''
  auditQuery.startTime = ''
  auditQuery.endTime = ''
  auditQuery.currentPage = 1
  fetchAuditData()
}

// 分页处理
function onAuditPageSizeChange(val: number) {
  auditQuery.pageSize = val
  auditQuery.currentPage = 1
  fetchAuditData()
}

function onAuditPageChange(val: number) {
  auditQuery.currentPage = val
  fetchAuditData()
}

// 获取技能变更数据
async function fetchSkillData() {
  skillLoading.value = true
  try {
    const resp = await FetchSkillChanges({
      name: skillQuery.name,
      applicant: skillQuery.applicant,
      status: skillQuery.status,
      page: skillQuery.currentPage,
      pageSize: skillQuery.pageSize
    })
    if (resp.code === 1) {
      skillDataList.value = resp.data.list || []
      skillTotal.value = resp.data.total || 0
    } else {
      message.error(resp.msg || '获取技能变更数据失败')
    }
  } catch (err) {
    message.error(`获取技能变更数据异常: ${err}`)
  } finally {
    skillLoading.value = false
  }
}

// 重置技能变更查询条件
function resetSkillQuery() {
  skillQuery.name = ''
  skillQuery.applicant = ''
  skillQuery.status = ''
  skillQuery.currentPage = 1
  fetchSkillData()
}

// 分页处理
function onSkillPageSizeChange(val: number) {
  skillQuery.pageSize = val
  skillQuery.currentPage = 1
  fetchSkillData()
}

function onSkillPageChange(val: number) {
  skillQuery.currentPage = val
  fetchSkillData()
}

// 审批单个技能变更
async function handleApprove(id: number) {
  try {
    const resp = await ApproveSkillChange({ id })
    if (resp.code === 1) {
      message.success('技能变更审批通过')
      fetchSkillData()
    } else {
      message.error(resp.msg || '审批失败')
    }
  } catch (err) {
    message.error(`审批异常: ${err}`)
  }
}

async function handleReject(id: number) {
  try {
    const resp = await RejectSkillChange({ id })
    if (resp.code === 1) {
      message.success('技能变更审批拒绝')
      fetchSkillData()
    } else {
      message.error(resp.msg || '拒绝失败')
    }
  } catch (err) {
    message.error(`拒绝异常: ${err}`)
  }
}

// 批量审批
async function batchApprove() {
  if (skillDataList.value.length === 0) {
    message.warning('没有数据可操作')
    return
  }
  try {
    const ids = skillDataList.value
      .filter(item => item.status === 'pending')
      .map(item => item.id)
    
    if (ids.length === 0) {
      message.warning('没有待审批的数据')
      return
    }
    
    const resp = await BatchApproveSkillChanges({ ids })
    if (resp.code === 1) {
      message.success('批量通过成功')
      fetchSkillData()
    } else {
      message.error(resp.msg || '批量通过失败')
    }
  } catch (err) {
    message.error(`批量通过异常: ${err}`)
  }
}

// 批量拒绝
async function batchReject() {
  if (skillDataList.value.length === 0) {
    message.warning('没有数据可操作')
    return
  }
  try {
    const ids = skillDataList.value
      .filter(item => item.status === 'pending')
      .map(item => item.id)
    
    if (ids.length === 0) {
      message.warning('没有待审批的数据')
      return
    }
    
    const resp = await BatchRejectSkillChanges({ ids })
    if (resp.code === 1) {
      message.success('批量拒绝成功')
      fetchSkillData()
    } else {
      message.error(resp.msg || '批量拒绝失败')
    }
  } catch (err) {
    message.error(`批量拒绝异常: ${err}`)
  }
}

// 初始化
onMounted(() => {
  fetchAuditData()
  fetchSkillData()
})
</script>

<style scoped>
.chat-box {
  height: 100%;
  overflow: hidden;
}
</style>