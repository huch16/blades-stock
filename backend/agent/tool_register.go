package agent

import (
	"encoding/json"
	"sync"

	"blades-stock/backend/agent/tools"

	bladesTools "github.com/go-kratos/blades/tools"
)

// ToolDescriptor describes a tool's capabilities, schema, and behavior.
// This enables runtime validation, UI display, and orchestrator planning.
type ToolDescriptor struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Group        string   `json:"group"`
	Description  string   `json:"description"`
	InputSchema  string   `json:"inputSchema"`  // JSON Schema
	OutputSchema string   `json:"outputSchema"` // JSON Schema
	Idempotent   bool     `json:"idempotent"`
	SideEffects  []string `json:"sideEffects"`
	Enabled      bool     `json:"enabled"`
}

// RegisteredTool holds a tool along with its descriptor.
type RegisteredTool struct {
	Name       string
	Descriptor ToolDescriptor
	Tool       bladesTools.Tool
	Enabled    bool
}

var (
	registeredTools []RegisteredTool
	regMu           sync.RWMutex
)

// RegisterTool registers a tool with default descriptor inferred from the tool's metadata.
// Backward compatible - continues to work without explicit descriptor.
func RegisterTool(name string, tool bladesTools.Tool) {
	regMu.Lock()
	defer regMu.Unlock()

	for i, rt := range registeredTools {
		if rt.Name == name {
			registeredTools[i].Tool = tool
			registeredTools[i].Enabled = true
			// Update descriptor if we can infer more
			if registeredTools[i].Descriptor.Name == "" {
				registeredTools[i].Descriptor = inferDescriptor(name, tool)
			}
			return
		}
	}
	registeredTools = append(registeredTools, RegisteredTool{
		Name:       name,
		Descriptor: inferDescriptor(name, tool),
		Tool:       tool,
		Enabled:    true,
	})
}

// RegisterToolWithDescriptor registers a tool with an explicit descriptor.
// Use this when you need full control over the tool's metadata.
func RegisterToolWithDescriptor(desc ToolDescriptor, tool bladesTools.Tool) {
	regMu.Lock()
	defer regMu.Unlock()

	for i, rt := range registeredTools {
		if rt.Name == desc.Name {
			registeredTools[i].Tool = tool
			registeredTools[i].Descriptor = desc
			registeredTools[i].Enabled = true
			return
		}
	}
	registeredTools = append(registeredTools, RegisteredTool{
		Name:       desc.Name,
		Descriptor: desc,
		Tool:       tool,
		Enabled:    true,
	})
}

// inferDescriptor attempts to extract a basic descriptor from a tool.
// Since bladesTools.Tool is an opaque interface, we use known tool names.
func inferDescriptor(name string, _ bladesTools.Tool) ToolDescriptor {
	// Default descriptors for known built-in tools
	descriptors := map[string]ToolDescriptor{
		"WebSearch": {
			ID:          "web_search",
			Name:        "WebSearch",
			Group:       "search",
			Description: "SearXNG/百度 通用互联网搜索工具。优先使用 SearXNG JSON API 搜索，仅当 SearXNG 和百度 HTTP 均无结果时回退到浏览器。",
			InputSchema: `{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词，建议简洁且包含关键限定词"},"count":{"type":"integer","description":"返回结果条数，默认5，最大20"}},"required":["query"]}`,
			OutputSchema: `{"type":"string"}`,
			Idempotent:   true,
			SideEffects:  []string{"network"},
			Enabled:      true,
		},
		"FetchWebPage": {
			ID:          "fetch_webpage",
			Name:        "FetchWebPage",
			Group:       "search",
			Description: "抓取指定 URL 的网页内容并转为 Markdown。用于获取搜索结果链接的详细内容。",
			InputSchema:  `{"type":"object","properties":{"url":{"type":"string","description":"目标网页 URL"}}, "required":["url"]}`,
			OutputSchema: `{"type":"string"}`,
			Idempotent:   true,
			SideEffects:  []string{"network"},
			Enabled:      true,
		},
		"LearnSkillFromProject": {
			ID:          "learn_skill",
			Name:        "LearnSkillFromProject",
			Group:       "skill",
			Description: "从 GitHub 仓库或本地路径学习项目结构并生成 Skill。",
			InputSchema:  `{"type":"object","properties":{"source_url":{"type":"string","description":"Git 克隆地址或本地路径"}}, "required":["source_url"]}`,
			OutputSchema: `{"type":"string"}`,
			Idempotent:   false,
			SideEffects:  []string{"network", "filesystem", "database"},
			Enabled:      true,
		},
	}

	if desc, ok := descriptors[name]; ok {
		return desc
	}

	// Fallback for unknown tools
	return ToolDescriptor{
		ID:          name,
		Name:        name,
		Group:       "unknown",
		Description: "Tool: " + name,
		InputSchema:  "{}",
		OutputSchema: "{}",
		Idempotent:   false,
		SideEffects:  []string{},
		Enabled:      true,
	}
}

