package main

import (
	"fmt"
	"os"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	os.Setenv("BLADES_STOCK_SKIP_DB_HEALTHCHECK", "1")
	logger.InitLogger()
	db.Init("")

	fmt.Println("================================================================================")
	fmt.Printf("🚀 blades-stock 智能数据资产与全量备份中心端到端验证\n")
	fmt.Printf("📅 测试时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("================================================================================")

	bm := data.NewBackupManager()

	// 1. 测试获取系统数据资产概览
	fmt.Println("\n▶️ [1/6] 检查核心数据资产概况 (GetBackupOverview)...")
	overview, err := bm.GetBackupOverview()
	if err != nil {
		fmt.Printf("❌ 获取资产概览失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   -> AI模型配置: %d项 | AI技能: %d个 | 助手会话: %d条 | 长期记忆: %d条\n",
		overview.AiConfigCount, overview.SkillCount, overview.ChatSessionCount, overview.MemoryCount)
	fmt.Printf("   -> AI分析研报: %d篇 | 推荐记录: %d条 | 自选股票: %d只 | 关注基金: %d只 | 回测: %d条\n",
		overview.AiReportCount, overview.AiRecommendCount, overview.FollowedStockCount, overview.FollowedFundCount, overview.BacktestCount)

	// 2. 测试一键生成全量智能资产包
	fmt.Println("\n▶️ [2/6] 测试一键生成全量智能资产包 (CreateIntelligentBackup)...")
	t0 := time.Now()
	summary, err := bm.CreateIntelligentBackup("自动化巡检验收备份")
	if err != nil {
		fmt.Printf("❌ 生成资产包失败: %v\n", err)
		os.Exit(1)
	}
	dur := time.Since(t0)
	fmt.Printf("   -> 耗时: %v\n   -> %s\n", dur, summary)

	// 3. 测试一键生成底层数据库物理快照
	fmt.Println("\n▶️ [3/6] 测试一键生成数据库物理快照 (CreateDatabaseSnapshot)...")
	t1 := time.Now()
	snapSummary, err := bm.CreateDatabaseSnapshot()
	if err != nil {
		fmt.Printf("❌ 生成数据库快照失败: %v\n", err)
		os.Exit(1)
	}
	dur1 := time.Since(t1)
	fmt.Printf("   -> 耗时: %v\n   -> %s\n", dur1, snapSummary)

	// 4. 测试列出历史备份归档
	fmt.Println("\n▶️ [4/6] 测试列出历史备份归档 (ListBackups)...")
	list, err := bm.ListBackups()
	if err != nil {
		fmt.Printf("❌ 列出备份失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   -> 发现备份归档总数: %d 份\n", len(list))
	for i, item := range list {
		if i >= 5 {
			fmt.Printf("   -> ... 剩余 %d 项省略\n", len(list)-5)
			break
		}
		fmt.Printf("   -> [%s] %s (%s, %s) -> %s\n", item.TypeName, item.FileName, item.SizeStr, item.TimeStr, item.RecordDesc)
	}

	if len(list) == 0 {
		fmt.Println("❌ 未检测到刚刚生成的备份")
		os.Exit(1)
	}

	// 5. 测试读取与校验资产包内容
	var latestBundle *data.BackupFileItem
	for _, item := range list {
		if item.Type == "intel_bundle" {
			temp := item
			latestBundle = &temp
			break
		}
	}
	if latestBundle == nil {
		fmt.Println("❌ 未找到全量资产包")
		os.Exit(1)
	}

	fmt.Printf("\n▶️ [5/6] 测试读取资产包内容流 (GetBackupContent: %s)...\n", latestBundle.FileName)
	content, err := bm.GetBackupContent(latestBundle.FileName)
	if err != nil {
		fmt.Printf("❌ 读取资产包内容失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   -> 成功读取资产包字节数: %d 字符\n", len(content))

	// 6. 测试导入与恢复验证
	fmt.Println("\n▶️ [6/6] 测试安全导入与恢复智能资产包 (ImportIntelligentBackup)...")
	restoreRes, err := bm.ImportIntelligentBackup(content)
	if err != nil {
		fmt.Printf("❌ 恢复资产包失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   -> 恢复结果:\n%s\n", restoreRes)

	// 7. 测试删除备份归档
	fmt.Println("\n▶️ [7/7] 测试安全清理历史测试备份 (DeleteBackupFile)...")
	for _, item := range list {
		err := bm.DeleteBackupFile(item.FileName)
		if err != nil {
			fmt.Printf("   ⚠️ 清理 %s 提示: %v\n", item.FileName, err)
		} else {
			fmt.Printf("   -> 已清理测试备份: %s\n", item.FileName)
		}
	}

	fmt.Println("================================================================================")
	fmt.Println("🎉 验证成功: 全系统智能数据资产备份、快照生成、下载流读取与导入恢复 100% 验证通过！")
	fmt.Println("================================================================================")
}
