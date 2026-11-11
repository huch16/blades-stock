package agent

import (
	"strings"
	"testing"

	"blades-stock/backend/data"
)

// TestEmptyOutputUserMessage 锁定「静默空答」用户文案的核心契约。
//
// 回归背景：生产环境曾出现「同一张 AI 卡片、同一个问题，有时返回 2000+ 字符，
// 有时跑满 55 秒却一个字不给」，而后端记为 ok=true/finish=stop，前端只渲染
// 无信息量的「（空回复）」。用户完全无法判断是软件坏了、网络不通还是模型问题。
//
// 因此这里断言的不只是"有文案"，而是：
//  1. 三种归因各自给出**不同**的说明（用户能区分原因）
//  2. 每条都给出**可执行**的操作（动哪个开关），而不是"请重试"
//  3. 绝不退化成「（空回复）」
func TestEmptyOutputUserMessage(t *testing.T) {
	const cfgID = 9

	cases := []struct {
		name        string
		err         data.EmptyOutputError
		wantReason  string   // 必须出现的归因关键词
		wantActions []string // 必须出现的可执行指引关键词
	}{
		{
			name:       "过程有内容但无结论（思考/工具吃掉了预算）",
			err:        data.EmptyOutputError{ProcessChars: 1200, FirstChunkSeen: true},
			wantReason: "始终没有写出最终答案",
			wantActions: []string{
				"深度思考",      // 关闭 thinking
				"maxTokens", // 或调大输出上限
			},
		},
		{
			name:       "上游完全没返回内容",
			err:        data.EmptyOutputError{ProcessChars: 0, FirstChunkSeen: false},
			wantReason: "没有返回任何内容",
			wantActions: []string{
				"检测连通性",
			},
		},
		{
			name:       "返回了响应但正文为空",
			err:        data.EmptyOutputError{ProcessChars: 0, FirstChunkSeen: true},
			wantReason: "正文为空",
			wantActions: []string{
				"检测连通性",
			},
		},
	}

	reasons := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := emptyOutputUserMessage(c.err, cfgID)

			// 1. 归因必须说清楚本次到底发生了什么
			if !strings.Contains(got, c.wantReason) {
				t.Errorf("归因缺失：期望包含 %q\n实际输出：\n%s", c.wantReason, got)
			}

			// 2. 必须给出可执行的操作，而不是空洞的"请重试"
			for _, a := range c.wantActions {
				if !strings.Contains(got, a) {
					t.Errorf("可执行指引缺失：期望包含 %q\n实际输出：\n%s", a, got)
				}
			}

			// 3. 必须带上本次使用的配置 ID，便于用户去设置页定位卡片
			if !strings.Contains(got, "AI 配置 ID") {
				t.Errorf("缺少 AI 配置 ID 提示\n实际输出：\n%s", got)
			}

			// 4. 绝不能是原来那句无信息量的占位文案
			if strings.Contains(got, "（空回复）") {
				t.Errorf("退化为无信息量占位文案：\n%s", got)
			}

			// 5. 三种归因的说明必须互不相同，否则用户无法区分原因
			if reasons[c.wantReason] {
				t.Errorf("归因文案重复：%q", c.wantReason)
			}
			reasons[c.wantReason] = true
		})
	}
}

// TestEmptyOutputErrorMessageIsActionable 反向护栏：
// 以前"空答"根本没有错误类型，错误文案也无从谈起。这里确保
// EmptyOutputError.Error() 自身就携带归因，使运行记录（「最近运行记录」面板）
// 单看 err 字段也能定位原因，而不必回捞日志。
func TestEmptyOutputErrorMessageIsActionable(t *testing.T) {
	if got := (data.EmptyOutputError{ProcessChars: 50}).Error(); !strings.Contains(got, "思考/工具过程") {
		t.Errorf("Error() 应指明有过程无结论，实际 %q", got)
	}
	if got := (data.EmptyOutputError{FirstChunkSeen: false}).Error(); !strings.Contains(got, "上游未返回任何响应内容") {
		t.Errorf("Error() 应指明上游未返回内容，实际 %q", got)
	}
	if got := (data.EmptyOutputError{FirstChunkSeen: true}).Error(); !strings.Contains(got, "空响应") {
		t.Errorf("Error() 应指明空响应，实际 %q", got)
	}
}
