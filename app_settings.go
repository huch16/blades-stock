package main

import (
	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"

	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	runtime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetMachineId() string {
	return a.settingsService.GetMachineId()
}

func (a *App) CheckDeviceBinding(token string, apiBase string) map[string]any {
	return a.settingsService.CheckDeviceBinding(token, apiBase)
}

func (a *App) QuitApp() {
	if a.ctx != nil {
		if a.cron != nil {
			a.cron.Stop()
		}
		runtime.Quit(a.ctx)
	}
}

// domReady is called after front-end resources have been loaded

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	defer PanicHandler()

	// 初始化 VPS 数据服务
	go func() {
		defer PanicHandler()
		// 交易日历（AData → Baostock → 周末判断）
		data.InitTradingCalendar()

		// AData 健康检查
		if err := data.NewAdataClient().Ping(); err != nil {
			logger.SugaredLogger.Warnf("AData 服务连接失败: %v", err)
		}

		// AKShare 健康检查
		if err := data.NewAkshareClient().Ping(); err != nil {
			logger.SugaredLogger.Warnf("AKShare 服务连接失败: %v", err)
		}
	}()

	defer func() {
		// 增加延迟确保前端已准备好接收事件
		go func() {
			defer PanicHandler()
			time.Sleep(2 * time.Second)
			runtime.EventsEmit(a.ctx, "loadingMsg", "done")
		}()
	}()

	//if stocksBin != nil && len(stocksBin) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查A股基础信息...")
	//	go initStockData(a.ctx)
	//}
	//
	//if stocksBinHK != nil && len(stocksBinHK) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查港股基础信息...")
	//	go initStockDataHK(a.ctx)
	//}
	//
	//if stocksBinUS != nil && len(stocksBinUS) > 0 {
	//	go runtime.EventsEmit(a.ctx, "loadingMsg", "检查美股基础信息...")
	//	go initStockDataUS(a.ctx)
	//}
	updateBasicInfo()

	// Add your action here
	//定时更新数据
	config := data.GetSettingConfig()
	go func() {
		defer PanicHandler()
		go func() { defer PanicHandler(); data.NewMarketNewsApi().TelegraphList(30) }()
		go func() { defer PanicHandler(); data.NewMarketNewsApi().GetSinaNews(30) }()
		go func() { defer PanicHandler(); data.NewMarketNewsApi().TradingViewNews() }()

		interval := config.RefreshInterval
		// 工业级风控防雪崩约束：行情监控最小周期限制为 >=3 秒，彻底根除 1 秒短轮询堵死网络与引发浏览器卡顿
		if interval < 3 {
			interval = 3
		}
		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
			MonitorStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorStockPrices", id)
		}
		entryID, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			//news := data.NewMarketNewsApi().GetNewTelegraph(30)
			news := data.NewMarketNewsApi().TelegraphList(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newTelegraph", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("GetNewTelegraph", entryID)
		}

		entryIDSina, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().GetSinaNews(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newSinaNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("newSinaNews", entryIDSina)
		}

		entryIDTradingViewNews, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().TradingViewNews()
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "tradingViewNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("tradingViewNews", entryIDTradingViewNews)
		}
	}()

	//刷新基金净值信息
	go func() {
		defer PanicHandler()
		//ticker := time.NewTicker(time.Second * time.Duration(60))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorFundPrices(a)
		//}
		if config.EnableFund {
			id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
				MonitorFundPrices(a)
			})
			if err != nil {
				logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
			} else {
				a.setCronEntry("MonitorFundPrices", id)
			}
		}

		// AI 推荐股票价格监控定时器
		idAiStock, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorAiRecommendStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorAiRecommendStockPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorAiRecommendStockPrices", idAiStock)
		}

		// 自选股成本价监控定时器
		idCostPrice, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorFollowedStockCostPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorFollowedStockCostPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorFollowedStockCostPrices", idCostPrice)
		}

	}()

	if config.EnableNews {
		//go func() {
		//	ticker := time.NewTicker(time.Second * time.Duration(60))
		//	defer ticker.Stop()
		//	for range ticker.C {
		//		telegraph := refreshTelegraphList()
		//		if telegraph != nil {
		//			go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
		//		}
		//	}
		//
		//}()

		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			telegraph := refreshTelegraphList()
			if telegraph != nil {
				go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("refreshTelegraphList", id)
		}

		go runtime.EventsEmit(a.ctx, "telegraph", refreshTelegraphList())
	}
	go MonitorStockPrices(a)
	if config.EnableFund {
		go MonitorFundPrices(a)
		go data.NewFundApi().AllFund()
	}
	// AI 推荐股票价格监控
	go MonitorAiRecommendStockPrices(a)
	// 自选股成本价监控
	go MonitorFollowedStockCostPrices(a)
	// 市场统计数据采集（交易日每5分钟）
	go func() {
		defer PanicHandler()
		a.FetchAndSaveMarketStatistic()
		idMarketStat, err := a.cron.AddFunc("0 */5 9-15 * * 1-5", func() {
			a.FetchAndSaveMarketStatistic()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc FetchAndSaveMarketStatistic error:%s", err.Error())
		} else {
			a.setCronEntry("FetchAndSaveMarketStatistic", idMarketStat)
		}
	}()
	// 板块资金流向数据采集（交易日每60秒）
	go func() {
		defer PanicHandler()
		data.NewBKFundFlowApi().FetchAndSave()
		idBKFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewBKFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc BKFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("BKFundFlowFetchAndSave", idBKFundFlow)
		}
	}()
	// 概念资金流向数据采集（交易日每60秒）
	go func() {
		defer PanicHandler()
		data.NewConceptFundFlowApi().FetchAndSave()
		idConceptFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewConceptFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc ConceptFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("ConceptFundFlowFetchAndSave", idConceptFundFlow)
		}
	}()
	//检查股票基础信息
	go func() {
		defer PanicHandler()
		cfg := a.GetConfig()
		var count int64
		db.Dao.Model(&data.StockBasic{}).Count(&count)

		// 仅在首次启动无数据（count == 0）或用户开启「启动时更新基础信息」时，才在启动期拉取全量
		if count == 0 || (cfg != nil && cfg.UpdateBasicInfoOnStart) {
			logger.SugaredLogger.Infof("启动期按策略同步全市场标的基础数据 (count=%d, updateOnStart=%v)...", count, cfg != nil && cfg.UpdateBasicInfoOnStart)
			// 串行执行，杜绝多个后台协程同时执行全量清空与批量写入引发 SQLite BUSY 死锁
			a.CheckStockBaseInfo(a.ctx)
			syncAllStockInfo(a.ctx)
		} else {
			logger.SugaredLogger.Infof("本地已有股票基础数据 (%d 条)，跳过启动期重复全量同步", count)
		}

		a.cron.AddFunc("0 0 2 * * *", func() {
			logger.SugaredLogger.Infof("定时维护：更新股票基础数据...")
			a.CheckStockBaseInfo(a.ctx)
		})
		a.cron.AddFunc("30 05 8,12,20 * * *", func() {
			syncAllStockInfo(a.ctx)
		})
		// 进化闭环：每日从高质量轨迹提取技能草稿（7 天限频，仅记录供人工审阅）。
		a.cron.AddFunc("30 2 * * *", agent.RunSkillExtractionPass)
		// 历史回测与战绩复盘闭环：每个交易日收盘后自动回测 AI 报告的预测准确度与收益率
		a.cron.AddFunc("30 15 * * 1-5", func() {
			logger.SugaredLogger.Infof("盘后触发 AI 报告历史回测与胜率检验...")
			mgr := quant.NewBacktestManager()
			if res, err := mgr.RunAIReportBacktest(context.Background(), 100); err == nil && res.Processed > 0 {
				logger.SugaredLogger.Infof("盘后 AI 报告回测完成: 处理 %d 份，准确 %d (%.1f%%)",
					res.Processed, res.Accurate, res.AccuracyRate*100)
			}
		})
	}()

	//检查谷歌浏览器
	//go func() {
	//	f := checkChromeOnWindows()
	//	if !f {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "谷歌浏览器未安装,ai分析功能可能无法使用")
	//	}
	//}()

	//检查Edge浏览器
	//go func() {
	//	path, e := checkEdgeOnWindows()
	//	if !e {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "Edge浏览器未安装,ai分析功能可能无法使用")
	//	} else {
	//		logger.SugaredLogger.Infof("Edge浏览器已安装，路径为: %s", path)
	//	}
	//}()
	followList := data.NewStockDataApi().GetFollowList(0)
	for _, follow := range *followList {
		if follow.Cron == nil || *follow.Cron == "" {
			continue
		}
		entryID, err := a.cron.AddFunc(*follow.Cron, a.AddCronTask(follow))
		if err != nil {
			logger.SugaredLogger.Errorf("添加自动分析任务失败:%s cron=%s entryID:%v", follow.Name, *follow.Cron, entryID)
			continue
		}
		a.setCronEntry(follow.StockCode, entryID)
	}
	//logger.SugaredLogger.Infof("domReady-cronEntrys:%+v", a.cronEntrys)

}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// 窗口大小已在 beforeClose 中保存（app_windows.go / app_linux.go）。
	// 这里不再调用 runtime.WindowGetSize —— 在 Wails 内部控件销毁阶段调用
	// 会在 winc.ScaleToDefaultDPI 触发 panic，导致关窗卡死 / 进程残留。
	if a.cron != nil {
		a.cron.Stop()
	}
	if db.Dao != nil {
		if err := db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			logger.SugaredLogger.Warnf("shutdown WAL checkpoint error: %v", err)
		} else {
			logger.SugaredLogger.Info("shutdown WAL checkpoint ok")
		}
	}
}

