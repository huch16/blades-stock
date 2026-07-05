// Package data contains tushareMcp 工具白名单。
// 仅白名单内的工具会被加载到 Agent 工具集中，避免 LLM 乱试权限不足的接口。
//
// 2120 积分可访问的工具清单（核心数据，涵盖行情/财务/资金/股东）：
//   - 行情类：stock_basic, trade_cal, daily, adj_factor, suspend, daily_basic, weekly, monthly, stk_mins
//   - 资金类：moneyflow_hsgt, hsgt_top10, ggt_top10, margin, margin_detail, moneyflow
//   - 财务类：income, balancesheet, cashflow, forecast, express, fina_indicator, fina_audit, fina_mainbz
//   - 股东类：top10_holders, top10_floatholders, pledge_stat, pledge_detail
//   - 指数类：index_basic, index_daily, index_weight, index_weekly, index_monthly
//   - 基本面：stock_company, namechange, dividend, concept, concept_detail
//   - 特殊：limit_list, top_list, top_inst, new_share, repurchase
package data

// TushareAllowList 包含 tushareMcp 中 2120 积分档位可访问的工具名称。
// 工具名称必须与 MCP 服务器返回的 tool.name 完全一致（区分大小写）。
var TushareAllowList = map[string]bool{
	// ━━━ 行情数据 ━━━
	"stock_basic":      true,  // 股票列表（基础信息）
	"trade_cal":        true,  // 交易日历
	"daily":            true,  // 日线行情（OHLCV）
	"adj_factor":       true,  // 复权因子
	"suspend":          true,  // 停牌信息
	"daily_basic":      true,  // 日线市场统计（成交量/市盈率等）
	"weekly":           true,  // 周线行情
	"monthly":         true,  // 月线行情
	"stk_mins":        true,  // 分时数据

	// ━━━ 资金流向 ━━━
	"moneyflow_hsgt":  true,  // 沪深港通资金流向
	"hsgt_top10":      true,  // 沪深港通前10大成交
	"ggt_top10":       true,  // 港股通前10大成交
	"margin":          true,  // 融资融券汇总
	"margin_detail":   true,  // 融资融券明细
	"moneyflow":       true,  // 个股资金流向

	// ━━━ 财务报表 ━━━
	"income":           true,  // 利润表
	"balancesheet":     true,  // 资产负债表
	"cashflow":         true,  // 现金流量表
	"forecast":        true,  // 业绩预告
	"express":         true,  // 业绩快报
	"fina_indicator":  true,  // 财务指标
	"fina_audit":      true,  // 财务审计意见
	"fina_mainbz":     true,  // 主营业务构成

	// ━━━ 股东数据 ━━━
	"top10_holders":    true,  // 前10大股东
	"top10_floatholders": true, // 前10大流通股东
	"pledge_stat":     true,  // 股权质押统计
	"pledge_detail":   true,  // 股权质押明细

	// ━━━ 指数数据 ━━━
	"index_basic":     true,  // 指数基本信息
	"index_daily":     true,  // 指数日线
	"index_weight":   true,  // 指数成分及权重
	"index_weekly":   true,  // 指数周线
	"index_monthly":   true,  // 指数月线

	// ━━━ 基本面/其他 ━━━
	"stock_company":    true,  // 上市公司基本信息
	"namechange":      true,  // 股票曾用名
	"dividend":        true,  // 分红送股
	// concept_detail 也返回 40101，Tushare Pro 已废弃整个 concept 接口体系

	// ━━━ 涨停/龙虎 ━━━
	"limit_list":      true,  // 每日涨跌停统计
	"top_list":        true,  // 龙虎榜汇总
	"top_inst":        true,  // 龙虎榜机构明细
	"new_share":       true,  // 新股/可转债申购
	"repurchase":      true,  // 股票回购
}

// TushareAllowListMap 返回便于快速查找的 map
func TushareAllowListMap() map[string]bool {
	return TushareAllowList
}
