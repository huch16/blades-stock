package data

import (
	_ "embed"

	"regexp"
	"strings"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
)

// @Author spark
// @Date 2025/2/13 13:08
// @Desc
//-----------------------------------------------------------------------------------

// SensitiveWords 敏感词
//
// v3.0.0 优化：词表原为单行 747 KB 的字符串字面量，导致 git diff / 编辑器 /
// 代码检索严重退化（任何小改动都会重写整行）。现改为独立数据文件 + go:embed，
// 运行时语义与原先完全等价（仍是一次 strings.Split 切分）。
//
//go:embed sensitive_words.txt
var sensitiveWordsRaw string

var SensitiveWords = strings.Split(sensitiveWordsRaw, "、")

// ReplaceSensitiveWords 过滤敏感词
func ReplaceSensitiveWords(text string) string {
	for _, word := range SensitiveWords {
		if strings.Contains(text, word) {
			text = strings.ReplaceAll(text, word, "")
		}
	}
	return text
}

var (
	reSpace  = regexp.MustCompile(`\s`)
	reNonDig = regexp.MustCompile(`\D`)
	reDig    = regexp.MustCompile(`\d`)
)

// RemoveAllBlankChar  使用正则表达式移除字符串中的空白字符
func RemoveAllBlankChar(s string) string {
	return reSpace.ReplaceAllString(s, "")
}

// RemoveAllNonDigitChar 去除所有非数字字符
func RemoveAllNonDigitChar(s string) string {
	return reNonDig.ReplaceAllString(s, "")
}

// RemoveAllDigitChar 去除所有数字字符
func RemoveAllDigitChar(s string) string {
	return reDig.ReplaceAllString(s, "")
}

// ConvertStockCodeToTushareCode 将股票代码转换为tushare的股票代码
func ConvertStockCodeToTushareCode(stockCode string) string {
	//提取非数字
	if strutil.HasPrefixAny(stockCode, []string{"SH", "sh", "SZ", "sz", "BJ", "bj", "HK", "hk"}) {
		stockCode = RemoveAllNonDigitChar(stockCode) + "." + strings.ToUpper(RemoveAllDigitChar(stockCode))
	}
	return stockCode
}

// ConvertTushareCodeToStockCode 将tushare股票代码转换为的普通股票代码
func ConvertTushareCodeToStockCode(stockCode string) string {
	//提取非数字
	stockCode = strings.ToLower(RemoveAllDigitChar(stockCode)) + RemoveAllNonDigitChar(stockCode)
	return strings.ReplaceAll(stockCode, ".", "")
}
func ConvertTushareCodeToStockCodes(a []string) []string {
	return slice.Map(a, func(i int, s string) string {
		return ConvertTushareCodeToStockCode(s)
	})
}

// isVisible 函数用于判断元素是否可见
