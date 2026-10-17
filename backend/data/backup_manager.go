package data

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"
)

// BackupOverview 系统核心智能数据与资产概览
type BackupOverview struct {
	AiConfigCount      int64  `json:"aiConfigCount"`
	SkillCount         int64  `json:"skillCount"`
	McpServerCount     int64  `json:"mcpServerCount"`
	ChatSessionCount   int64  `json:"chatSessionCount"`
	MemoryCount        int64  `json:"memoryCount"`
	AiReportCount      int64  `json:"aiReportCount"`
	AiRecommendCount   int64  `json:"aiRecommendCount"`
	FollowedStockCount int64  `json:"followedStockCount"`
	FollowedFundCount  int64  `json:"followedFundCount"`
	BacktestCount      int64  `json:"backtestCount"`
	CronTaskCount      int64  `json:"cronTaskCount"`
	TotalBackupFiles   int    `json:"totalBackupFiles"`
	TotalBackupSizeStr string `json:"totalBackupSizeStr"`
	LastBackupTime     string `json:"lastBackupTime"`
}

// BackupFileItem 备份归档历史文件条目
type BackupFileItem struct {
	FileName   string    `json:"fileName"`
	Type       string    `json:"type"`       // "intel_bundle" | "db_snapshot" | "csv"
	TypeName   string    `json:"typeName"`   // "全量智能资产包" | "数据库快照" | "专项CSV"
	Size       int64     `json:"size"`       // 字节大小
	SizeStr    string    `json:"sizeStr"`    // 可读格式，如 "1.2 MB"
	CreatedAt  time.Time `json:"createdAt"`
	TimeStr    string    `json:"timeStr"`    // "2026-10-17 11:30:00"
	RecordDesc string    `json:"recordDesc"` // "包含 3项AI配置, 2个技能, 114条对话, 230篇研报, 18只自选"
}

// IntelligentBundle 全量智能资产导出包结构（自描述且跨系统兼容）
type IntelligentBundle struct {
	Version   string           `json:"version"`
	Format    string           `json:"format"` // "blades_stock_intelligent_bundle"
	CreatedAt time.Time        `json:"createdAt"`
	AppName   string           `json:"appName"`
	Note      string           `json:"note"`
	Stats     map[string]int64 `json:"stats"`
	Data      BundleData       `json:"data"`
}

// BundleData 详细数据载荷
type BundleData struct {
	Settings            *SettingConfig               `json:"settings,omitempty"`
	AiConfigs           []AIConfig                   `json:"aiConfigs,omitempty"`
	Skills              []models.Skill               `json:"skills,omitempty"`
	SkillEmbeddings     []SkillEmbeddingBackup       `json:"skillEmbeddings,omitempty"`
	McpServers          []models.MCPServer           `json:"mcpServers,omitempty"`
	McpServerTools      []models.MCPServerTool       `json:"mcpServerTools,omitempty"`
	AiAssistantSessions []models.AiAssistantSession  `json:"aiAssistantSessions,omitempty"`
	ChatMemories        []db.ChatMemory              `json:"chatMemories,omitempty"`
	ChatMemorySummaries []ChatMemorySummaryBackup    `json:"chatMemorySummaries,omitempty"`
	AgentMemories       []AgentMemoryBackup          `json:"agentMemories,omitempty"`
	AIReports           []models.AIResponseResult    `json:"aiReports,omitempty"`
	AIRecommendStocks   []models.AiRecommendStocks   `json:"aiRecommendStocks,omitempty"`
	FollowedStocks      []FollowedStock              `json:"followedStocks,omitempty"`
	FollowedFunds       []FollowedFund               `json:"followedFunds,omitempty"`
	StockGroups         []Group                      `json:"stockGroups,omitempty"`
	GroupStockInfo      []GroupStock                 `json:"groupStockInfo,omitempty"`
	Tags                []TagBackup                  `json:"tags,omitempty"`
	BacktestResults     []models.BacktestResult      `json:"backtestResults,omitempty"`
	CronTasks           []models.CronTask            `json:"cronTasks,omitempty"`
}

