package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// result is the per-tool verdict.
//   - NOPERM    确定无权限 (40203)
//   - BADAPI    接口不存在/已下线 (40101)
//   - VERIFIED  空参+最小参数均通过，实测返回数据
//   - OKARGS    通过权限检查，但最小参数仍参数错误（需人工补参数确认）
//   - RATELIMIT 通过权限检查，仅触发频率/积分限流
//   - TIMEOUT   重试仍超时，需人工确认
type result struct {
	name   string
	status string
	msg    string
}

func main() {
	out := report{}
	out.add("== Tushare 权限实测 ==")
	out.add("time: " + time.Now().Format("2006-01-02 15:04:05"))

	db.Init("")
	var server models.MCPServer
	if err := db.Dao.Where("name = ?", "Tushare官方MCP").First(&server).Error; err != nil {
		fmt.Println("server not found:", err)
		os.Exit(1)
	}
	out.add(fmt.Sprintf("server id=%d name=%s", server.ID, server.Name))

	cli, err := client.NewStreamableHttpClient(server.URL)
	if err != nil {
		fmt.Println("new client err:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "blades-whitelist-probe", Version: "1.0.0"},
		},
	}); err != nil {
		fmt.Println("init err:", err)
		os.Exit(1)
	}
	tl, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		fmt.Println("list err:", err)
		os.Exit(1)
	}
	out.add(fmt.Sprintf("total tools: %d", len(tl.Tools)))

	// per-tool throttle to respect Tushare rate limits (global throttling observed: keep it serial)
	sem := make(chan struct{}, 1)
	var mu sync.Mutex
	var results []result
	var wg sync.WaitGroup

	callOnce := func(name string, args map[string]any, timeout time.Duration) (string, bool, error) {
		cctx, ccancel := context.WithTimeout(context.Background(), timeout)
		defer ccancel()
		res, err := cli.CallTool(cctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: name, Arguments: args},
		})
		if err != nil {
			return "", false, err
		}
		var texts []string
		for _, c := range res.Content {
			if txt, ok := c.(mcp.TextContent); ok {
				texts = append(texts, txt.Text)
			}
		}
		return strings.Join(texts, "\n"), res.IsError, nil
	}

	callWithRetry := func(name string, args map[string]any) (string, bool) {
		var msg string
		var isErr bool
		var lastErr error
		for i := 0; i < 4; i++ {
			var err error
			msg, isErr, err = callOnce(name, args, 25*time.Second)
			if err == nil {
				return msg, isErr
			}
			lastErr = err
			time.Sleep(500 * time.Millisecond)
		}
		if lastErr != nil {
			return "TIMEOUT:" + lastErr.Error(), true
		}
		return "TIMEOUT:no content", true
	}

	for _, t := range tl.Tools {
		wg.Add(1)
		sem <- struct{}{}
		go func(t mcp.Tool) {
			defer wg.Done()
			defer func() { <-sem }()
			v := probeTool(cli, t, callWithRetry)
			mu.Lock()
			results = append(results, v)
			mu.Unlock()
			time.Sleep(300 * time.Millisecond)
		}(t)
	}
	wg.Wait()

	// Recovery pass: transport throttling can starve some tools; wait for the
	// rate window to clear, then re-probe TIMEOUT tools sequentially.
	var timeouts []string
	for _, r := range results {
		if r.status == "TIMEOUT" {
			timeouts = append(timeouts, r.name)
		}
	}
	if len(timeouts) > 0 {
		sort.Strings(timeouts)
		out.add(fmt.Sprintf("recovery pass: %d TIMEOUT tools, waiting 60s for rate window...", len(timeouts)))
		time.Sleep(60 * time.Second)
		remaining := results[:0]
		for _, r := range results {
			if r.status == "TIMEOUT" {
				v := probeTool(cli, mcp.Tool{Name: r.name}, callWithRetry)
				remaining = append(remaining, v)
				time.Sleep(300 * time.Millisecond)
			} else {
				remaining = append(remaining, r)
			}
		}
		results = remaining
	}

	sort.Slice(results, func(i, j int) bool {
		rank := map[string]int{"VERIFIED": 0, "OKARGS": 1, "RATELIMIT": 2, "TIMEOUT": 3, "BADAPI": 4, "NOPERM": 5}
		if rank[results[i].status] != rank[results[j].status] {
			return rank[results[i].status] < rank[results[j].status]
		}
		return results[i].name < results[j].name
	})

	groups := map[string][]result{}
	for _, r := range results {
		groups[r.status] = append(groups[r.status], r)
	}

	out.add("")
	out.add("=== [实测可用] VERIFIED（空参+最小参数均通过，返回真实数据）===")
	for _, r := range groups["VERIFIED"] {
		out.add(fmt.Sprintf("  %s\t%s", r.name, r.msg))
	}
	out.add("")
	out.add("=== [有权限·参数未确认] OKARGS ===")
	for _, r := range groups["OKARGS"] {
		out.add(fmt.Sprintf("  %s\t%s", r.name, r.msg))
	}
	out.add("")
	out.add("=== [有权限·限流] RATELIMIT ===")
	for _, r := range groups["RATELIMIT"] {
		out.add(fmt.Sprintf("  %s\t%s", r.name, r.msg))
	}
	out.add("")
	out.add("=== [超时·需人工确认] TIMEOUT ===")
	for _, r := range groups["TIMEOUT"] {
		out.add(fmt.Sprintf("  %s\t%s", r.name, r.msg))
	}
	out.add("")
	out.add("=== [接口不存在/已下线] BADAPI ===")
	for _, r := range groups["BADAPI"] {
		out.add(fmt.Sprintf("  %s", r.name))
	}
	out.add("")
	out.add("=== [确定无权限] NOPERM ===")
	for _, r := range groups["NOPERM"] {
		out.add(fmt.Sprintf("  %s", r.name))
	}

	out.add("")
	out.add("=== 统计 ===")
	totals := map[string]int{}
	for _, r := range results {
		totals[r.status]++
	}
	for _, k := range []string{"VERIFIED", "OKARGS", "RATELIMIT", "TIMEOUT", "BADAPI", "NOPERM"} {
		out.add(fmt.Sprintf("  %s: %d", k, totals[k]))
	}
	perm := totals["VERIFIED"] + totals["OKARGS"] + totals["RATELIMIT"]
	out.add(fmt.Sprintf("  有权限合计(VERIFIED+OKARGS+RATELIMIT): %d", perm))

	// Reset server tools to disabled, then enable only definitive permissions.
	// TIMEOUT stays disabled (conservative). This rebuilds a consistent state
	// regardless of previous runs.
	if err := db.Dao.Model(&models.MCPServerTool{}).
		Where("mcp_server_id = ?", server.ID).Update("enabled", false).Error; err != nil {
		out.add(fmt.Sprintf("ERR reset enabled: %s", err))
	}
	changed := 0
	for _, r := range results {
		if r.status == "TIMEOUT" {
			continue
		}
		ok := r.status != "NOPERM" && r.status != "BADAPI"
		var cur models.MCPServerTool
		if err := db.Dao.Where("mcp_server_id = ? AND tool_name = ?", server.ID, r.name).First(&cur).Error; err == nil {
			if cur.Enabled != ok {
				if err := db.Dao.Model(&models.MCPServerTool{}).
					Where("id = ?", cur.ID).Update("enabled", ok).Error; err != nil {
					out.add(fmt.Sprintf("ERR update %s: %s", r.name, err))
				} else {
					changed++
				}
			}
		}
	}
	out.add(fmt.Sprintf("DB enabled 重建: %d 条变更（TIMEOUT 保持禁用）", changed))

	if err := os.WriteFile("tushare_whitelist.txt", []byte(out.s), 0o644); err != nil {
		fmt.Println("write report err:", err)
	}
	fmt.Println("report written to tushare_whitelist.txt")
}

