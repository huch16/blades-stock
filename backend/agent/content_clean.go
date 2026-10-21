package agent

import (
	"strings"
	"unicode/utf8"
)

// thinkStreamFilter drops <think>...</think> reasoning blocks from assistant
// content streams. It is stateful so a block split across multiple streamed
// deltas is removed correctly. Both <think> and <thinking> variants are
// handled via the "<think" open prefix; each tag is consumed through its
// closing '>' so nested or malformed blocks cannot re-open mid-content.
type thinkStreamFilter struct {
	inThink bool
}

// Clean processes one content chunk and returns the portion outside any
// reasoning block. It mutates the filter's inThink state across calls.
func (f *thinkStreamFilter) Clean(text string) string {
	if text == "" {
		return ""
	}
	var sb strings.Builder
	rest := text
	for rest != "" {
		if f.inThink {
			idx := strings.Index(rest, "</think")
			if idx < 0 {
				return sb.String()
			}
			end := strings.IndexByte(rest[idx:], '>')
			if end < 0 {
				return sb.String()
			}
			rest = rest[idx+end+1:]
			f.inThink = false
			continue
		}
		idx := strings.Index(rest, "<think")
		if idx < 0 {
			sb.WriteString(rest)
			return sb.String()
		}
		sb.WriteString(rest[:idx])
		rest = rest[idx:]
		end := strings.IndexByte(rest, '>')
		if end < 0 {
			f.inThink = true
			return sb.String()
		}
		rest = rest[end+1:]
		f.inThink = true
	}
	return sb.String()
}

// cleanThinkBlocks removes all <think>...</think> reasoning blocks from a full
// text using a fresh filter state.
func cleanThinkBlocks(text string) string {
	return (&thinkStreamFilter{}).Clean(text)
}

// collapseConsecutiveDuplicates removes the largest block that appears twice
// near the end of text, keeping the first copy. The two copies may be separated
// by a small amount of separator-like junk (whitespace, markdown "---" lines,
// short thinking sentences) which is dropped along with the second copy. Only
// blocks >=minBlock chars trigger, so normal prose is never affected. Trailing
// whitespace of the whole text is preserved.
func collapseConsecutiveDuplicates(text string, minBlock int) string {
	if minBlock <= 0 {
		minBlock = 400
	}
	// gapMax bounds how much separator-like junk may sit between the two copies.
	const gapMax = 300
	s := text
	trailing := ""
	if trimmed := strings.TrimRight(s, " \t\r\n"); trimmed != s {
		trailing = s[len(trimmed):]
		s = trimmed
	}
	for i := 0; i < 5; i++ {
		if len(s) < minBlock*2 {
			break
		}
		best := 0
		// L is where the second copy starts; B = s[L:] is the suffix. Try the
		// longest second copy first (largest B). For each L we look for a
		// previous copy of B ending within gapMax chars of L (so the first copy
		// may have a small junk suffix before the second copy begins).
		for L := minBlock; L <= len(s)-minBlock; L++ {
			B := s[L:]
			lenB := len(s) - L
			regionStart := L - lenB - gapMax
			if regionStart < 0 {
				regionStart = 0
			}
			region := s[regionStart:L]
			idx := strings.LastIndex(region, B)
			if idx < 0 {
				continue
			}
			// E0 is the end of the first copy; the junk sits in s[E0:L].
			E0 := regionStart + idx + lenB
			if !isCollapseJunk(s[E0:L]) {
				continue
			}
			best = E0
			break
		}
		if best == 0 {
			break
		}
		s = s[:best]
	}
	return s + trailing
}

// isHrLine reports whether a trimmed line is a markdown horizontal rule.
func isHrLine(t string) bool {
	if t == "" {
		return false
	}
	for _, r := range t {
		if r != '-' && r != '*' && r != '_' {
			return false
		}
	}
	return true
}

// isCollapseJunk reports whether the separator between two duplicated copies
// looks like streamed thinking residue + a separator, never real content: it
// must consist only of whitespace, markdown "---" rules, and short sentence
// lines (<=60 runes ending with punctuation).
func isCollapseJunk(j string) bool {
	if strings.TrimSpace(j) == "" {
		return true
	}
	for _, line := range strings.Split(j, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isHrLine(t) {
			continue
		}
		if utf8.RuneCountInString(t) <= 60 {
			if strings.ContainsAny(t[len(t)-1:], "。！？；：.!?;:）】〉\"'…—") {
				continue
			}
		}
		return false
	}
	return true
}

// salvageRawContent 尝试从全量原始输出中挽救被意外包裹在 <think> 或思维标签中的有效分析正文。
// 仅当常规清洗后内容为空，但原始输出存在实质内容时调用。
func salvageRawContent(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	s := raw
	// 1. 如果有 </think> 且后面有非空白内容，优先取其后
	if idx := strings.LastIndex(s, "</think>"); idx >= 0 {
		after := strings.TrimSpace(s[idx+8:])
		if len(after) > 50 {
			return after
		}
	}
	// 2. 剥离所有的 <think> 和 </think> 等标签
	s = strings.ReplaceAll(s, "<think>", "")
	s = strings.ReplaceAll(s, "</think>", "")
	s = strings.ReplaceAll(s, "<thinking>", "")
	s = strings.ReplaceAll(s, "</thinking>", "")
	s = strings.TrimSpace(s)

	// 3. 寻找 Markdown 研报正文标志性开头（如 # 标题、一、等）
	for _, marker := range []string{"# ", "## ", "一、", "1. ", "【", "综上所述", "结论先行"} {
		if pos := strings.Index(s, marker); pos >= 0 {
			return strings.TrimSpace(s[pos:])
		}
	}
	return s
}

// cleanFinalContent is applied once after streaming completes: strip reasoning
// blocks first, then collapse a repeated answer if the model emitted one.
func cleanFinalContent(raw string) string {
	cleaned := collapseConsecutiveDuplicates(cleanThinkBlocks(raw), 400)
	// 防空兜底：若过滤后为空，但原始文本包含大量字符，智能挽救正文内容
	if strings.TrimSpace(cleaned) == "" && len(strings.TrimSpace(raw)) > 100 {
		salvaged := salvageRawContent(raw)
		if salvaged != "" {
			return collapseConsecutiveDuplicates(salvaged, 400)
		}
	}
	return cleaned
}
