package main

import (
	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"fmt"
	"strings"

	"github.com/duke-git/lancet/v2/convertor"
	cron "github.com/robfig/cron/v3"
	lo "github.com/samber/lo"
	runtime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setCronEntry(key string, id cron.EntryID) {
	a.cronEntrysMu.Lock()
	a.cronEntrys[key] = id
	a.cronEntrysMu.Unlock()
}

func (a *App) getCronEntry(key string) (cron.EntryID, bool) {
	a.cronEntrysMu.Lock()
	id, exists := a.cronEntrys[key]
	a.cronEntrysMu.Unlock()
	return id, exists
}

func (a *App) AddCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(a.ctx, "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		question := fmt.Sprintf("全面分析%s(%s)的技术面、基本面、消息面和资金面", follow.Name, follow.StockCode)
		var res strings.Builder
		thinking := data.GetSettingConfig().GetAIConfigThinking(follow.AiConfigId)
		for msg := range agent.NewStockAiAgentApi().ChatWithContext(a.ctx, question, follow.AiConfigId, nil, false, 0, thinking, "") {
			if msg == nil {
				continue
			}
			if msg.ReasoningContent != "" {
				res.WriteString(msg.ReasoningContent)
			}
			res.WriteString(msg.Content)
		}
		data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), "", question)
		go runtime.EventsEmit(a.ctx, "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)
	}
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	cronApi := agent.NewCronTaskApi()
	if !cronApi.ExistsByTaskType("stock_change_save") {
		task := &models.CronTask{
			Name:        "异动数据保存",
			CronExpr:    "0 */10 * * * *",
			TaskType:    "stock_change_save",
			Enable:      true,
			Status:      "active",
			Description: "每10分钟自动保存A股异动数据（火箭发射、快速反弹、大笔买入、封涨停板等），交易时间外自动跳过",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建异动数据保存任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建异动数据保存定时任务")
		}
	}

	// 盘前智能前瞻晨报: 每个交易日 09:10 生成
	if !cronApi.ExistsByTaskType("pre_market_briefing") {
		task := &models.CronTask{
			Name:        "盘前智能前瞻晨报",
			CronExpr:    "0 10 9 * * 1-5",
			TaskType:    "pre_market_briefing",
			Enable:      true,
			Status:      "active",
			Description: "交易日 09:10 自动生成盘前智能前瞻晨报，含隔夜外盘、核心要闻与今日策略指引",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建盘前晨报任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建盘前智能前瞻晨报任务")
		}
	}

	// 盘后收盘复盘: 每个交易日 15:05 自动触发
	if !cronApi.ExistsByTaskType("post_market_review") {
		task := &models.CronTask{
			Name:        "盘后收盘深度复盘",
			CronExpr:    "0 5 15 * * 1-5",
			TaskType:    "post_market_review",
			Enable:      true,
			Status:      "active",
			Description: "交易日 15:05 自动生成盘后收盘复盘报告，含资金筹码博弈、AI预测回测验证与次日推演",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建收盘复盘任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建盘后收盘深度复盘任务")
		}
	}
	tasks := cronApi.GetAll()
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		taskCopy := t
		entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
			err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
			if err != nil {
				logger.SugaredLogger.Errorf("启动任务失败：%v %s", err, taskCopy.Name)
				return
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建定时任务失败：%v %s", err, taskCopy.Name)
			continue
		}
		a.setCronEntry(convertor.ToString(taskCopy.ID)+"_"+taskCopy.Name, entryID)
	}
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答

// @Description: 创建定时任务
// @receiver a
// @param task 定时任务信息
// @return string 操作结果
func (a *App) CreateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Create(task)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return "任务创建成功,但定时失败"
	}
	return "创建成功"
}

func (a *App) UpdateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Update(task)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	if entryID, exists := a.getCronEntry(convertor.ToString(task.ID) + "_" + task.Name); exists {
		a.cron.Remove(entryID)
	}
	taskCopy := *task
	entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

