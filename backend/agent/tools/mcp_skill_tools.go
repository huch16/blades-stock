package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"encoding/json"
	"fmt"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
)

func GetMCPServerTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"ListMCPServers",
		"查询MCP服务器列表。可以按名称、状态、启用状态筛选，支持分页。MCP服务器是外部工具服务，提供额外的AI工具能力。",
		map[string]*types.ParamDef{
			"name": {
				Type:     "string",
				Desc:     "按名称模糊搜索（可选）",
				Required: false,
			},
			"status": {
				Type:     "string",
				Desc:     "按状态筛选：available=可用, unavailable=不可用, stopped=已停止（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "按启用状态筛选（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := types.ExtractStringParam(args, "name")
			status := types.ExtractStringParam(args, "status")
			page, _ := types.ExtractIntParam(args, "page", "page_number")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 10
			}

			query := &models.MCPServerQuery{
				Name:     name,
				Status:   status,
				Page:     page,
				PageSize: pageSize,
			}
			if enable, ok := types.ExtractBoolParam(args, "enable"); ok {
				query.Enable = &enable
			}

			result := data.NewMCPServerApi().List(query)
			if result == nil || len(result.Data) == 0 {
				return "未找到符合条件的MCP服务器", nil
			}

			type serverRow struct {
				ID          uint   `md:"ID"`
				Name        string `md:"名称"`
				Description string `md:"描述"`
				URL         string `md:"URL"`
				Enable      bool   `md:"启用"`
				Status      string `md:"状态"`
				TestResult  string `md:"测试结果"`
			}
			var rows []serverRow
			for _, s := range result.Data {
				rows = append(rows, serverRow{
					ID:          s.ID,
					Name:        s.Name,
					Description: s.Description,
					URL:         s.URL,
					Enable:      s.Enable,
					Status:      s.Status,
					TestResult:  s.TestResult,
				})
			}
			summary := fmt.Sprintf("共找到 %d 个MCP服务器，当前第 %d 页", result.Total, page)
			return summary + "\n\n" + markdownTableWithTitle("MCP服务器列表", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMCPServerDetail",
		"根据ID获取MCP服务器的详细信息，包括名称、描述、URL、状态等",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "serverId", "server_id")
			id := uint(idInt)
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			server, err := data.NewMCPServerApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP服务器", id), nil
			}
			var md strings.Builder
			md.WriteString("### MCP服务器详情\n\n")
			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")
			md.WriteString(fmt.Sprintf("| ID | %d |\n", server.ID))
			md.WriteString(fmt.Sprintf("| 名称 | %s |\n", server.Name))
			md.WriteString(fmt.Sprintf("| 描述 | %s |\n", server.Description))
			md.WriteString(fmt.Sprintf("| URL | %s |\n", server.URL))
			md.WriteString(fmt.Sprintf("| 命令 | %s |\n", server.Command))
			md.WriteString(fmt.Sprintf("| 参数 | %s |\n", server.Args))
			md.WriteString(fmt.Sprintf("| 启用 | %v |\n", server.Enable))
			md.WriteString(fmt.Sprintf("| 状态 | %s |\n", server.Status))
			md.WriteString(fmt.Sprintf("| 测试结果 | %s |\n", server.TestResult))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", server.CreatedAt.Format("2006-01-02 15:04:05")))
			return md.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"CreateMCPServer",
		"创建一个新的MCP服务器配置。MCP服务器是外部工具服务，通过URL连接提供额外的AI工具能力。",
		map[string]*types.ParamDef{
			"name": {
				Type:     "string",
				Desc:     "服务器名称",
				Required: true,
			},
			"description": {
				Type:     "string",
				Desc:     "服务器描述",
				Required: false,
			},
			"url": {
				Type:     "string",
				Desc:     "服务器URL地址",
				Required: true,
			},
			"command": {
				Type:     "string",
				Desc:     "启动命令（可选，用于本地MCP服务）",
				Required: false,
			},
			"args": {
				Type:     "string",
				Desc:     "命令参数（可选）",
				Required: false,
			},
			"env": {
				Type:     "string",
				Desc:     "环境变量，JSON格式（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用，默认true",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := types.ExtractStringParam(args, "name")
			url := types.ExtractStringParam(args, "url", "link")
			if name == "" || url == "" {
				return "名称和URL不能为空", nil
			}
			enable := true
			if v, ok := types.ExtractBoolParam(args, "enable"); ok {
				enable = v
			}
			server := &models.MCPServer{
				Name:        name,
				Description: types.ExtractStringParam(args, "description"),
				URL:         url,
				Command:     types.ExtractStringParam(args, "command"),
				Args:        types.ExtractStringParam(args, "args"),
				Env:         types.ExtractStringParam(args, "env"),
				Enable:      enable,
			}
			if err := data.NewMCPServerApi().Create(server); err != nil {
				return "创建MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器创建成功，ID: %d", server.ID), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"UpdateMCPServer",
		"更新MCP服务器配置信息",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "服务器ID",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "服务器名称（可选）",
				Required: false,
			},
			"description": {
				Type:     "string",
				Desc:     "服务器描述（可选）",
				Required: false,
			},
			"url": {
				Type:     "string",
				Desc:     "服务器URL地址（可选）",
				Required: false,
			},
			"command": {
				Type:     "string",
				Desc:     "启动命令（可选）",
				Required: false,
			},
			"args": {
				Type:     "string",
				Desc:     "命令参数（可选）",
				Required: false,
			},
			"env": {
				Type:     "string",
				Desc:     "环境变量，JSON格式（可选）",
				Required: false,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "是否启用（可选）",
				Required: false,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "serverId", "server_id")
			id := uint(idInt)
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			existing, err := data.NewMCPServerApi().GetByID(id)
			if err != nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP服务器", id), nil
			}
			// 注意：空字符串不再清空字段（旧代码用 Exists() 语义，传入 "" 会把字段置空）；
			// 如需清空字段请在界面上直接操作数据库。
			if v := types.ExtractStringParam(args, "name"); v != "" {
				existing.Name = v
			}
			if v := types.ExtractStringParam(args, "description"); v != "" {
				existing.Description = v
			}
			if v := types.ExtractStringParam(args, "url", "link"); v != "" {
				existing.URL = v
			}
			if v := types.ExtractStringParam(args, "command"); v != "" {
				existing.Command = v
			}
			if v := types.ExtractStringParam(args, "args"); v != "" {
				existing.Args = v
			}
			if v := types.ExtractStringParam(args, "env"); v != "" {
				existing.Env = v
			}
			if v, ok := types.ExtractBoolParam(args, "enable"); ok {
				existing.Enable = v
			}
			if err := data.NewMCPServerApi().Update(existing); err != nil {
				return "更新MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器 %s 更新成功", existing.Name), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"DeleteMCPServer",
		"删除MCP服务器配置",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "serverId", "server_id")
			id := uint(idInt)
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			if err := data.NewMCPServerApi().Delete(id); err != nil {
				return "删除MCP服务器失败: " + err.Error(), nil
			}
			return fmt.Sprintf("MCP服务器(ID:%d)已删除", id), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"EnableMCPServer",
		"启用或禁用MCP服务器",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
			"enable": {
				Type:     "boolean",
				Desc:     "true=启用，false=禁用",
				Required: true,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "serverId", "server_id")
			id := uint(idInt)
			enable, _ := types.ExtractBoolParam(args, "enable")
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			if err := data.NewMCPServerApi().EnableServer(id, enable); err != nil {
				return "操作失败: " + err.Error(), nil
			}
			status := "禁用"
			if enable {
				status = "启用"
			}
			return fmt.Sprintf("MCP服务器(ID:%d)已%s", id, status), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"TestMCPServer",
		"测试MCP服务器连接是否可用，并获取该服务器提供的工具列表",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "MCP服务器ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "serverId", "server_id")
			id := uint(idInt)
			if id == 0 {
				return "请提供有效的MCP服务器ID", nil
			}
			result, err := data.NewMCPServerApi().TestConnection(id)
			if err != nil {
				return "测试连接失败: " + err.Error(), nil
			}
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"ListMCPServerTools",
		"获取MCP服务器提供的工具列表，包括工具名称和描述。可按服务器ID筛选，不传则返回所有服务器的工具。",
		map[string]*types.ParamDef{
			"serverId": {
				Type:     "integer",
				Desc:     "MCP服务器ID，不传则返回所有服务器的工具",
				Required: false,
			},
		},
		func(args string) (string, error) {
			serverIDInt, _ := types.ExtractIntParam(args, "serverId", "server_id", "id")
			serverID := uint(serverIDInt)
			var toolList []models.MCPServerTool
			if serverID > 0 {
				toolList = data.NewMCPServerApi().GetToolsByServerID(serverID)
			} else {
				toolList = data.NewMCPServerApi().GetAllTools()
			}
			if len(toolList) == 0 {
				return "未找到MCP工具", nil
			}
			type toolRow struct {
				ID          uint   `md:"ID"`
				ServerID    uint   `md:"服务器ID"`
				ToolName    string `md:"工具名称"`
				Description string `md:"描述"`
			}
			var rows []toolRow
			for _, t := range toolList {
				desc := t.Description
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				rows = append(rows, toolRow{
					ID:          t.ID,
					ServerID:    t.MCPServerID,
					ToolName:    t.ToolName,
					Description: desc,
				})
			}
			return markdownTableWithTitle("MCP工具列表", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMCPToolDetail",
		"获取MCP工具的详细信息，包括完整的描述和参数定义（JSON Schema格式）。需要提供工具ID。",
		map[string]*types.ParamDef{
			"toolId": {
				Type:     "integer",
				Desc:     "MCP工具ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			toolIDInt, _ := types.ExtractIntParam(args, "toolId", "tool_id", "id")
			toolID := uint(toolIDInt)
			if toolID == 0 {
				return "请提供有效的MCP工具ID", nil
			}
			allTools := data.NewMCPServerApi().GetAllTools()
			var found *models.MCPServerTool
			for i := range allTools {
				if allTools[i].ID == toolID {
					found = &allTools[i]
					break
				}
			}
			if found == nil {
				return fmt.Sprintf("未找到ID为 %d 的MCP工具", toolID), nil
			}
			var md strings.Builder
			md.WriteString(fmt.Sprintf("### MCP工具详情：%s\n\n", found.ToolName))
			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")
			md.WriteString(fmt.Sprintf("| ID | %d |\n", found.ID))
			md.WriteString(fmt.Sprintf("| 服务器ID | %d |\n", found.MCPServerID))
			md.WriteString(fmt.Sprintf("| 工具名称 | %s |\n", found.ToolName))
			md.WriteString(fmt.Sprintf("| 描述 | %s |\n", found.Description))
			if found.ParamsSchema != "" {
				md.WriteString(fmt.Sprintf("\n#### 参数定义（JSON Schema）\n\n```json\n%s\n```\n", found.ParamsSchema))
			} else {
				md.WriteString("\n#### 参数定义\n\n无参数\n")
			}
			return md.String(), nil
		},
	))

	return result
}

func markdownTableWithTitle(title string, d any) string {
	jsonData, err := json.Marshal(d)
	if err != nil {
		return fmt.Sprintf("生成表格失败: %v", err)
	}
	table, err := data.JSONToMarkdownTable(jsonData)
	if err != nil {
		return fmt.Sprintf("生成表格失败: %v", err)
	}
	return "### " + title + "\n" + table
}
