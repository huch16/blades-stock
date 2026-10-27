package tools

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/quant"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

// RunAutoAlphaExplorationTool 触发策略参数网格优化与自发现
func RunAutoAlphaExplorationTool() bladesTools.Tool {
	return types.NewTool(
		"RunAutoAlphaExploration",
		"触发量化策略（如ETF动量轮动）参数空间网格遍历搜索与因子自进化探索，基于历史多周期K线寻找最优胜率与夏普比率参数组",
		map[string]*types.ParamDef{
			"targetPool": {
				Type:     "string",
				Desc:     "优化目标ETF池代码（逗号分隔），如 '512480, 159915, 515050, 588000'，留空则默认科技成长核心池",
				Required: false,
			},
		},
		func(args string) (string, error) {
			targetPoolRaw := strings.TrimSpace(gjson.Get(args, "targetPool").String())
			var codes []string
			if targetPoolRaw != "" {
				parts := strings.Split(targetPoolRaw, ",")
				for _, p := range parts {
					c := strings.TrimSpace(p)
					if c != "" {
						codes = append(codes, c)
					}
				}
			}

			explorer := quant.NewAutoAlphaExplorer()
			proposal, err := explorer.ExploreETFMomentumParams(codes)
			if err != nil {
				return fmt.Sprintf("❌ 执行量化因子与参数优化失败: %v", err), nil
			}

			var sb strings.Builder
			sb.WriteString("### 🧬 量化策略自进化与参数网格优化提案\n\n")
			fmt.Fprintf(&sb, "- **策略类型**：%s\n", proposal.StrategyName)
			fmt.Fprintf(&sb, "- **目标资产池**：%s\n", proposal.TargetAsset)
			fmt.Fprintf(&sb, "- **基准夏普比率 (Sharpe)**：`%.2f` ➔ **优化后夏普**：`%.2f`\n",
				proposal.OriginalSharpe, proposal.OptimizedSharpe)
			fmt.Fprintf(&sb, "- **预期年化收益率**：`%+.2f%%` | **最大回撤收敛**：`%.2f%%`\n",
				proposal.ExpectedAnnualReturn, proposal.MaxDrawdownReduction)
			fmt.Fprintf(&sb, "- **回测历史胜率**：`%.1f%%` | **提案状态**：`%s`\n\n",
				proposal.WinRate, proposal.Status)
			fmt.Fprintf(&sb, "- **基准参数**：`%s`\n", proposal.OldParams)
			fmt.Fprintf(&sb, "- **最优推荐参数**：`%s`\n\n", proposal.OptimalParams)
			sb.WriteString("#### 📝 量化评估与鲁棒性说明\n")
			sb.WriteString(proposal.EvaluationSummary)
			sb.WriteString("\n\n*(注：本提案已保存至系统量化策略库，用户可在策略管理中一键采纳生效)*\n")

			return sb.String(), nil
		},
	)
}

// GetAutoAlphaProposalsTool 获取历史策略自进化提案
func GetAutoAlphaProposalsTool() bladesTools.Tool {
	return types.NewTool(
		"GetAutoAlphaProposals",
		"获取系统夜间自动探索产出的量化策略优化提案列表，包含参数对比、夏普改善度与回测评估",
		map[string]*types.ParamDef{
			"limit": {
				Type:     "integer",
				Desc:     "查询条数，默认 5，上限 20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 5
			}

			explorer := quant.NewAutoAlphaExplorer()
			list, err := explorer.GetActiveProposals(limit)
			if err != nil {
				return fmt.Sprintf("❌ 获取策略自进化提案失败: %v", err), nil
			}

			if len(list) == 0 {
				return "暂无已保存的量化策略自进化提案。可调用 RunAutoAlphaExploration 立即启动一次探索。", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### 📜 系统最近 %d 项策略自进化与参数优化提案\n\n", len(list)))
			for i, p := range list {
				fmt.Fprintf(&sb, "%d. **[%s] %s** (ID: %d, 生成时间: %s)\n",
					i+1, p.Status, p.StrategyName, p.ID, p.CreatedAt.Format("2006-01-02 15:04"))
				fmt.Fprintf(&sb, "   - 标的池: %s\n", p.TargetAsset)
				fmt.Fprintf(&sb, "   - 夏普提升: `%.2f` ➔ `%.2f` | 年化收益: `%+.2f%%` | 最大回撤改善: `%.2f%%`\n",
					p.OriginalSharpe, p.OptimizedSharpe, p.ExpectedAnnualReturn, p.MaxDrawdownReduction)
				fmt.Fprintf(&sb, "   - 最优参数: `%s`\n", p.OptimalParams)
			}

			return sb.String(), nil
		},
	)
}
