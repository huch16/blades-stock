package data

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/go-resty/resty/v2"
)

// realQuote 单只股票的实时核对基准（价格元、总市值亿元）。
type realQuote struct {
	Price     float64 // 最新价（元）
	TotalMcap float64 // 总市值（亿元）；<=0 表示未取到
	Name      string  // 名称
}

// verifyAndCorrectContent 对泛化报告（stock_code='agent'/'市场分析'等）做事后硬校验：
//  1. 抽取 6 位数字代码，解析交易所前缀后调实时行情核验代码存在性与价格合理性；
//  2. 对文中出现的"当前价/买入区间/止损/止盈 X~Y 元"及"市值 X 亿"等表述，与实时数据比对，
//     偏差 >50% 时直接替换为实时值并加"[已校正:...]"标注；
//  3. 代码在行情接口查不到时标注"[代码不存在]"。
//
// 个股报告（IsStockReportCode）不在此处校验，由回测层做准确度追踪。
func verifyAndCorrectContent(content, stockCode, question string) string {
	if models.IsStockReportCode(stockCode) {
		return content
	}
	rawCodes := extractSixDigitCodes(content)
	if len(rawCodes) == 0 {
		return content
	}
	quotes := fetchRealQuotes(rawCodes)
	if len(quotes) == 0 {
		logger.SugaredLogger.Warnf("verifyAndCorrectContent: 行情获取失败 codes=%v", rawCodes)
		return content
	}
	return correctReportContent(content, rawCodes, quotes)
}

// extractSixDigitCodes 抽取内容中所有 6 位数字（去重，保留首次出现顺序）。
func extractSixDigitCodes(content string) []string {
	codeRegex := regexp.MustCompile(`\b\d{6}\b`)
	matches := codeRegex.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var codes []string
	for _, m := range matches {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		codes = append(codes, m)
	}
	return codes
}

// fetchRealQuotes 批量获取实时行情（价格 + 腾讯总市值）。
// 价格走 GetStockCodeRealTimeData（新浪→腾讯兜底，含缓存）；市值走 qt.gtimg.cn 独立字段。
func fetchRealQuotes(rawCodes []string) map[string]realQuote {
	prefixed := make([]string, 0, len(rawCodes))
	for _, c := range rawCodes {
		prefixed = append(prefixed, sinaSymbolFromStockCode(c))
	}
	quotes := make(map[string]realQuote, len(rawCodes))

	// 价格
	infos, err := NewStockDataApi().GetStockCodeRealTimeData(prefixed...)
	if err == nil && infos != nil {
		for i := range *infos {
			code := (*infos)[i].Code
			price, _ := strconv.ParseFloat((*infos)[i].Price, 64)
			q := quotes[code]
			q.Price = price
			q.Name = (*infos)[i].Name
			quotes[code] = q
		}
	}

	// 总市值（腾讯 qt.gtimg.cn，field 45）
	mcaps := fetchTencentMarketCaps(prefixed)
	for _, c := range prefixed {
		if mc, ok := mcaps[c]; ok {
			q := quotes[c]
			q.TotalMcap = mc
			quotes[c] = q
		}
	}
	return quotes
}

// fetchTencentMarketCaps 从腾讯行情批量取总市值（亿元）。key 为带 sh/sz/bj 前缀代码。
func fetchTencentMarketCaps(codes []string) map[string]float64 {
	result := make(map[string]float64)
	if len(codes) == 0 {
		return result
	}
	url := "http://qt.gtimg.cn/?_=1&q=" + strings.Join(codes, ",")
	client := resty.New().
		SetTimeout(10 * time.Second).
		SetRetryCount(1)
	resp, err := client.R().
		SetHeader("Host", "qt.gtimg.cn").
		SetHeader("Referer", "https://gu.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Warnf("fetchTencentMarketCaps error: %v", err)
		return result
	}
	body := GB18030ToUTF8(resp.Body())
	lines := strings.Split(strings.TrimSpace(body), ";")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		eq := strings.Index(line, "=")
		if eq <= 0 || !strings.HasPrefix(line, "v_") {
			continue
		}
		code := line[2:eq]
		payload := strings.Trim(line[eq+1:], `"`)
		fields := strings.Split(payload, "~")
		if len(fields) < 46 {
			continue
		}
		// field 45 = 总市值（单位：亿元）
		mc, err := strconv.ParseFloat(fields[45], 64)
		if err != nil || mc <= 0 {
			continue
		}
		result[code] = mc
	}
	return result
}

