package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/machineid"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
)

// AiModelInfo 模型元信息返回结构
type AiModelInfo struct {
	ModelName string `json:"modelName"`
	MaxTokens int    `json:"maxTokens"`
	Source    string `json:"source"`
}

// SettingsService 系统配置、交易风控、审计日志与设备管理服务
type SettingsService struct {
	versionInfo *models.VersionInfo
}

// NewSettingsService 创建系统配置服务实例
func NewSettingsService() *SettingsService {
	return &SettingsService{
		versionInfo: &models.VersionInfo{
			Version:           "1.6.0",
			Content:           "Settings Command Center & Industrial Disaster Recovery Overhaul",
			OfficialStatement: "未经授权,禁止商业目的! 数据来源于网络,仅供参考;投资有风险,入市需谨慎",
		},
	}
}

// SetVersionInfo 设置系统版本信息
func (s *SettingsService) SetVersionInfo(info *models.VersionInfo) {
	if info != nil {
		s.versionInfo = info
	}
}

// GetVersionInfo 获取系统版本信息
func (s *SettingsService) GetVersionInfo() *models.VersionInfo {
	return s.versionInfo
}

// ExportConfig 导出系统配置为 JSON 字符串
func (s *SettingsService) ExportConfig() string {
	return data.NewSettingsApi().Export()
}

// OpenURL 打开外部 URL 链接（Web 模式下主要由前端 Bridge window.open 接管，后端记录审计）
func (s *SettingsService) OpenURL(url string) {
	logger.SugaredLogger.Infof("[SettingsService] OpenURL request: %s", url)
}

