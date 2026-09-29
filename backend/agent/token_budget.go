// Package agent 的 tokenizer 预算模块（P2.1）。
//
// 设计目标：用真实 tokenizer（tiktoken-go）替代 byte 长度作为内容压缩预算单位。
// 收益：
//   - 中文 / 英文 / 数字串 / 代码 都能更精确估算 token
//   - 避免"按 byte 算剩 800 字符但实际还能塞 1000 token"或反向溢出
//
// 设计原则：
//   - 单一全局缓存的 tokenizer（Coder/cl100k_base），懒初始化
//   - EncodeCount(text) → int：O(n) 一次性计数，无副作用
//   - EncodeTrimToTokens(text, maxTokens) → string：返回不超过 maxTokens 的截断结果
//   - 失败时降级为 byte 估算（保持向后兼容）
package agent

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/tiktoken-go/tokenizer"
)

var (
	tokenizerOnce sync.Once
	tokenizerEnc  tokenizer.Codec
	tokenizerErr  error
)

// getTokenizer 懒初始化 tiktoken Coder codec（兼容 GPT-4 / GPT-3.5-turbo / GPT-4o）。
// 选 cl100k_base 是因为它覆盖最多项目使用的模型族（GPT-3.5+ / GPT-4 / DeepSeek / Qwen 都用类似 BPE）。
//
// 失败时（如下载 BPE 文件）tokenizerErr 非 nil；调用方应降级到 byte 估算。
func getTokenizer() (tokenizer.Codec, error) {
	tokenizerOnce.Do(func() {
		tokenizerEnc, tokenizerErr = tokenizer.Get(tokenizer.Cl100kBase)
	})
	return tokenizerEnc, tokenizerErr
}

// EncodeCount 返回 text 的真实 token 数。
// 失败时降级为旧启发式（中文 1.5 字符/token，英文 4 字符/token）。
func EncodeCount(text string) int {
	enc, err := getTokenizer()
	if err != nil {
		return fallbackEstimateTokens(text)
	}
	ids, _, err := enc.Encode(text)
	if err != nil {
		return fallbackEstimateTokens(text)
	}
	return len(ids)
}

// EncodeTrimToTokens 返回不超过 maxTokens 的截断文本。
// 行为：
//   - 若 EncodeCount(text) ≤ maxTokens：直接返回原文
//   - 否则二分查找最大可保留长度，UTF-8 安全（不会切到 rune 中间）
//   - 不加 "已截断" 后缀：调用方负责判断并加上语义化标记（避免预算被后缀吃光）
//   - 失败时降级为 byte 截断
func EncodeTrimToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 || text == "" {
		return ""
	}
	if EncodeCount(text) <= maxTokens {
		return text
	}
	enc, err := getTokenizer()
	if err != nil {
		return fallbackTrimByBytes(text, maxTokens*4)
	}

	// 二分 max runes 数（用 runes 而非 bytes，避免 UTF-8 中间切）
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		candidate := string(runes[:mid])
		count, _, _ := enc.Encode(candidate)
		if len(count) <= maxTokens {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo])
}

// fallbackEstimateTokens 是 tokenizer 失败时的降级启发式。
// 与 token_utils.go 中既有 estimateTokens 一致，保持行为兼容。
func fallbackEstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	chineseCount, otherCount := 0, 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else {
			otherCount++
		}
	}
	// 中文 1.5 字符/token，英文/数字 4 字符/token
	tokens := float64(chineseCount)/1.5 + float64(otherCount)/4.0
	return int(tokens) + 1
}

// fallbackTrimByBytes 是 tokenizer 失败时的降级 byte 截断。
// 用 UTF-8 安全的方式从尾部往前保留 maxBytes 字节。
func fallbackTrimByBytes(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	// 找最近的 UTF-8 rune 边界
	for i := maxBytes; i > maxBytes-4 && i > 0; i-- {
		if utf8.ValidString(text[:i]) {
			return text[:i] + "…(已截断)"
		}
	}
	return safeTruncateString(text, maxBytes)
}

// EncodeLinesSplit 把多行文本按 token 预算逐行累加，直到超过预算为止。
// 返回 [保留的行, 省略的行数, 省略指示]。
// 这是 token 版 smartTruncateLines 的替代实现。
func EncodeLinesSplit(lines []string, maxTokens int) (kept []string, omitted int) {
	if maxTokens <= 0 || len(lines) == 0 {
		return nil, len(lines)
	}
	used := 0
	for i, line := range lines {
		// 每行至少 +1 token 算换行
		lineTokens := EncodeCount(line) + 1
		if used+lineTokens > maxTokens && len(kept) > 0 {
			omitted = len(lines) - i
			break
		}
		kept = append(kept, line)
		used += lineTokens
	}
	if len(kept) == len(lines) {
		omitted = 0
	}
	return kept, omitted
}

// EncodeLinesKeepTail 与 EncodeLinesSplit 类似但从尾部保留（用于"最近 N 行最重要"场景）。
func EncodeLinesKeepTail(lines []string, maxTokens int) (kept []string, omitted int) {
	if maxTokens <= 0 || len(lines) == 0 {
		return nil, len(lines)
	}
	used := 0
	for i := len(lines) - 1; i >= 0; i-- {
		lineTokens := EncodeCount(lines[i]) + 1
		if used+lineTokens > maxTokens && len(kept) > 0 {
			omitted = i + 1
			break
		}
		kept = append([]string{lines[i]}, kept...)
		used += lineTokens
	}
	if len(kept) == len(lines) {
		omitted = 0
	}
	return kept, omitted
}

// joinLinesAndOmitted 把 kept 行与 omitted 标记拼成单字符串，便于直接当 prompt 一部分用。
func joinLinesAndOmitted(kept []string, omitted int) string {
	if omitted <= 0 {
		return strings.Join(kept, "\n")
	}
	return strings.Join(kept, "\n") + "\n…(已省略 " + itoa(omitted) + " 行)"
}

// itoa 是轻量级 int→string，避免 strconv 引入开销。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}