type SkillEmbeddingBackup struct {
	ID        uint   `json:"id"`
	SkillID   uint   `json:"skillId"`
	Embedding string `json:"embedding,omitempty"`
}

type ChatMemorySummaryBackup struct {
	ID        uint      `json:"id"`
	SessionID string    `json:"sessionId"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}

type AgentMemoryBackup struct {
	ID        uint      `json:"id"`
	SessionID string    `json:"sessionId"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TagBackup struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
}

// BackupManager 现代化智能数据资产备份与恢复中枢
type BackupManager struct{}

func NewBackupManager() *BackupManager {
	return &BackupManager{}
}

// EnsureBackupDir 确保权威备份目录存在并完成旧目录文件的向后兼容迁移
func EnsureBackupDir() string {
	// 优先存放在 paths.DataDir()/backups，以便 Docker 生产环境中挂载卷 ./data 完整持久化
	primaryDir := filepath.Join(paths.DataDir(), "backups")
	_ = os.MkdirAll(primaryDir, 0755)

	// 检查旧路径 paths.BaseDir()/backups，若存在且不同，自动同步软链或平滑迁移
	legacyDir := filepath.Join(paths.BaseDir(), "backups")
	if legacyDir != primaryDir {
		if entries, err := os.ReadDir(legacyDir); err == nil && len(entries) > 0 {
			for _, e := range entries {
				oldPath := filepath.Join(legacyDir, e.Name())
				newPath := filepath.Join(primaryDir, e.Name())
				if _, err := os.Stat(newPath); os.IsNotExist(err) {
					_ = copyFile(oldPath, newPath)
				}
			}
		}
	}
	return primaryDir
}

// GetBackupOverview 获取当前系统核心数据资产概况及备份历史统计
func (m *BackupManager) GetBackupOverview() (*BackupOverview, error) {
	overview := &BackupOverview{}

	db.Dao.Model(&AIConfig{}).Count(&overview.AiConfigCount)
	db.Dao.Model(&models.Skill{}).Count(&overview.SkillCount)
	db.Dao.Model(&models.MCPServer{}).Count(&overview.McpServerCount)
	db.Dao.Model(&models.AiAssistantSession{}).Count(&overview.ChatSessionCount)

	var chatMemCount, agentMemCount int64
	db.Dao.Model(&db.ChatMemory{}).Count(&chatMemCount)
	db.Dao.Table("agent_memories").Count(&agentMemCount)
	overview.MemoryCount = chatMemCount + agentMemCount

	db.Dao.Model(&models.AIResponseResult{}).Count(&overview.AiReportCount)
	db.Dao.Model(&models.AiRecommendStocks{}).Count(&overview.AiRecommendCount)
	db.Dao.Model(&FollowedStock{}).Where("is_del = 0").Count(&overview.FollowedStockCount)
	db.Dao.Model(&FollowedFund{}).Count(&overview.FollowedFundCount)
	db.Dao.Model(&models.BacktestResult{}).Count(&overview.BacktestCount)
	db.Dao.Model(&models.CronTask{}).Count(&overview.CronTaskCount)

	backups, err := m.ListBackups()
	if err == nil {
		overview.TotalBackupFiles = len(backups)
		var totalBytes int64
		for _, b := range backups {
			totalBytes += b.Size
		}
		overview.TotalBackupSizeStr = formatBytes(totalBytes)
		if len(backups) > 0 {
			overview.LastBackupTime = backups[0].TimeStr
		} else {
			overview.LastBackupTime = "暂未执行备份"
		}
	}

	return overview, nil
}

