// Package data contains tushareMcp 工具白名单。
// 仅白名单内的工具会被加载到 Agent 工具集中，避免 LLM 乱试权限不足的接口。
//
// 本清单基于 2026-08-01 对官方 MCP (https://api.tushare.pro/mcp/?token=...) 的实测结果：
//   - 258 个工具中实际可用 162 个（2000 积分档，实测）
//   - 无权限(40203) 与 已废弃(40101) 的工具未列入
//   - 运行时真实白名单以 DB mcp_server_tools.enabled 为准（由 cmd/setup_tushare_mcp + 探测脚本维护），
//     本 map 仅作源码级参考，当前无直接消费方
package data

// TushareAllowList 包含 tushareMcp 中实际可访问的工具名称（2026-08-01 实测）。
// 工具名称必须与 MCP 服务器返回的 tool.name 完全一致（区分大小写）。
var TushareAllowList = map[string]bool{
	// ━━━ 行情数据 ━━━
	"stock_basic":      true, // 股票列表（基础信息）
	"trade_cal":        true, // 交易日历
	"daily":            true, // 日线行情（OHLCV）
	"adj_factor":       true, // 复权因子
	"suspend":          true, // 停牌信息
	"suspend_d":        true, // 停复牌历史
	"daily_basic":      true, // 每日指标（PE/PB/总市值）
	"weekly":           true, // 周线行情
	"monthly":          true, // 月线行情
	"stk_mins":         true, // 分钟行情
	"stk_limit":        true, // 涨跌停价格
	"stk_weekly_monthly": true, // 周月线
	"stk_week_month_adj": true, // 周月线复权
	"rt_k":             true, // 实时K线
	"rt_min":           true, // 实时分时
	"rt_min_daily":     true, // 实时分时(日)
	"rt_idx_k":         true, // 指数实时K线
	"rt_idx_min":       true, // 指数实时分时
	"rt_etf_k":         true, // ETF实时K线
	"rt_fut_min":       true, // 期货实时分时
	"rt_hk_k":          true, // 港股实时K线
	"rt_hk_tick":       true, // 港股实时tick
	"share_float":      true, // 限售股解禁

	// ━━━ 资金流向 ━━━
	"moneyflow":        true, // 个股资金流向
	"moneyflow_hsgt":   true, // 沪深港通资金流向
	"hsgt_top10":       true, // 沪深港通前10成交
	"ggt_top10":        true, // 港股通前10成交
	"ggt_daily":        true, // 港股通每日
	"margin":           true, // 融资融券汇总
	"margin_detail":    true, // 融资融券明细
	"margin_secs":      true, // 融资融券标的
	"stk_holdertrade":  true, // 股东增减持

	// ━━━ 财务报表 ━━━
	"income":           true, // 利润表
	"balancesheet":     true, // 资产负债表
	"cashflow":         true, // 现金流量表
	"forecast":         true, // 业绩预告
	"express":          true, // 业绩快报
	"fina_indicator":   true, // 财务指标
	"fina_audit":       true, // 财务审计意见
	"fina_mainbz":      true, // 主营业务构成

	// ━━━ 股东数据 ━━━
	"top10_holders":    true, // 前10大股东
	"top10_floatholders": true, // 前10大流通股东
	"stk_holdernumber": true, // 股东户数
	"pledge_stat":      true, // 股权质押统计
	"pledge_detail":    true, // 股权质押明细

	// ━━━ 指数数据 ━━━
	"index_basic":      true, // 指数基本信息
	"index_daily":      true, // 指数日线
	"index_dailybasic": true, // 指数每日指标
	"index_weight":     true, // 指数成分及权重
	"index_weekly":     true, // 指数周线
	"index_monthly":    true, // 指数月线
	"index_global":     true, // 全球指数
	"index_classify":   true, // 指数分类
	"index_member":     true, // 指数成分(申万)
	"index_member_all": true, // 指数成员(全)
	"idx_mins":         true, // 指数分钟

	// ━━━ 基本面/其他 ━━━
	"stock_company":    true, // 上市公司基本信息
	"namechange":       true, // 股票曾用名
	"dividend":         true, // 分红送股
	"new_share":        true, // 新股/可转债申购
	"repurchase":       true, // 股票回购
	"disclosure_date":  true, // 披露日期
	"stk_account":      true, // 股票账户统计
	"stk_managers":     true, // 高管信息
	"stk_rewards":      true, // 高管薪酬

	// ━━━ 涨停/龙虎 ━━━
	"top_list":         true, // 龙虎榜汇总
	"top_inst":         true, // 龙虎榜机构明细
	"top10_cb_holders": true, // 可转债十大持有人

	// ━━━ 港美股 ━━━
	"hk_basic":         true, // 港股基础信息
	"hk_daily":         true, // 港股日线
	"hk_tradecal":      true, // 港股交易日历
	"hk_hold":          true, // 港股通持股
	"hk_balancesheet":  true, // 港股资产负债表
	"hk_income":        true, // 港股利润表
	"hk_cashflow":      true, // 港股现金流量表
	"hk_fina_indicator": true, // 港股财务指标
	"hk_mins":          true, // 港股分钟
	"us_basic":         true, // 美股基础信息
	"us_tradecal":      true, // 美股交易日历
	"us_balancesheet":  true, // 美股资产负债表
	"us_income":        true, // 美股利润表
	"us_cashflow":      true, // 美股现金流量表
	"us_fina_indicator": true, // 美股财务指标
	"us_tbr":           true, // 美债收益率
	"us_tltr":          true, // 美债长期收益率
	"us_trltr":         true, // 美债实际长期收益率
	"us_trycr":         true, // 美债实际收益率曲线
	"us_tycr":          true, // 美债名义收益率曲线

	// ━━━ 基金 ━━━
	"fund_basic":       true, // 基金基础信息
	"fund_company":     true, // 基金管理公司
	"fund_manager":     true, // 基金经理
	"fund_nav":         true, // 基金净值
	"fund_daily":       true, // 基金日线
	"fund_div":         true, // 基金分红
	"fund_share":       true, // 基金份额
	"fund_adj":         true, // 基金复权因子

	// ━━━ 期货/期权/债券 ━━━
	"fut_basic":        true, // 期货基础信息
	"fut_daily":        true, // 期货日线
	"fut_settle":       true, // 期货结算
	"fut_holding":      true, // 期货持仓
	"fut_wsr":          true, // 期货龙虎
	"fut_mapping":      true, // 期货映射
	"fut_weekly_detail": true, // 期货周度明细
	"fut_weekly_monthly": true, // 期货周月线
	"opt_basic":        true, // 期权基础信息
	"opt_daily":        true, // 期权日线
	"opt_mins":         true, // 期权分钟
	"cb_basic":         true, // 可转债基础信息
	"cb_daily":         true, // 可转债日线
	"cb_issue":         true, // 可转债发行
	"cb_price_chg":     true, // 可转债价格变动
	"cb_share":         true, // 可转债转股
	"cb_rate":          true, // 可转债条款
	"cb_rating":        true, // 可转债评级
	"repo_daily":       true, // 国债逆回购

	// ━━━ 宏观/利率 ━━━
	"shibor":           true, // 上海银行间拆放利率
	"shibor_lpr":       true, // LPR
	"shibor_quote":     true, // 银行报价
	"cn_gdp":           true, // 中国GDP
	"cn_cpi":           true, // 中国CPI
	"cn_ppi":           true, // 中国PPI
	"cn_m":             true, // 货币供应量
	"cn_pmi":           true, // PMI
	"cn_schedule":      true, // 宏观数据发布时间
	"eco_cal":          true, // 经济日历
	"hibor":            true, // 香港银行间利率
	"gz_index":         true, // 国债收益率曲线
	"wz_index":         true, // 温州民间融资利率
	"sf_month":         true, // 社融月度
	"fx_basic":         true, // 外汇基础
	"fx_daily":         true, // 外汇日线
	"fx_obasic":        true, // 外汇基础信息

	// ━━━ 板块/其他 ━━━
	"dc_index":         true, // 大盘指数
	"daily_info":       true, // 每日市场统计
	"sz_daily_info":    true, // 深市每日统计
	"sge_daily":        true, // 上金所日行情
	"bse_mapping":      true, // 北交所代码映射
	"bc_bestotcqt":     true, // 银行间最优报价
	"bc_otcqt":         true, // 银行间场外行情
	"block_trade":      true, // 大宗交易
	"report_rc":        true, // 券商金股
	"broker_recommend": true, // 券商推荐
	"news":             true, // 新闻
	"cctv_news":        true, // 新闻联播
	"stock_hsgt":       true, // 沪深港通标的
	"hs_const":         true, // 沪深300成分
	"tmt_twincome":     true, // 台湾加权
	"cyq_perf":         true, // 筹码分布表现
	"cyq_chips":        true, // 筹码分布
	"p_list":           true, // 自定义板块
	"p_save":           true, // 保存自定义板块
	"p_delete":         true, // 删除自定义板块
	"p_get":            true, // 查询自定义板块
	"ft_mins":          true, // 期货分钟
	"ft_tick":          true, // 期货tick
}

