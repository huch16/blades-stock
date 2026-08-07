package agent

import (
	"context"
	"sync"
	"time"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
)

var (
	marketBriefMu sync.Mutex
	marketBrief   string
	marketBriefAt time.Time
)

const marketBriefTTL = 3 * time.Minute

// getDailyMarketBrief returns a cached compact overview of today's market for
// market-related questions. Best-effort: any failure or timeout yields "" and
// the prompt is skipped rather than blocking the conversation.
func getDailyMarketBrief(question string) string {
	// 只对市场相关问题注入，控制每次请求的上下文开销。
	if !tools.ClassifyQuestion(question)[tools.GroupMarket] {
		return ""
	}

	marketBriefMu.Lock()
	defer marketBriefMu.Unlock()
	if marketBrief != "" && time.Since(marketBriefAt) < marketBriefTTL {
		return marketBrief
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		b, err := data.GetMarketBrief()
		if err != nil {
			done <- ""
			return
		}
		done <- b
	}()

	select {
	case b := <-done:
		marketBrief = b
		marketBriefAt = time.Now()
		return b
	case <-ctx.Done():
		return ""
	}
}

// toolNameList extracts tool names for metadata injection (available_tools).
func toolNameList(ts []bladesTools.Tool) []string {
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		names = append(names, t.Name())
	}
	return names
}