// BatchGreet returns stock info for multiple stock codes in a single call

// BatchGreet returns stock info for multiple stock codes in a single call
func (a *App) BatchGreet(stockCodes []string) []*data.StockInfo {
	return a.marketService.BatchGreet(stockCodes)
}

// Greet returns a greeting for the given name

// Greet returns a greeting for the given name
func (a *App) Greet(stockCode string) *data.StockInfo {
	return a.marketService.Greet(stockCode)
}

func (a *App) GetVersionInfo() *models.VersionInfo {
	return &models.VersionInfo{
		Version:           Version,
		Icon:              GetImageBase(icon),
		Alipay:            GetImageBase(alipay),
		Wxpay:             GetImageBase(wxpay),
		Content:           VersionCommit,
		OfficialStatement: OFFICIAL_STATEMENT,
	}
}

func (a *App) GetUserManual() string {
	return string(userManual)
}

//// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
//func checkChromeOnWindows() bool {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	_, _, err = key.GetValue("Path", nil)
//	return err == nil
//}
//
//// checkEdgeOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
//func checkEdgeOnWindows() (string, bool) {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return "", false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	path, _, err := key.GetStringValue("Path")
//	if err != nil {
//		return "", false
//	}
//	return path, true
//}

func GetImageBase(bytes []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
}

