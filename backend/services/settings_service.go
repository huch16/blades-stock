package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/machineid"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/strutil"
)

// AiModelInfo 模型元信息返回结构
type AiModelInfo struct {
	ModelName string `json:"modelName"`
	MaxTokens int    `json:"maxTokens"`
	Source    string `json:"source"`
}

// truncateForLog 截断上游响应体，避免把完整 HTML / 超长 JSON 刷进服务日志。
const maxLogBodyLen = 500

func truncateForLog(body string) string {
	body = strings.TrimSpace(body)
	if len(body) <= maxLogBodyLen {
		return body
	}
	return body[:maxLogBodyLen] + "...(truncated)"
}

// resolveRealApiKey 把前端回传的**掩码**密钥还原为数据库中的真实密钥。
//
// 为什么必须有这一步（用户反馈「测试连通性还是不成功？？？」）：
//
//	GetMaskedSettingConfig 下发时会把 ApiKey 打码为 sk-a****1234；
//	设置页在用户未改动密钥时，表单里保存的就是这个掩码；
//	而 testAiConfig() / fetchAiModels() 会把表单里的 apiKey 原样送到后端。
//	后端再拿它当 Bearer 去请求真正的供应商 → 供应商返回 401，
//	用户看到的是一句与真实原因完全无关的报错。
//
// RPC 签名（baseUrl, apiKey, modelName）不能改（Wails 绑定兼容），
// 因此只能在后端按 baseUrl（+ modelName）回到已保存配置里找真实密钥。
//
// 返回 ok=false 表示“提交的是掩码但匹配不到任何已保存配置”，
// 调用方**必须**据此拒绝请求，而不是把掩码发给上游。
func resolveRealApiKey(baseURL, apiKey, modelName string) (string, bool) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return apiKey, false
	}
	// 只有掩码才需要还原；明文（含本地模型常用的 "none"）原样使用。
	if !data.IsMaskedSecret(apiKey) {
		return apiKey, true
	}

	cfg := data.GetSettingConfig()
	if cfg == nil || len(cfg.AiConfigs) == 0 {
		return apiKey, false
	}

	normURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normModel := strings.TrimSpace(modelName)

	var urlOnlyFallback string
	for _, c := range cfg.AiConfigs {
		if c == nil {
			continue
		}
		stored := strings.TrimSpace(c.ApiKey)
		// 跳过空密钥与同样是掩码的条目（内存里的应该是明文，防御性跳过）
		if stored == "" || data.IsMaskedSecret(stored) {
			continue
		}
		if strings.TrimRight(strings.TrimSpace(c.BaseUrl), "/") != normURL {
			continue
		}
		// 优先 baseUrl + modelName 精确匹配，避免同地址多卡片时张冠李戴
		if normModel != "" && strings.TrimSpace(c.ModelName) == normModel {
			return stored, true
		}
		if urlOnlyFallback == "" {
			urlOnlyFallback = stored
		}
	}
	if urlOnlyFallback != "" {
		return urlOnlyFallback, true
	}
	return apiKey, false
}

// maskedKeyUnresolvedMsg 掩码无法匹配到已保存配置时给用户的可行动指引。
// 绝不能把上游那句 "Invalid API key" 当成结论抛给用户 —— 真正的原因在这里。
const maskedKeyUnresolvedMsg = "当前 API Key 显示为掩码（例如 sk-a****1234），" +
	"系统未能从已保存的配置中匹配到对应的真实密钥（接口地址或多与已保存的卡片不一致）。" +
	"请在该卡片里重新粘贴一次完整密钥后再测试；若是新建卡片，请先点「💾 保存 AI 模型配置」保存后再测试。"

// SettingsService 系统配置、交易风控、审计日志与设备管理服务
type SettingsService struct {
	versionInfo *models.VersionInfo
}