// probeTool performs: empty-args probe (decisive for 40203/40101), then minimal-args probe.
func probeTool(cli *client.Client, t mcp.Tool, call func(string, map[string]any) (string, bool)) result {
	// Phase 1: empty args
	msg, _ := call(t.Name, map[string]any{})
	if st := classify(msg); st == "NOPERM" || st == "BADAPI" || st == "RATELIMIT" || st == "TIMEOUT" {
		return result{name: t.Name, status: st, msg: truncate(msg, 120)}
	}
	// Phase 2: minimal args from schema
	args := buildArgs(t)
	msg2, _ := call(t.Name, args)
	if st := classify(msg2); st == "NOPERM" || st == "BADAPI" || st == "RATELIMIT" || st == "TIMEOUT" {
		return result{name: t.Name, status: st, msg: truncate(msg2, 120)}
	}
	if isOkData(msg2) {
		return result{name: t.Name, status: "VERIFIED", msg: truncate(msg2, 120)}
	}
	if isOkData(msg) {
		return result{name: t.Name, status: "VERIFIED", msg: truncate(msg, 120)}
	}
	return result{name: t.Name, status: "OKARGS", msg: truncate(firstNonEmpty(msg, msg2), 120)}
}

func classify(msg string) string {
	if msg == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(msg, "TIMEOUT:"):
		return "TIMEOUT"
	case strings.Contains(msg, "频率超限") || strings.Contains(msg, "每分钟") ||
		strings.Contains(msg, "积分不足") || strings.Contains(msg, "次数超限"):
		return "RATELIMIT"
	case strings.Contains(msg, "没有接口(") || strings.Contains(msg, "没有访问该接口的权限") ||
		strings.Contains(msg, "无权限") || strings.Contains(msg, "该接口需要"):
		return "NOPERM"
	case strings.Contains(msg, "[40101]") || strings.Contains(msg, "接口不存在") ||
		strings.Contains(msg, "已下线") || strings.Contains(msg, "接口已下线"):
		return "BADAPI"
	case strings.Contains(msg, "Input validation error") || strings.Contains(msg, "[50101]") ||
		strings.Contains(msg, "参数校验失败") || strings.Contains(msg, "至少输入") ||
		strings.Contains(msg, "至少填写") || strings.Contains(msg, "不能都为空") ||
		strings.Contains(msg, "必选其一"):
		return "OKARGS"
	}
	return ""
}

