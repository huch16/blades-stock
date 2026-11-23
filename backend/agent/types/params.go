package types

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// 工具调用参数提取 helper（本轮新增）。
//
// 背景：模型在同一会话里见到的参数键名并不统一——DiagnoseStock 只认
// stock_code、SaveStockThesis/GetStockThesis 与游资/筹码工具用 stockCode、
// RunVectorizedBacktest 用 stock_codes（线上实测 601318 L4 体检因此被
// 误判为空，见 backtest_tools_diagnose_test.go）。本文件提供唯一的
// 参数提取入口：多键名按优先级回落 + JSON 数字/单元素数组兼容，各
// 工具复用它，而不是每个工具各写一份 gjson.Get(args, "…")。
//
// 约定：
//   - 键名按传入顺序尝试，第一个非空值胜出；空字符串/null/空数组
//     都会继续回落到下一个键名；
//   - 字符串值统一 TrimSpace；JSON 数字按字面量取字符串（601318.0
//     → "601318"，前导零只在字符串形态下保留）；
//   - 数组取第一个非空元素（字符串或数字），其余忽略。

// StockCodeKeys 是单只股票/ETF 代码接受的键名（按优先级）。
// 与第三轮 extractDiagnoseStockCode 的键序一致，首位 stock_code 不动。
var StockCodeKeys = []string{"stock_code", "stockCode", "code", "stock_codes", "symbol", "ticker"}

// StockCodesKeys 是多只股票代码列表接受的键名（按优先级）：复数键在前，
// 单数键在后（单只值回落为单元素列表）。
var StockCodesKeys = []string{
	"stock_codes", "stockCodes", "codeList", "codes", "symbols",
	"stock_list", "prodCodes",
	"stock_code", "stockCode", "code", "symbol", "ticker",
}

// FundCodeKeys 是基金代码接受的键名（按优先级）。
var FundCodeKeys = []string{"fundCode", "fund_code", "code", "symbol"}

// DateKeys 是日期参数接受的键名（snake_case 与 camelCase 互为别名）。
var DateKeys = []string{"date", "targetDate", "startDate", "start_date", "day"}

// resultToString 把一个 gjson 结果转成去空白的字符串：字符串取原文，
// 数字取字面量，数组取第一个非空元素；对象/null/空值返回 ""。
func resultToString(res gjson.Result) string {
	if res.IsArray() {
		for _, el := range res.Array() {
			if s := resultToString(el); s != "" {
				return s
			}
		}
		return ""
	}
	switch res.Type {
	case gjson.String, gjson.Number, gjson.True, gjson.False:
		return strings.TrimSpace(res.String())
	default:
		return ""
	}
}

// ExtractStringParam 按 keys 顺序取第一个非空字符串参数。
func ExtractStringParam(args string, keys ...string) string {
	for _, key := range keys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if s := resultToString(res); s != "" {
			return s
		}
	}
	return ""
}

// splitListValue 把逗号/中文逗号/分号/空白分隔的列表字符串拆成干净的元素。
// 代码类值不含空白，按空白切分是安全的（如 "600519 000001"）。
func splitListValue(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ExtractStringListParam 按 keys 顺序取第一个非空列表参数：接受逗号分隔
// 字符串、JSON 字符串/数字数组，或单个值（回落为单元素列表）。
func ExtractStringListParam(args string, keys ...string) []string {
	for _, key := range keys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if res.IsArray() {
			var out []string
			for _, el := range res.Array() {
				if s := resultToString(el); s != "" {
					out = append(out, splitListValue(s)...)
				}
			}
			if len(out) > 0 {
				return out
			}
			continue
		}
		if s := resultToString(res); s != "" {
			return splitListValue(s)
		}
	}
	return nil
}

// ExtractFloatParam 按 keys 顺序取第一个数字参数：接受 JSON 数字、
// 数字字符串（"10.5"）或单元素数组；取不到或解析失败返回 false。
func ExtractFloatParam(args string, keys ...string) (float64, bool) {
	for _, key := range keys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if res.IsArray() {
			arr := res.Array()
			if len(arr) == 0 {
				continue
			}
			res = arr[0]
		}
		switch res.Type {
		case gjson.Number:
			return res.Float(), true
		case gjson.String:
			s := strings.TrimSpace(res.String())
			if s == "" {
				continue
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// ExtractIntParam 按 keys 顺序取第一个整数参数：接受 JSON 数字、
// 数字字符串（"10"、"10.7" → 10）或单元素数组；取不到返回 false。
func ExtractIntParam(args string, keys ...string) (int, bool) {
	for _, key := range keys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if res.IsArray() {
			arr := res.Array()
			if len(arr) == 0 {
				continue
			}
			res = arr[0]
		}
		switch res.Type {
		case gjson.Number:
			return int(res.Int()), true
		case gjson.String:
			s := strings.TrimSpace(res.String())
			if s == "" {
				continue
			}
			if n, err := strconv.Atoi(s); err == nil {
				return n, true
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return int(f), true
			}
		}
	}
	return 0, false
}

// ExtractBoolParam 按 keys 顺序取第一个布尔参数：接受 JSON 布尔值、
// "true"/"false"/"1"/"0" 字符串、非零数字或单元素数组；取不到返回 false。
func ExtractBoolParam(args string, keys ...string) (bool, bool) {
	for _, key := range keys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if res.IsArray() {
			arr := res.Array()
			if len(arr) == 0 {
				continue
			}
			res = arr[0]
		}
		switch res.Type {
		case gjson.True, gjson.False:
			return res.Bool(), true
		case gjson.Number:
			return res.Int() != 0, true
		case gjson.String:
			s := strings.ToLower(strings.TrimSpace(res.String()))
			if s == "" {
				continue
			}
			if b, err := strconv.ParseBool(s); err == nil {
				return b, true
			}
		}
	}
	return false, false
}

// ExtractStockCode 提取单只股票/ETF 代码：多键名回落（见 StockCodeKeys），
// 兼容 JSON 数字与单元素数组；若值是逗号列表只取第一只（单标的工具
// 从不要一串代码，取首只比把整串当代码更不易误判）。
func ExtractStockCode(args string) string {
	s := ExtractStringParam(args, StockCodeKeys...)
	if s == "" {
		return ""
	}
	if parts := splitListValue(s); len(parts) > 0 {
		return parts[0]
	}
	return s
}

// ExtractStockCodes 提取多只股票代码列表：多键名回落（见 StockCodesKeys），
// 兼容逗号字符串、JSON 数组与单个值。
func ExtractStockCodes(args string) []string {
	return ExtractStringListParam(args, StockCodesKeys...)
}

// ExtractFundCode 提取单只基金代码：多键名回落（见 FundCodeKeys），
// 兼容 JSON 数字与单元素数组。
func ExtractFundCode(args string) string {
	return ExtractStringParam(args, FundCodeKeys...)
}