// ListBackups 列出所有历史备份，解析元数据与记录描述并按时间倒序排列
func (m *BackupManager) ListBackups() ([]BackupFileItem, error) {
	dir := EnsureBackupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var list []BackupFileItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		info, err := e.Info()
		if err != nil {
			continue
		}

		item := BackupFileItem{
			FileName:  name,
			Size:      info.Size(),
			SizeStr:   formatBytes(info.Size()),
			CreatedAt: info.ModTime(),
			TimeStr:   info.ModTime().Format("2006-01-02 15:04:05"),
		}

		switch {
		case strings.HasPrefix(name, "intel_bundle_") && strings.HasSuffix(name, ".json"):
			item.Type = "intel_bundle"
			item.TypeName = "全量智能资产包"
			item.RecordDesc = parseBundleSummary(filepath.Join(dir, name))
		case strings.HasPrefix(name, "stock_snapshot_") && strings.HasSuffix(name, ".db"):
			item.Type = "db_snapshot"
			item.TypeName = "数据库原子快照"
			item.RecordDesc = "SQLite 底层全量物理镜像，包含全部表与索引结构"
		case strings.HasSuffix(name, ".csv"):
			item.Type = "csv"
			item.TypeName = "专项CSV"
			item.RecordDesc = formatCsvDescription(name)
		default:
			item.Type = "other"
			item.TypeName = "归档文件"
			item.RecordDesc = "外部归档数据文件"
		}

		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	return list, nil
}

// CreateIntelligentBackup 一键创建全系统智能与用户资产包 (JSON 格式，自包含)
func (m *BackupManager) CreateIntelligentBackup(note string) (string, error) {
	dir := EnsureBackupDir()
	ts := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("intel_bundle_%s.json", ts)
	filePath := filepath.Join(dir, fileName)

	bundle := IntelligentBundle{
		Version:   "1.4.0",
		Format:    "blades_stock_intelligent_bundle",
		CreatedAt: time.Now(),
		AppName:   "blades-stock",
		Note:      strings.TrimSpace(note),
		Stats:     make(map[string]int64),
	}

	// 1. 系统核心配置与 AI 模型
	bundle.Data.Settings = GetSettingConfig()
	db.Dao.Find(&bundle.Data.AiConfigs)
	bundle.Stats["aiConfigs"] = int64(len(bundle.Data.AiConfigs))

	// 2. AI 技能与 MCP
	db.Dao.Find(&bundle.Data.Skills)
	bundle.Stats["skills"] = int64(len(bundle.Data.Skills))
	db.Dao.Table("skill_embeddings").Find(&bundle.Data.SkillEmbeddings)
	db.Dao.Find(&bundle.Data.McpServers)
	bundle.Stats["mcpServers"] = int64(len(bundle.Data.McpServers))
	db.Dao.Find(&bundle.Data.McpServerTools)
	bundle.Stats["mcpServerTools"] = int64(len(bundle.Data.McpServerTools))

	// 3. 智能体会话与记忆
	db.Dao.Find(&bundle.Data.AiAssistantSessions)
	bundle.Stats["chatSessions"] = int64(len(bundle.Data.AiAssistantSessions))
	db.Dao.Find(&bundle.Data.ChatMemories)
	bundle.Stats["chatMemories"] = int64(len(bundle.Data.ChatMemories))

	db.Dao.Table("chat_memory_summary").Find(&bundle.Data.ChatMemorySummaries)
	bundle.Stats["chatMemorySummaries"] = int64(len(bundle.Data.ChatMemorySummaries))

	// 读取 agent_memories (排除庞大 embedding 字节，保留纯结构和文本)
	db.Dao.Table("agent_memories").Select("id, session_id, role, content, category, summary, created_at, updated_at").Find(&bundle.Data.AgentMemories)
	bundle.Stats["agentMemories"] = int64(len(bundle.Data.AgentMemories))

	// 4. AI 深度研报与推荐
	db.Dao.Find(&bundle.Data.AIReports)
	bundle.Stats["aiReports"] = int64(len(bundle.Data.AIReports))
	db.Dao.Find(&bundle.Data.AIRecommendStocks)
	bundle.Stats["aiRecommendStocks"] = int64(len(bundle.Data.AIRecommendStocks))

	// 5. 用户自选资产
	db.Dao.Where("is_del = 0").Find(&bundle.Data.FollowedStocks)
	bundle.Stats["followedStocks"] = int64(len(bundle.Data.FollowedStocks))
	db.Dao.Find(&bundle.Data.FollowedFunds)
	bundle.Stats["followedFunds"] = int64(len(bundle.Data.FollowedFunds))
	db.Dao.Find(&bundle.Data.StockGroups)
	db.Dao.Find(&bundle.Data.GroupStockInfo)
	db.Dao.Table("tags").Find(&bundle.Data.Tags)
	bundle.Stats["tags"] = int64(len(bundle.Data.Tags))

	// 6. 量化回测与任务
	db.Dao.Find(&bundle.Data.BacktestResults)
	bundle.Stats["backtestResults"] = int64(len(bundle.Data.BacktestResults))
	db.Dao.Find(&bundle.Data.CronTasks)
	bundle.Stats["cronTasks"] = int64(len(bundle.Data.CronTasks))

	rawJSON, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化全量资产包失败: %w", err)
	}

	if err := os.WriteFile(filePath, rawJSON, 0644); err != nil {
		return "", fmt.Errorf("写入资产包文件失败: %w", err)
	}

	summary := fmt.Sprintf("✅ 成功创建全量智能资产包 %s (%s)\n包含: %d项AI模型配置, %d个AI技能, %d条对话会话, %d篇AI分析研报, %d只自选股, %d只关注基金, %d条回测记录",
		fileName, formatBytes(int64(len(rawJSON))),
		bundle.Stats["aiConfigs"], bundle.Stats["skills"],
		bundle.Stats["chatSessions"], bundle.Stats["aiReports"],
		bundle.Stats["followedStocks"], bundle.Stats["followedFunds"],
		bundle.Stats["backtestResults"],
	)
	logger.SugaredLogger.Info(summary)
	return summary, nil
}

