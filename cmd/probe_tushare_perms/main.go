package main

import (
	"context"
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

type verdict struct {
	name   string
	status string // "OK" | "NOPERM" | "BADAPI" | "TIMEOUT"
	msg    string
}

func main() {
	db.Init("")

	var server models.MCPServer
	if err := db.Dao.Where("name = ?", "Tushare官方MCP").First(&server).Error; err != nil {
		fmt.Println("server not found:", err)
		os.Exit(1)
	}

	cli, err := client.NewStreamableHttpClient(server.URL)
	if err != nil {
		fmt.Println("new client err:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "blades-probe", Version: "1.0.0"}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		fmt.Println("init err:", err)
		os.Exit(1)
	}

	tools, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		fmt.Println("list err:", err)
		os.Exit(1)
	}
	fmt.Printf("total tools: %d\n\n", len(tools.Tools))

	var results []verdict
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	probeOnce := func(name string) (string, bool) {
		cctx, ccancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer ccancel()
		res, err := cli.CallTool(cctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Name: name, Arguments: map[string]any{}},
		})
		if err != nil {
			return "TIMEOUT:" + err.Error(), false
		}
		for _, c := range res.Content {
			if txt, ok := c.(mcp.TextContent); ok {
				return txt.Text, res.IsError
			}
		}
		return "(no content)", res.IsError
	}

	probe := func(name string) verdict {
		var msg string
		for attempt := 0; attempt < 3; attempt++ {
			var isErr bool
			msg, isErr = probeOnce(name)
			if !strings.HasPrefix(msg, "TIMEOUT:") {
				return classify(name, msg, isErr)
			}
		}
		return verdict{name: name, status: "TIMEOUT", msg: truncate(msg, 100)}
	}

	for _, t := range tools.Tools {
		wg.Add(1)
		sem <- struct{}{}
		go func(tool mcp.Tool) {
			defer wg.Done()
			defer func() { <-sem }()
			v := probe(tool.Name)
			mu.Lock()
			results = append(results, v)
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		rank := map[string]int{"OK": 0, "NOPERM": 1, "BADAPI": 2, "TIMEOUT": 3}
		if rank[results[i].status] != rank[results[j].status] {
			return rank[results[i].status] < rank[results[j].status]
		}
		return results[i].name < results[j].name
	})

	enable := map[string]bool{}
	groups := map[string][]string{}
	for _, r := range results {
		ok := r.status == "OK"
		enable[r.name] = ok
		groups[r.status] = append(groups[r.status], r.name)
		if r.status == "OK" {
			fmt.Printf("  [可用] %-28s %s\n", r.name, truncate(r.msg, 60))
		}
	}

	fmt.Println("\n=== 无权限(40203) ===")
	for _, n := range groups["NOPERM"] {
		fmt.Printf("  %s\n", n)
	}
	fmt.Println("\n=== 接口不存在/废弃(40101) ===")
	for _, n := range groups["BADAPI"] {
		fmt.Printf("  %s\n", n)
	}
	fmt.Println("\n=== 超时(重试3次仍失败,需人工确认) ===")
	for _, n := range groups["TIMEOUT"] {
		fmt.Printf("  %s\n", n)
	}

	fmt.Printf("\n=== 统计 ===\n")
	for k, v := range groups {
		fmt.Printf("  %-8s %d\n", k, len(v))
	}

	// 写入 DB enabled 状态
	for _, t := range tools.Tools {
		en := enable[t.Name]
		if err := db.Dao.Model(&models.MCPServerTool{}).
			Where("mcp_server_id = ? AND tool_name = ?", server.ID, t.Name).
			Update("enabled", en).Error; err != nil {
			fmt.Printf("ERR update %s: %v\n", t.Name, err)
		}
	}
	fmt.Println("\nDB enabled 状态已更新")
}

func classify(name, msg string, isErr bool) verdict {
	switch {
	case strings.Contains(msg, "频率超限"):
		// 有权限但探测触发限流，视为可用
		return verdict{name: name, status: "OK", msg: msg}
	case strings.Contains(msg, "[40203]") || strings.Contains(msg, "没有接口("):
		return verdict{name: name, status: "NOPERM", msg: msg}
	case strings.Contains(msg, "[40101]"):
		return verdict{name: name, status: "BADAPI", msg: msg}
	default:
		// 其余（含参数校验失败 50101、Input validation error）视为可用
		return verdict{name: name, status: "OK", msg: msg}
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