// GetToolRegistry returns all registered tools with their descriptors.
// Thread-safe copy for external use.
func GetToolRegistry() []RegisteredTool {
	regMu.RLock()
	defer regMu.RUnlock()

	out := make([]RegisteredTool, len(registeredTools))
	copy(out, registeredTools)
	return out
}

// GetEnabledTools returns only enabled tools (for agent binding).
func GetEnabledTools() []bladesTools.Tool {
	regMu.RLock()
	defer regMu.RUnlock()

	var out []bladesTools.Tool
	for _, rt := range registeredTools {
		if rt.Enabled {
			out = append(out, rt.Tool)
		}
	}
	return out
}

// GetToolDescriptor returns the descriptor for a specific tool by name.
func GetToolDescriptor(name string) (ToolDescriptor, bool) {
	regMu.RLock()
	defer regMu.RUnlock()

	for _, rt := range registeredTools {
		if rt.Name == name {
			return rt.Descriptor, true
		}
	}
	return ToolDescriptor{}, false
}

// HasTool checks if a tool is registered (regardless of enabled state).
func HasTool(name string) bool {
	regMu.RLock()
	defer regMu.RUnlock()

	for _, rt := range registeredTools {
		if rt.Name == name {
			return true
		}
	}
	return false
}

// EnableTool / DisableTool control tool availability at runtime.
func EnableTool(name string) bool {
	regMu.Lock()
	defer regMu.Unlock()

	for i := range registeredTools {
		if registeredTools[i].Name == name {
			registeredTools[i].Enabled = true
			registeredTools[i].Descriptor.Enabled = true
			return true
		}
	}
	return false
}

func DisableTool(name string) bool {
	regMu.Lock()
	defer regMu.Unlock()

	for i := range registeredTools {
		if registeredTools[i].Name == name {
			registeredTools[i].Enabled = false
			registeredTools[i].Descriptor.Enabled = false
			return true
		}
	}
	return false
}

// ToolRegistryJSON returns the full registry as JSON for API/UI consumption.
func ToolRegistryJSON() (string, error) {
	registry := GetToolRegistry()
	// Only expose descriptors, not the internal tool references
	type publicDescriptor struct {
		ToolDescriptor
	}
	descs := make([]publicDescriptor, len(registry))
	for i, rt := range registry {
		descs[i] = publicDescriptor{rt.Descriptor}
	}
	b, err := json.MarshalIndent(descs, "", "  ")
	return string(b), err
}

func init() {
	// Register built-in tools with explicit descriptors for full metadata
	RegisterToolWithDescriptor(ToolDescriptor{
		ID:          "web_search",
		Name:        "WebSearch",
		Group:       "search",
		Description: "SearXNG/百度 通用互联网搜索工具。优先使用 SearXNG JSON API 搜索，仅当 SearXNG 和百度 HTTP 均无结果时回退到浏览器。当用户需要查询实时信息、无法通过本地股票接口直接获得的列表/地域/新闻/非结构化数据时调用。例如：总部位于某城市的上市公司、最新热点、实时资讯等。优先调用此工具而不是直接回答无法查询。",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词，建议简洁且包含关键限定词"},"count":{"type":"integer","description":"返回结果条数，默认5，最大20"}},"required":["query"]}`,
		OutputSchema: `{"type":"string"}`,
		Idempotent:   true,
		SideEffects:  []string{"network"},
		Enabled:      true,
	}, tools.GetWebSearchTool())

	RegisterToolWithDescriptor(ToolDescriptor{
		ID:          "fetch_webpage",
		Name:        "FetchWebPage",
		Group:       "search",
		Description: "抓取指定 URL 的网页内容并转为 Markdown。用于获取搜索结果链接的详细内容。",
		InputSchema:  `{"type":"object","properties":{"url":{"type":"string","description":"目标网页 URL"}}, "required":["url"]}`,
		OutputSchema: `{"type":"string"}`,
		Idempotent:   true,
		SideEffects:  []string{"network"},
		Enabled:      true,
	}, tools.GetFetchWebPageTool())

	RegisterToolWithDescriptor(ToolDescriptor{
		ID:          "learn_skill",
		Name:        "LearnSkillFromProject",
		Group:       "skill",
		Description: "从 GitHub 仓库或本地路径学习项目结构并生成 Skill。",
		InputSchema:  `{"type":"object","properties":{"source_url":{"type":"string","description":"Git 克隆地址或本地路径"}}, "required":["source_url"]}`,
		OutputSchema: `{"type":"string"}`,
		Idempotent:   false,
		SideEffects:  []string{"network", "filesystem", "database"},
		Enabled:      true,
	}, tools.GetLearnSkillTool())
}