// CreateDatabaseSnapshot 创建底层 SQLite 数据库无锁原子物理快照
func (m *BackupManager) CreateDatabaseSnapshot() (string, error) {
	dir := EnsureBackupDir()
	ts := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("stock_snapshot_%s.db", ts)
	destPath := filepath.Join(dir, fileName)

	// 1. 先触发 WAL Checkpoint，将临时日志刷入主数据库文件
	if err := db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		logger.SugaredLogger.Warnf("WAL checkpoint 预处理提示: %v", err)
	}

	// 2. 尝试使用 SQLite 3 原生 VACUUM INTO 进行原子在线镜像复制（无锁、零停机）
	vacuumCmd := fmt.Sprintf("VACUUM INTO '%s'", filepath.ToSlash(destPath))
	vacuumErr := db.Dao.Exec(vacuumCmd).Error

	if vacuumErr != nil {
		logger.SugaredLogger.Warnf("VACUUM INTO 失败，切换文件安全拷贝流: %v", vacuumErr)
		srcDBPath := filepath.Join(paths.DataDir(), "stock.db")
		if err := copyFile(srcDBPath, destPath); err != nil {
			return "", fmt.Errorf("物理拷贝数据库快照失败: %w", err)
		}
	}

	fi, err := os.Stat(destPath)
	if err != nil {
		return "", fmt.Errorf("校验快照文件失败: %w", err)
	}

	summary := fmt.Sprintf("✅ 成功创建数据库物理快照 %s (%s)\n该快照已完成 WAL 校验与物理对齐，可在灾备场景下直接用于整库替换恢复。",
		fileName, formatBytes(fi.Size()))
	logger.SugaredLogger.Info(summary)
	return summary, nil
}

