package tools

import (
	"context"
	"sort"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	bladesTools "github.com/go-kratos/blades/tools"
)

// TestCapabilityRegistry_DeclareAndQuery 锁定能力声明与查询的核心契约。
func TestCapabilityRegistry_DeclareAndQuery(t *testing.T) {
	// 使用一个不会与内置工具冲突的独立工具名，避免污染全局 registry
	testID := "TestCapability_RegistryProbe"

	// 清理：测试结束后移除探针，恢复原始状态
	defer func() {
		capabilityMu.Lock()
		delete(capabilities, testID)
		capabilityMu.Unlock()
	}()

	DeclareCapabilities(Capability{
		ID:     testID,
		Group:  GroupMarket,
		Tier:   TierCore,
		Source: SourceBuiltin,
	})

	c, ok := CapabilityOf(testID)
	if !ok {
		t.Fatalf("CapabilityOf(%q) 应返回 true", testID)
	}
	if c.ID != testID {
		t.Errorf("CapabilityOf ID = %q, want %q", c.ID, testID)
	}
	if c.Group != GroupMarket {
		t.Errorf("CapabilityOf Group = %v, want %v", c.Group, GroupMarket)
	}
	if c.Tier != TierCore {
		t.Errorf("CapabilityOf Tier = %v, want %v", c.Tier, TierCore)
	}
	if c.Source != SourceBuiltin {
		t.Errorf("CapabilityOf Source = %v, want %v", c.Source, SourceBuiltin)
	}
}

// TestCapabilityRegistry_AllCapabilitiesSorted 锁定 AllCapabilities 按 ID 排序。
func TestCapabilityRegistry_AllCapabilitiesSorted(t *testing.T) {
	all := AllCapabilities()
	if len(all) == 0 {
		t.Fatal("AllCapabilities 不应为空")
	}
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = c.ID
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for i := range ids {
		if ids[i] != sorted[i] {
			t.Errorf("AllCapabilities 第 %d 项未排序: %q vs %q", i, ids[i], sorted[i])
		}
	}
}

// TestCapabilityRegistry_BuiltinPriorityOverMCP 锁定内置声明优先于外部 MCP 动态登记。
func TestCapabilityRegistry_BuiltinPriorityOverMCP(t *testing.T) {
	testID := "TestCapability_PriorityProbe"
	defer func() {
		capabilityMu.Lock()
		delete(capabilities, testID)
		capabilityMu.Unlock()
	}()

	// 先登记内置
	DeclareCapabilities(Capability{ID: testID, Group: GroupMarket, Tier: TierCore, Source: SourceBuiltin})
	// 再登记同名 MCP，应被忽略
	DeclareCapabilities(Capability{ID: testID, Group: GroupStockAnalysis, Tier: TierNiche, Source: SourceMCP})

	c, ok := CapabilityOf(testID)
	if !ok {
		t.Fatalf("CapabilityOf(%q) 应返回 true", testID)
	}
	if c.Source != SourceBuiltin {
		t.Errorf("Source 应保留为 SourceBuiltin，实际 %v", c.Source)
	}
	if c.Group != GroupMarket {
		t.Errorf("Group 应保留为 GroupMarket，实际 %v", c.Group)
	}
}

// TestCapabilityRegistry_UndeclaredBuiltinTools 锁定“已组装但未登记”探测。
func TestCapabilityRegistry_UndeclaredBuiltinTools(t *testing.T) {
	// 构造一个假装已组装但未注册的工具
	probeTool := &fakeTool{name: "TestCapability_UndeclaredProbe"}
	tools := []bladesTools.Tool{probeTool}

	undeclared := UndeclaredBuiltinTools(tools)
	if len(undeclared) != 1 || undeclared[0] != "TestCapability_UndeclaredProbe" {
		t.Errorf("UndeclaredBuiltinTools = %v, want [TestCapability_UndeclaredProbe]", undeclared)
	}

	// 已登记的工具不应出现在未登记列表中
	DeclareCapabilities(Capability{ID: "TestCapability_UndeclaredProbe", Group: GroupMarket, Tier: TierCore, Source: SourceBuiltin})
	defer func() {
		capabilityMu.Lock()
		delete(capabilities, "TestCapability_UndeclaredProbe")
		capabilityMu.Unlock()
	}()

	undeclared = UndeclaredBuiltinTools(tools)
	if len(undeclared) != 0 {
		t.Errorf("UndeclaredBuiltinTools 应为空，实际 %v", undeclared)
	}
}

// fakeTool 实现 bladesTools.Tool 接口，仅用于测试
type fakeTool struct {
	name string
}

func (f *fakeTool) Name() string                     { return f.name }
func (f *fakeTool) Description() string              { return "fake tool for test" }
func (f *fakeTool) InputSchema() *jsonschema.Schema  { return nil }
func (f *fakeTool) OutputSchema() *jsonschema.Schema { return nil }
func (f *fakeTool) Handle(ctx context.Context, input string) (string, error) {
	return "fake", nil
}

// TestCapabilityRegistry_TierString 锁定 ToolTier 的字符串表示。
func TestCapabilityRegistry_TierString(t *testing.T) {
	cases := []struct {
		t    ToolTier
		want string
	}{
		{TierCore, "core"},
		{TierExtended, "extended"},
		{TierNiche, "niche"},
		{ToolTier(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.t.String(); got != c.want {
			t.Errorf("ToolTier(%d).String() = %q, want %q", c.t, got, c.want)
		}
	}
}

// TestCapabilityRegistry_SourceString 锁定 ToolSource 的字符串表示。
func TestCapabilityRegistry_SourceString(t *testing.T) {
	cases := []struct {
		s    ToolSource
		want string
	}{
		{SourceBuiltin, "builtin"},
		{SourceMCP, "mcp"},
		{ToolSource(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("ToolSource(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// TestCapabilityRegistry_InitRegistersBuiltins 锁定 init() 阶段会注册全部内置工具。
func TestCapabilityRegistry_InitRegistersBuiltins(t *testing.T) {
	// builtinToolGroups 中的每个工具名都应能被 CapabilityOf 查到
	all := AllCapabilities()
	registered := make(map[string]bool, len(all))
	for _, c := range all {
		registered[c.ID] = true
	}

	for name := range builtinToolGroups {
		if !registered[name] {
			t.Errorf("init() 未注册内置工具: %s", name)
		}
	}
}

// TestCapabilityRegistry_EmptyDeclare 是幂等边界：空参数不应 panic。
func TestCapabilityRegistry_EmptyDeclare(t *testing.T) {
	DeclareCapabilities() // 不应 panic
}

// TestCapabilityRegistry_EmptyIDSkip 锁定空 ID 被跳过。
func TestCapabilityRegistry_EmptyIDSkip(t *testing.T) {
	before := len(AllCapabilities())
	DeclareCapabilities(Capability{ID: "", Group: GroupMarket, Tier: TierCore, Source: SourceBuiltin})
	after := len(AllCapabilities())
	if before != after {
		t.Errorf("空 ID 的能力不应被登记，数量从 %d 变为 %d", before, after)
	}
}
