package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServerApi struct{}

func NewMCPServerApi() *MCPServerApi {
	return &MCPServerApi{}
}

func (a *MCPServerApi) Create(server *models.MCPServer) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return db.Dao.Create(server).Error
}

func (a *MCPServerApi) Update(server *models.MCPServer) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	if server == nil || server.ID == 0 {
		return fmt.Errorf("无效的服务器ID")
	}

	updates := map[string]any{
		"name":        server.Name,
		"description": server.Description,
		"url":         server.URL,
		"type":        server.Type,
		"headers":     server.Headers,
		"command":     server.Command,
		"args":        server.Args,
		"env":         server.Env,
		"enable":      server.Enable,
		"status":      server.Status,
	}

	return db.Dao.Model(&models.MCPServer{}).
		Where("id = ?", server.ID).
		Updates(updates).Error
}

func (a *MCPServerApi) Delete(id uint) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return db.Dao.Delete(&models.MCPServer{}, id).Error
}

func (a *MCPServerApi) GetByID(id uint) (*models.MCPServer, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库连接未就绪")
	}
	var server models.MCPServer
	err := db.Dao.First(&server, id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (a *MCPServerApi) List(query *models.MCPServerQuery) *models.MCPServerPageResp {
	servers := make([]models.MCPServer, 0)
	var total int64

	if db.Dao == nil {
		return &models.MCPServerPageResp{
			Total: 0,
			Data:  servers,
		}
	}

	if query == nil {
		query = &models.MCPServerQuery{
			Page:     1,
			PageSize: 10,
		}
	}

	dbQuery := db.Dao.Model(&models.MCPServer{})

	if query.Name != "" {
		dbQuery = dbQuery.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != "" {
		dbQuery = dbQuery.Where("status = ?", query.Status)
	}
	if query.Enable != nil {
		dbQuery = dbQuery.Where("enable = ?", *query.Enable)
	}

	dbQuery.Count(&total)

	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	err := dbQuery.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Errorf("查询MCP服务器列表失败:%s", err.Error())
		return &models.MCPServerPageResp{
			Total: 0,
			Data:  servers,
		}
	}

	return &models.MCPServerPageResp{
		Total: int(total),
		Data:  servers,
	}
}

func (a *MCPServerApi) EnableServer(id uint, enable bool) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return db.Dao.Model(&models.MCPServer{}).Where("id = ?", id).Updates(map[string]any{
		"enable": enable,
	}).Error
}

func (a *MCPServerApi) UpdateStatus(id uint, status string, testResult string) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return db.Dao.Model(&models.MCPServer{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"test_result": testResult,
	}).Error
}

func ParseMCPHeaders(headersStr string) map[string]string {
	if headersStr == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(headersStr), &headers); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Headers失败: %v", err)
		return nil
	}
	return headers
}

func CreateMCPClient(server *models.MCPServer) (*client.Client, error) {
	headers := ParseMCPHeaders(server.Headers)

	switch server.Type {
	case "sse":
		opts := []transport.ClientOption{}
		if len(headers) > 0 {
			opts = append(opts, transport.WithHeaders(headers))
		}
		return client.NewSSEMCPClient(server.URL, opts...)
	default:
		opts := []transport.StreamableHTTPCOption{}
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		return client.NewStreamableHttpClient(server.URL, opts...)
	}
}

func InitMCPClient(ctx context.Context, server *models.MCPServer) (*client.Client, error) {
	cli, err := CreateMCPClient(server)
	if err != nil {
		return nil, fmt.Errorf("创建MCP客户端失败: %w", err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Blades Stock",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("初始化MCP连接失败: %w", err)
	}

	return cli, nil
}

func (a *MCPServerApi) TestConnection(id uint) (string, error) {
	server, err := a.GetByID(id)
	if err != nil {
		return "", err
	}

	a.UpdateStatus(id, "testing", "")

	if server.URL == "" {
		a.UpdateStatus(id, "unavailable", "URL不能为空")
		return "", fmt.Errorf("URL不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := InitMCPClient(ctx, server)
	if err != nil {
		errMsg := err.Error()
		a.UpdateStatus(id, "unavailable", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	result, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		errMsg := fmt.Sprintf("获取工具列表失败: %s", err.Error())
		a.UpdateStatus(id, "unavailable", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	toolCount := len(result.Tools)

	a.UpdateStatus(id, "available", fmt.Sprintf("可用，共 %d 个工具", toolCount))

	db.Dao.Where("mcp_server_id = ?", id).Delete(&models.MCPServerTool{})

	if toolCount == 0 {
		return "连接成功！但未发现可用工具", nil
	}

	var toolNames []string
	for _, t := range result.Tools {
		toolNames = append(toolNames, t.Name)

		var paramsJSON string
		if data, err := json.Marshal(t.InputSchema); err == nil {
			paramsJSON = string(data)
		}

		toolRecord := models.MCPServerTool{
			MCPServerID:  id,
			ToolName:     t.Name,
			Description:  t.Description,
			ParamsSchema: paramsJSON,
		}
		if err := db.Dao.Create(&toolRecord).Error; err != nil {
			logger.SugaredLogger.Warnf("保存MCP工具信息失败 [%s]: %v", t.Name, err)
		}
	}

	return fmt.Sprintf("连接成功!发现 %d 个工具: %s", toolCount, strings.Join(toolNames, ", ")), nil
}

func (a *MCPServerApi) EnableTool(id uint, enable bool) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return db.Dao.Model(&models.MCPServerTool{}).Where("id = ?", id).Update("enabled", enable).Error
}

func (a *MCPServerApi) GetToolsByServerID(serverID uint) []models.MCPServerTool {
	tools := make([]models.MCPServerTool, 0)
	if db.Dao == nil {
		return tools
	}
	db.Dao.Where("mcp_server_id = ?", serverID).Order("tool_name ASC").Find(&tools)
	return tools
}

func (a *MCPServerApi) GetEnabledToolsByServerID(serverID uint) []models.MCPServerTool {
	tools := make([]models.MCPServerTool, 0)
	if db.Dao == nil {
		return tools
	}
	db.Dao.Where("mcp_server_id = ? AND enabled = ?", serverID, true).Order("tool_name ASC").Find(&tools)
	return tools
}

func (a *MCPServerApi) GetAllTools() []models.MCPServerTool {
	tools := make([]models.MCPServerTool, 0)
	if db.Dao == nil {
		return tools
	}
	db.Dao.Order("mcp_server_id ASC, tool_name ASC").Find(&tools)
	return tools
}
