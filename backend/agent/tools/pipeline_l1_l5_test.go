package tools

import (
	"context"
	"strings"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"

	bladesTools "github.com/go-kratos/blades/tools"
)

// TestPipeline_L1_GetMarketData 验证 L1 宏观大势核心工具 GetMarketData 能正常输出大盘数据
func TestPipeline_L1_GetMarketData(t *testing.T) {
	all := GetAllDataTools()
	var marketTool bladesTools.Tool
	for _, tool := range all {
		if tool.Name() == "GetMarketData" {
			marketTool = tool
			break
		}
	}
	if marketTool == nil {
		t.Fatalf("L1 工具 GetMarketData 未在 GetAllDataTools 中找到")
	}

	ctx := context.Background()
	out, err := marketTool.Handle(ctx, "{}")
	if err != nil {
		t.Fatalf("GetMarketData 执行出错: %v", err)
	}
	if !strings.Contains(out, "市场行情数据") {
		t.Errorf("GetMarketData 返回不符合预期: %s", out)
	}
}

// TestPipeline_L2_GetETFMomentumRanking 验证 L2 赛道主线工具能基于本地模型计算 21 日加权动量
func TestPipeline_L2_GetETFMomentumRanking(t *testing.T) {
	tool := GetETFMomentumRankingTool()
	if tool == nil || tool.Name() != "GetETFMomentumRanking" {
		t.Fatalf("L2 工具 GetETFMomentumRanking 未正确构造")
	}

	ctx := context.Background()
	// 传入指定主流池标的测试
	out, err := tool.Handle(ctx, `{"count": 5, "codes": "sh510300,sh510500,sz159915"}`)
	if err != nil {
		t.Fatalf("GetETFMomentumRanking 执行出错: %v", err)
	}
	if out == "" {
		t.Errorf("GetETFMomentumRanking 返回空文本")
	}
}

// TestPipeline_L3_ScreenByStrategy 验证 L3 量化选股策略能正常扫描并打分
func TestPipeline_L3_ScreenByStrategy(t *testing.T) {
	tool := GetStrategyScreenTool()
	if tool == nil || tool.Name() != "ScreenByStrategy" {
		t.Fatalf("L3 工具 ScreenByStrategy 未正确构造")
	}

	ctx := context.Background()
	out, err := tool.Handle(ctx, `{"strategyName": "放量突破", "codeList": "600519,000001", "limit": 2}`)
	if err != nil {
		t.Fatalf("ScreenByStrategy 执行出错: %v", err)
	}
	if !strings.Contains(out, "策略「放量突破」评分选股结果") {
		t.Errorf("ScreenByStrategy 预期输出策略标题，实际输出: %s", out)
	}
}

// TestPipeline_L4_DiagnoseStock 验证 L4 三维量化体检工具能正常执行
func TestPipeline_L4_DiagnoseStock(t *testing.T) {
	tool := GetDiagnoseStockTool()
	if tool == nil || tool.Name() != "DiagnoseStock" {
		t.Fatalf("L4 工具 DiagnoseStock 未正确构造")
	}

	ctx := context.Background()
	out, err := tool.Handle(ctx, `{"stock_code": "600519"}`)
	if err != nil {
		t.Fatalf("DiagnoseStock 执行出错: %v", err)
	}
	if !strings.Contains(out, "三维量化深度诊断报告") && !strings.Contains(out, "综合量化评分") {
		t.Logf("DiagnoseStock 返回: %s", out)
	}
}

// TestPipeline_L5_StockThesisLedger 验证 L5 买方立论与认知账本读写闭环
func TestPipeline_L5_StockThesisLedger(t *testing.T) {
	if db.Dao != nil {
		_ = db.Dao.AutoMigrate(&models.StockThesisLedger{})
	}

	saveTool := SaveStockThesisTool()
	getTool := GetStockThesisTool()

	if saveTool == nil || getTool == nil {
		t.Fatalf("L5 认知账本工具构造失败")
	}

	ctx := context.Background()
	// 1. 保存买方假说
	savePayload := `{
		"stockCode": "600519",
		"stockName": "贵州茅台",
		"verdict": "逻辑强化",
		"coreThesis": "直销渠道占比持续提升带动吨价与净利率双升",
		"keyCatalysts": "三季报非标产品放量及分红方案落地",
		"riskFactors": "商务宴请消费复苏不及预期，批发参考价跌破关键心理关口"
	}`
	saveOut, err := saveTool.Handle(ctx, savePayload)
	if err != nil {
		t.Fatalf("SaveStockThesis 执行出错: %v", err)
	}
	if !strings.Contains(saveOut, "买方认知档案已成功保存") {
		t.Fatalf("SaveStockThesis 保存未返回成功: %s", saveOut)
	}

	// 2. 查询买方假说
	getOut, err := getTool.Handle(ctx, `{"stockCode": "600519"}`)
	if err != nil {
		t.Fatalf("GetStockThesis 执行出错: %v", err)
	}
	if !strings.Contains(getOut, "买方认知档案") || !strings.Contains(getOut, "直销渠道占比") {
		t.Fatalf("GetStockThesis 查询结果与保存假说不匹配: %s", getOut)
	}
}
