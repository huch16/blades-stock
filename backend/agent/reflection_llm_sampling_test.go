package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"blades-stock/backend/data"
)

// TestLoadReflectionLLMConfigDefaults 锁定环境变量配置的默认值与非法值回落。
func TestLoadReflectionLLMConfigDefaults(t *testing.T) {
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_ENABLED", "")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE", "")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_DAILY_CAP", "")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_MODEL", "")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_MIN_RUNES", "")
	// 空字符串解析失败时应回落到默认值（ParseBool/ParseFloat/Atoi 报错即忽略）。
	cfg := loadReflectionLLMConfig()
	if !cfg.Enabled || cfg.SampleRate != 0.1 || cfg.DailyCap != 20 ||
		cfg.ModelPreference != "deepseek" || cfg.MinRunes != 150 {
		t.Errorf("defaults wrong: %+v", cfg)
	}
}

func TestLoadReflectionLLMConfigOverrides(t *testing.T) {
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_ENABLED", "false")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE", "0.5")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_DAILY_CAP", "5")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_MODEL", "qwen")
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_MIN_RUNES", "50")
	cfg := loadReflectionLLMConfig()
	if cfg.Enabled || cfg.SampleRate != 0.5 || cfg.DailyCap != 5 ||
		cfg.ModelPreference != "qwen" || cfg.MinRunes != 50 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadReflectionLLMConfigClamps(t *testing.T) {
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE", "2.5") // >1 → 1
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_DAILY_CAP", "-3")    // <0 → 0
	t.Setenv("BLADES_STOCK_REFLECTION_LLM_MIN_RUNES", "-1")    // <0 → 0
	cfg := loadReflectionLLMConfig()
	if cfg.SampleRate != 1 || cfg.DailyCap != 0 || cfg.MinRunes != 0 {
		t.Errorf("clamps not applied: %+v", cfg)
	}
}

// TestReflectionLLMGateSampling 锁定采样契约：确定性、约 1/10、短回答跳过、
// 关闭时全跳过、负分不走旁路（与旧 ~1/3 + 负分必打策略的区别，见采样文件头注释）。
func TestReflectionLLMGateSampling(t *testing.T) {
	gate := newReflectionLLMGate(reflectionLLMConfig{
		Enabled: true, SampleRate: 0.1, DailyCap: 20, MinRunes: 150,
	})
	long := strings.Repeat("分析结论：今日行情正常，数据完整。", 40) // >500 runes
	mid := strings.Repeat("今日行情正常，数据完整。", 30)       // 150~500
	short := "今天涨了"

	if gate.shouldRun(short) {
		t.Error("short answer should skip LLM reflection")
	}
	// 同一回答采样结果必须确定（无随机性）。
	a := mid + "特定后缀"
	if gate.shouldRun(a) != gate.shouldRun(a) {
		t.Error("sampling must be deterministic per answer")
	}
	// 中等长度答案按约 1/10 采样：500 条里既不能为 0 也不能全中。
	runs := 0
	for i := 0; i < 500; i++ {
		s := fmt.Sprintf("问题%d 的数据分析结果：%s", i, mid)
		if gate.shouldRun(s) {
			runs++
		}
	}
	if runs < 20 || runs > 100 {
		t.Errorf("sampling should be ~1/10, got %d/500", runs)
	}
	// 长回答与负分也不再必打：同样只按采样（500 条长回答里必有未入选的）。
	longRuns := 0
	for i := 0; i < 500; i++ {
		s := fmt.Sprintf("长回答%d：%s", i, long)
		if gate.shouldRun(s) {
			longRuns++
		}
	}
	if longRuns == 0 || longRuns >= 500 {
		t.Errorf("long answers should also be sampled (~1/10), got %d/500", longRuns)
	}
	// 关闭时全跳过。
	off := newReflectionLLMGate(reflectionLLMConfig{
		Enabled: false, SampleRate: 1, DailyCap: 20, MinRunes: 0,
	})
	if off.shouldRun(long) {
		t.Error("disabled gate should never run")
	}
}

