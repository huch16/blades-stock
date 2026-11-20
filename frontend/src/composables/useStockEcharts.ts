// 分时 / 日 K 的 ECharts 绘制：分时图、K 线图、MA 计算、颜色常量与分时轮询。
// 由 stock.vue 抽出（R-22b），逻辑逐字搬运；data 与两个图表容器 ref、
// feishiInterval 由组件以 ctx 传入（feishiInterval 的声明仍在组件，
// onBeforeUnmount 里也要清它）。
import * as echarts from '../utils/echarts'
import {
  GetStockKLine,
  GetStockMinutePriceLineData,
} from '../../wailsjs/go/main/App'

export function useStockEcharts(ctx) {
  const data = ctx.data
  const feishiInterval = ctx.feishiInterval
  const kLineChartRef = ctx.kLineChartRef
  const kLineChartRef2 = ctx.kLineChartRef2

const upColor = '#ec0000';
const downColor = '#00da3c';
function clearFeishi() {
  //console.log("clearFeishi")
  clearInterval(feishiInterval.value)
}

function showFsChart(code, name) {
  data.name = name
  data.code = code
  const chart = echarts.getInstanceByDom(kLineChartRef2.value) || echarts.init(kLineChartRef2.value);
  GetStockMinutePriceLineData(code, name).then(result => {
    // console.log("GetStockMinutePriceLineData", result)
    const priceData = result.priceData
    const category = []
    const price = []
    // 说明（no-useless-assignment / prefer-const）：二者仅由行情数据赋值一次，
    // 此前先声明为 let 并赋初值 0，初值从未被读取，现直接以 const 就地初始化。
    const openprice = priceData[0].price
    const closeprice = priceData[priceData.length - 1].price
    const volume = []
    const volumeRate = []
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
      if (i > 0) {
        const b = priceData[i].volume - priceData[i - 1].volume
        volumeRate.push(((b - volume[i - 1]) / volume[i - 1] * 100).toFixed(2))
        volume.push(b)
      } else {
        volume.push(priceData[i].volume)
        volumeRate.push(0)
      }
    }

    const option = {
      title: {
        subtext: "[" + result.date + "] 开盘:" + openprice + " 最新:" + closeprice + " 最高:" + max + " 最低:" + min,
        left: 'center',
        top: '10',
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        }
      },
      legend: {
        data: ['股价', '成交量'],
        //orient: 'vertical',
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
        right: 50,
      },
      darkMode: data.darkTheme,
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          animation: false,
          label: {
            backgroundColor: '#505765'
          }
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
      xAxis: [
        {
          type: 'category',
          data: category,
          axisLabel: {
            show: false
          }
        },
        {
          gridIndex: 1,
          type: 'category',
          data: category,
        },
      ],
      grid: [
        {
          left: '8%',
          right: '8%',
          height: '50%',
        },
        {
          left: '8%',
          right: '8%',
          top: '70%',
          height: '15%'
        },
      ],
      yAxis: [
        {
          axisLine: {
            show: true
          },
          splitLine: {
            show: false
          },
          name: "股价",
          min: (min - min * 0.01).toFixed(2),
          max: (max + max * 0.01).toFixed(2),
          minInterval: 0.01,
          type: 'value'
        },
        {
          gridIndex: 1,
          axisLine: {
            show: true
          },
          splitLine: {
            show: false
          },
          name: "成交量",
          type: 'value',
        },
      ],
      visualMap: {
        type: 'piecewise',
        seriesIndex: 0,
        top: 0,
        left: 10,
        orient: 'horizontal',
        textStyle: {
          color: data.darkTheme ? '#fff' : '#456'
        },
        pieces: [
          {
            text: '低于开盘价',
            gt: 0,
            lte: openprice,
            color: '#31F113',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          },
          {
            text: '大于开盘价小于收盘价',
            gt: openprice,
            lte: closeprice,
            color: '#1651EF',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          },
          {
            text: '大于收盘价',
            gt: closeprice,
            color: '#AC3B2A',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          }
        ],
      },
      series: [
        {
          name: "股价",
          data: price,
          type: 'line',
          smooth: false,
          showSymbol: false,
          lineStyle: {
            width: 3
          },
          markPoint: {
            symbol: 'arrow',
            symbolRotate: 90,
            symbolSize: [10, 20],
            symbolOffset: [10, 0],
            itemStyle: {
              color: '#FC290D'
            },
            label: {
              position: 'right',
            },
            data: [
              {type: 'max', name: 'Max'},
              {type: 'min', name: 'Min'}
            ]
          },
          markLine: {
            symbol: 'none',
            data: [
              {type: 'average', name: 'Average'},
              {
                lineStyle: {
                  color: '#FFCB00',
                  width: 0.5
                },
                yAxis: openprice,
                name: '开盘价'
              },
              {
                yAxis: closeprice,
                symbol: 'none',
                lineStyle: {
                  color: 'red',
                  width: 0.5
                },
              }
            ]
          },
        },
        {
          xAxisIndex: 1,
          yAxisIndex: 1,
          name: "成交量",
          data: volume,
          type: 'bar',
        },

      ]
    };
    chart.setOption(option);
  })
}
function handleFeishi() {
  showFsChart(data.code, data.name);
  feishiInterval.value = setInterval(() => {
    showFsChart(data.code, data.name);
  }, 1000 * 10)
}

