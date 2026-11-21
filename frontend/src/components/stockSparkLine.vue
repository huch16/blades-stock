<script setup lang="ts">
import {onMounted, onUnmounted, ref, watchEffect} from "vue";
import { loadEcharts } from '../utils/echarts-loader';
import {GetStockMinutePriceLineData} from "../../wailsjs/go/main/App"; // 如果您使用多个组件，请将此样式导入放在您的主文件中
// darkTheme 虽在 props 中保留（供父组件继续传参），但本组件并未消费，故不解构
const {idSuffix,stockCode,stockName,lastPrice,openPrice} = defineProps({
  idSuffix: {
    type: String,
    default: ""
  },
  stockCode: {
    type: String,
    default: ""
  },
  stockName: {
    type: String,
    default: ""
  },
  lastPrice: {
    type: Number,
    default: 0
  },
  openPrice: {
    type: Number,
    default: 0
  },
  darkTheme: {
    type: Boolean,
    default: true
  },
})


// ECharts 按需加载：迷你图首次挂载时才拉取完整 ECharts，首屏不引入。
// echartsApi 在 onMounted 里就绪后才画图；watchEffect 只在 chart 已创建时重绘，
// 此时 echartsApi 一定已就绪。
let echartsApi = null
async function ensureEcharts() {
  if (!echartsApi) echartsApi = await loadEcharts()
  return echartsApi
}

function setChartData(chart) {
  //console.log("setChartData")
  GetStockMinutePriceLineData(stockCode, stockName).then(result => {
    //console.log("GetStockMinutePriceLineData",result)
    const priceData = result.priceData
    const category = []
    const price = []
    let min = 0
    let max = 0
    for (let i = 0; i < priceData.length; i++) {
      category.push(priceData[i].time)
      price.push(priceData[i].price)
      if (min === 0 || min > priceData[i].price) {
        min = priceData[i].price
      }
      if (max < priceData[i].price) {
        max = priceData[i].price
      }
    }
    const option = {
      padding: [0, 0, 0, 0],
      grid: {
        top: 0,
        left: 0,
        right: 0,
        bottom: 0
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          label: {
            backgroundColor: '#6a7985'
          }
        }
      },
      xAxis: {
        show: false,
        type: 'category',
        data: category
      },
      yAxis: {
        show: false,
        type: 'value',
        min: (min).toFixed(2),
        max: (max).toFixed(2),
        minInterval: 0.01,
      },
      // visualMap: {
      //   show: false,
      //   type: 'piecewise',
      //   pieces: [
      //     {
      //       min: Number(min),
      //       max: Number(openPrice),
      //       color: 'green'
      //     },
      //     {
      //       min: Number(openPrice),
      //       max: Number(max),
      //       color: 'red'
      //     }
      //   ]
      // },
      series: [
        {
          data: price,
          type: 'line',
          smooth: false,
          stack: '总量',
          showSymbol: false,
          lineStyle: {
            color: lastPrice > openPrice ? 'rgba(245, 0, 0, 1)' : 'rgb(6,251,10)'
          },
          areaStyle: {
            color: new echartsApi.graphic.LinearGradient(0, 0, 0, 1, [{
              offset: 0,
              color: lastPrice > openPrice ? 'rgba(245, 0, 0, 1)' : 'rgba(6,251,10, 1)'
            }, {
              offset: 1,
              color: lastPrice > openPrice ? 'rgba(245, 0, 0, 0.25)' : 'rgba(6,251,10, 0.25)'
            }])
          },
        }
      ]
    };
    chart.setOption(option);
  })
}
const chart =ref( null)

onMounted(async () => {
  await ensureEcharts()
  chart.value = echartsApi.init( document.getElementById('sparkLine'+stockCode+idSuffix));
  setChartData(chart.value);
})


watchEffect(() => {
  if (chart.value) {
    setChartData(chart.value);
  }
})

onUnmounted(() => {
  if (chart.value) {
    chart.value.dispose();
    chart.value = null;
  }
})
</script>
<template>
  <div
    :id="'sparkLine'+stockCode+idSuffix"
    style="height: 20px;width: 100%"
  />
</template>