func isOkData(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	return trimmed != "[]" && trimmed != "[] "
}

func buildArgs(t mcp.Tool) map[string]any {
	args := map[string]any{}
	raw, err := json.Marshal(t.InputSchema)
	if err != nil {
		return args
	}
	schema := map[string]any{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return args
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return args
	}
	required, _ := schema["required"].([]any)
	needed := map[string]bool{}
	for _, r := range required {
		if s, ok := r.(string); ok {
			needed[s] = true
		}
	}
	lastDay := lastWeekday("20060102")
	samples := map[string]string{
		"ts_code":    "000001.SZ",
		"trade_date": lastDay,
		"ann_date":   lastDay,
		"start_date": "20260701",
		"end_date":   lastDay,
		"date":       lastDay,
		"period":     "20241231",
		"month":      "202412",
		"ticker":     "000001",
		"symbol":     "000001",
		"code":       "000001",
		"index_code": "000001.SH",
		"fund_code":  "510300.SH",
		"limit":      "5",
	}
	for name := range props {
		if !needed[name] {
			continue
		}
		if v, ok := samples[strings.ToLower(name)]; ok {
			args[name] = v
			continue
		}
		ptype, _ := props[name].(map[string]any)["type"].(string)
		if ptype == "integer" {
			args[name] = 5
		} else if ptype == "number" {
			args[name] = 5
		}
	}
	return args
}

func lastWeekday(layout string) string {
	d := time.Now()
	for {
		if w := d.Weekday(); w != time.Saturday && w != time.Sunday {
			return d.Format(layout)
		}
		d = d.AddDate(0, 0, -1)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func err2str(msg string) string {
	if msg == "" {
		return "no content"
	}
	return msg
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

type report struct {
	s string
}

func (r *report) add(line string) {
	r.s += line + "\n"
	fmt.Println(line)
}