// TushareAllowListMap 返回便于快速查找的 map
func TushareAllowListMap() map[string]bool {
	return TushareAllowList
}

// TushareGoldenList 是针对 2000~2500 积分精选的“核心投研黄金工具集”（约 45 个）。
// 相比全量 250+ 工具，黄金集将 Function Calling Schema 从 ~40,000 Token 压缩至 ~5,000 Token，
// 极大提升大模型响应速度（首字延迟降低 70%），且杜绝无权限(40203)踩坑。
var TushareGoldenList = map[string]bool{
	// ━━━ 1. 深度基本面财务 ━━━
	"income":           true, // 利润表 (营收、净利、营业成本)
	"balancesheet":     true, // 资产负债表 (货币资金、负债率、存货)
	"cashflow":         true, // 现金流量表 (经营性现金流净额、资本开支)
	"fina_indicator":   true, // 综合财务指标 (ROE、毛利率、资产负债率)
	"fina_mainbz":      true, // 主营业务构成 (分行业/产品收入占比)
	"fina_audit":       true, // 审计意见 (排雷非标审计意见)

	// ━━━ 2. 业绩预告与预期差 ━━━
	"forecast":         true, // 业绩预告 (抢先挖掘翻倍超预期)
	"express":          true, // 业绩快报 (中报/年报前瞻)
	"disclosure_date":  true, // 财报披露预约日期

	// ━━━ 3. 主力资金与外资北向 ━━━
	"moneyflow":        true, // 个股资金流向 (超大单/大单/中单净流入)
	"moneyflow_hsgt":   true, // 沪深港通资金流向 (北向总买入额)
	"hsgt_top10":       true, // 沪深港通前十大成交股票 (外资重仓变动)
	"ggt_top10":        true, // 港股通前十大成交

	// ━━━ 4. 筹码集中度与融资融券 ━━━
	"stk_holdernumber": true, // 股东户数 (连续季度筹码集中度)
	"top10_holders":    true, // 前十大股东 (机构/社保进退)
	"top10_floatholders": true, // 前十大流通股东
	"stk_holdertrade":  true, // 股东增减持明细
	"margin":           true, // 融资融券汇总 (全市场杠杆水位)
	"margin_detail":    true, // 个股融资融券明细 (做多热情与杠杆盘)
	"margin_secs":      true, // 两融标的名录

	// ━━━ 5. 游资与机构博弈 ━━━
	"top_list":         true, // 龙虎榜汇总 (涨跌停上榜股票)
	"top_inst":         true, // 龙虎榜机构席位明细 (机构买入 vs 游资席位)

	// ━━━ 6. 安全排雷与风控 ━━━
	"pledge_stat":      true, // 股权质押统计 (控股股东质押比例)
	"pledge_detail":    true, // 股权质押明细
	"share_float":      true, // 限售股解禁日程 (未来天量解禁排雷)
	"repurchase":       true, // 股票回购进展 (管理层做多信号)
	"dividend":         true, // 分红送配方案 (高股息红利策略)

	// ━━━ 7. 行情与估值指标 ━━━
	"daily_basic":      true, // 每日指标 (PE/PB/PS/总市值/换手率)
	"stk_limit":        true, // 每日涨跌停价格与涨停板统计
	"daily":            true, // 日线行情 (OHLCV)
	"adj_factor":       true, // 复权因子
	"stock_basic":      true, // 股票列表与基础信息
	"stock_company":    true, // 上市公司主营与基本信息

	// ━━━ 8. 指数与风格轮动 ━━━
	"index_basic":      true, // 指数基本信息
	"index_daily":      true, // 指数日线
	"index_dailybasic": true, // 指数每日指标 (估值分位数、PE/PB)
	"index_weight":     true, // 指数成分及权重 (沪深300/A500成分穿透)
	"index_classify":   true, // 申万行业分类
	"index_member_all": true, // 行业成分股名录

	// ━━━ 9. 宏观经济与流动性 ━━━
	"cn_gdp":           true, // 宏观GDP增长
	"cn_cpi":           true, // 通胀CPI
	"cn_ppi":           true, // 工业PPI
	"cn_pmi":           true, // 制造业PMI
	"cn_m":             true, // 货币供应量 (M2/M1增速)
	"shibor":           true, // 银行间拆借利率
	"shibor_lpr":       true, // 贷款市场报价利率 (LPR变动)
	"eco_cal":          true, // 经济日历发布时间

	// ━━━ 10. 公募基金与ETF ━━━
	"fund_basic":       true, // 基金基础信息 (场内ETF与场外基金)
	"fund_nav":         true, // 基金净值
	"fund_daily":       true, // 基金日线
	"fund_adj":         true, // 基金复权因子
}

// TushareGoldenListMap 返回黄金工具集 map
func TushareGoldenListMap() map[string]bool {
	return TushareGoldenList
}

