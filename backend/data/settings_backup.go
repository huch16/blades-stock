package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupData 清理应用产生的中间/临时/历史数据
func (s *SettingsApi) CleanupData(cleanupType string) string {
	logger.SugaredLogger.Infof("Cleaning up data type: %s", cleanupType)

	cleanupFuncs := map[string]func() string{
		CleanupLogs:        cleanupLogs,
		CleanupCache:       cleanupCache,
		CleanupChat:        cleanupChatSessions,
		CleanupAIResults:   cleanupAIResults,
		CleanupTestImages:  cleanupTestImages,
		CleanupBrowserTemp: cleanupBrowserTemp,
	}

	if cleanupType == CleanupAll {
		var results []string
		for _, key := range []string{CleanupLogs, CleanupCache, CleanupChat, CleanupAIResults, CleanupTestImages, CleanupBrowserTemp} {
			if fn := cleanupFuncs[key]; fn != nil {
				results = append(results, fn())
			}
		}
		return strings.Join(results, "\n")
	}

	fn, ok := cleanupFuncs[cleanupType]
	if !ok {
		return "未知清理类型: " + cleanupType
	}
	return fn()
}

func cleanupLogs() string {
	logDir := "logs"
	files, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "日志目录不存在，跳过"
		}
		return "清理日志失败: " + err.Error()
	}
	var removed int
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".log") {
			path := filepath.Join(logDir, f.Name())
			if err := os.Truncate(path, 0); err != nil {
				logger.SugaredLogger.Warnf("truncate log %s failed: %v", path, err)
			} else {
				removed++
			}
		}
	}
	return fmt.Sprintf("已清理 %d 个日志文件", removed)
}

func cleanupCache() string {
	return "缓存清理已注册到应用层，请使用设置页清理按钮或重启应用"
}

func cleanupChatSessions() string {
	result := db.Dao.Exec("DELETE FROM ai_assistant_sessions")
	if result.Error != nil {
		return "清理AI对话失败: " + result.Error.Error()
	}
	return fmt.Sprintf("已清理 %d 条AI对话记录", result.RowsAffected)
}

func cleanupAIResults() string {
	result := db.Dao.Exec("DELETE FROM ai_response_result")
	if result.Error != nil {
		return "清理AI分析结果失败: " + result.Error.Error()
	}
	return fmt.Sprintf("已清理 %d 条AI分析结果", result.RowsAffected)
}

func cleanupTestImages() string {
	dir := "backend/data/test_images"
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "测试图片目录不存在，跳过"
		}
		return "清理测试图片失败: " + err.Error()
	}
	var removed int
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		path := filepath.Join(dir, f.Name())
		if err := os.Remove(path); err != nil {
			logger.SugaredLogger.Warnf("remove test image %s failed: %v", path, err)
		} else {
			removed++
		}
	}
	return fmt.Sprintf("已清理 %d 个测试图片", removed)
}

func cleanupBrowserTemp() string {
	tempDir := os.TempDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "清理浏览器临时目录失败: " + err.Error()
	}
	var removed int
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "chromedp") {
			path := filepath.Join(tempDir, e.Name())
			if err := os.RemoveAll(path); err != nil {
				logger.SugaredLogger.Warnf("remove chromedp temp dir %s failed: %v", path, err)
			} else {
				removed++
			}
		}
	}
	return fmt.Sprintf("已清理 %d 个浏览器临时目录", removed)
}

func backupDirPath() string {
	return EnsureBackupDir()
}

// GetBackupOverview 获取系统数据资产概览

// GetBackupOverview 获取系统数据资产概览
func (s *SettingsApi) GetBackupOverview() (*BackupOverview, error) {
	return NewBackupManager().GetBackupOverview()
}

// ListBackups 列出所有历史备份归档

// ListBackups 列出所有历史备份归档
func (s *SettingsApi) ListBackups() ([]BackupFileItem, error) {
	return NewBackupManager().ListBackups()
}

// CreateIntelligentBackup 一键创建全量智能资产备份包

// CreateIntelligentBackup 一键创建全量智能资产备份包
func (s *SettingsApi) CreateIntelligentBackup(note string) string {
	res, err := NewBackupManager().CreateIntelligentBackup(note)
	if err != nil {
		return "❌ 备份失败: " + err.Error()
	}
	return res
}

// CreateDatabaseSnapshot 一键创建底层数据库物理快照

// CreateDatabaseSnapshot 一键创建底层数据库物理快照
func (s *SettingsApi) CreateDatabaseSnapshot() string {
	res, err := NewBackupManager().CreateDatabaseSnapshot()
	if err != nil {
		return "❌ 创建快照失败: " + err.Error()
	}
	return res
}

// RestoreBackupFile 精确恢复指定文件名的备份