// 价格/市值表述的匹配模式（覆盖表格与叙述两种写法）
var (
	priceRangeRe  = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[~～\-]\s*(\d+(?:\.\d+)?)\s*元`)
	mcapKeywordRe = regexp.MustCompile(`(?i)(总市值|流通市值|市值)\s*[:：]?\s*[~≈]?\s*(\d+(?:\.\d+)?)\s*亿`)
	mcapTildeRe   = regexp.MustCompile(`(?i)[~≈]\s*(\d+(?:\.\d+)?)\s*亿`)
	mcapRangeRe   = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)\s*[~～\-]\s*(\d+(?:\.\d+)?)\s*亿`)
)

// correctReportContent 纯函数：按实时行情逐行校对内容。可单测（不依赖网络）。
// 策略：逐行处理，每行含且仅含一个代码时，才对该行的价格/市值表述做校对，
// 避免跨行误伤；含多个代码的行只做"代码不存在"标注。
// 若行内无代码但出现股票名称（如表格"| 正元地信 | 20% | 45-50 元 |"），
// 通过名称映射解析后再校对。
func correctReportContent(content string, rawCodes []string, quotes map[string]realQuote) string {
	byRaw := buildQuoteMap(rawCodes, quotes)
	byName := buildNameMap(byRaw)
	// 从报告自身提取"名称+代码"配对（如"**首创股份 600008.SH**"），
	// 用报告使用的名称（可能是旧名"首创股份"）映射到实时行情。
	reportNames := extractReportNameCodePairs(content, byRaw)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = correctLineClaims(line, byRaw, byName, reportNames)
	}
	return strings.Join(lines, "\n")
}

// extractReportNameCodePairs 从内容中抽取"中文名称 + 6位代码"相邻配对，返回 名称->quote。
func extractReportNameCodePairs(content string, byRaw map[string]realQuote) map[string]realQuote {
	result := make(map[string]realQuote)
	re := regexp.MustCompile(`([\p{Han}]{2,8})\s*([A-Za-z.]{0,4}?\d{6})`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) != 3 {
			continue
		}
		name, rawCode := m[1], m[2]
		digits := regexp.MustCompile(`\d{6}`).FindString(rawCode)
		if q, ok := byRaw[digits]; ok {
			result[name] = q
		}
	}
	return result
}

// buildNameMap 构建 股票名称 -> quote 映射（名称是行内无代码时的兜底解析）。
// 同时注册去尾缀变体（"首创环保"→"首创"），兼容报告旧名"首创股份"等写法。
func buildNameMap(byRaw map[string]realQuote) map[string]realQuote {
	byName := make(map[string]realQuote, len(byRaw)*2)
	for _, q := range byRaw {
		if q.Name == "" {
			continue
		}
		byName[q.Name] = q
		trimmed := strings.TrimSuffix(q.Name, "股份")
		trimmed = strings.TrimSuffix(trimmed, "集团")
		if trimmed != q.Name && len(trimmed) >= 2 {
			byName[trimmed] = q
		}
	}
	return byName
}

// buildQuoteMap 构建 raw(6位) -> quote 映射；行情 key 可能是带前缀或纯数字，两种都兼容。
func buildQuoteMap(rawCodes []string, quotes map[string]realQuote) map[string]realQuote {
	byRaw := make(map[string]realQuote, len(rawCodes))
	for _, c := range rawCodes {
		if q, ok := quotes[c]; ok {
			byRaw[c] = q
			continue
		}
		if q, ok := quotes[sinaSymbolFromStockCode(c)]; ok {
			byRaw[c] = q
		}
	}
	return byRaw
}

// correctLineClaims 校对单行内某只股票的价格/市值表述。
// 行内无代码时按名称匹配（先报告内配对名，再实时名及去尾缀变体）。
func correctLineClaims(line string, byRaw map[string]realQuote, byName map[string]realQuote, reportNames map[string]realQuote) string {
	codes := extractSixDigitCodes(line)
	if len(codes) == 0 {
		if q, ok := matchNameInLine(line, reportNames, byName); ok {
			return correctLineQuote(line, q)
		}
		return line
	}
	out := line
	for _, code := range codes {
		q, ok := byRaw[code]
		if !ok || q.Price <= 0 {
			out = strings.ReplaceAll(out, code, code+"[代码不存在]")
			continue
		}
		if q.Price > 10000 {
			out = strings.ReplaceAll(out, code, code+"[行情异常]")
			continue
		}
		// 该行含多个代码时不校对其价格/市值（归属不明确），仅标注不存在的代码
		if len(codes) > 1 {
			continue
		}
		out = correctLineQuote(out, q)
	}
	return out
}

