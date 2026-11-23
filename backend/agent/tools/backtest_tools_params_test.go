package tools

import (
	"context"
	"reflect"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

// TestVectorBacktest_MissingCodeWithAliases 验证 VectorBacktest /
// VectorGridSearch 在各种键名下缺代码时都返回“缺少必需参数”而不是
// 继续跑回测：单键问题修掉后，别名形态也不应再被误判为空/非空。
// （空路径在调 quant 之前即返回，不触 DB/网络/外部 AI；
// 有代码的路径会进 quant 加载 K 线，不在此测。）
func TestVectorBacktest_MissingCodeWithAliases(t *testing.T) {
	for _, tool := range []struct {
		name string
		tool bladesTools.Tool
	}{
		{"VectorBacktest", GetVectorBacktestTool()},
		{"VectorGridSearch", GetVectorGridSearchTool()},
	} {
		for _, args := range []string{
			`{}`,
			`{"stock_code": ""}`,
			`{"stockCode": ""}`,
			`{"code": ""}`,
			`{"stock_codes": ""}`,
			`{"symbol": ""}`,
			`{"stock_code": null}`,
			`{"stock_code": "   "}`,
		} {
			out, err := tool.tool.Handle(context.Background(), args)
			if err != nil {
				t.Errorf("%s Handle(%s) err = %v", tool.name, args, err)
			}
			if out != "错误：缺少必需参数 stock_code" {
				t.Errorf("%s Handle(%s) = %q, want %q", tool.name, args, out, "错误：缺少必需参数 stock_code")
			}
		}
	}
}

// TestRunVectorizedBacktest_MissingCodes 同上：多标的回测缺代码时
// 返回“请提供 stock_codes 参数”。
func TestRunVectorizedBacktest_MissingCodes(t *testing.T) {
	tool := GetRunVectorizedBacktestTool()
	for _, args := range []string{
		`{}`,
		`{"stock_codes": ""}`,
		`{"stock_code": ""}`,
		`{"stockCode": ""}`,
		`{"code": ""}`,
		`{"codes": []}`,
		`{"stock_codes": null}`,
	} {
		out, err := tool.Handle(context.Background(), args)
		if err != nil {
			t.Errorf("Handle(%s) err = %v", args, err)
		}
		if out != "错误：请提供 stock_codes 参数" {
			t.Errorf("Handle(%s) = %q, want %q", args, out, "错误：请提供 stock_codes 参数")
		}
	}
}

// TestExpandHS300Shortcut 验证 hs300 快捷展开只在明确要求时触发：
// 旧实现用 strings.Contains(rawCodes, "300")，单只 300750 也会被
// 误触展开成 10 只权重股。
func TestExpandHS300Shortcut(t *testing.T) {
	hs300 := []string{"600519", "300750", "601318", "600036", "000858", "000333", "601899", "601012", "002594", "600900"}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"hs300 expands", []string{"hs300"}, hs300},
		{"HS300 case-insensitive", []string{"HS300"}, hs300},
		{"chinese name expands", []string{"沪深300"}, hs300},
		{"bare 300 expands", []string{"300"}, hs300},
		{"single 300750 must NOT expand", []string{"300750"}, []string{"300750"}},
		{"single 600519 unchanged", []string{"600519"}, []string{"600519"}},
		{"multi list unchanged", []string{"600519", "000001"}, []string{"600519", "000001"}},
		{"empty stays empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandHS300Shortcut(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expandHS300Shortcut(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestVectorStrategyParams 验证策略参数提取：多键名、数字字符串、
// 单元素数组兼容；缺失时不填（quant 侧用默认值）。
func TestVectorStrategyParams(t *testing.T) {
	got := vectorStrategyParams(`{"fast": 5, "slow": "20", "signal": [9], "window": 14, "num_std": 2.0, "k": 0.5, "period": 5}`)
	want := map[string]any{
		"fast": float64(5), "slow": float64(20), "signal": float64(9),
		"window": float64(14), "num_std": float64(2.0), "k": float64(0.5), "period": float64(5),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vectorStrategyParams = %v, want %v", got, want)
	}

	if got := vectorStrategyParams(`{}`); len(got) != 0 {
		t.Errorf("empty args should yield no params, got %v", got)
	}
	if got := vectorStrategyParams(`{"fast": "abc"}`); len(got) != 0 {
		t.Errorf("non-numeric fast should be dropped, got %v", got)
	}
	if got := vectorStrategyParams(`{"numStd": 2.5}`); got["num_std"] != float64(2.5) {
		t.Errorf("numStd alias should map to num_std, got %v", got)
	}
}
