package agent

import "context"

// 运行类型（写入 agent_run_record.Kind，用于区分“这次是谁在跑”）
const (
	RunKindChat          = "chat"           // Agent 对话
	RunKindPreMarket     = "pre_market"     // 盘前前瞻
	RunKindPostMarket    = "post_market"    // 盘后复盘
	RunKindETFRotation   = "etf_rotation"   // ETF 轮动决策
	RunKindSerenity      = "serenity"       // Serenity 产业拆解
	RunKindSectorAnomaly = "sector_anomaly" // 赛道异动拆解
	RunKindSummarize     = "summarize"      // 资讯/新闻摘要
	RunKindCron          = "cron"           // 定时任务
	RunKindHoldingScan   = "holding_scan"   // 盘前持仓风控扫描（纯 Go 路径）
	RunKindPipeline      = "pipeline"       // 五阶流水线（BUIS L1→L5）
)

// ctxOrBackground 返回非空的 context：nil 时用 Background 兜底，避免
// context.WithTimeout / WithoutCancel 在 nil 父级上 panic。调用方在
// 从入参 ctx 派生子 context（同时又想保留其值）时使用。
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type runKindCtxKey struct{}

// WithRunKind 给上下文打上本次运行的类型标签。
//
// 用 context 而不是改 ChatWithContext 签名：签名变动会波及全部调用方与测试，
// 而“这次是谁在跑”本质上是**调用上下文**的信息，放在 ctx 里最自然。
func WithRunKind(ctx context.Context, kind string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if kind == "" {
		return ctx
	}
	return context.WithValue(ctx, runKindCtxKey{}, kind)
}

// WithRunKindIfUnset 仅在没有标注时设置，
// 让**外层调用方的更精确标注优先**（例如手动触发的 Serenity 请求会先标 serenity，
// 内部复用的通用赛道异动入口就不应该把它盖成 sector_anomaly）。
func WithRunKindIfUnset(ctx context.Context, kind string) context.Context {
	if ctx != nil {
		if v, ok := ctx.Value(runKindCtxKey{}).(string); ok && v != "" {
			return ctx
		}
	}
	return WithRunKind(ctx, kind)
}

// RunKindFrom 读取运行类型；未标注时按“普通对话”处理。
func RunKindFrom(ctx context.Context) string {
	if ctx == nil {
		return RunKindChat
	}
	if v, ok := ctx.Value(runKindCtxKey{}).(string); ok && v != "" {
		return v
	}
	return RunKindChat
}

// ── 注入能力摘要的回传通道 ────────────────────────────────────────────────
//
// 运行记录在 ChatWithContext 创建，而工具集在 GetStockAiAgent 深处解析。
// 两者相隔数层，用一个「指向字符串的指针」放进 context，
// 让深处能把摘要写回调用方，而不必把私有结构改成一串参数。

type injectedSinkCtxKey struct{}

// WithInjectedSink 把能力摘要的回写目标挂到 ctx。
func WithInjectedSink(ctx context.Context, sink *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, injectedSinkCtxKey{}, sink)
}

// injectedSinkFrom 取回写目标（无则 nil）。
func injectedSinkFrom(ctx context.Context) *string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(injectedSinkCtxKey{}).(*string); ok {
		return v
	}
	return nil
}