// TestReflectionLLMGateDailyCap 锁定每日上限契约：占满即静默拒绝、
// 归还名额可再用、跨天重置。
func TestReflectionLLMGateDailyCap(t *testing.T) {
	gate := newReflectionLLMGate(reflectionLLMConfig{
		Enabled: true, SampleRate: 1, DailyCap: 3, MinRunes: 0,
	})
	for i := 0; i < 3; i++ {
		if !gate.tryAcquire() {
			t.Fatalf("acquire %d should succeed", i+1)
		}
	}
	if gate.tryAcquire() {
		t.Error("4th acquire over cap 3 should silently fail")
	}
	// 归还一个名额后可再占一个。
	gate.release()
	if !gate.tryAcquire() {
		t.Error("acquire after release should succeed")
	}
	if gate.tryAcquire() {
		t.Error("over cap again should fail")
	}
	// 归还不应把计数减到负：连续归还后仍按上限工作。
	gate.release()
	gate.release()
	gate.release()
	gate.release()
	if _, used, _ := gate.stats(); used < 0 {
		t.Errorf("used must not go negative, got %d", used)
	}

	// 跨天重置：把日期拨回昨天，下一次 acquire 应从 0 开始计。
	yesterday := gate
	yesterday.mu.Lock()
	yesterday.date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	yesterday.usedToday = 3
	yesterday.mu.Unlock()
	if !yesterday.tryAcquire() {
		t.Error("acquire after date rollover should succeed")
	}
	if _, used, _ := yesterday.stats(); used != 1 {
		t.Errorf("used after rollover should be 1, got %d", used)
	}
}

// TestSelectReflectionConfig 锁定反思选卡：优先含偏好子串的模型卡
// （默认 deepseek，最便宜），无匹配时用首张带凭证卡，跳过无凭证卡。
func TestSelectReflectionConfig(t *testing.T) {
	cred := func(name string) *data.AIConfig {
		return &data.AIConfig{BaseUrl: "https://api.example.com/v1", ApiKey: "k", ModelName: name}
	}
	noCred := &data.AIConfig{BaseUrl: "", ApiKey: "", ModelName: "deepseek-chat"}

	configs := []*data.AIConfig{
		noCred,
		cred("gpt-4o"),
		cred("deepseek-chat"),
		cred("qwen-plus"),
	}
	if got := selectReflectionConfig(configs, "deepseek"); got == nil || got.ModelName != "deepseek-chat" {
		t.Errorf("should prefer deepseek card, got %+v", got)
	}
	if got := selectReflectionConfig(configs, "qwen"); got == nil || got.ModelName != "qwen-plus" {
		t.Errorf("should prefer qwen card when preference is qwen, got %+v", got)
	}
	// 无匹配偏好时用首张带凭证卡（不是无凭证的 deepseek 卡）。
	if got := selectReflectionConfig(configs, "claude"); got == nil || got.ModelName != "gpt-4o" {
		t.Errorf("no match should fall back to first credentialed card, got %+v", got)
	}
	// 偏好为空时同样用首张带凭证卡。
	if got := selectReflectionConfig(configs, ""); got == nil || got.ModelName != "gpt-4o" {
		t.Errorf("empty preference should use first credentialed card, got %+v", got)
	}
	// 全无凭证时返回 nil（调用方走确定性评分）。
	if got := selectReflectionConfig([]*data.AIConfig{noCred, nil}, "deepseek"); got != nil {
		t.Errorf("no credentialed card should return nil, got %+v", got)
	}
	// 大小写不敏感。
	if got := selectReflectionConfig(configs, "DeepSeek"); got == nil || got.ModelName != "deepseek-chat" {
		t.Errorf("preference match should be case-insensitive, got %+v", got)
	}
}
