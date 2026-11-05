<script setup>

import {GetStockKLine} from "../../wailsjs/go/main/App";
import * as echarts from "../utils/echarts";
import {onMounted, onUnmounted, ref} from "vue";
import _ from "lodash";
const { code,stockName,darkTheme,kDays ,chartHeight} = defineProps({
  code: {
    type: String,
    default: ''
  },
  stockName: {
    type: String,
    default: ''
  },
  kDays: {
    type: Number,
    default: 14
  },
  chartHeight: {
    type: Number,
    default: 500
  },
  darkTheme: {
    type: Boolean,
    default: false
  }
})
const upColor = '#ec0000';
const downColor = '#00da3c';
const kLineChartRef = ref(null);

let resizeHandler = null

onMounted(() => {
  handleKLine(code,stockName)
  resizeHandler = () => {
    if (kLineChartRef.value) {
      echarts.getInstanceByDom(kLineChartRef.value)?.resize()
    }
  }
  window.addEventListener('resize', resizeHandler)
})

onUnmounted(() => {
  if (resizeHandler) {
    window.removeEventListener('resize', resizeHandler)
  }
  if (kLineChartRef.value) {
    echarts.getInstanceByDom(kLineChartRef.value)?.dispose()
  }
})

function handleKLine(code,stockName){
  if (!kLineChartRef.value) return
  const chart = echarts.getInstanceByDom(kLineChartRef.value) || echarts.init(kLineChartRef.value);
  chart.showLoading()
  GetStockKLine(code,stockName,365).then(result => {
    chart.hideLoading()
    if (!result || !Array.isArray(result) || result.length < 2) {
      chart.setOption({
        darkMode: darkTheme,
        title: {
          text: (stockName || code) + ' 暂无日K线历史数据',
          left: 'center',
          top: 'middle',
          textStyle: {
            color: darkTheme ? '#aaa' : '#888',
            fontSize: 14
          }
        }
      })
      return
    }
    const categoryData = [];
    const values = [];
    const volumns=[];
    for (let i = 0; i < result.length; i++) {
      const resultElement=result[i]
      categoryData.push(resultElement.day)
      let flag=Number(resultElement.close)>Number(resultElement.open)?1:-1
      if(i>0){
         flag=Number(resultElement.close)>Number(result[i-1].close)?1:-1
      }
      values.push([
        Number(resultElement.open),
        Number(resultElement.close),
        Number(resultElement.low),
        Number(resultElement.high)
      ])
      volumns.push([i,Number(resultElement.volume)/10000,flag])
    }
    const lastClose = Number(values[values.length-1][1])
    const prevClose = Number(values[values.length-2][1])
    const changeRate = prevClose > 0 ? (((lastClose - prevClose) / prevClose) * 100).toFixed(2) : '0.00'
    const option = {
      title: {
        text: `${stockName || code} ${categoryData[values.length-1]}  ${lastClose} (${changeRate}%)`,
        left: '0px',
        textStyle: {
          color: lastClose >= prevClose ? '#ef4444' : '#22c55e',
          fontSize: 14
        },
      },
      darkMode: darkTheme,
      //backgroundColor: '#1c1c1c',
      // color:['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc'],
      animation: false,
      legend: {
        right: 20,
        top: 0,
        data: ['日K', 'MA5', 'MA10', 'MA20', 'MA30'],
        textStyle: {
          color: darkTheme?'#ccc':'#456'
        },
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          lineStyle: {
            color: '#376df4',
            width: 2,
            opacity: 1
          }
        },
        borderWidth: 2,
        borderColor: darkTheme?'#456':'#ccc',
        backgroundColor: darkTheme?'#456':'#fff',
        padding: 10,
        textStyle: {
          color: darkTheme?'#ccc':'#456'
        },
        formatter: function (params) {//修改鼠标划过显示为中文
          //console.log("params",params)
          const currentItemData =  _.filter(params,  (param) => param.seriesIndex === 0)[0].data;
          const ma5=_.filter(params,  (param) => param.seriesIndex === 1)[0].data;//ma5的值
          const ma10=_.filter(params,  (param) => param.seriesIndex === 2)[0].data;//ma10的值
          const ma20=_.filter(params,  (param) => param.seriesIndex === 3)[0].data;//ma20的值
          const ma30=_.filter(params,  (param) => param.seriesIndex === 4)[0].data;//ma30的值
          const volum=_.filter(params,  (param) => param.seriesIndex === 5)[0].data;
          return _.filter(params,  (param) => param.seriesIndex === 0)[0].name + '<br>' +
              '开盘:' + currentItemData[1] + '<br>' +
              '收盘:' + currentItemData[2] + '<br>' +
              '最低:' + currentItemData[3] + '<br>' +
              '最高:' + currentItemData[4] + '<br>' +
              '成交量(万手):' + volum[1] + '<br>' +
              'MA5日均线:' + ma5 + '<br>' +
              'MA10日均线:' + ma10 + '<br>' +
              'MA20日均线:' + ma20 + '<br>' +
              'MA30日均线:' + ma30
        }
      },
      axisPointer: {
        link: [
          {
            xAxisIndex: 'all'
          }
        ],
        label: {
          backgroundColor: '#888'
        }
      },
      visualMap: {
        show: false,
        seriesIndex: 5,
        dimension: 2,
        pieces: [
          {
            value: -1,
            color: downColor
          },
          {
            value: 1,
            color: upColor
          }
        ]
      },
      grid: [
        {
          left: '8%',
          right: '8%',
          height: '50%',
        },
        {
          left: '8%',
          right: '8%',
          top: '66%',
          height: '18%'
        }
      ],
      xAxis: [
        {
          type: 'category',
          data: categoryData,
          boundaryGap: false,
          axisLine: { onZero: false },
          splitLine: { show: false },
          min: 'dataMin',
          max: 'dataMax',
          axisPointer: {
            z: 100
          }
        },
        {
          type: 'category',
          gridIndex: 1,
          data: categoryData,
          boundaryGap: false,
          axisLine: { onZero: false },
          axisTick: { show: false },
          splitLine: { show: false },
          axisLabel: { show: false },
          min: 'dataMin',
          max: 'dataMax'
        }
      ],
      yAxis: [
        {
          scale: true,
          splitArea: {
            show: true
          },
          axisLabel: { show: true },
          axisLine: { show: true },
          axisTick: { show: true },
          splitLine: { show: false }
        },
        {
          scale: true,
          gridIndex: 1,
          splitNumber: 2,
          axisLabel: { show: false },
          axisLine: { show: false },
          axisTick: { show: false },
          splitLine: { show: false }
        }
      ],
      dataZoom: [
        {
          type: 'inside',
          xAxisIndex: [0, 1],
          start: 100-kDays,
          end: 100
        },
        {
          show: true,
          xAxisIndex: [0, 1],
          type: 'slider',
          top: '85%',
          start: 100-kDays,
          end: 100
        }
      ],

      series: [
        {
          name: '日K',
          type: 'candlestick',
          data: values,
          itemStyle: {
            color: upColor,
            color0: downColor,
            // borderColor: upBorderColor,
            // borderColor0: downBorderColor
          },
          markPoint: {
            //symbol: 'none',
            label: {
              formatter: function (param) {
                return param != null ? param.value + '' : '';
              }
            },
            data: [
              {
                name: '最高',
                type: 'max',
                valueDim: 'highest'
              },
              {
                name: '最低',
                type: 'min',
                valueDim: 'lowest'
              },
              {
                name: '平均收盘价',
                type: 'average',
                valueDim: 'close'
              }
            ],
            tooltip: {
              formatter: function (param) {
                return param.name + '<br>' + (param.data.coord || '');
              }
            }
          },
          markLine: {
            symbol: ['none', 'none'],
            data: [
              [
                {
                  name: 'from lowest to highest',
                  type: 'min',
                  valueDim: 'lowest',
                  symbol: 'circle',
                  symbolSize: 10,
                  label: {
                    show: false
                  },
                  emphasis: {
                    label: {
                      show: false
                    }
                  }
                },
                {
                  type: 'max',
                  valueDim: 'highest',
                  symbol: 'circle',
                  symbolSize: 10,
                  label: {
                    show: false
                  },
                  emphasis: {
                    label: {
                      show: false
                    }
                  }
                }
              ],
              {
                name: 'min line on close',
                type: 'min',
                valueDim: 'close'
              },
              {
                name: 'max line on close',
                type: 'max',
                valueDim: 'close'
              }
            ]
          }
        },
        {
          name: 'MA5',
          type: 'line',
          data: calculateMA(5,values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA10',
          type: 'line',
          data: calculateMA(10,values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA20',
          type: 'line',
          data: calculateMA(20,values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA30',
          type: 'line',
          data: calculateMA(30,values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: '成交量(手)',
          type: 'bar',
          xAxisIndex: 1,
          yAxisIndex: 1,
          itemStyle: {
            color: '#7fbe9e'
          },
          data: volumns
        }
      ]
    };
    chart.setOption(option);
    chart.hideLoading()
  }).catch(err => {
    console.warn('GetStockKLine error:', err)
    chart.hideLoading()
    chart.setOption({
      darkMode: darkTheme,
      title: {
        text: (stockName || code) + ' K线加载超时或异常',
        left: 'center',
        top: 'middle',
        textStyle: { color: darkTheme ? '#aaa' : '#888', fontSize: 13 }
      }
    })
  })
}
function calculateMA(dayCount,values) {
  const result = [];
  for (let i = 0, len = values.length; i < len; i++) {
    if (i < dayCount) {
      result.push('-');
      continue;
    }
    let sum = 0;
    for (let j = 0; j < dayCount; j++) {
      sum += +values[i - j][1];
    }
    result.push((sum / dayCount).toFixed(2));
  }
  return result;
}
</script>

<template>
  <div
    ref="kLineChartRef"
    style="width: 100%;height: auto;--wails-draggable:no-drag"
    :style="{height:chartHeight+'px'}"
  />
</template>

<style scoped>

</style>