package agent

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// 反思 LLM 打分：采样与上限（第五轮恢复，恢复条件见 ROUND4-REPORT §6）
//
// 背景：第三轮只恢复了零 Token 的确定性评分；LLM 打分因「收益未被证明」
// 一直未恢复（第四轮 D 组结论）。本轮按用户批准的条件恢复 LLM 打分：
// 约 1/10 采样 + 每日上限约 20 次 + 最便宜的模型。超限时静默跳过，
// 绝不报错、不打断主流程（GenerateReflection 本身遇 LLM 失败也会
// 兜底到确定性评分）。
//
// 配置项（环境变量；进程启动时读取一次，改后需重启生效）：
//
//	BLADES_STOCK_REFLECTION_LLM_ENABLED     是否启用 LLM 打分，默认 true
//	BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE 采样率 0~1，默认 0.1（约 1/10）
//	BLADES_STOCK_REFLECTION_LLM_DAILY_CAP   每日 LLM 打分上限（次），默认 20
//	BLADES_STOCK_REFLECTION_LLM_MODEL       优先选用的模型（子串匹配），默认 "deepseek"
//	BLADES_STOCK_REFLECTION_LLM_MIN_RUNES   参与采样的最短回答（rune 数），默认 150
//
// 成本估算（DeepSeek deepseek-chat 价目，LiteLLM 2026-11-23 核对：
// 输入 $0.28/百万 token、输出 $0.42/百万 token；官价下限 $0.14/$0.28）：
// 单次打分约输入 ~1900 token（prompt 模板 + 问题 + 工具 + 2000 字截断回答）、
// 输出 ~200 token JSON，约 $0.00062/次 ≈ ¥0.0044/次。
// 按每日上限 20 次打满、每月 30 天：约 $0.37/月 ≈ ¥2.6/月；
// 按 1/10 采样、每日约 100 次回答的实际量级：约 ¥0.7–1.3/月。
// 即使按峰时价目，上限也在约 ¥4/月以内，与第四轮的 ¥1–12/月判断一致。
// 注意：低分触发的进化建议（OptimizePrompt，6 小时/会话限频）是另一条
// 既有 LLM 成本，不在本预算内，见 evolution_wiring.go。
// ---------------------------------------------------------------------------

// reflectionLLMConfig 是反思 LLM 打分的全部可调参数。
type reflectionLLMConfig struct {
	Enabled         bool
	SampleRate      float64 // 0~1，默认 0.1
	DailyCap        int     // 每日上限（次），默认 20；0 表示不允许任何调用
	ModelPreference string  // 优先选用的模型子串，默认 "deepseek"
	MinRunes        int     // 最短回答（rune 数），默认 150
}

func defaultReflectionLLMConfig() reflectionLLMConfig {
	return reflectionLLMConfig{
		Enabled:         true,
		SampleRate:      0.1,
		DailyCap:        20,
		ModelPreference: "deepseek",
		MinRunes:        150,
	}
}

// loadReflectionLLMConfig 从环境变量读取配置，非法值回落到默认值
// （不报错：配置读失败绝不能打断主流程）。
func loadReflectionLLMConfig() reflectionLLMConfig {
	cfg := defaultReflectionLLMConfig()
	if v, ok := os.LookupEnv("BLADES_STOCK_REFLECTION_LLM_ENABLED"); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			cfg.Enabled = b
		}
	}
	if v, ok := os.LookupEnv("BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE"); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			cfg.SampleRate = f
		}
	}
	if cfg.SampleRate < 0 {
		cfg.SampleRate = 0
	}
	if cfg.SampleRate > 1 {
		cfg.SampleRate = 1
	}
	if v, ok := os.LookupEnv("BLADES_STOCK_REFLECTION_LLM_DAILY_CAP"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.DailyCap = n
		}
	}
	if cfg.DailyCap < 0 {
		cfg.DailyCap = 0
	}
	if v, ok := os.LookupEnv("BLADES_STOCK_REFLECTION_LLM_MODEL"); ok && strings.TrimSpace(v) != "" {
		cfg.ModelPreference = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("BLADES_STOCK_REFLECTION_LLM_MIN_RUNES"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.MinRunes = n
		}
	}
	if cfg.MinRunes < 0 {
		cfg.MinRunes = 0
	}
	return cfg
}

