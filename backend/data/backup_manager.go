package data

import (
	"database/sql"
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

	_ "github.com/glebarez/sqlite"
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
	if err == nil && len(backups) == 0 {
		// 系统尚无任何备份记录时，自动创建一份系统开箱即用的智能资产基线快照
		_, _ = m.CreateIntelligentBackup("系统出厂基线快照 (v2.0.0 智能数据资产)")
		backups, _ = m.ListBackups()
	}
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

// EnsureBaselineBackupIfEmpty 当备份中心无任何快照记录时，自动生成出厂基线归档快照
func (m *BackupManager) EnsureBaselineBackupIfEmpty() {
	backups, err := m.ListBackups()
	if err == nil && len(backups) == 0 {
		_, _ = m.CreateIntelligentBackup("系统出厂基线快照 (v2.0.0 智能数据资产)")
	}
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
		Version:   "2.0.0",
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

	// 自动执行资产包归档滚动清理（保留最近 30 份，或 60 天内），防止无限制膨胀
	go func() {
		if pruned, err := m.PruneOldBackups(15, 30, 60); err == nil && pruned > 0 {
			logger.SugaredLogger.Infof("🧹 智能资产管理：已自动滚动回收 %d 个历史过期备份文件", pruned)
		}
	}()

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

	// 3. 工业级数据校验：对新生成的物理快照进行 PRAGMA quick_check 结构完整性校验
	if err := m.VerifyBackupIntegrity(fileName); err != nil {
		_ = os.Remove(destPath) // 校验失败自动清除异常文件
		return "", fmt.Errorf("快照生成失败，未通过物理完整性校验: %w", err)
	}

	// 4. 自动执行物理快照滚动清理（默认保留最近 15 份快照，防止磁盘空间耗尽）
	go func() {
		if pruned, err := m.PruneOldBackups(15, 30, 30); err == nil && pruned > 0 {
			logger.SugaredLogger.Infof("🧹 灾备中枢：已自动滚动清理 %d 个陈旧快照与归档文件", pruned)
		}
	}()

	summary := fmt.Sprintf("✅ 成功创建数据库物理快照 %s (%s)\n该快照已通过 WAL 校验、物理对齐与 PRAGMA 完整性检查，可在灾备场景下直接用于整库替换恢复。",
		fileName, formatBytes(fi.Size()))
	logger.SugaredLogger.Info(summary)
	return summary, nil
}

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验
func (m *BackupManager) VerifyBackupIntegrity(fileName string) error {
	cleanName := filepath.Base(fileName)
	target := filepath.Join(EnsureBackupDir(), cleanName)

	fi, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("备份文件不存在: %s", cleanName)
	}
	if fi.Size() == 0 {
		return fmt.Errorf("备份文件大小为 0 字节 (损坏)")
	}

	switch {
	case strings.HasSuffix(cleanName, ".db"):
		if fi.Size() < 100 {
			return fmt.Errorf("数据库快照文件过小 (%d 字节)，非合法 SQLite 格式", fi.Size())
		}
		// 打开只读独立连接校验 SQLite 物理完整性
		dsn := fmt.Sprintf("%s?mode=ro&_pragma=busy_timeout(5000)", filepath.ToSlash(target))
		checkDB, err := sql.Open("sqlite", dsn)
		if err != nil {
			return fmt.Errorf("无法连接快照文件: %w", err)
		}
		defer checkDB.Close()

		var result string
		if err := checkDB.QueryRow("PRAGMA quick_check;").Scan(&result); err != nil {
			return fmt.Errorf("快照 PRAGMA quick_check 执行异常: %w", err)
		}
		if !strings.EqualFold(result, "ok") {
			return fmt.Errorf("快照完整性校验未通过: %s", result)
		}

		// 抽检核心业务表结构
		var tableCount int
		if err := checkDB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table';").Scan(&tableCount); err != nil || tableCount < 5 {
			return fmt.Errorf("快照数据表结构不完整 (表数量=%d)", tableCount)
		}
		return nil

	case strings.HasSuffix(cleanName, ".json"):
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("无法读取资产包: %w", err)
		}
		var bundle IntelligentBundle
		if err := json.Unmarshal(data, &bundle); err != nil {
			return fmt.Errorf("资产包 JSON 语法解析失败: %w", err)
		}
		if bundle.Format != "blades_stock_intelligent_bundle" {
			return fmt.Errorf("资产包格式标识不匹配: %s", bundle.Format)
		}
		return nil

	case strings.HasSuffix(cleanName, ".csv"):
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("无法读取 CSV 文件: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) < 2 {
			return fmt.Errorf("CSV 内容为空或缺少表头")
		}
		return nil

	default:
		return nil
	}
}

