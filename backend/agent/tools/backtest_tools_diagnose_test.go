package tools

import (
	"context"
	"testing"
)

// TestExtractDiagnoseStockCode 回归测试：线上实测 L4 体检 601318 多次返回
// “请提供股票或ETF代码”。实测证明根因不在 601318 本身——字符串、JSON 数字、
// 带 .SH/SH 前缀都能解析；根因是工具只认 stock_code 一个键，而模型在同
// 一会话里见到的键名并不统一（SaveStockThesis 等用 stockCode，
// RunVectorizedBacktest 用 stock_codes），用错键名即被误判为空。
// 本测试只测纯参数解析，不触 DB/网络/外部 AI。
func TestExtractDiagnoseStockCode(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"canonical string 601318", `{"stock_code": "601318"}`, "601318"},
		{"canonical string 000001 keeps leading zeros", `{"stock_code": "000001"}`, "000001"},
		{"JSON number 601318", `{"stock_code": 601318}`, "601318"},
		{"JSON number 601318.0", `{"stock_code": 601318.0}`, "601318"},
		{"suffix 601318.SH", `{"stock_code": "601318.SH"}`, "601318.SH"},
		{"prefix SH601318", `{"stock_code": "SH601318"}`, "SH601318"},
		{"shenzhen 000001", `{"stock_code": "000001"}`, "000001"},
		{"ETF 510300", `{"stock_code": "510300"}`, "510300"},
		{"camelCase stockCode", `{"stockCode": "601318"}`, "601318"},
		{"bare code key", `{"code": "601318"}`, "601318"},
		{"plural stock_codes", `{"stock_codes": "601318"}`, "601318"},
		{"symbol key", `{"symbol": "601318"}`, "601318"},
		{"ticker key", `{"ticker": "601318"}`, "601318"},
		{"single-element array", `{"stock_code": ["601318"]}`, "601318"},
		{"whitespace trimmed", `{"stock_code": "  601318  "}`, "601318"},
		{"stock_code wins over alias", `{"stock_code": "601318", "stockCode": "000001"}`, "601318"},
		{"empty stock_code falls back to alias", `{"stock_code": "", "stockCode": "601318"}`, "601318"},
		{"missing entirely", `{}`, ""},
		{"empty string", `{"stock_code": ""}`, ""},
		{"null", `{"stock_code": null}`, ""},
		{"alias empty", `{"stockCode": ""}`, ""},
		{"alias null", `{"stockCode": null}`, ""},
		{"unrelated keys only", `{"strategy": "MA"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractDiagnoseStockCode(tc.args); got != tc.want {
				t.Errorf("extractDiagnoseStockCode(%s) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// TestDiagnoseStockTool_StillEmptyWithoutCode 仍为空的形态必须继续返回
// “请提供股票或ETF代码”。这些分支在调用 quant 之前即返回，不触 DB/网络。
// （别名形态已能解析出代码，会继续走 quant.DiagnoseStock，故不在此测 Handle。）
func TestDiagnoseStockTool_StillEmptyWithoutCode(t *testing.T) {
	tool := GetDiagnoseStockTool()
	for _, args := range []string{
		`{}`,
		`{"stock_code": ""}`,
		`{"stock_code": null}`,
		`{"stockCode": ""}`,
		`{"code": ""}`,
		`{"stock_codes": ""}`,
		`{"stock_code": "   "}`,
	} {
		out, err := tool.Handle(context.Background(), args)
		if err != nil {
			t.Errorf("Handle(%s) err = %v", args, err)
		}
		if out != "请提供股票或ETF代码" {
			t.Errorf("Handle(%s) = %q, want %q", args, out, "请提供股票或ETF代码")
		}
	}
}
