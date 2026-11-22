<script setup lang="ts">
import {onBeforeMount, ref} from 'vue'
import { IndustryResearchReport,EMDictCode} from "../../wailsjs/go/main/App";
import { RefreshCircleSharp,} from "@vicons/ionicons5";

import {useMessage} from "naive-ui";
import {BrowserOpenURL} from "../../wailsjs/runtime";

const message=useMessage()
const list  = ref([])

const options =  ref([])

function getIndustryResearchReport(value) {
  // message.loading() 返回销毁函数，必须在 settled 后调掉；旧代码直接丢弃
  // 返回值，导致「正在刷新数据...」常驻不消失，且失败时无任何提示、列表
  // 静默停留在旧数据上。
  const destroyLoading = message.loading("正在刷新数据...")
  IndustryResearchReport(value).then(result => {
    list.value = result || []
  }).catch(err => {
    console.error('[IndustryResearchReport] failed:', err)
    message.error('刷新行业研报失败: ' + (err?.message || err))
  }).finally(() => {
    destroyLoading()
  })
}

onBeforeMount(()=>{
  getIndustryResearchReport('');
})

function ratingChangeName(ratingChange){
  if(ratingChange===0){
    return '调高'
  }else if(ratingChange===1){
    return '调低'
  }else if(ratingChange===2){
    return '首次'
  }else if(ratingChange===3){
    return '维持'
  }else if (ratingChange===4){
    return '无变化'
  }else{
    return ''
  }
}
function openWin(code) {
  BrowserOpenURL("https://pdf.dfcfw.com/pdf/H3_"+code+"_1.pdf?1749744888000.pdf")
}

function EMDictCodeList(keyVal){
  if (keyVal){
    EMDictCode('016').then(result => {
        options.value=result.filter((value,_index,_array) => value.bkName.includes(keyVal)||value.firstLetter.includes(keyVal)||value.bkCode.includes(keyVal)).map(item => {
          return {
            label: item.bkName+" - "+item.bkCode,
            value: item.bkCode
          }
        })
    }).catch(err => {
      console.error('[IndustryResearchReport] EMDictCode failed:', err)
      message.error('加载行业字典失败: ' + (err?.message || err))
    })
  }else{
    getIndustryResearchReport('')
  }

}
function handleSearch(value) {
  getIndustryResearchReport(value)
}
</script>

<template>
  <n-card>
    <n-auto-complete
      :options="options"
      placeholder="请输入行业名称关键词搜索"
      clearable
      filterable
      :on-select="handleSearch"
      :on-update:value="EMDictCodeList"
    />
  </n-card>
  <n-table
    striped
    size="small"
  >
    <n-thead>
      <n-tr>
        <n-th>行业</n-th>
        <n-th>标题</n-th>
        <n-th>东财评级</n-th>
        <n-th>评级变动</n-th>
        <n-th>机构评级</n-th>
        <n-th>分析师</n-th>
        <n-th>机构</n-th>
        <n-th>
          <n-flex justify="space-between">
            日期<n-icon
              color="#409EFF"
              :size="20"
              :component="RefreshCircleSharp"
              @click="getIndustryResearchReport"
            />
          </n-flex>
        </n-th>
      </n-tr>
    </n-thead>
    <n-tbody>
      <n-tr
        v-for="item in list"
        :key="item.infoCode"
      >
        <n-td>
          <n-tag
            type="info"
            :bordered="false"
          >
            {{ item.industryName }}
          </n-tag>
        </n-td>
        <n-td>
          <n-a
            type="info"
            @click="openWin(item.infoCode)"
          >
            <n-text type="success">
              {{ item.title }}
            </n-text>
          </n-a>
        </n-td>
        <n-td>
          <n-text :type="item.emRatingName==='增持'?'error':'info'">
            {{ item.emRatingName }}
          </n-text>
        </n-td>
        <n-td>
          <n-text :type="item.ratingChange===0?'error':'info'">
            {{ ratingChangeName(item.ratingChange) }}
          </n-text>
        </n-td>
        <n-td>{{ item.sRatingName	}}</n-td>
        <n-td>
          <n-ellipsis style="max-width: 120px">
            {{ item.researcher }}
          </n-ellipsis>
        </n-td>
        <n-td>{{ item.orgSName }}</n-td>
        <n-td>{{ item.publishDate ? item.publishDate.substring(0,10) : '--' }}</n-td>
      </n-tr>
    </n-tbody>
  </n-table>
</template>

<style scoped>

</style>