// matchNameInLine 在行中查找已知股票名称，返回对应行情。reportNames 优先（报告自带旧名）。
func matchNameInLine(line string, reportNames, byName map[string]realQuote) (realQuote, bool) {
	if len(reportNames) == 0 && len(byName) == 0 {
		return realQuote{}, false
	}
	// 名称按长度降序，优先匹配更长（更精确）的名称
	type kv struct {
		name string
		q    realQuote
	}
	var candidates []kv
	for name, q := range reportNames {
		candidates = append(candidates, kv{name, q})
	}
	for name, q := range byName {
		candidates = append(candidates, kv{name, q})
	}
	sort.Slice(candidates, func(i, j int) bool { return len(candidates[i].name) > len(candidates[j].name) })
	for _, c := range candidates {
		if strings.Contains(line, c.name) {
			return c.q, true
		}
	}
	return realQuote{}, false
}

// correctLineQuote 对单行内价格/市值表述做校对。
func correctLineQuote(line string, q realQuote) string {
	out := correctLinePrice(line, q.Price)
	if q.TotalMcap > 0 {
		out = correctLineMcap(out, q.TotalMcap)
	}
	return out
}

// correctLinePrice 校正单行内"X~Y 元"价格区间（真实价落在区间外且偏差>50%）。
func correctLinePrice(line string, realPrice float64) string {
	if !priceRangeRe.MatchString(line) {
		return line
	}
	return priceRangeRe.ReplaceAllStringFunc(line, func(m string) string {
		parts := priceRangeRe.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		lo, e1 := strconv.ParseFloat(parts[1], 64)
		hi, e2 := strconv.ParseFloat(parts[2], 64)
		if e1 != nil || e2 != nil || lo <= 0 || hi <= 0 || lo > hi {
			return m
		}
		if !rangeDeviates(lo, hi, realPrice) {
			return m
		}
		return m + "[已校正:实时价格约" + formatYi(realPrice) + "元]"
	})
}

// correctLineMcap 校正单行内"市值 X 亿"、"~X 亿"及"X~Y 亿"市值表述。
func correctLineMcap(line string, realMcap float64) string {
	if !mcapKeywordRe.MatchString(line) && !mcapTildeRe.MatchString(line) && !mcapRangeRe.MatchString(line) {
		return line
	}
	// 带"市值"关键字的单值表述
	line = mcapKeywordRe.ReplaceAllStringFunc(line, func(m string) string {
		parts := mcapKeywordRe.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		claimed, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || claimed <= 0 || !singleDeviates(claimed, realMcap) {
			return m
		}
		return m + "[已校正:实时市值约" + formatYi(realMcap) + "亿]"
	})
	// "~X 亿" 叙述写法（不含市值关键字，如"/ ~110 亿"）
	line = mcapTildeRe.ReplaceAllStringFunc(line, func(m string) string {
		parts := mcapTildeRe.FindStringSubmatch(m)
		if len(parts) != 2 {
			return m
		}
		claimed, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || claimed <= 0 || !singleDeviates(claimed, realMcap) {
			return m
		}
		return m + "[已校正:实时市值约" + formatYi(realMcap) + "亿]"
	})
	// "X~Y 亿" 区间（表格市值列，须先排除已被上面替换的）
	line = mcapRangeRe.ReplaceAllStringFunc(line, func(m string) string {
		if strings.Contains(m, "已校正") {
			return m
		}
		parts := mcapRangeRe.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		lo, e1 := strconv.ParseFloat(parts[1], 64)
		hi, e2 := strconv.ParseFloat(parts[2], 64)
		if e1 != nil || e2 != nil || lo <= 0 || hi <= 0 || lo > hi {
			return m
		}
		if !rangeDeviates(lo, hi, realMcap) {
			return m
		}
		return m + "[已校正:实时市值约" + formatYi(realMcap) + "亿]"
	})
	return line
}

// rangeDeviates 判断区间 [lo,hi] 与 real 偏差：真实值落在区间之外且与最近端点偏差>50%。
func rangeDeviates(lo, hi, real float64) bool {
	if real <= 0 {
		return false
	}
	if real >= lo && real <= hi {
		return false
	}
	if real < lo {
		return (lo-real)/real > 0.5
	}
	return (real-hi)/real > 0.5
}

// singleDeviates 判断单值 claimed 相对 real 偏差是否超过 50%。
func singleDeviates(claimed, real float64) bool {
	if real <= 0 {
		return false
	}
	ratio := claimed / real
	return ratio < 0.5 || ratio > 1.5
}

// formatYi 格式化亿元/元数值（保留最多 2 位小数）。
func formatYi(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if strings.Contains(s, ".") {
		if len(s) > 1 && s[len(s)-1] == '0' {
			s = strings.TrimRight(s, "0")
			s = strings.TrimRight(s, ".")
		}
	}
	return s
}