// RestoreBackupFile 精确恢复指定文件名的备份
func (s *SettingsApi) RestoreBackupFile(fileName string) string {
	res, err := NewBackupManager().RestoreBackupFile(fileName)
	if err != nil {
		return "❌ 恢复失败: " + err.Error()
	}
	return res
}

// DeleteBackupFile 删除指定的历史备份文件

// DeleteBackupFile 删除指定的历史备份文件
func (s *SettingsApi) DeleteBackupFile(fileName string) string {
	err := NewBackupManager().DeleteBackupFile(fileName)
	if err != nil {
		return "❌ 删除失败: " + err.Error()
	}
	return "✅ 已成功删除备份文件: " + fileName
}

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）
func (s *SettingsApi) GetBackupContent(fileName string) string {
	content, err := NewBackupManager().GetBackupContent(fileName)
	if err != nil {
		return "ERROR:" + err.Error()
	}
	return content
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入
func (s *SettingsApi) ImportIntelligentBackup(content string) string {
	res, err := NewBackupManager().ImportIntelligentBackup(content)
	if err != nil {
		return "❌ 导入恢复失败: " + err.Error()
	}
	return res
}

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验
func (s *SettingsApi) VerifyBackupIntegrity(fileName string) string {
	err := NewBackupManager().VerifyBackupIntegrity(fileName)
	if err != nil {
		return fmt.Sprintf("❌ 校验失败: %v", err)
	}
	return "✅ 校验通过：文件结构完整合法，数据表与索引一致性良好"
}

// PruneOldBackups 智能滚动修剪过期的历史备份

// PruneOldBackups 智能滚动修剪过期的历史备份
func (s *SettingsApi) PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays int) string {
	pruned, err := NewBackupManager().PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays)
	if err != nil {
		return fmt.Sprintf("❌ 清理失败: %v", err)
	}
	return fmt.Sprintf("✅ 历史备份清理完成，共自动回收 %d 个陈旧/超额备份文件", pruned)
}

// BackupData 导出用户数据到 CSV，返回生成的文件路径

// BackupData 导出用户数据到 CSV，返回生成的文件路径
func (s *SettingsApi) BackupData(backupType string) string {
	logger.SugaredLogger.Infof("Backing up data type: %s", backupType)

	backupDir := backupDirPath()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "❌ 创建备份目录失败: " + err.Error()
	}

	ts := time.Now().Format("20060102_150405")
	baseFileName := fmt.Sprintf("%s_%s.csv", backupType, ts)
	fileName := filepath.Join(backupDir, baseFileName)

	var result string
	switch backupType {
	case BackupFollowedStocks:
		result = backupFollowedStocks(fileName)
	case BackupRecommendStocks:
		result = backupRecommendStocks(fileName)
	case BackupAIReports:
		result = backupAIReports(fileName)
	case BackupFollowedFunds:
		result = backupFollowedFunds(fileName)
	case BackupBacktestResults:
		result = backupBacktestResults(fileName)
	case BackupStockChangeHistory:
		result = backupStockChangeHistory(fileName)
	default:
		return "❌ 未知备份类型: " + backupType
	}
	if strings.Contains(result, "失败") {
		return "❌ " + result
	}
	return fmt.Sprintf("✅ %s\nFILE:%s\n备份目录: %s", result, baseFileName, backupDir)
}

// RestoreData 从 CSV 文件恢复数据到数据库

// RestoreData 从 CSV 文件恢复数据到数据库
func (s *SettingsApi) RestoreData(backupType string) string {
	logger.SugaredLogger.Infof("Restoring data type: %s", backupType)

	backupDir := backupDirPath()
	pattern := filepath.Join(backupDir, backupType+"_*.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "❌ 查找备份文件失败: " + err.Error()
	}
	if len(matches) == 0 {
		return "❌ 未找到 " + backupType + " 的备份文件，请先执行备份\n备份目录: " + backupDir
	}

	// 使用最新的文件
	fileName := matches[len(matches)-1]

	var res string
	switch backupType {
	case BackupFollowedStocks:
		res = restoreFollowedStocks(fileName)
	case BackupRecommendStocks:
		res = restoreRecommendStocks(fileName)
	case BackupAIReports:
		res = restoreAIReports(fileName)
	case BackupFollowedFunds:
		res = restoreFollowedFunds(fileName)
	case BackupBacktestResults:
		res = restoreBacktestResults(fileName)
	case BackupStockChangeHistory:
		res = restoreStockChangeHistory(fileName)
	default:
		return "❌ 未知恢复类型: " + backupType
	}
	if strings.Contains(res, "失败") {
		return "❌ " + res
	}
	return "✅ " + res
}