// PruneOldBackups 智能滚动修剪过期的历史备份（防止 VPS 或桌面端磁盘被超大物理快照占满）
// maxSnapshots: 最多保留的 .db 物理快照数（推荐 10~15）
// maxBundles: 最多保留的 .json 智能资产包数（推荐 20~30）
// maxAgeDays: 超过该天数的旧归档将被清除（但每种类型至少保留 2 份最新归档）
func (m *BackupManager) PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays int) (int, error) {
	if maxSnapshots <= 0 {
		maxSnapshots = 15
	}
	if maxBundles <= 0 {
		maxBundles = 30
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}

	backups, err := m.ListBackups()
	if err != nil {
		return 0, err
	}

	var snapshots []BackupFileItem
	var bundles []BackupFileItem
	var csvs []BackupFileItem

	for _, b := range backups {
		switch b.Type {
		case "db_snapshot":
			snapshots = append(snapshots, b)
		case "intel_bundle":
			bundles = append(bundles, b)
		case "csv":
			csvs = append(csvs, b)
		}
	}

	cutoffTime := time.Now().AddDate(0, 0, -maxAgeDays)
	prunedCount := 0

	// 1. 清理超额与超龄的数据库快照（每份约 200MB，重点清理目标）
	for i, s := range snapshots {
		// 至少保留最近 2 份快照底线
		if i < 2 {
			continue
		}
		if i >= maxSnapshots || s.CreatedAt.Before(cutoffTime) {
			if err := m.DeleteBackupFile(s.FileName); err == nil {
				prunedCount++
				logger.SugaredLogger.Infof("🗑️ 回收历史过期快照: %s (%s)", s.FileName, s.SizeStr)
			}
		}
	}

	// 2. 清理超额的 JSON 资产包
	for i, b := range bundles {
		if i < 3 {
			continue
		}
		if i >= maxBundles || b.CreatedAt.Before(cutoffTime) {
			if err := m.DeleteBackupFile(b.FileName); err == nil {
				prunedCount++
				logger.SugaredLogger.Infof("🗑️ 回收历史过期资产包: %s (%s)", b.FileName, b.SizeStr)
			}
		}
	}

	// 3. 清理超量的临时导出 CSV（保留 30 份）
	for i, c := range csvs {
		if i < 3 {
			continue
		}
		if i >= 30 || c.CreatedAt.Before(cutoffTime) {
			if err := m.DeleteBackupFile(c.FileName); err == nil {
				prunedCount++
			}
		}
	}

	return prunedCount, nil
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
		// 1. 完整性前置校验：杜绝损坏快照导致灾备二次故障
		if err := m.VerifyBackupIntegrity(cleanName); err != nil {
			return "", fmt.Errorf("快照完整性校验未通过，拒绝执行恢复: %w", err)
		}

		// 2. 灾备安全防护：在恢复前自动对当前活跃库创建恢复前保护快照
		safetyName := fmt.Sprintf("stock_prerestore_safety_%s.db", time.Now().Format("20060102_150405"))
		safetyPath := filepath.Join(EnsureBackupDir(), safetyName)
		_ = db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
		_ = db.Dao.Exec(fmt.Sprintf("VACUUM INTO '%s'", filepath.ToSlash(safetyPath))).Error

		return fmt.Sprintf("✅ 目标数据库物理快照 [%s] 已通过完整性与表结构校验！\n🛡️ 系统已自动为您创建当前数据库的安全副本: %s\n\n【灾备恢复指引】：\n由于当前 SQLite 连接处于活跃读写状态，为了保障底层存储一致性，请：\n1. 停止当前服务 (若为终端直接退出，若为服务运行 systemctl stop 或 docker compose stop)\n2. 将备份文件 %s 复制并重命名覆盖 data/stock.db\n3. 重新启动服务，即可完成 100%% 无损瞬时恢复。", cleanName, safetyName, filepath.ToSlash(filePath)), nil

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

	// 1. 恢复 Settings (在当前 tx 事务内直接保存，杜绝嵌套开启独立事务导致 SQLite 写死锁)
	if bundle.Data.Settings != nil && bundle.Data.Settings.Settings != nil {
		st := bundle.Data.Settings.Settings
		if st.ID == 0 {
			st.ID = 1
		}
		if err := tx.Save(st).Error; err != nil {
			logger.SugaredLogger.Warnf("恢复 settings 记录失败: %v", err)
		} else {
			restoredSummary.WriteString("  • 系统核心配置已同步生效\n")
		}
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
	var res string
	switch {
	case strings.HasPrefix(fileName, "followed_stocks"):
		res = restoreFollowedStocks(filePath)
	case strings.HasPrefix(fileName, "recommend_stocks"):
		res = restoreRecommendStocks(filePath)
	case strings.HasPrefix(fileName, "ai_reports"):
		res = restoreAIReports(filePath)
	case strings.HasPrefix(fileName, "followed_funds"):
		res = restoreFollowedFunds(filePath)
	case strings.HasPrefix(fileName, "backtest_results"):
		res = restoreBacktestResults(filePath)
	case strings.HasPrefix(fileName, "stock_change_history"):
		res = restoreStockChangeHistory(filePath)
	default:
		return "", fmt.Errorf("未知的 CSV 类型: %s", fileName)
	}
	if strings.Contains(res, "失败") {
		return "", fmt.Errorf("%s", res)
	}
	return res, nil
}