// SendDingDingMessageByType 按消息类型向钉钉发送告警推送
func (s *SettingsService) SendDingDingMessageByType(message string, stockCode string, msgType int) string {
	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// SaveAsMarkdown 获取并返回 Markdown 分析结果（Web 模式下载兜底）
func (s *SettingsService) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(context.Background(), 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 0 {
		return res.Content
	}
	return "分析结果异常,无法保存。"
}

// SaveWordFile 跨平台保存 Word 文件（Web 模式返回文件名）
func (s *SettingsService) SaveWordFile(filename string, base64Data string) string {
	return filename
}

// SaveImage 跨平台保存图片（Web 模式返回图片名）
func (s *SettingsService) SaveImage(name, base64Data string) string {
	return name
}

// ShareAnalysis 将个股研报结果分享至社区（已废弃：本项目无社区服务）
func (s *SettingsService) ShareAnalysis(stockCode, stockName string) string {
	return "本项目未启用社区分享服务"
}

// ShareText 将文本内容直接分享至社区（已废弃：本项目无社区服务）
func (s *SettingsService) ShareText(text, title string) string {
	return "本项目未启用社区分享服务"
}

// FetchAuditLogs 前端兼容别名：分页获取审计日志
func (s *SettingsService) FetchAuditLogs(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	return s.GetAuditLogList(query)
}

// GetMachineId 获取当前机器硬件标识
func (s *SettingsService) GetMachineId() string {
	return machineid.GetMachineId()
}

// CheckDeviceBinding 校验设备许可绑定
func (s *SettingsService) CheckDeviceBinding(token string, apiBase string) map[string]any {
	uuid := machineid.GetMachineId()
	result := map[string]any{
		"bound":       false,
		"deviceCount": 0,
		"maxDevices":  5,
	}

	if token == "" || apiBase == "" {
		return result
	}

	url := fmt.Sprintf("%s/user/device-check?uuid=%s", apiBase, uuid)
	resp, err := data.SharedHTTPClient.R().
		SetHeader("Authorization", "Bearer "+token).
		Get(url)
	if err != nil {
		return result
	}

	var respData struct {
		Code int `json:"code"`
		Data struct {
			Bound       bool `json:"bound"`
			DeviceCount int  `json:"deviceCount"`
			MaxDevices  int  `json:"maxDevices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &respData); err != nil {
		return result
	}
	if respData.Code != 0 {
		return result
	}

	result["bound"] = respData.Data.Bound
	result["deviceCount"] = respData.Data.DeviceCount
	result["maxDevices"] = respData.Data.MaxDevices
	return result
}

// GetConfig 获取当前配置项
func (s *SettingsService) GetConfig() *data.SettingConfig {
	return data.GetSettingConfig()
}

// UpdateConfig 更新系统配置项
func (s *SettingsService) UpdateConfig(settingConfig *data.SettingConfig) string {
	return data.UpdateConfig(settingConfig)
}

// GetAiConfigs 获取 AI 配置列表
func (s *SettingsService) GetAiConfigs() []*data.AIConfig {
	return data.GetSettingConfig().AiConfigs
}

// GetAiConfigTemplates 获取预置 AI 配置模板
func (s *SettingsService) GetAiConfigTemplates() []*data.AiConfigTemplate {
	return data.NewSettingsApi().GetAiConfigTemplates()
}

// CleanupData 数据清理
func (s *SettingsService) CleanupData(cleanupType string) string {
	return data.NewSettingsApi().CleanupData(cleanupType)
}

// BackupData 数据备份 (兼容旧版单模块 CSV 导出)
func (s *SettingsService) BackupData(backupType string) string {
	return data.NewSettingsApi().BackupData(backupType)
}

// RestoreData 数据恢复 (兼容旧版单模块 CSV 恢复)
func (s *SettingsService) RestoreData(backupType string) string {
	return data.NewSettingsApi().RestoreData(backupType)
}

// GetBackupOverview 获取系统数据资产概览
func (s *SettingsService) GetBackupOverview() (*data.BackupOverview, error) {
	return data.NewSettingsApi().GetBackupOverview()
}

// ListBackups 列出所有历史备份归档
func (s *SettingsService) ListBackups() ([]data.BackupFileItem, error) {
	return data.NewSettingsApi().ListBackups()
}

// CreateIntelligentBackup 一键创建全量智能资产备份包
func (s *SettingsService) CreateIntelligentBackup(note string) string {
	return data.NewSettingsApi().CreateIntelligentBackup(note)
}

// CreateDatabaseSnapshot 一键创建底层数据库物理快照
func (s *SettingsService) CreateDatabaseSnapshot() string {
	return data.NewSettingsApi().CreateDatabaseSnapshot()
}

// RestoreBackupFile 精确恢复指定文件名的备份
func (s *SettingsService) RestoreBackupFile(fileName string) string {
	return data.NewSettingsApi().RestoreBackupFile(fileName)
}

// DeleteBackupFile 删除指定的历史备份文件
func (s *SettingsService) DeleteBackupFile(fileName string) string {
	return data.NewSettingsApi().DeleteBackupFile(fileName)
}

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）
func (s *SettingsService) GetBackupContent(fileName string) string {
	return data.NewSettingsApi().GetBackupContent(fileName)
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入
func (s *SettingsService) ImportIntelligentBackup(content string) string {
	return data.NewSettingsApi().ImportIntelligentBackup(content)
}

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验
func (s *SettingsService) VerifyBackupIntegrity(fileName string) string {
	return data.NewSettingsApi().VerifyBackupIntegrity(fileName)
}

// PruneOldBackups 智能滚动修剪过期的历史备份
func (s *SettingsService) PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays int) string {
	return data.NewSettingsApi().PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays)
}

// ExportConfigString 导出配置 JSON 字符串
func (s *SettingsService) ExportConfigString() string {
	return data.NewSettingsApi().Export()
}

// FetchAiModels 获取支持的 OpenAI 兼容模型列表
func (s *SettingsService) FetchAiModels(baseUrl, apiKey string) []string {
	baseUrl = strings.TrimRight(strutil.Trim(baseUrl), "/")
	apiKey = strutil.Trim(apiKey)
	if baseUrl == "" {
		return []string{}
	}
	if apiKey == "" {
		apiKey = "none"
	}

	type modelItem struct {
		ID string `json:"id"`
	}
	var respData struct {
		Data []modelItem `json:"data"`
	}

	client := resty.New()
	client.SetBaseURL(baseUrl)
	client.SetTimeout(10 * time.Second)

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData).
		Get("/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	modelsList := make([]string, 0, len(respData.Data))
	for _, m := range respData.Data {
		if strings.TrimSpace(m.ID) != "" {
			modelsList = append(modelsList, m.ID)
		}
	}
	return modelsList
}

// TestAiConnection 测试与目标 LLM 服务的连通性
func (s *SettingsService) TestAiConnection(baseUrl, apiKey, modelName string) string {
	baseUrl = strings.TrimRight(strutil.Trim(baseUrl), "/")
	apiKey = strutil.Trim(apiKey)
	modelName = strutil.Trim(modelName)
	if baseUrl == "" {
		return "接口地址不能为空"
	}
	if modelName == "" {
		return "请先在「模型名」中选择或填写一个模型"
	}
	if apiKey == "" {
		apiKey = "none"
	}

	client := resty.New()
	client.SetBaseURL(baseUrl)
	client.SetTimeout(20 * time.Second)

	// 若系统启用了全局 HTTP 代理，透传给测试客户端
	if cfg := data.GetSettingConfig(); cfg != nil && cfg.HttpProxyEnabled && strings.TrimSpace(cfg.HttpProxy) != "" {
		client.SetProxy(strings.TrimSpace(cfg.HttpProxy))
	}

	payload := map[string]any{
		"model":      modelName,
		"max_tokens": 16,
		"stream":     false,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	}
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post("/chat/completions")
	if err != nil {
		logger.SugaredLogger.Errorf("TestAiConnection error: %v", err)
		return "无法访问接口：" + err.Error()
	}
	if resp.IsError() {
		msg := parseOpenAIErrorBody(resp.Body())
		if msg == "" {
			msg = "HTTP " + resp.Status()
		}
		logger.SugaredLogger.Errorf("TestAiConnection http error: %s body: %s", resp.Status(), string(resp.Body()))
		return msg
	}
	return ""
}

func parseOpenAIErrorBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var v struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err == nil && v.Error.Message != "" {
		if v.Error.Type != "" {
			return v.Error.Type + ": " + v.Error.Message
		}
		return v.Error.Message
	}
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err == nil && e.Error != "" {
		return e.Error
	}
	return ""
}

// FetchAiModelInfo 获取指定模型详细配置
func (s *SettingsService) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	baseUrl = strings.TrimRight(strutil.Trim(baseUrl), "/")
	modelName = strutil.Trim(modelName)
	if baseUrl == "" || modelName == "" {
		return nil
	}

	info := &AiModelInfo{
		ModelName: modelName,
		MaxTokens: 0,
		Source:    "",
	}

	if apiKey != "" {
		type modelDetail struct {
			ID             string `json:"id"`
			MaxContextLen  int    `json:"max_context_length"`
			ContextLength  int    `json:"context_length"`
			MaxOutputTok   int    `json:"max_output_tokens"`
			MaxTokensField int    `json:"max_tokens"`
		}
		var detail modelDetail

		client := data.SharedHTTPClient
		client.SetBaseURL(baseUrl)

		resp, err := client.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetResult(&detail).
			Get("/models/" + modelName)

		if err == nil && !resp.IsError() && detail.ID != "" {
			if detail.MaxContextLen > 0 {
				info.MaxTokens = detail.MaxContextLen
				info.Source = "api"
			} else if detail.ContextLength > 0 {
				info.MaxTokens = detail.ContextLength
				info.Source = "api"
			} else if detail.MaxOutputTok > 0 {
				info.MaxTokens = detail.MaxOutputTok
				info.Source = "api"
			} else if detail.MaxTokensField > 0 {
				info.MaxTokens = detail.MaxTokensField
				info.Source = "api"
			}
		}
	}

	return info
}

// GetAuditLogList 分页获取审计日志
func (s *SettingsService) GetAuditLogList(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	offset := (query.Page - 1) * query.PageSize

	var list []models.AuditLog
	var total int64

	q := db.Dao.Model(&models.AuditLog{})

	if query.UserID != "" {
		q.Where("user_id = ?", query.UserID)
	}
	if query.AgentID != "" {
		q.Where("agent_id = ?", query.AgentID)
	}
	if query.SkillID != 0 {
		q.Where("skill_id = ?", query.SkillID)
	}
	if query.ToolName != "" {
		q.Where("tool_name LIKE ?", "%"+query.ToolName+"%")
	}
	if query.ServerName != "" {
		q.Where("server_name LIKE ?", "%"+query.ServerName+"%")
	}
	if query.Success != nil {
		q.Where("success = ?", *query.Success)
	}
	if query.StartDate != "" {
		q.Where("DATE(created_at) >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		q.Where("DATE(created_at) <= ?", query.EndDate)
	}
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		q.Where("request_body LIKE ? OR response LIKE ?", keyword, keyword)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := q.Order("created_at desc").Offset(offset).Limit(query.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}

	return &models.AuditLogPageResp{
		Code:    0,
		Message: "success",
		Data: models.AuditLogPageData{
			List:     list,
			Total:    total,
			Page:     query.Page,
			PageSize: query.PageSize,
		},
	}, nil
}

// ExportAuditLogCSV 导出审计日志为 CSV
func (s *SettingsService) ExportAuditLogCSV(query models.AuditLogQuery) (string, error) {
	var list []models.AuditLog
	q := db.Dao.Model(&models.AuditLog{})

	if query.UserID != "" {
		q.Where("user_id = ?", query.UserID)
	}
	if query.AgentID != "" {
		q.Where("agent_id = ?", query.AgentID)
	}
	if query.SkillID != 0 {
		q.Where("skill_id = ?", query.SkillID)
	}
	if query.ToolName != "" {
		q.Where("tool_name LIKE ?", "%"+query.ToolName+"%")
	}
	if query.ServerName != "" {
		q.Where("server_name LIKE ?", "%"+query.ServerName+"%")
	}
	if query.Success != nil {
		q.Where("success = ?", *query.Success)
	}
	if query.StartDate != "" {
		q.Where("DATE(created_at) >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		q.Where("DATE(created_at) <= ?", query.EndDate)
	}
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		q.Where("request_body LIKE ? OR response LIKE ?", keyword, keyword)
	}

	if err := q.Order("created_at desc").Find(&list).Error; err != nil {
		return "", err
	}

	var lines []string
	header := []string{
		"ID", "CreatedAt", "UserID", "AgentID", "SkillID", "ToolName",
		"ServerName", "RequestBody", "Response", "Success", "ErrorMsg", "DurationMs",
	}
	lines = append(lines, strings.Join(header, ","))

	for _, entry := range list {
		escape := func(str string) string {
			str = strings.ReplaceAll(str, `"`, `""`)
			if strings.ContainsAny(str, `",\n`) {
				return `"` + str + `"`
			}
			return str
		}
		createdAt := entry.CreatedAt.Format("2006-01-02 15:04:05")
		record := []string{
			fmt.Sprintf("%d", entry.ID),
			escape(createdAt),
			escape(entry.UserID),
			escape(entry.AgentID),
			fmt.Sprintf("%d", entry.SkillID),
			escape(entry.ToolName),
			escape(entry.ServerName),
			escape(entry.RequestBody),
			escape(entry.Response),
			fmt.Sprintf("%t", entry.Success),
			escape(entry.ErrorMsg),
			fmt.Sprintf("%d", entry.DurationMs),
		}
		lines = append(lines, strings.Join(record, ","))
	}
	return strings.Join(lines, "\n"), nil
}

// AddTradingRecord 添加交易记录
func (s *SettingsService) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().AddTradingRecord(record)
}

// GetTradingRecordList 获取交易记录列表
func (s *SettingsService) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	page, err := data.NewStockDataApi().GetTradingRecordList(query)
	if err != nil {
		return &data.TradingRecordPageData{}
	}
	return page
}

// GetTradingRecordById 获取交易记录详情
func (s *SettingsService) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return data.NewStockDataApi().GetTradingRecordById(id)
}

// GetTradingRecordStatistics 获取交易统计分析
func (s *SettingsService) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := data.NewStockDataApi().GetTradingRecordStatistics()
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return stats
}

// UpdateTradingRecord 更新交易记录
func (s *SettingsService) UpdateTradingRecord(record data.TradingRecord) error {
	return data.NewStockDataApi().UpdateTradingRecord(record)
}

// DeleteTradingRecord 删除交易记录
func (s *SettingsService) DeleteTradingRecord(id uint) error {
	return data.NewStockDataApi().DeleteTradingRecord(id)
}

// CheckFrequentTrading 检查频繁交易风控
func (s *SettingsService) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := data.NewStockDataApi().CheckFrequentTrading(stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}