func backupFollowedStocks(fileName string) string {
	rows, err := db.Dao.Raw("SELECT stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price FROM followed_stock WHERE is_del=0 ORDER BY sort, stock_code").Rows()
	if err != nil {
		return "导出自选股票失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"股票代码", "名称", "持仓量", "成本价", "当前价", "涨跌幅%", "预警涨跌幅%", "预警价", "时间", "排序", "AI配置ID", "入场价", "止盈价", "止损价"})
}

func backupRecommendStocks(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, data_time, model_name, rating, stock_code, stock_name, bk_name, stock_price, stock_current_price, stock_current_price_time, stock_close_price, stock_pre_price, recommend_reason, recommend_buy_price, recommend_buy_price_min, recommend_buy_price_max, recommend_stop_profit_price, recommend_stop_profit_price_min, recommend_stop_profit_price_max, recommend_stop_loss_price, risk_remarks, remarks, enable_alert FROM ai_recommend_stocks ORDER BY id DESC").Rows()
	if err != nil {
		return "导出推荐股票失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "数据时间", "模型", "评级", "股票代码", "股票名称", "板块", "推荐价", "当前价", "当前价时间", "收盘价", "昨收", "推荐理由", "建议买入价", "建议买入最低价", "建议买入最高价", "建议止盈价", "建议止盈最低价", "建议止盈最高价", "建议止损价", "风险提示", "备注", "启用提醒"})
}

func backupAIReports(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, model_name, stock_code, stock_name, question, content, is_del FROM ai_response_result ORDER BY id DESC").Rows()
	if err != nil {
		return "导出AI分析报告失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "模型", "股票代码", "股票名称", "问题", "内容", "是否删除"})
}

func backupFollowedFunds(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, code, name, net_unit_value, net_unit_value_date, net_estimated_unit, net_estimated_time, net_accumulated, net_estimated_rate, net_unit_value_prev, net_actual_rate FROM followed_fund ORDER BY id DESC").Rows()
	if err != nil {
		return "导出关注基金失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "基金代码", "基金名称", "单位净值", "净值日期", "估算净值", "估算时间", "累计净值", "估算涨跌幅", "昨日净值", "实际涨跌幅"})
}

func backupBacktestResults(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, created_at, report_id, stock_code, stock_name, analysis_date, direction, target_price, verify_date, verify_price, price_change, max_drawdown, max_upside, is_accurate, confidence, summary FROM backtest_results ORDER BY id DESC").Rows()
	if err != nil {
		return "导出回测结果失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "创建时间", "报告ID", "股票代码", "股票名称", "分析日期", "方向", "目标价", "验证日期", "验证价", "价格变化", "最大回撤", "最大上涨", "是否准确", "置信度", "摘要"})
}

func backupStockChangeHistory(fileName string) string {
	rows, err := db.Dao.Raw("SELECT id, change_date, change_time, stock_code, stock_name, market, change_type, type_name, volume, price, change_rate, amount, industry, concept FROM stock_change_history ORDER BY id DESC LIMIT 10000").Rows()
	if err != nil {
		return "导出异动历史失败: " + err.Error()
	}
	defer rows.Close()
	return dumpRowsToCSV(fileName, rows, []string{"ID", "日期", "时间", "股票代码", "股票名称", "市场", "异动类型", "类型名", "成交量", "价格", "涨跌幅%", "金额", "行业", "概念"})
}

func dumpRowsToCSV(fileName string, rows *sql.Rows, headers []string) string {
	f, err := os.Create(fileName)
	if err != nil {
		return "创建备份文件失败: " + err.Error()
	}
	defer f.Close()

	// UTF-8 BOM for Excel
	f.Write([]byte{0xEF, 0xBB, 0xBF})

	columns, err := rows.Columns()
	if err != nil {
		return "读取列信息失败: " + err.Error()
	}

	w := csv.NewWriter(f)
	w.Write(headers)

	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var count int
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			logger.SugaredLogger.Warnf("scan row failed: %v", err)
			continue
		}
		record := make([]string, len(columns))
		for i, v := range values {
			record[i] = fmt.Sprintf("%v", v)
		}
		w.Write(record)
		count++
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "写入CSV失败: " + err.Error()
	}
	return fmt.Sprintf("已备份 %d 条记录到 %s", count, fileName)
}

func readCSV(fileName string) ([]map[string]string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	allRows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(allRows) < 2 {
		return nil, nil
	}
	headers := allRows[0]
	var records []map[string]string
	for _, row := range allRows[1:] {
		rec := make(map[string]string)
		for i, h := range headers {
			if i < len(row) {
				rec[h] = row[i]
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

func restoreFollowedStocks(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		code := strings.TrimSpace(r["股票代码"])
		if code == "" {
			continue
		}
		var count int64
		tx.Model(&FollowedStock{}).Where("stock_code = ?", code).Count(&count)
		if count > 0 {
			err := tx.Model(&FollowedStock{}).Where("stock_code = ?", code).Updates(map[string]any{
				"name":                 r["名称"],
				"volume":               r["持仓量"],
				"cost_price":           r["成本价"],
				"price":                r["当前价"],
				"change_percent":       r["涨跌幅%"],
				"alarm_change_percent": r["预警涨跌幅%"],
				"alarm_price":          r["预警价"],
				"time":                 r["时间"],
				"sort":                 r["排序"],
				"ai_config_id":         r["AI配置ID"],
				"entry_price":          r["入场价"],
				"take_profit_price":    r["止盈价"],
				"stop_loss_price":      r["止损价"],
				"is_del":               0,
			}).Error
			if err != nil {
				tx.Rollback()
				return "恢复自选股票失败: " + err.Error()
			}
		} else {
			err := tx.Exec("INSERT INTO followed_stock (stock_code, name, volume, cost_price, price, change_percent, alarm_change_percent, alarm_price, time, sort, ai_config_id, entry_price, take_profit_price, stop_loss_price, is_del) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,0)",
				code, r["名称"], r["持仓量"], r["成本价"], r["当前价"], r["涨跌幅%"], r["预警涨跌幅%"], r["预警价"], r["时间"], r["排序"], r["AI配置ID"], r["入场价"], r["止盈价"], r["止损价"],
			).Error
			if err != nil {
				tx.Rollback()
				return "恢复自选股票失败: " + err.Error()
			}
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条自选股票记录", len(records))
}

func restoreRecommendStocks(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO ai_recommend_stocks (id, data_time, model_name, rating, stock_code, stock_name, bk_name, stock_price, stock_current_price, stock_current_price_time, stock_close_price, stock_pre_price, recommend_reason, recommend_buy_price, recommend_buy_price_min, recommend_buy_price_max, recommend_stop_profit_price, recommend_stop_profit_price_min, recommend_stop_profit_price_max, recommend_stop_loss_price, risk_remarks, remarks, enable_alert) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["数据时间"], r["模型"], r["评级"], r["股票代码"], r["股票名称"], r["板块"], r["推荐价"], r["当前价"], r["当前价时间"], r["收盘价"], r["昨收"], r["推荐理由"], r["建议买入价"], r["建议买入最低价"], r["建议买入最高价"], r["建议止盈价"], r["建议止盈最低价"], r["建议止盈最高价"], r["建议止损价"], r["风险提示"], r["备注"], r["启用提醒"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复推荐股票失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条推荐股票记录", len(records))
}

func restoreAIReports(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO ai_response_result (id, created_at, model_name, stock_code, stock_name, question, content, is_del) VALUES (?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["模型"], r["股票代码"], r["股票名称"], r["问题"], r["内容"], r["是否删除"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复AI分析报告失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条AI分析报告", len(records))
}

func restoreFollowedFunds(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO followed_fund (id, created_at, code, name, net_unit_value, net_unit_value_date, net_estimated_unit, net_estimated_time, net_accumulated, net_estimated_rate, net_unit_value_prev, net_actual_rate) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["基金代码"], r["基金名称"], r["单位净值"], r["净值日期"], r["估算净值"], r["估算时间"], r["累计净值"], r["估算涨跌幅"], r["昨日净值"], r["实际涨跌幅"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复关注基金失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条关注基金记录", len(records))
}

func restoreBacktestResults(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO backtest_results (id, created_at, report_id, stock_code, stock_name, analysis_date, direction, target_price, verify_date, verify_price, price_change, max_drawdown, max_upside, is_accurate, confidence, summary) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["创建时间"], r["报告ID"], r["股票代码"], r["股票名称"], r["分析日期"], r["方向"], r["目标价"], r["验证日期"], r["验证价"], r["价格变化"], r["最大回撤"], r["最大上涨"], r["是否准确"], r["置信度"], r["摘要"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复回测结果失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条回测结果记录", len(records))
}

func restoreStockChangeHistory(fileName string) string {
	records, err := readCSV(fileName)
	if err != nil {
		return "读取备份文件失败: " + err.Error()
	}
	if len(records) == 0 {
		return "备份文件为空，无数据可恢复"
	}
	tx := db.Dao.Begin()
	for _, r := range records {
		err := tx.Exec("INSERT OR REPLACE INTO stock_change_history (id, change_date, change_time, stock_code, stock_name, market, change_type, type_name, volume, price, change_rate, amount, industry, concept) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r["ID"], r["日期"], r["时间"], r["股票代码"], r["股票名称"], r["市场"], r["异动类型"], r["类型名"], r["成交量"], r["价格"], r["涨跌幅%"], r["金额"], r["行业"], r["概念"],
		).Error
		if err != nil {
			tx.Rollback()
			return "恢复异动历史失败: " + err.Error()
		}
	}
	tx.Commit()
	return fmt.Sprintf("已恢复 %d 条异动历史记录", len(records))
}
