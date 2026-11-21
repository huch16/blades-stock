package datasource

// backend/datasource 包的首个单元测试文件。
// 只覆盖纯逻辑（不碰网络/DB/外部 AI）：代码转换、K 线行解析、聚合。
// ValidateStockCode 的 KNOWN-GAP 用例见 TestValidateStockCode_KnownGap：
// 当前实现对任意非空输入都返回 true，见文件内注释。

import (
	"testing"
)

func TestConvertStockCode(t *testing.T) {
	api := &EastMoneyKLineApi{}
	cases := []struct {
		input string
		want  string
	}{
		{"000001.SZ", "0.000001"},
		{"600519.SH", "1.600519"},
		{"600519.sh", "1.600519"}, // 小写后缀规范化
		{"600519.SS", "1.600519"}, // .SS 视同沪市
		{"920000.BJ", "0.920000"}, // 北交所
		{"00700.HK", "128.00700"},
		{"000001.BK", "90.000001"}, // 板块
		{"sz000001", "0.000001"},
		{"sh600519", "1.600519"},
		{"000001", "0.000001"},
		{"600000", "1.600000"},
		{"510300", "1.510300"},           // 沪市 ETF
		{"159915", "0.159915"},           // 深市 ETF
		{"430047", "0.430047"},           // 4 开头按北交所/新三板走深市通道
		{"天目药业 - 600671.SH", "1.600671"}, // “名称 - 代码”先剥离名称
		{"天目药业 600671", "1.600671"},
		{"", ""},
	}
	for _, c := range cases {
		if got := api.ConvertStockCode(c.input); got != c.want {
			t.Errorf("ConvertStockCode(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ConvertStockCode 对无法识别的纯字母输入会原样返回（不返回 ""），
// 这正是 ValidateStockCode 形同虚设的根因，见 TestValidateStockCode_KnownGap。
func TestConvertStockCode_UnrecognizedInputPassesThrough(t *testing.T) {
	api := &EastMoneyKLineApi{}
	if got := api.ConvertStockCode("invalid"); got != "INVALID" {
		t.Errorf("ConvertStockCode(%q) = %q, want %q (current pass-through behavior)", "invalid", got, "INVALID")
	}
	if got := api.ConvertStockCode("AAPL"); got != "AAPL" {
		t.Errorf("ConvertStockCode(%q) = %q, want %q", "AAPL", got, "AAPL")
	}
}

func TestCleanCodeOnly(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"天目药业 - 600671.SH", "600671.SH"},
		{"天目药业 600671", "600671"},
		{"贵州茅台—600519.SH", "600519.SH"}, // 中文破折号分隔
		{"600519.SH", "600519.SH"},
		{"  600519.SH  ", "600519.SH"},
		{"2906", "002906"}, // JSON 丢前导零时补齐 6 位
		{"002906", "002906"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := CleanCodeOnly(c.input); got != c.want {
			t.Errorf("CleanCodeOnly(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsStockCodeClassifiers(t *testing.T) {
	if !IsAStockCode("000001.SZ") || !IsAStockCode("600519.SH") || !IsAStockCode("920000.BJ") {
		t.Error("IsAStockCode should accept .SZ/.SH/.BJ")
	}
	if IsAStockCode("00700.HK") || IsAStockCode("AAPL.US") {
		t.Error("IsAStockCode should reject .HK/.US")
	}
	if IsAStockCode("000001.sz") {
		t.Error("IsAStockCode is case-sensitive on the suffix (lowercase .sz is not A-share)")
	}
	if !IsHKStockCode("00700.HK") || !IsHKStockCode("00700.hk") || !IsHKStockCode("hk00700") {
		t.Error("IsHKStockCode should accept .HK suffix (any case) and HK prefix")
	}
	if IsHKStockCode("600519.SH") {
		t.Error("IsHKStockCode should reject .SH")
	}
	if !IsUSStockCode("AAPL.US") || !IsUSStockCode("usAAPL") || !IsUSStockCode("GB_AAPL") {
		t.Error("IsUSStockCode should accept .US suffix, US prefix and GB_ prefix")
	}
	if IsUSStockCode("600519.SH") {
		t.Error("IsUSStockCode should reject .SH")
	}
}

func TestParseKLine(t *testing.T) {
	api := &EastMoneyKLineApi{}
	// 11 字段标准行：日期,开,收,高,低,量,额,振幅,涨跌幅,涨跌额,换手率
	k := api.parseKLine("2026-11-20,10.00,12.22,12.50,9.80,123456,1500000,5.10,2.30,0.28,1.50", "")
	if k == nil {
		t.Fatal("parseKLine(valid row) = nil")
	}
	if k.Day != "2026-11-20" || k.Open != "10.00" || k.Close != "12.22" ||
		k.High != "12.50" || k.Low != "9.80" || k.Volume != "123456" ||
		k.Amount != "1500000" || k.TurnoverRate != "1.50" {
		t.Errorf("parseKLine fields mismatch: %+v", k)
	}
	// 字段不足 11 个直接丢弃，不产生半条 K 线。
	if got := api.parseKLine("2026-11-20,10.00,12.22", ""); got != nil {
		t.Errorf("parseKLine(short row) = %+v, want nil", got)
	}
	// "-" / 空 / null 字段归一化为 "0"，不让脏字符串流入均线与绘图计算。
	k2 := api.parseKLine("2026-11-20,-,-,-,-,-,-,-,-,-,-", "")
	if k2 == nil {
		t.Fatal("parseKLine(dash row) = nil")
	}
	if k2.Open != "0" || k2.Close != "0" || k2.Volume != "0" {
		t.Errorf("parseKLine dash fields = %+v, want zeros", k2)
	}
}

func TestParseFloat(t *testing.T) {
	api := &EastMoneyKLineApi{}
	for _, s := range []string{"", "-", "null"} {
		if got := api.parseFloat(s); got != "0" {
			t.Errorf("parseFloat(%q) = %q, want %q", s, got, "0")
		}
	}
	if got := api.parseFloat("12.22"); got != "12.22" {
		t.Errorf("parseFloat(%q) = %q, want %q", "12.22", got, "12.22")
	}
}

func TestParseFloatToFloat(t *testing.T) {
	if v, err := ParseFloatToFloat(""); v != 0 || err != nil {
		t.Errorf("ParseFloatToFloat(%q) = %v, %v", "", v, err)
	}
	if v, err := ParseFloatToFloat("-"); v != 0 || err != nil {
		t.Errorf("ParseFloatToFloat(%q) = %v, %v", "-", v, err)
	}
	if v, err := ParseFloatToFloat("12.5"); v != 12.5 || err != nil {
		t.Errorf("ParseFloatToFloat(%q) = %v, %v", "12.5", v, err)
	}
	if _, err := ParseFloatToFloat("abc"); err == nil {
		t.Error("ParseFloatToFloat(abc) should error")
	}
}

func TestAggregateKLineEveryN(t *testing.T) {
	mk := func(day, open, close, high, low, vol, amt string) KLineData {
		return KLineData{Day: day, Open: open, Close: close, High: high, Low: low, Volume: vol, Amount: amt}
	}
	src := []KLineData{
		mk("2026-11-20 09:31", "10.00", "10.10", "10.20", "9.90", "100", "1000.00"),
		mk("2026-11-20 09:32", "10.10", "10.05", "10.15", "10.00", "200", "2000.00"),
		mk("2026-11-20 09:33", "10.05", "10.30", "10.40", "10.00", "300", "3000.00"),
		mk("2026-11-20 09:34", "10.30", "10.20", "10.35", "10.10", "400", "4000.00"),
	}
	out := AggregateKLineEveryN(&src, 2)
	if out == nil || len(*out) != 2 {
		t.Fatalf("AggregateKLineEveryN(4 rows, n=2) len = %v, want 2", out)
	}
	first, second := (*out)[0], (*out)[1]
	// 第一根：开=首根开，高=区间最高，低=区间最低，收=尾根收，量/额求和，日取尾根。
	if first.Open != "10.00" || first.Close != "10.05" || first.High != "10.2000" ||
		first.Low != "9.9000" || first.Volume != "300" || first.Amount != "3000.00" ||
		first.Day != "2026-11-20 09:32" {
		t.Errorf("first aggregated bar = %+v", first)
	}
	if second.Open != "10.05" || second.Close != "10.20" || second.High != "10.4000" ||
		second.Low != "10.0000" || second.Volume != "700" || second.Amount != "7000.00" {
		t.Errorf("second aggregated bar = %+v", second)
	}
	// 余数不足 n 根的尾巴单独成一根，不丢数据。
	out3 := AggregateKLineEveryN(&src, 3)
	if out3 == nil || len(*out3) != 2 {
		t.Fatalf("AggregateKLineEveryN(4 rows, n=3) len = %v, want 2", out3)
	}
	if (*out3)[1].Volume != "400" || (*out3)[1].Open != "10.30" {
		t.Errorf("remainder bar = %+v, want single-row bar", (*out3)[1])
	}
	// n<2 / 空输入原样返回，不 panic。
	if got := AggregateKLineEveryN(&src, 1); got != &src {
		t.Error("AggregateKLineEveryN(n=1) should return src unchanged")
	}
	var nilSrc *[]KLineData
	if got := AggregateKLineEveryN(nilSrc, 2); got != nil {
		t.Error("AggregateKLineEveryN(nil) should return nil")
	}
	empty := []KLineData{}
	if got := AggregateKLineEveryN(&empty, 2); got != &empty {
		t.Error("AggregateKLineEveryN(empty) should return src unchanged")
	}
}

// TestValidateStockCode 定长校验：合法码 true、空/乱码/非法 false。
// 历史：2026-11-22 前 ValidateStockCode 只看 ConvertStockCode 返回是否为空，
// 而 ConvertStockCode 对任意非空输入几乎总返回非空，"invalid" 与 "600500000119.SH"
// （2026-11-21 线上乱码代码）都返回 true；后端完全拦不住，前端 isValidEastMoneyCode
// 是当时唯一防线。本测试曾以 TestValidateStockCode_KnownGap 钉住该缺陷行为，
// 2026-11-22 ValidateStockCode 改为定长校验后翻转为期望行为；同时与
// backend/data/eastmoney_kline_api_test.go 的集成测试期望一致（"invalid" → false）。
func TestValidateStockCode(t *testing.T) {
	api := &EastMoneyKLineApi{}
	for _, code := range []string{"000001.SZ", "600519.SH", "00700.HK", "AAPL.US", "000001", "600519", "sh600519", "sz000001", "2906"} {
		if !api.ValidateStockCode(code) {
			t.Errorf("ValidateStockCode(%q) = false, want true", code)
		}
	}
	for _, code := range []string{"", "invalid", "600500000119.SH", "600500000119", "0070.HK", "007000.HK", "ABCDEF.US", "600519.BK", "AAPL"} {
		if api.ValidateStockCode(code) {
			t.Errorf("ValidateStockCode(%q) = true, want false", code)
		}
	}
}
