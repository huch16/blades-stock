package tools

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"blades-stock/backend/logger"
)

// toolGroupConfigOverride maps a ToolGroup name to extra trigger keywords.
// Loaded once from env BLADES_STOCK_TOOLGROUP_CONFIG (JSON file). Keywords are
// merged (appended + deduped) onto the built-in groupKeywordsList so existing
// behavior is preserved unless explicitly extended.
type toolGroupConfigOverride struct {
	Keywords map[string][]string `json:"keywords"`
}

var (
	toolGroupCfgOnce sync.Once
)

// applyToolGroupConfig loads the optional keyword config file and merges it
// into groupKeywordsList. Safe to call multiple times; actual work happens once.
func applyToolGroupConfig() {
	toolGroupCfgOnce.Do(func() {
		path := strings.TrimSpace(os.Getenv("BLADES_STOCK_TOOLGROUP_CONFIG"))
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			logger.SugaredLogger.Warnf("读取工具组关键词配置文件失败，使用内置关键词: %v", err)
			return
		}
		var cfg toolGroupConfigOverride
		if err := json.Unmarshal(data, &cfg); err != nil {
			logger.SugaredLogger.Warnf("解析工具组关键词配置文件失败，使用内置关键词: %v", err)
			return
		}
		if len(cfg.Keywords) == 0 {
			return
		}
		byGroup := make(map[ToolGroup][]string, len(groupKeywordsList))
		for _, gk := range groupKeywordsList {
			byGroup[gk.group] = append(byGroup[gk.group], gk.keywords...)
		}
		for name, extras := range cfg.Keywords {
			group := ToolGroup(name)
			existing := byGroup[group]
			seen := make(map[string]bool, len(existing))
			for _, k := range existing {
				seen[k] = true
			}
			for _, k := range extras {
				k = strings.TrimSpace(k)
				if k != "" && !seen[k] {
					existing = append(existing, k)
					seen[k] = true
				}
			}
			byGroup[group] = existing
		}
		// Rebuild list preserving original group order.
		merged := make([]groupKeywords, 0, len(groupKeywordsList))
		for _, gk := range groupKeywordsList {
			merged = append(merged, groupKeywords{group: gk.group, keywords: byGroup[gk.group]})
		}
		// Append any brand-new groups not in the built-in list.
		builtIn := make(map[ToolGroup]bool, len(groupKeywordsList))
		for _, gk := range groupKeywordsList {
			builtIn[gk.group] = true
		}
		for g, kws := range byGroup {
			if !builtIn[g] {
				merged = append(merged, groupKeywords{group: g, keywords: kws})
			}
		}
		groupKeywordsList = merged
		logger.SugaredLogger.Infof("已从 %s 加载工具组关键词配置", path)
	})
}