// RestoreBackupFile 精确恢复指定文件名的历史备份
func (m *BackupManager) RestoreBackupFile(fileName string) (string, error) {
	cleanName := filepath.Base(fileName)
	filePath := filepath.Join(EnsureBackupDir(), cleanName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("备份文件不存在: %s", cleanName)
	}

	switch {
	case strings.HasPrefix(cleanName, "intel_bundle_") && strings.HasSuffix(cleanName, ".json"):
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("读取资产包失败: %w", err)
		}
		return m.ImportIntelligentBackup(string(data))

	case strings.HasPrefix(cleanName, "stock_snapshot_") && strings.HasSuffix(cleanName, ".db"):
		return fmt.Sprintf("ℹ️ 数据库物理快照 %s 为二进制 SQLite 镜像。\n如需还原整个库，请先停止服务，将该文件复制并重命名替换 data/stock.db 后重新启动即可。", cleanName), nil

	case strings.HasSuffix(cleanName, ".csv"):
		return restoreSpecificCSV(filePath, cleanName)

	default:
		return "", fmt.Errorf("不支持直接恢复的文件格式: %s", cleanName)
	}
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON 字符串)，执行事务安全的幂等合并与恢复
func (m *BackupManager) ImportIntelligentBackup(jsonContent string) (string, error) {
	var bundle IntelligentBundle
	if err := json.Unmarshal([]byte(jsonContent), &bundle); err != nil {
		return "", fmt.Errorf("解析智能资产包失败，非法的 JSON 格式: %w", err)
	}

	if bundle.Format != "blades_stock_intelligent_bundle" && bundle.Stats == nil {
		return "", fmt.Errorf("文件格式校验未通过，缺少智能资产包关键标识")
	}

	tx := db.Dao.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var restoredSummary strings.Builder
	restoredSummary.WriteString("🎉 智能资产包恢复完成：\n")

	// 1. 恢复 Settings
	if bundle.Data.Settings != nil {
		UpdateConfig(bundle.Data.Settings)
		restoredSummary.WriteString("  • 系统核心配置已同步生效\n")
	}

	// 2. 恢复 AI 配置
	if len(bundle.Data.AiConfigs) > 0 {
		for _, cfg := range bundle.Data.AiConfigs {
			tx.Save(&cfg)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 大模型配置: %d 项\n", len(bundle.Data.AiConfigs)))
	}

	// 3. 恢复 AI 技能与 MCP
	if len(bundle.Data.Skills) > 0 {
		for _, s := range bundle.Data.Skills {
			tx.Save(&s)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 技能/提示词: %d 个\n", len(bundle.Data.Skills)))
	}
	for _, se := range bundle.Data.SkillEmbeddings {
		tx.Table("skill_embeddings").Save(&se)
	}
	for _, ms := range bundle.Data.McpServers {
		tx.Save(&ms)
	}
	for _, mst := range bundle.Data.McpServerTools {
		tx.Save(&mst)
	}

	// 4. 恢复智能体会话与记忆
	if len(bundle.Data.AiAssistantSessions) > 0 {
		for _, sess := range bundle.Data.AiAssistantSessions {
			tx.Save(&sess)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 助手对话会话: %d 个\n", len(bundle.Data.AiAssistantSessions)))
	}
	for _, cm := range bundle.Data.ChatMemories {
		tx.Save(&cm)
	}
	for _, cms := range bundle.Data.ChatMemorySummaries {
		tx.Exec("INSERT OR REPLACE INTO chat_memory_summary (id, session_id, summary, created_at) VALUES (?, ?, ?, ?)",
			cms.ID, cms.SessionID, cms.Summary, cms.CreatedAt)
	}
	if len(bundle.Data.AgentMemories) > 0 {
		for _, am := range bundle.Data.AgentMemories {
			tx.Exec("INSERT OR REPLACE INTO agent_memories (id, session_id, role, content, category, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				am.ID, am.SessionID, am.Role, am.Content, am.Category, am.Summary, am.CreatedAt, am.UpdatedAt)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 智能体长期记忆: %d 条\n", len(bundle.Data.AgentMemories)))
	}

	// 5. 恢复 AI 研报与推荐
	if len(bundle.Data.AIReports) > 0 {
		for _, rep := range bundle.Data.AIReports {
			tx.Save(&rep)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 深度分析研报: %d 篇\n", len(bundle.Data.AIReports)))
	}
	if len(bundle.Data.AIRecommendStocks) > 0 {
		for _, rec := range bundle.Data.AIRecommendStocks {
			tx.Save(&rec)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复 AI 股票推荐记录: %d 条\n", len(bundle.Data.AIRecommendStocks)))
	}

	// 6. 恢复用户自选标的与分组
	if len(bundle.Data.FollowedStocks) > 0 {
		for _, fs := range bundle.Data.FollowedStocks {
			tx.Save(&fs)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复自选股票: %d 只\n", len(bundle.Data.FollowedStocks)))
	}
	if len(bundle.Data.FollowedFunds) > 0 {
		for _, ff := range bundle.Data.FollowedFunds {
			tx.Save(&ff)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复关注基金: %d 只\n", len(bundle.Data.FollowedFunds)))
	}
	for _, sg := range bundle.Data.StockGroups {
		tx.Save(&sg)
	}
	for _, gsi := range bundle.Data.GroupStockInfo {
		tx.Save(&gsi)
	}
	for _, t := range bundle.Data.Tags {
		tx.Exec("INSERT OR REPLACE INTO tags (id, created_at, updated_at, name, type) VALUES (?, ?, ?, ?, ?)",
			t.ID, t.CreatedAt, t.UpdatedAt, t.Name, t.Type)
	}

	// 7. 恢复量化回测与定时任务
	if len(bundle.Data.BacktestResults) > 0 {
		for _, br := range bundle.Data.BacktestResults {
			tx.Save(&br)
		}
		restoredSummary.WriteString(fmt.Sprintf("  • 恢复量化回测记录: %d 条\n", len(bundle.Data.BacktestResults)))
	}
	for _, ct := range bundle.Data.CronTasks {
		tx.Save(&ct)
	}

	if err := tx.Commit().Error; err != nil {
		return "", fmt.Errorf("事务提交失败: %w", err)
	}

	// 重新载入全局配置缓存
	_ = GetSettingConfig()

	return restoredSummary.String(), nil
}

// DeleteBackupFile 安全删除指定的备份归档
func (m *BackupManager) DeleteBackupFile(fileName string) error {
	cleanName := filepath.Base(fileName)
	target := filepath.Join(EnsureBackupDir(), cleanName)

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", cleanName)
	}
	return os.Remove(target)
}

// GetBackupContent 获取备份文件内容，用于直接流式下载
func (m *BackupManager) GetBackupContent(fileName string) (string, error) {
	cleanName := filepath.Base(fileName)
	target := filepath.Join(EnsureBackupDir(), cleanName)

	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}

	if strings.HasSuffix(cleanName, ".db") {
		// 二进制文件使用 base64 编码供前端接收
		return base64.StdEncoding.EncodeToString(data), nil
	}
	return string(data), nil
}

// -----------------------------------------------------------------------------
// 内部工具函数
// -----------------------------------------------------------------------------

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func parseBundleSummary(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "无法解析资产包元数据"
	}
	var bundle IntelligentBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return "非标准资产包结构"
	}
	s := bundle.Stats
	return fmt.Sprintf("包含: %d项AI配置, %d个技能, %d条对话, %d篇研报, %d只自选股, %d只基金",
		s["aiConfigs"], s["skills"], s["chatSessions"], s["aiReports"], s["followedStocks"], s["followedFunds"])
}

func formatCsvDescription(name string) string {
	switch {
	case strings.HasPrefix(name, "followed_stocks"):
		return "自选股票清单 (CSV 表格格式，支持 Excel 直接查阅)"
	case strings.HasPrefix(name, "recommend_stocks"):
		return "AI 推荐股票清单 (CSV 表格格式)"
	case strings.HasPrefix(name, "ai_reports"):
		return "AI 分析报告记录 (CSV 表格格式)"
	case strings.HasPrefix(name, "followed_funds"):
		return "关注基金清单 (CSV 表格格式)"
	case strings.HasPrefix(name, "backtest_results"):
		return "量化预测回测结果 (CSV 表格格式)"
	case strings.HasPrefix(name, "stock_change_history"):
		return "历史异动监控数据 (CSV 表格格式)"
	default:
		return "专项导出的 CSV 数据表格"
	}
}

func restoreSpecificCSV(filePath, fileName string) (string, error) {
	switch {
	case strings.HasPrefix(fileName, "followed_stocks"):
		return restoreFollowedStocks(filePath), nil
	case strings.HasPrefix(fileName, "recommend_stocks"):
		return restoreRecommendStocks(filePath), nil
	case strings.HasPrefix(fileName, "ai_reports"):
		return restoreAIReports(filePath), nil
	case strings.HasPrefix(fileName, "followed_funds"):
		return restoreFollowedFunds(filePath), nil
	case strings.HasPrefix(fileName, "backtest_results"):
		return restoreBacktestResults(filePath), nil
	case strings.HasPrefix(fileName, "stock_change_history"):
		return restoreStockChangeHistory(filePath), nil
	default:
		return "", fmt.Errorf("未知的 CSV 类型: %s", fileName)
	}
}