// reflectionLLMGate 是反思 LLM 打分的采样 + 每日上限门禁。
// 采样是确定性的（同一回答永远落在同一桶），不依赖随机数，
// 便于复现与测试；每日计数按自然日重置，进程内内存计数
// （重启后当日计数清零——上限是成本护栏而非精确账本，
// 重启导致当日多跑几次的代价可忽略：最坏多 20 次，约 ¥0.09）。
type reflectionLLMGate struct {
	mu        sync.Mutex
	cfg       reflectionLLMConfig
	date      string // "2006-01-02"，本地日期
	usedToday int
}

func newReflectionLLMGate(cfg reflectionLLMConfig) *reflectionLLMGate {
	return &reflectionLLMGate{
		cfg:  cfg,
		date: time.Now().Format("2006-01-02"),
	}
}

// defaultReflectionLLMGate 是生产调用点的唯一门禁实例。
var defaultReflectionLLMGate = newReflectionLLMGate(loadReflectionLLMConfig())

// config 返回门禁持有的配置快照（供模型选择读取 ModelPreference）。
func (g *reflectionLLMGate) config() reflectionLLMConfig {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cfg
}

// sampleBucket 把字符串稳定地映射到 0..buckets-1 桶（FNV-1a，
// 全文哈希；同一回答永远同一桶，采样结果可复现）。
func sampleBucket(s string, buckets int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32() % uint32(buckets))
}

// shouldRun 是纯采样判定（不碰每日计数）：是否值得为这次回答
// 花一次 LLM 打分。短回答不值得打分；其余按采样率抽取。
// 注意：低分回答不再像旧实现（~1/3 采样时代）那样无条件打分——
// 用户批准的预算是统一的约 1/10 采样，负分不走旁路；
// 若日后想给负分预留名额，应在每日上限内分桶，而不是放开采样。
func (g *reflectionLLMGate) shouldRun(answer string) bool {
	g.mu.Lock()
	cfg := g.cfg
	g.mu.Unlock()
	if !cfg.Enabled {
		return false
	}
	if len([]rune(answer)) < cfg.MinRunes {
		return false
	}
	if cfg.SampleRate <= 0 {
		return false
	}
	if cfg.SampleRate >= 1 {
		return true
	}
	// 千分桶：采样率 0.1 → 桶号 <100 的回答入选（约 1/10）。
	return sampleBucket(answer, 1000) < int(cfg.SampleRate*1000)
}

// tryAcquire 在真正要调 LLM 前占用一个当日名额。返回 false 表示
// 当日上限已满——调用方应静默跳过（只走确定性评分），不得报错。
func (g *reflectionLLMGate) tryAcquire() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if g.date != today {
		g.date = today
		g.usedToday = 0
	}
	if g.usedToday >= g.cfg.DailyCap {
		return false
	}
	g.usedToday++
	return true
}

// release 归还一个已占用但最终未发起调用的名额
// （例如占用后发现没有可用的模型卡）。计数不会减到 0 以下。
func (g *reflectionLLMGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.usedToday > 0 {
		g.usedToday--
	}
}

// stats 供测试与运维观测：当日日期、已用名额、上限。
func (g *reflectionLLMGate) stats() (date string, used, cap int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.date, g.usedToday, g.cfg.DailyCap
}

// shouldRunLLMReflection 是生产调用点的采样判定入口（纯判定，
// 不占名额；占名额由 tryAcquire 在确认要调 LLM 时做）。
// 旧实现（~1/3 采样 + 长回答/负分必打）已被本轮批准的
// 1/10 采样 + 每日上限取代，见本文件头注释与 ROUND4-REPORT §6。
func shouldRunLLMReflection(answer string) bool {
	return defaultReflectionLLMGate.shouldRun(answer)
}
