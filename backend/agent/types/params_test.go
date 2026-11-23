package types

import (
	"reflect"
	"testing"
)

func TestExtractStringParam(t *testing.T) {
	cases := []struct {
		name string
		args string
		keys []string
		want string
	}{
		{"canonical", `{"stock_code": "601318"}`, StockCodeKeys, "601318"},
		{"leading zeros kept", `{"stock_code": "000001"}`, StockCodeKeys, "000001"},
		{"JSON number", `{"stock_code": 601318}`, StockCodeKeys, "601318"},
		{"JSON float number", `{"stock_code": 601318.0}`, StockCodeKeys, "601318"},
		{"camelCase alias", `{"stockCode": "601318"}`, StockCodeKeys, "601318"},
		{"bare code alias", `{"code": "601318"}`, StockCodeKeys, "601318"},
		{"plural alias", `{"stock_codes": "601318"}`, StockCodeKeys, "601318"},
		{"symbol alias", `{"symbol": "601318"}`, StockCodeKeys, "601318"},
		{"ticker alias", `{"ticker": "601318"}`, StockCodeKeys, "601318"},
		{"single-element array", `{"stock_code": ["601318"]}`, StockCodeKeys, "601318"},
		{"single-element number array", `{"stock_code": [601318]}`, StockCodeKeys, "601318"},
		{"array skips empty first", `{"stock_code": ["", "601318"]}`, StockCodeKeys, "601318"},
		{"whitespace trimmed", `{"stock_code": "  601318  "}`, StockCodeKeys, "601318"},
		{"first key wins", `{"stock_code": "601318", "stockCode": "000001"}`, StockCodeKeys, "601318"},
		{"empty first falls back", `{"stock_code": "", "stockCode": "601318"}`, StockCodeKeys, "601318"},
		{"null falls back", `{"stock_code": null, "code": "601318"}`, StockCodeKeys, "601318"},
		{"missing", `{}`, StockCodeKeys, ""},
		{"empty string", `{"stock_code": ""}`, StockCodeKeys, ""},
		{"object value ignored", `{"stock_code": {"a": 1}}`, StockCodeKeys, ""},
		{"unrelated keys only", `{"strategy": "MA"}`, StockCodeKeys, ""},
		{"date snake_case", `{"start_date": "2024-01-01"}`, []string{"start_date", "startDate"}, "2024-01-01"},
		{"date camelCase alias", `{"startDate": "2024-01-01"}`, []string{"start_date", "startDate"}, "2024-01-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractStringParam(tc.args, tc.keys...); got != tc.want {
				t.Errorf("ExtractStringParam(%s) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestExtractStockCode_CommaListTakesFirst(t *testing.T) {
	// 单标的工具收到逗号列表时，取首只比把整串当代码更不易误判。
	if got := ExtractStockCode(`{"stock_codes": "600519,000001"}`); got != "600519" {
		t.Errorf("ExtractStockCode comma list = %q, want %q", got, "600519")
	}
	if got := ExtractStockCode(`{"stock_code": "600519"}`); got != "600519" {
		t.Errorf("ExtractStockCode = %q, want %q", got, "600519")
	}
}

func TestExtractStringListParam(t *testing.T) {
	cases := []struct {
		name string
		args string
		keys []string
		want []string
	}{
		{"comma string", `{"stock_codes": "600519,000001"}`, StockCodesKeys, []string{"600519", "000001"}},
		{"chinese comma", `{"stock_codes": "600519，000001"}`, StockCodesKeys, []string{"600519", "000001"}},
		{"JSON array", `{"stock_codes": ["600519", "000001"]}`, StockCodesKeys, []string{"600519", "000001"}},
		{"JSON number array", `{"stock_codes": [600519, 1]}`, StockCodesKeys, []string{"600519", "1"}},
		{"single value falls back to list", `{"stock_code": "600519"}`, StockCodesKeys, []string{"600519"}},
		{"codeList alias", `{"codeList": "600519,000001"}`, StockCodesKeys, []string{"600519", "000001"}},
		{"codes alias", `{"codes": "600519"}`, StockCodesKeys, []string{"600519"}},
		{"empty", `{}`, StockCodesKeys, nil},
		{"empty string", `{"stock_codes": ""}`, StockCodesKeys, nil},
		{"blank entries dropped", `{"stock_codes": "600519,, 000001"}`, StockCodesKeys, []string{"600519", "000001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractStringListParam(tc.args, tc.keys...); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractStringListParam(%s) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestExtractIntParam(t *testing.T) {
	cases := []struct {
		name string
		args string
		keys []string
		want int
		ok   bool
	}{
		{"number", `{"limit": 10}`, []string{"limit"}, 10, true},
		{"numeric string", `{"limit": "10"}`, []string{"limit"}, 10, true},
		{"float string truncated", `{"limit": "10.7"}`, []string{"limit"}, 10, true},
		{"float number truncated", `{"limit": 10.9}`, []string{"limit"}, 10, true},
		{"zero is a value", `{"limit": 0}`, []string{"limit"}, 0, true},
		{"snake_case alias", `{"page_size": 20}`, []string{"pageSize", "page_size"}, 20, true},
		{"single-element array", `{"limit": [5]}`, []string{"limit"}, 5, true},
		{"missing", `{}`, []string{"limit"}, 0, false},
		{"non-numeric string", `{"limit": "abc"}`, []string{"limit"}, 0, false},
		{"empty string falls back", `{"limit": "", "count": 7}`, []string{"limit", "count"}, 7, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractIntParam(tc.args, tc.keys...)
			if got != tc.want || ok != tc.ok {
				t.Errorf("ExtractIntParam(%s) = (%d, %v), want (%d, %v)", tc.args, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestExtractFloatParam(t *testing.T) {
	if got, ok := ExtractFloatParam(`{"price": 12.5}`, "price"); !ok || got != 12.5 {
		t.Errorf("ExtractFloatParam number = (%v, %v), want (12.5, true)", got, ok)
	}
	if got, ok := ExtractFloatParam(`{"price": "12.5"}`, "price"); !ok || got != 12.5 {
		t.Errorf("ExtractFloatParam numeric string = (%v, %v), want (12.5, true)", got, ok)
	}
	if _, ok := ExtractFloatParam(`{"price": "abc"}`, "price"); ok {
		t.Error("ExtractFloatParam non-numeric string should return ok=false")
	}
	if _, ok := ExtractFloatParam(`{}`, "price"); ok {
		t.Error("ExtractFloatParam missing should return ok=false")
	}
}

func TestExtractBoolParam(t *testing.T) {
	cases := []struct {
		name string
		args string
		want bool
		ok   bool
	}{
		{"true", `{"enable": true}`, true, true},
		{"false", `{"enable": false}`, false, true},
		{"string true", `{"enable": "true"}`, true, true},
		{"string false", `{"enable": "false"}`, false, true},
		{"string 1", `{"enable": "1"}`, true, true},
		{"number 1", `{"enable": 1}`, true, true},
		{"number 0", `{"enable": 0}`, false, true},
		{"missing", `{}`, false, false},
		{"junk string", `{"enable": "yes"}`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractBoolParam(tc.args, "enable")
			if got != tc.want || ok != tc.ok {
				t.Errorf("ExtractBoolParam(%s) = (%v, %v), want (%v, %v)", tc.args, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestExtractFundCode(t *testing.T) {
	if got := ExtractFundCode(`{"fundCode": "000001"}`); got != "000001" {
		t.Errorf("ExtractFundCode = %q, want %q", got, "000001")
	}
	if got := ExtractFundCode(`{"fund_code": "000002"}`); got != "000002" {
		t.Errorf("ExtractFundCode snake_case = %q, want %q", got, "000002")
	}
	if got := ExtractFundCode(`{"code": "000003"}`); got != "000003" {
		t.Errorf("ExtractFundCode code alias = %q, want %q", got, "000003")
	}
}
