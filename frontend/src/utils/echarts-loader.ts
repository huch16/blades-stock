// ECharts 懒加载器：首屏不引入完整 ECharts（vendor-echarts 约 700KB），
// 首次真正画图时才动态拉取；多次调用共用同一份加载 Promise。
// 注意：utils/echarts.ts 仍是按需引入（echarts/core + 用到的图/组件），
// 这里只解决“何时加载”，不改变“加载什么”。
// 另见：stock.vue（首页）静态 import 过 utils/echarts 仅为卸载时 dispose，
// 现改为 useStockEcharts 导出的 disposeKlineCharts（只清已加载的实例，不触发拉取）。

type EchartsApi = typeof import('./echarts').default

let echartsPromise: Promise<EchartsApi> | null = null
let echartsModule: EchartsApi | null = null

export function loadEcharts(): Promise<EchartsApi> {
  if (!echartsPromise) {
    echartsPromise = import('./echarts').then((m) => {
      echartsModule = m.default ?? (m as unknown as EchartsApi)
      return echartsModule
    })
  }
  return echartsPromise
}

// 仅当 ECharts 已加载时返回模块实例；从不触发拉取（供卸载清理用）。
export function getLoadedEcharts(): EchartsApi | null {
  return echartsModule
}