func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	//s1, _ := json.Marshal(settingConfig)
	//logger.SugaredLogger.Infof("UpdateConfig:%s", s1)
	if settingConfig.RefreshInterval > 0 {
		interval := settingConfig.RefreshInterval
		if interval < 3 {
			interval = 3
		}
		if entryID, exists := a.getCronEntry("MonitorStockPrices"); exists {
			a.cron.Remove(entryID)
		}
		id, _ := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
			MonitorStockPrices(a)
		})
		a.setCronEntry("MonitorStockPrices", id)
	}

	return data.UpdateConfig(settingConfig)
}

func (a *App) GetConfig() *data.SettingConfig {
	return a.settingsService.GetConfig()
}

// GetAiConfigTemplates 返回预设的 AI 配置模板，供前端向导使用

// GetAiConfigTemplates 返回预设的 AI 配置模板，供前端向导使用
func (a *App) GetAiConfigTemplates() []*data.AiConfigTemplate {
	return a.settingsService.GetAiConfigTemplates()
}

func (a *App) ExportConfig() string {
	config := data.NewSettingsApi().Export()
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "导出配置文件",
		CanCreateDirectories: true,
		DefaultFilename:      "blades-stock-config.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return "导出失败: " + err.Error()
	}
	if file == "" {
		return "取消导出"
	}
	err = os.WriteFile(file, []byte(config), os.ModePerm)
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return "导出失败: " + err.Error()
	}
	return "导出成功:" + file
}

// CleanupData 暴露给前端的清理入口

// CleanupData 暴露给前端的清理入口
func (a *App) CleanupData(cleanupType string) string {
	return a.settingsService.CleanupData(cleanupType)
}

// BackupData 暴露给前端的数据备份入口 (兼容旧版 CSV 导出)

// BackupData 暴露给前端的数据备份入口 (兼容旧版 CSV 导出)
func (a *App) BackupData(backupType string) string {
	return a.settingsService.BackupData(backupType)
}

// RestoreData 暴露给前端的恢复入口 (兼容旧版 CSV 恢复)

// RestoreData 暴露给前端的恢复入口 (兼容旧版 CSV 恢复)
func (a *App) RestoreData(backupType string) string {
	return a.settingsService.RestoreData(backupType)
}

// GetBackupOverview 获取系统数据资产概览

// GetBackupOverview 获取系统数据资产概览
func (a *App) GetBackupOverview() (*data.BackupOverview, error) {
	return a.settingsService.GetBackupOverview()
}

// ListBackups 列出所有历史备份归档

// ListBackups 列出所有历史备份归档
func (a *App) ListBackups() ([]data.BackupFileItem, error) {
	return a.settingsService.ListBackups()
}

