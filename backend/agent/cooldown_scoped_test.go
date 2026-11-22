package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

// TestCooldownKeyScopedByModelAndGateway 守卫冷却键按“模型名 + 网关”细分
// （2026-11-23 第三轮 C 组任务三）。
//
// 旧实现按裸模型名冷却：A 网关鉴权失败 → B 网关同名卡也被排除 30 分钟。
// 新冷却键 = 模型名 + BaseUrl + ApiKey 指纹（见 FallbackProviderName），
// 同名不同网关、同网关不同凭证互不牵连，同网关同凭证同模型正常冷却。
func TestCooldownKeyScopedByModelAndGateway(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	model := "scoped-model-" + suffix

	cfgA := data.AIConfig{ModelName: model, BaseUrl: "https://gateway-a.example.com/v1", ApiKey: "sk-gateway-a-key"}
	cfgB := data.AIConfig{ModelName: model, BaseUrl: "https://gateway-b.example.com/v1", ApiKey: "sk-gateway-a-key"} // 同名同 Key，不同网关
	cfgA2 := data.AIConfig{ModelName: model, BaseUrl: "https://gateway-a.example.com/v1", ApiKey: "sk-other-key"}    // 同名同网关，不同凭证
	cfgSame := data.AIConfig{ModelName: model, BaseUrl: "https://gateway-a.example.com/v1", ApiKey: "sk-gateway-a-key"}

	nameA := FallbackProviderName(cfgA)
	nameB := FallbackProviderName(cfgB)
	nameA2 := FallbackProviderName(cfgA2)
	nameSame := FallbackProviderName(cfgSame)
	unmarkModelCooldownForTest(t, nameA, nameB, nameA2, nameSame)

	// 内部身份名必须区分网关与凭证，展示名必须还原为旧格式且不含指纹。
	if nameA == nameB {
		t.Errorf("同名不同网关的身份名应不同，got %q == %q", nameA, nameB)
	}
	if nameA == nameA2 {
		t.Errorf("同名同网关不同凭证的身份名应不同，got %q == %q", nameA, nameA2)
	}
	if nameA != nameSame {
		t.Errorf("同一张卡的身份名应稳定，got %q != %q", nameA, nameSame)
	}
	if got := DisplayFallbackProviderName(nameA); got != model+"(https://gateway-a.example.com/v1)" {
		t.Errorf("展示名 = %q, want %q（不得含 #指纹）", got, model+"(https://gateway-a.example.com/v1)")
	}
	if got := CleanFallbackModelName(nameA); got != model {
		t.Errorf("CleanFallbackModelName = %q, want %q", got, model)
	}

	// A 网关卡冷却后：B 网关同名卡与同网关不同凭证卡均不受牵连。
	MarkModelPermanentFailure(nameA)
	if !IsModelInCooldown(nameA) || !IsModelInCooldown(nameSame) {
		t.Errorf("同一张卡冷却后 IsModelInCooldown 应为 true")
	}
	if IsModelInCooldown(nameB) {
		t.Errorf("同名不同网关的卡不应被牵连进冷却")
	}
	if IsModelInCooldown(nameA2) {
		t.Errorf("同名同网关不同凭证的卡不应被牵连进冷却")
	}
	// 其它模型同网关也不受牵连。
	other := FallbackProviderName(data.AIConfig{ModelName: "other-" + suffix, BaseUrl: "https://gateway-a.example.com/v1", ApiKey: "sk-gateway-a-key"})
	unmarkModelCooldownForTest(t, other)
	if IsModelInCooldown(other) {
		t.Errorf("不同模型的卡不应被牵连进冷却")
	}
}

// TestCooldownLegacyBareName 兼容：裸模型名（旧测试/历史调用格式）仍能自洽地
// 标记与查询冷却——网关与凭证均未知时退化为“仅模型名”键，不跨格式串扰。
func TestCooldownLegacyBareName(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	bare := "bare-model-" + suffix
	scoped := bare + "(https://gateway.example.com/v1)"
	unmarkModelCooldownForTest(t, bare, scoped)

	MarkModelPermanentFailure(bare)
	if !IsModelInCooldown(bare) {
		t.Errorf("裸模型名标记后应处于冷却期")
	}
	if IsModelInCooldown(scoped) {
		t.Errorf("裸模型名冷却不应牵连带网关的同名卡")
	}
}

