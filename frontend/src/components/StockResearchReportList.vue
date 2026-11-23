<script setup lang="ts">
import {onBeforeMount, ref} from 'vue'
import {GetStockList, StockResearchReport} from "../../wailsjs/go/main/App";
import { RefreshCircleSharp,} from "@vicons/ionicons5";

import KLineChart from "./KLineChart.vue";
import {BrowserOpenURL} from "../../wailsjs/runtime";
import { useMessage } from "naive-ui";

const message = useMessage()

const {stockCode}=defineProps(
    {
      stockCode: {
        type: String,
        default: ''
      }
    }
)

const list  = ref([])

const options =  ref([])

function getStockResearchReport(value) {
  // 旧代码无任何错误处理：后端失败时 promise  rejection 未捕获、
  // 列表静默停留在旧数据上，用户无从得知刷新失败。
  StockResearchReport(value).then(result => {
    list.value = result || []
  }).catch(err => {
    console.error('[StockResearchReport] failed:', err)
    message.error('刷新个股研报失败: ' + (err?.message || err))
  })
}

onBeforeMount(()=>{
  getStockResearchReport(stockCode);
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
function getmMarketCode(market,code) {
  if(market==="SHENZHEN"){
    return "sz"+code
  }else if(market==="SHANGHAI"){
    return "sh"+code
  }else if(market==="BEIJING"){
    return "bj"+code
  }else if(market==="HONGKONG"){
    return "hk"+code
  }else{
    return code
  }
}
function openWin(code) {
  BrowserOpenURL("https://pdf.dfcfw.com/pdf/H3_"+code+"_1.pdf?1749744888000.pdf")
}

function findStockList(query){
  if (query){
    GetStockList(query).then(result => {
      options.value=(result || []).map(item => {
        return {
          label: item.name+" - "+item.ts_code,
          value: item.ts_code
        }
      })
    }).catch(err => {
      console.error('[StockResearchReport] GetStockList failed:', err)
    })
  }else{
    getStockResearchReport('')
  }
}
function handleSearch(value) {
  getStockResearchReport(value)
}
</script>

<template>
  <n-card>
    <n-auto-complete
      :options="options"
      placeholder="请输入A股名称或者代码"
      clearable
      filterable
      :on-select="handleSearch"
      :on-update:value="findStockList"
    />
  </n-card>
  <n-table
    striped
    size="small"
  >
    <n-thead>
      <n-tr>
        <!--        <n-th>代码</n-th>-->
        <n-th>名称</n-th>
        <n-th>行业</n-th>
        <n-th>标题</n-th>
        <n-th>东财评级</n-th>
        <n-th>评级变动</n-th>
        <n-th>机构评级</n-th>
        <n-th>分析师</n-th>
        <n-th>机构</n-th>
        <n-th>
          <n-flex justify="space-between">
            <!-- 刷新必须显传 stockCode：@click 直接写函数名会把 MouseEvent 当参数传给后端 -->
            日期<n-icon
              color="#409EFF"
              :size="20"
              :component="RefreshCircleSharp"
              @click="getStockResearchReport(stockCode)"
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
        <!--        <n-td>{{item.stockCode}}</n-td>-->
        <n-td :title="item.stockCode">
          <n-popover
            trigger="hover"
            placement="right"
          >
            <template #trigger>
              <n-tag
                type="info"
                :bordered="false"
              >
                {{ item.stockName }}
              </n-tag>
            </template>
            <k-line-chart
              style="width: 800px"
              :code="getmMarketCode(item.market,item.stockCode)"
              :chart-height="500"
              :stock-name="item.stockName"
              :k-days="20"
              :dark-theme="true"
            />
          </n-popover>
        </n-td>
        <n-td>
          <n-tag
            type="info"
            :bordered="false"
          >
            {{ item.indvInduName }}
          </n-tag>
        </n-td>
        <n-td>
          <n-a
            type="info"
            @click="openWin(item.infoCode)"
          >
            {{ item.title }}
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
        <n-td>{{ item.sRatingName }}</n-td>
        <n-td>{{ item.researcher }}</n-td>
        <n-td>{{ item.orgSName }}</n-td>
        <n-td>{{ item.publishDate ? item.publishDate.substring(0,10) : '--' }}</n-td>
      </n-tr>
    </n-tbody>
  </n-table>
</template>

<style scoped>

</style>