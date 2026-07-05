//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	bladesmc "github.com/go-kratos/blades/contrib/mcp"
)

func main() {
	logger.InitLogger()
	db.Init("G:\\stock\\blades-stock\\data\\stock.db")

	fmt.Println("=== MCP Server 实际连通性诊断 ===\n")

	parseHeaders := func(h string) map[string]string {
		if h == "" {
			return nil
		}
		var m map[string]string
		json.Unmarshal([]byte(h), &m)
		return m
	}

	parseArgs := func(a string) []string {
		if a == "" {
			return nil
		}
		var args []string
		json.Unmarshal([]byte(a), &args)
		return args
	}

	var servers []models.MCPServer
	db.Dao.Where("enable = ?", true).Find(&servers)

	skillIDs := getSkillServerIDs()
	for _, sid := range skillIDs {
		found := false
		for _, s := range servers {
			if s.ID == sid {
				found = true
				break
			}
		}
		if !found {
			var s models.MCPServer
			if db.Dao.Where("id = ? AND enable = ?", sid, true).First(&s).Error == nil {
				servers = append(servers, s)
			}
		}
	}

	fmt.Printf("共计 %d 个 MCP Server 待检测\n\n", len(servers))

	for _, srv := range servers {
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Server: 【%s】 ID=%d\n", srv.Name, srv.ID)

		if srv.URL == "" && srv.Command == "" {
			fmt.Printf("❌ 无 URL 也无 Command，跳过\n\n")
			continue
		}

		headers := parseHeaders(srv.Headers)
		if len(headers) > 0 {
			for k, v := range headers {
				display := v
				if (strings.Contains(strings.ToLower(k), "token") ||
					strings.Contains(strings.ToLower(k), "auth") ||
					strings.Contains(strings.ToLower(k), "key")) && len(v) > 16 {
					display = v[:8] + "..." + v[len(v)-6:]
				}
				fmt.Printf("Header: %s = %s\n", k, display)
			}
		}

		var transport bladesmc.TransportType
		var endpoint, command string
		var args []string
		var env map[string]string

		if srv.Command != "" {
			transport = bladesmc.TransportStdio
			command = srv.Command
			args = parseArgs(srv.Args)
			env = parseHeaders(srv.Env)
		} else {
			transport = bladesmc.TransportHTTP
			endpoint = srv.URL
		}

		client, err := bladesmc.NewClient(bladesmc.ClientConfig{
			Name:      srv.Name,
			Transport: transport,
			Endpoint:  endpoint,
			Command:   command,
			Args:      args,
			Env:       env,
			Headers:   headers,
			Timeout:   30 * time.Second,
		})
		if err != nil {
			fmt.Printf("❌ NewClient 失败: %v\n\n", err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = client.Connect(ctx)
		cancel()
		if err != nil {
			fmt.Printf("❌ Connect 失败: %v\n\n", err)
			continue
		}
		fmt.Printf("✅ Connect 成功\n")

		ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
		tools, err := client.Resolve(ctx2)
		cancel2()
		if err != nil {
			fmt.Printf("❌ Resolve 失败: %v\n\n", err)
			continue
		}

		fmt.Printf("✅ Resolve 成功，共 %d 个工具:\n", len(tools))
		for i, t := range tools {
			if i < 20 {
				fmt.Printf("   [%2d] %s\n", i+1, t.Name())
			} else if i == 20 {
				fmt.Printf("   ...（共 %d 个）\n", len(tools))
				break
			}
		}
		fmt.Println()
	}

	fmt.Println("=== 诊断完成 ===")
}

func getSkillServerIDs() []uint {
	var skills []models.Skill
	db.Dao.Where("enable = ?", true).Find(&skills)
	var ids []uint
	seen := make(map[uint]bool)
	for _, sk := range skills {
		if sk.MCPServerIDs == "" {
			continue
		}
		for _, id := range data.NewSkillApi().GetMCPServerIDs(&sk) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}