// NewSettingsService 创建系统配置服务实例
func NewSettingsService() *SettingsService {
	return &SettingsService{
		versionInfo: &models.VersionInfo{
			Version:           "4.0.0",
			Content:           "Institutional Buy-Side Quant Platform: High-Availability, Zero-Defect, Anti-Avalanche & Compliance Core",
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

// OpenURL 打开外部 URL 链接（Web 模式下主要由前端 Bridge window.open 接管，后端记录审计与 SSRF 拦截）
func (s *SettingsService) OpenURL(rawURL string) {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return
	}
	u, err := url.Parse(cleanURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		logger.SugaredLogger.Warnf("[SettingsService] OpenURL rejected non-http(s) protocol: %s", cleanURL)
		return
	}
	// 防御 SSRF：封禁访问云端元数据服务与本地危险端口
	if u.Hostname() == "169.254.169.254" || strings.HasPrefix(u.Hostname(), "169.254.") {
		logger.SugaredLogger.Warnf("[SettingsService] OpenURL rejected cloud metadata IP: %s", cleanURL)
		return
	}
	logger.SugaredLogger.Infof("[SettingsService] OpenURL verified request: %s", cleanURL)
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

// GetReportById 按报告 ID 取单条 AI 分析结果（供桌面端按行导出时取文件名与内容）。
// id<=0 时返回 nil；未找到时返回 ID==0 的零值记录。
func (s *SettingsService) GetReportById(id int) *models.AIResponseResult {
	if id <= 0 {
		return nil
	}
	return data.NewDeepSeekOpenAi(context.Background(), 0).GetAIResponseResultById(uint(id))
}

// SaveReportAsMarkdownById 按报告 ID 导出指定行的 Markdown（Web 模式返回正文）。
//
// 与按 stockCode 取最新的 SaveAsMarkdown 不同：用户在研报列表里点开一条
// 旧报告再点导出，必须拿到该行的旧内容，而不是同一标的的最新一条。
// 未找到或内容为空时返回与 SaveAsMarkdown 相同的错误文案，前端按原有分支处理。
func (s *SettingsService) SaveReportAsMarkdownById(id int) string {
	res := s.GetReportById(id)
	if res != nil && res.ID > 0 && len(res.Content) > 0 {
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
	client := data.CreateHTTPClientWithTimeout(5 * time.Second)
	resp, err := client.R().
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

// GetConfig 获取当前配置项（下发脱敏配置，确保商业 API 密钥不泄露给前端）
func (s *SettingsService) GetConfig() *data.SettingConfig {
	return data.GetMaskedSettingConfig()
}

// UpdateConfig 更新系统配置项
func (s *SettingsService) UpdateConfig(settingConfig *data.SettingConfig) string {
	return data.UpdateConfig(settingConfig)
}

// GetAiConfigs 获取 AI 配置列表（下发脱敏配置）
func (s *SettingsService) GetAiConfigs() []*data.AIConfig {
	masked := data.GetMaskedSettingConfig()
	if masked == nil {
		return []*data.AIConfig{}
	}
	return masked.AiConfigs
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
	// 掩码无法还原时不能拿掩码去请求上游：那只会换来一个误导性的 401。
	// 这里直接返回空列表并打日志（该方法签名只能返回 []string，
	// 具体的可行动指引由 TestAiConnection 与会话测试路径给出）。
	if realKey, ok := resolveRealApiKey(baseUrl, apiKey, ""); ok {
		apiKey = realKey
	} else {
		logger.SugaredLogger.Errorf("FetchAiModels 中止：提交的是掩码密钥且无法按 baseUrl=%s 还原真实密钥", baseUrl)
		return []string{}
	}

	type modelItem struct {
		ID string `json:"id"`
	}
	var respData struct {
		Data []modelItem `json:"data"`
	}

	// 使用**共享 transport**（继承用户配置的全局代理、UA 与连接池），而不是 resty.New()：
	// 旧实现完全绕过了代理配置，导致需要代理的网络环境下“拉取列表”必然失败，
	// 而同一张卡片的对话却能正常工作（对话走的是共享客户端）——这是典型的“能聊不能列”。
	client := data.CreateHTTPClientWithTimeout(10 * time.Second)
	client.SetBaseURL(baseUrl)

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData).
		Get("/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels 请求失败 (baseUrl=%s): %v", baseUrl, err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels HTTP 错误 (baseUrl=%s): %s body=%s",
			baseUrl, resp.Status(), truncateForLog(string(resp.Body())))
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
	// 关键：前端提交的很可能是掩码（用户未改动密钥时的常态）。
	// 必须还原为真实密钥，否则必然 401 —— 这正是「测试连通性总是不成功」的根因。
	realKey, resolved := resolveRealApiKey(baseUrl, apiKey, modelName)
	if !resolved {
		return maskedKeyUnresolvedMsg
	}
	apiKey = realKey

	// ── 总耗时预算（10s）──────────────────────────────────────────
	//
	// 历史缺陷①：这里是「2 次尝试 × 30s 客户端超时」= 最坏 60s+，
	//   超过前端 30s 熔断与多数反向代理容忍度，用户看到误导性的 502。
	// 历史缺陷②：20 秒预算仍然不够。实测生产入口（Cloudflare/instabox）
	//   对单请求约 **20 秒**硬切断 —— 任何耗时逼近它的请求都会被代理层
	//   变成 502 HTML 页，用户看到的是「本系统网关错误」，方向全错。
	//
	// 因此现在：
	//   1) 先用 4 秒 TCP 预检（dial）——不可达/被防火墙丢弃的地址在
	//      **1 秒级**就失败并给出精确原因，而不是挂满整个预算；
	//   2) 再给 HTTP 尝试 10 秒总预算，加上预检也稳稳落在 20s 切断之内。
	const connTestBudget = 10 * time.Second
	deadline := time.Now().Add(connTestBudget)

	timeoutMsg := fmt.Sprintf(
		"连通性测试未能完成：%d 秒内未收到模型服务端任何响应。"+
			"常见原因：该地址网络不可达、需要走代理（可在「基础与运行环境」开启全局代理并先保存）、"+
			"或 Base URL 指向的网关/中转已无响应。",
		int(connTestBudget.Seconds()))

	// TCP 预检：把「连不上」和「连上了但服务不理」区分开，且让前者秒级失败。
	if preflightErr := preflightDial(baseUrl); preflightErr != "" {
		return "连接失败：" + preflightErr
	}

	// 使用共享 transport（继承全局代理 / UA / 连接池）。
	// 注意：**不要**在这里调用 resty 的 SetProxy——resty v2 的 SetProxy 会直接写
	// transport.Proxy 字段，若传入的是共享 transport 就会污染全局所有并发请求。
	// 全局代理已由 ConfigureFromSettings 在每次保存设置时同步到共享 transport。

	// 输出预算：历史缺陷是固定 max_tokens=16。
	// 对**推理模型**（MiniMax-M3 / deepseek-reasoner / GLM-Z 等）而言，
	// 16 个 token 会在  thinking 阶段就被耗尽，服务端报「token 不够 / 输出被截断」，
	// 用户会误以为是自己配置错了。这里改为一套「先保守、后兜底」的策略：
	//   第 1 次：max_tokens=512（足够容纳 think 阶段）
	//   第 2 次：完全不传 max_tokens，交给服务端默认值
	//            （兼容各类对 max_tokens 取值有特殊限制的网关）
	attempts := []map[string]any{
		{
			"model":      modelName,
			"max_tokens": 512,
			"stream":     false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		},
		{
			"model":  modelName,
			"stream": false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		},
	}

	var lastMsg string
	for i, payload := range attempts {
		remaining := time.Until(deadline)
		if remaining < 1500*time.Millisecond {
			logger.SugaredLogger.Warnf("TestAiConnection 总预算 %v 已耗尽，停止重试", connTestBudget)
			if lastMsg == "" {
				lastMsg = timeoutMsg
			}
			break
		}
		// 每次尝试只允许使用「剩余预算」，绝不单独占用 30s 把总时长拉长
		client := data.CreateHTTPClientWithTimeout(remaining)
		client.SetBaseURL(baseUrl)

		resp, err := client.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetBody(payload).
			Post("/chat/completions")
		if err != nil {
			logger.SugaredLogger.Errorf("TestAiConnection error (attempt %d/%d): %v", i+1, len(attempts), err)
			if isNetworkTimeoutError(err) {
				lastMsg = timeoutMsg
				break
			}
			return "无法访问接口：" + err.Error()
		}
		if !resp.IsError() {
			return ""
		}
		raw := parseOpenAIErrorBody(resp.Body())
		lastMsg = classifyAIError(resp.StatusCode(), raw, modelName)
		logger.SugaredLogger.Errorf("TestAiConnection http error (attempt %d/%d): %s body: %s",
			i+1, len(attempts), resp.Status(), truncateForLog(string(resp.Body())))
		// 仅在错误确实与输出预算相关时，才值得改用「不传 max_tokens」重试
		if i == 0 && isTokenBudgetError(raw) {
			continue
		}
		break
	}
	return lastMsg
}

// isNetworkTimeoutError 判断错误是否属于「连接/读取超时」类问题。
// 这类情况必须给出「超时」结论，而不是把上游原始错误（或代理层 502）
// 直接丢给用户 —— 那会让他误判成"模型服务商挂了"。
func isNetworkTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	l := strings.ToLower(err.Error())
	return strings.Contains(l, "timeout") ||
		strings.Contains(l, "deadline exceeded") ||
		strings.Contains(l, "context canceled") ||
		strings.Contains(l, "i/o timeout")
}

// isTokenBudgetError 判断错误是否属于「输出预算不足」类问题
func isTokenBudgetError(msg string) bool {
	l := strings.ToLower(msg)
	for _, k := range []string{"max_tokens", "max tokens", "max_output", "max completion", "token", "length limit", "budget", "exceed"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}

// classifyAIError 把上游错误转成**可操作**的中文提示，避免用户只看到一串英文。
func classifyAIError(status int, raw, modelName string) string {
	l := strings.ToLower(raw)
	suffix := ""
	if strings.TrimSpace(raw) != "" {
		suffix = " 原始信息: " + raw
	}
	switch {
	case (strings.Contains(l, "model") &&
		(strings.Contains(l, "not found") || strings.Contains(l, "not exist") ||
			strings.Contains(l, "does not exist") || strings.Contains(l, "unknown") ||
			strings.Contains(l, "no such") || strings.Contains(l, "invalid model"))) ||
		(status == 404 && strings.Contains(l, "model")):
		return fmt.Sprintf("当前接口地址下不存在模型「%s」。请点「拉取列表」从服务端返回的模型中选择,或核对模型标识拼写。%s", modelName, suffix)
	case status == 401 || status == 403:
		return "API Key 无效或权限不足（HTTP " + strconv.Itoa(status) + "）。" + suffix
	case isTokenBudgetError(raw):
		return fmt.Sprintf("输出预算不足（推理模型会在思考阶段消耗 token，已自动重试放宽预算）。%s", suffix)
	case status == 404:
		return fmt.Sprintf("接口路径不存在（404）：请确认 Base URL 指向 .../v1（不是 .../v1/chat/completions）。%s", suffix)
	case status == 429:
		return "请求过于频繁或账户配额不足（429）。" + suffix
	}
	if strings.TrimSpace(raw) != "" {
		return raw
	}
	return "HTTP " + strconv.Itoa(status)
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
		// 同样要还原掩码（否则 /models/<id> 会 401，静默退化为拿不到元信息）
		if realKey, ok := resolveRealApiKey(baseUrl, apiKey, modelName); ok {
			apiKey = realKey
		} else {
			apiKey = ""
		}
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

		// 使用独立 client（共享 transport）：绝不能对 data.SharedHTTPClient 调用
		// SetBaseURL——那是全局单例，修改它会让并发请求串到别的供应商地址上。
		client := data.CreateHTTPClientWithTimeout(20 * time.Second)
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

	if err := q.Order("created_at desc").Limit(50000).Find(&list).Error; err != nil {
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

// preflightDial 对 BaseURL 的 host:port 做一次带超时的 TCP 连接测试。
// 返回空字符串表示可连通；否则返回可读的中文原因。
//
// 存在价值：模型服务商不可达（DNS 失败、端口拒绝、出口被防火墙丢弃）时，
// HTTP 尝试要等满整个预算才失败，而在生产入口约 20s 硬切断的环境下，
// 那已经足够让代理层把请求变成 502 —— 用户看到的是"网关错误"，
// 完全指不到真正的原因上。预检把这类情况前移到 4 秒内，并给出准确结论。
func preflightDial(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "Base URL 无法解析"
	}
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	conn, err := net.DialTimeout("tcp", host, 4*time.Second)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return host + " 连接超时（4 秒无响应）——通常是地址错误、出口被防火墙拦截，或该网关已下线"
		}
		return host + " 无法连接（" + err.Error() + "）"
	}
	_ = conn.Close()
	return ""
}