function calculateMA(dayCount, values) {
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
function handleKLine() {
  GetStockKLine(data.code, data.name, 365).then(result => {
    //console.log("GetStockKLine",result)
    const chart = echarts.getInstanceByDom(kLineChartRef.value) || echarts.init(kLineChartRef.value);
    const categoryData = [];
    const values = [];
    const volumns = [];
    for (let i = 0; i < result.length; i++) {
      const resultElement = result[i]
      //console.log("resultElement:{}",resultElement)
      categoryData.push(resultElement.day)
      const flag = resultElement.close > resultElement.open ? 1 : -1
      values.push([
        resultElement.open,
        resultElement.close,
        resultElement.low,
        resultElement.high
      ])
      volumns.push([i, resultElement.volume / 10000, flag])
    }
    ////console.log("categoryData",categoryData)
    ////console.log("values",values)
    const option = {
      darkMode: data.darkTheme,
      //backgroundColor: '#1c1c1c',
      // color:['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc'],
      animation: false,
      legend: {
        bottom: 10,
        left: 'center',
        data: ['日K', 'MA5', 'MA10', 'MA20', 'MA30'],
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          lineStyle: {
            color: '#376df4',
            width: 1,
            opacity: 1
          }
        },
        borderWidth: 2,
        borderColor: data.darkTheme ? '#456' : '#ccc',
        backgroundColor: data.darkTheme ? '#456' : '#fff',
        padding: 10,
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
        formatter: function (params) {//修改鼠标划过显示为中文
          //console.log("params",params)
          const volum = params[5].data;//ma5的值
          const ma5 = params[1].data;//ma5的值
          const ma10 = params[2].data;//ma10的值
          const ma20 = params[3].data;//ma20的值
          const ma30 = params[4].data;//ma30的值
          params = params[0];//开盘收盘最低最高数据汇总
          const currentItemData = params.data;

          return params.name + '<br>' +
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
        // position: function (pos, params, el, elRect, size) {
        //   const obj = {
        //     top: 10
        //   };
        //   obj[['left', 'right'][+(pos[0] < size.viewSize[0] / 2)]] = 30;
        //   return obj;
        // }
        // extraCssText: 'width: 170px'
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
          left: '10%',
          right: '8%',
          height: '50%',
        },
        {
          left: '10%',
          right: '8%',
          top: '63%',
          height: '16%'
        }
      ],
      xAxis: [
        {
          type: 'category',
          data: categoryData,
          boundaryGap: false,
          axisLine: {onZero: false},
          splitLine: {show: false},
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
          axisLine: {onZero: false},
          axisTick: {show: false},
          splitLine: {show: false},
          axisLabel: {show: false},
          min: 'dataMin',
          max: 'dataMax'
        }
      ],
      yAxis: [
        {
          scale: true,
          splitArea: {
            show: true
          }
        },
        {
          scale: true,
          gridIndex: 1,
          splitNumber: 2,
          axisLabel: {show: false},
          axisLine: {show: false},
          axisTick: {show: false},
          splitLine: {show: false}
        }
      ],
      dataZoom: [
        {
          type: 'inside',
          xAxisIndex: [0, 1],
          start: 86,
          end: 100
        },
        {
          show: true,
          xAxisIndex: [0, 1],
          type: 'slider',
          top: '85%',
          start: 86,
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
          data: calculateMA(5, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA10',
          type: 'line',
          data: calculateMA(10, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA20',
          type: 'line',
          data: calculateMA(20, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA30',
          type: 'line',
          data: calculateMA(30, values),
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
    chart.on('click', {seriesName: '日K'}, function (_params) {
      //console.log("click:",params);
    });
  })
}

  return { showFsChart, handleKLine, handleFeishi, clearFeishi }
}
