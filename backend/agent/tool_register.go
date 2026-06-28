package agent

import (
	"go-stock/backend/agent/tools"

	bladesTools "github.com/go-kratos/blades/tools"
)

var registeredTools []RegisteredTool

type RegisteredTool struct {
	Name    string
	Tool    bladesTools.Tool
	Enabled bool
}

func RegisterTool(name string, tool bladesTools.Tool) {
	for i, rt := range registeredTools {
		if rt.Name == name {
			registeredTools[i].Tool = tool
			registeredTools[i].Enabled = true
			return
		}
	}
	registeredTools = append(registeredTools, RegisteredTool{
		Name:    name,
		Tool:    tool,
		Enabled: true,
	})
}

func GetRegisteredTools() []RegisteredTool {
	out := make([]RegisteredTool, len(registeredTools))
	copy(out, registeredTools)
	return out
}

func GetEnabledTools() []bladesTools.Tool {
	var out []bladesTools.Tool
	for _, rt := range registeredTools {
		if rt.Enabled {
			out = append(out, rt.Tool)
		}
	}
	return out
}

func init() {
	RegisterTool("WebSearch", tools.GetWebSearchTool())
}