// TestBuildFallbackProvidersScopedCooldown 在 DB 层验证细分冷却与既有排除
// 规则的自洽：主模型冷却后，同名不同网关的卡仍可作备用；同网关同凭证的
// 不同模型卡仍按既有规则排除；容量型 429 不进熔断（见 TestIsPermanentAuthError）。
func TestBuildFallbackProvidersScopedCooldown(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	model := "shared-model-" + suffix

	primary := data.AIConfig{
		Name: "primary", BaseUrl: "https://gateway-a.example.com/v1",
		ApiKey: "sk-primary-key", ModelName: model,
		MaxTokens: 4096, Temperature: 0.3, TimeOut: 300,
	}
	otherGateway := data.AIConfig{
		Name: "other-gateway", BaseUrl: "https://gateway-b.example.com/v1",
		ApiKey: "sk-other-gateway-key", ModelName: model,
		MaxTokens: 4096, Temperature: 0.3, TimeOut: 300,
	}
	sameGatewaySameKey := data.AIConfig{
		Name: "same-gateway-same-key", BaseUrl: "https://gateway-a.example.com/v1",
		ApiKey: "sk-primary-key", ModelName: "other-model-" + suffix,
		MaxTokens: 4096, Temperature: 0.3, TimeOut: 300,
	}
	sameGatewayOtherKey := data.AIConfig{
		Name: "same-gateway-other-key", BaseUrl: "https://gateway-a.example.com/v1",
		ApiKey: "sk-other-key", ModelName: "other-model-" + suffix,
		MaxTokens: 4096, Temperature: 0.3, TimeOut: 300,
	}

	cfgs := []*data.AIConfig{&primary, &otherGateway, &sameGatewaySameKey, &sameGatewayOtherKey}
	for _, c := range cfgs {
		if err := db.Dao.Create(c).Error; err != nil {
			t.Fatalf("建测试卡失败: %v", err)
		}
	}
	t.Cleanup(func() {
		ids := make([]uint, 0, len(cfgs))
		for _, c := range cfgs {
			ids = append(ids, c.ID)
		}
		db.Dao.Delete(&data.AIConfig{}, ids)
	})
	primaryName := FallbackProviderName(primary)
	otherName := FallbackProviderName(otherGateway)
	sameKeyName := FallbackProviderName(sameGatewaySameKey)
	otherKeyName := FallbackProviderName(sameGatewayOtherKey)
	unmarkModelCooldownForTest(t, primaryName, otherName, sameKeyName, otherKeyName)

	contains := func(names []string, want string) bool {
		for _, n := range names {
			if n == want {
				return true
			}
		}
		return false
	}
	// 共享临时库里可能已有其它测试留下的卡片，只断言本用例的卡片是否在链中，
	// 不断言链的总长度。
	// 前置：主模型未冷却时，三张非主卡都应进入备用链。
	_, names := BuildFallbackProvidersFromDB(context.Background(), primary)
	for _, want := range []string{otherName, sameKeyName, otherKeyName} {
		if !contains(names, want) {
			t.Fatalf("主模型未冷却时备用链应含 %q，got %v", want, names)
		}
	}

	// 主模型因永久鉴权失败进冷却后：
	//   - 同名不同网关卡仍可用（本次修复的核心）；
	//   - 同网关同凭证的不同模型卡仍按既有规则排除；
	//   - 同网关不同凭证的不同模型卡可用（不同凭证，不共命运）。
	MarkModelPermanentFailure(primaryName)
	_, names = BuildFallbackProvidersFromDB(context.Background(), primary)
	if !contains(names, otherName) {
		t.Errorf("主模型冷却后，同名不同网关卡仍应留在备用链，got %v", names)
	}
	if contains(names, sameKeyName) {
		t.Errorf("主模型冷却后，同网关同凭证卡应按既有规则排除，got %v", names)
	}
	if !contains(names, otherKeyName) {
		t.Errorf("主模型冷却后，同网关不同凭证卡仍应留在备用链，got %v", names)
	}
	// 备用链内部名含 #指纹（冷却键载体），展示时才还原。
	for _, n := range names {
		if n == otherName || n == otherKeyName {
			if got := DisplayFallbackProviderName(n); got == n {
				t.Errorf("备用链名 %q 应含 #指纹（内部身份名），展示时才还原", n)
			}
		}
	}
}