// DeleteCronTask
//
//	@Description: 删除定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果

// @Description: 删除定时任务
// @receiver a
// @param id 任务 ID
// @return string 操作结果
func (a *App) DeleteCronTask(id uint) string {
	_ = agent.NewCronTaskApi().Delete(id)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
	}
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

// GetCronTaskByID
//
//	@Description: 根据 ID 获取定时任务
//	@receiver a
//	@param id 任务 ID
//	@return *models.CronTask 任务信息

// @Description: 根据 ID 获取定时任务
// @receiver a
// @param id 任务 ID
// @return *models.CronTask 任务信息
func (a *App) GetCronTaskByID(id uint) *models.CronTask {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return nil
	}
	return task
}

// GetCronTaskList
//
//	@Description: 获取定时任务列表
//	@receiver a
//	@param query 查询条件
//	@return *models.CronTaskPageResp 分页结果

// @Description: 获取定时任务列表
// @receiver a
// @param query 查询条件
// @return *models.CronTaskPageResp 分页结果
func (a *App) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	return agent.NewCronTaskApi().List(query)
}

// EnableCronTask
//
//	@Description: 启用/禁用定时任务
//	@receiver a

// @Description: 启用/禁用定时任务
// @receiver a
func (a *App) EnableCronTask(id uint, enable bool) string {
	_ = agent.NewCronTaskApi().EnableTask(id, enable)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
		if enable {
			taskCopy := *task
			entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
				err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
				if err != nil {
					logger.SugaredLogger.Errorf("%s 执行任务失败：%v", taskCopy.Name, err)
					return
				}
			})
			a.setCronEntry(convertor.ToString(id)+"_"+task.Name, entryID)
			if err != nil {
				return "操作成功,但定时失败"
			}
		}

	}
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

// ExecuteCronTaskNow
//
//	@Description: 立即执行定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果

// @Description: 立即执行定时任务
// @receiver a
// @param id 任务 ID
// @return string 操作结果
func (a *App) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}

	go func() {
		defer PanicHandler()
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
		}
	}()

	return "任务执行中"
}

// GetCronTaskTypes
//
//	@Description: 获取所有任务类型
//	@receiver a
//	@return []lo.Tuple2[string, string] 任务类型列表

// @Description: 获取所有任务类型
// @receiver a
// @return []lo.Tuple2[string, string] 任务类型列表
func (a *App) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

// ValidateCronExpr
//
//	@Description: 验证 Cron 表达式
//	@receiver a
//	@param expr Cron 表达式
//	@return string 验证结果

// @Description: 验证 Cron 表达式
// @receiver a
// @param expr Cron 表达式
// @return string 验证结果
func (a *App) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

// GetMarketLifecycleStatus
//
//	@Description: 获取市场生命周期状态（盘前/集合竞价/早盘/午休/下午盘/盘后/休市）
//	@receiver a
//	@return agent.MarketLifecycleStatus

// @Description: 搜索定时任务
// @receiver a
// @param keyword 搜索关键词
// @return []models.CronTask 搜索结果
func (a *App) SearchCronTasks(keyword string) []models.CronTask {
	return agent.NewCronTaskApi().SearchTasks(keyword)
}

// CalculateNextRunTime 根据 Cron 表达式计算下一次运行时间
// 参数:
//   - cron: Cron 表达式，用于定义任务调度的时间规则
//
// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串

// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串
func (a *App) CalculateNextRunTime(cron string) string {
	nextRunTime := agent.NewCronTaskApi().CalculateNextRunTime(cron)
	return nextRunTime.Format("2006-01-02 15:04:05")
}

// CalculateNextRunTimes 根据 Cron 表达式计算未来多次运行时间
// 参数:
//   - cron: Cron 表达式
//   - count: 需要计算的次数
//
// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"

// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"
func (a *App) CalculateNextRunTimes(cron string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cron, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// AddTradingRecord 添加交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - uint: 新添加的交易记录ID
//   - error: 错误信息
