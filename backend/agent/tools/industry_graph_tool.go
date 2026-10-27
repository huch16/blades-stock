package tools

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

// QuerySupplyChainCascadeTool 跨市场事件级联推演与A股映射雷达工具
func QuerySupplyChainCascadeTool() bladesTools.Tool {
	return types.NewTool(
		"QuerySupplyChainCascade",
		"基于A股产业传导知识图谱执行跨市场事件级联推演。输入催化源（如英伟达、苹果、特斯拉、ASML）或突发大事件，自动穿透下游受拉动板块与核心A股受益梯队，结合量价均线形态输出买方竞价与操盘雷达",
		map[string]*types.ParamDef{
			"source": {
				Type:     "string",
				Desc:     "催化源头或海外巨头，如 NVDA, AAPL, TSLA, ASML, 英伟达, 苹果, 特斯拉, 半导体设备",
				Required: true,
			},
			"headline": {
				Type:     "string",
				Desc:     "核心突发催化事件描述，如 'Blackwell架构全面量产交付，1.6T高速光模块与高阶服务器PCB需求爆发'",
				Required: true,
			},
		},
		func(args string) (string, error) {
			source := strings.TrimSpace(gjson.Get(args, "source").String())
			headline := strings.TrimSpace(gjson.Get(args, "headline").String())
			if source == "" {
				source = "NVDA"
			}
			if headline == "" {
				headline = "行业关键景气度催化与资本开支上修"
			}

			graph := quant.NewIndustryKnowledgeGraph()
			res, err := graph.CascadeEventReasoning(source, headline)
			if err != nil {
				return fmt.Sprintf("❌ 级联推演分析失败: %v", err), nil
			}

			return res.RadarSummary, nil
		},
	)
}

// GetIndustrySupplyGraphTool 查询产业上下游知识图谱节点与供需关系
func GetIndustrySupplyGraphTool() bladesTools.Tool {
	return types.NewTool(
		"GetIndustrySupplyGraph",
		"查询特定科技赛道或核心龙头标的的上下游供需图谱、业务暴露度与产业供货关系链",
		map[string]*types.ParamDef{
			"keyword": {
				Type:     "string",
				Desc:     "查询关键词，如 '光模块', 'PCB', '液冷', '机器人', '半导体', '英伟达', '苹果', '特斯拉'",
				Required: true,
			},
		},
		func(args string) (string, error) {
			keyword := strings.TrimSpace(gjson.Get(args, "keyword").String())
			if keyword == "" {
				return "❌ 请提供查询关键词", nil
			}

			quant.NewIndustryKnowledgeGraph().SeedDefaultGraphIfEmpty()

			if db.Dao == nil {
				return "❌ 数据库未初始化", nil
			}

			var nodes []models.IndustryNode
			db.Dao.Where("name LIKE ? OR description LIKE ? OR id LIKE ?",
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").Find(&nodes)

			if len(nodes) == 0 {
				return fmt.Sprintf("暂未在产业知识图谱中检索到关于 %q 的直接节点。建议使用 '光模块'、'PCB'、'液冷'、'机器人'、'半导体设备' 等核心赛道关键词。", keyword), nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### 🕸️ A 股产业传导知识图谱检索结果 (%q)\n\n", keyword))
			for i, n := range nodes {
				fmt.Fprintf(&sb, "%d. **%s** (`%s` | %s)\n", i+1, n.Name, n.ID, n.Category)
				if n.Description != "" {
					fmt.Fprintf(&sb, "   - 业务概况：%s\n", n.Description)
				}
				if n.StockCode != "" {
					fmt.Fprintf(&sb, "   - 标的代码：`%s` | 市值规模：%s\n", n.StockCode, n.MarketCap)
				}

				// 查询关联下游边
				var edges []models.IndustryEdge
				db.Dao.Where("source_id = ? OR target_id = ?", n.ID, n.ID).Find(&edges)
				if len(edges) > 0 {
					sb.WriteString("   - 传导关系链：\n")
					for _, e := range edges {
						fmt.Fprintf(&sb, "     • `%s` ➔ `%s` [%s, 权重: %.2f]: %s\n",
							e.SourceID, e.TargetID, e.Relation, e.Weight, e.Notes)
					}
				}
			}

			return sb.String(), nil
		},
	)
}