// CreateIntelligentBackup 一键创建全量智能资产备份包

// CreateIntelligentBackup 一键创建全量智能资产备份包
func (a *App) CreateIntelligentBackup(note string) string {
	return a.settingsService.CreateIntelligentBackup(note)
}

// CreateDatabaseSnapshot 一键创建底层数据库物理快照

// CreateDatabaseSnapshot 一键创建底层数据库物理快照
func (a *App) CreateDatabaseSnapshot() string {
	return a.settingsService.CreateDatabaseSnapshot()
}

// RestoreBackupFile 精确恢复指定文件名的备份

// RestoreBackupFile 精确恢复指定文件名的备份
func (a *App) RestoreBackupFile(fileName string) string {
	return a.settingsService.RestoreBackupFile(fileName)
}

// DeleteBackupFile 删除指定的历史备份文件

// DeleteBackupFile 删除指定的历史备份文件
func (a *App) DeleteBackupFile(fileName string) string {
	return a.settingsService.DeleteBackupFile(fileName)
}

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）

// GetBackupContent 获取备份文件内容（用于前端直接下载保存）
func (a *App) GetBackupContent(fileName string) string {
	return a.settingsService.GetBackupContent(fileName)
}

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入

// ImportIntelligentBackup 导入全量智能资产包 (JSON) 并安全合并写入
func (a *App) ImportIntelligentBackup(content string) string {
	return a.settingsService.ImportIntelligentBackup(content)
}

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验

// VerifyBackupIntegrity 对指定的备份文件执行工业级完整性与格式有效性校验
func (a *App) VerifyBackupIntegrity(fileName string) string {
	return a.settingsService.VerifyBackupIntegrity(fileName)
}

// PruneOldBackups 智能滚动修剪过期的历史备份

// PruneOldBackups 智能滚动修剪过期的历史备份
func (a *App) PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays int) string {
	return a.settingsService.PruneOldBackups(maxSnapshots, maxBundles, maxAgeDays)
}

func (a *App) ShareAnalysis(stockCode, stockName string) string {
	return a.settingsService.ShareAnalysis(stockCode, stockName)
}

// ShareText 直接把文本分享到社区（已废弃：本项目无社区服务）

// ShareText 直接把文本分享到社区（已废弃：本项目无社区服务）
func (a *App) ShareText(text, title string) string {
	return a.settingsService.ShareText(text, title)
}

func (a *App) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
		file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "保存为Markdown",
			DefaultFilename: fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime),
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown",
					Pattern:     "*.md;*.markdown",
				},
			},
		})
		if err != nil {
			return err.Error()
		}
		if file == "" {
			return "取消保存"
		}
		if err = os.WriteFile(file, []byte(res.Content), 0644); err != nil {
			return "保存失败：" + err.Error()
		}
		return "已保存至：" + file
	}
	return "分析结果异常,无法保存。"
}

// @Description:  跨平台打开默认浏览器（增强协议校验与 SSRF 拦截）
// @receiver a
// @param url
func (a *App) OpenURL(rawURL string) {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return
	}
	u, err := url.Parse(cleanURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		logger.SugaredLogger.Warnf("[DesktopApp] OpenURL rejected non-http(s) protocol: %s", cleanURL)
		return
	}
	if u.Hostname() == "169.254.169.254" || strings.HasPrefix(u.Hostname(), "169.254.") {
		logger.SugaredLogger.Warnf("[DesktopApp] OpenURL rejected cloud metadata IP: %s", cleanURL)
		return
	}
	runtime.BrowserOpenURL(a.ctx, cleanURL)
}

// SaveImage
//
//	@Description: 跨平台保存图片
//	@receiver a
//	@param name
//	@param base64Data
//	@return error

// @Description: 跨平台保存图片
// @receiver a
// @param name
// @param base64Data
// @return error
func (a *App) SaveImage(name, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: name + "AI分析.png",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG 图片",
				Pattern:     "*.png",
			},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}

	err = os.WriteFile(filepath.Clean(filePath), decodeString, os.ModePerm)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// SaveWordFile
//
//	@Description: // 跨平台保存word
//	@receiver a
//	@param filename
//	@param base64Data
//	@return error

// @Description: // 跨平台保存word
// @receiver a
// @param filename
// @param base64Data
// @return error
func (a *App) SaveWordFile(filename string, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Word 文件",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Word 文件", Pattern: "*.docx"},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}
	err = os.WriteFile(filepath.Clean(filePath), decodeString, 0777)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// GetAiConfigs
//
//	@Description: // 获取 AiConfig 列表
//	@receiver a
//	@return error
