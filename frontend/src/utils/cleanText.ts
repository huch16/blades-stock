// cleanText mirrors backend/agent/content_clean.go: strips <think>...</think>
// reasoning blocks a model wrote into its content (MiniMax-M3 style) and
// collapses a duplicated final answer emitted twice back-to-back.

// cleanThinkBlocks removes all <think>...</think> reasoning blocks (including
// nested / unclosed variants) from full text.
export function cleanThinkBlocks(text: string): string {
  if (!text) return ''
  let inThink = false
  let out = ''
  let rest = text
  while (rest.length > 0) {
    if (inThink) {
      const idx = rest.indexOf('</think')
      if (idx < 0) break
      const end = rest.indexOf('>', idx)
      if (end < 0) break
      rest = rest.slice(end + 1)
      inThink = false
      continue
    }
    const idx = rest.indexOf('<think')
    if (idx < 0) {
      out += rest
      break
    }
    out += rest.slice(0, idx)
    rest = rest.slice(idx)
    const end = rest.indexOf('>')
    if (end < 0) {
      // 未闭合的 "<think"：已消费到结尾，直接退出循环。
      // （此处曾写 inThink = true，但紧接着 break，赋值永远不会被读到）
      break
    }
    rest = rest.slice(end + 1)
    inThink = true
  }
  return out
}

// isHrLine reports whether a trimmed line is a markdown horizontal rule.
function isHrLine(t: string): boolean {
  if (!t) return false
  for (const ch of t) {
    if (ch !== '-' && ch !== '*' && ch !== '_') return false
  }
  return true
}

// isCollapseJunk reports whether the separator between two duplicated copies
// looks like streamed thinking residue + a separator, never real content: it
// must consist only of whitespace, markdown "---" rules, and short sentence
// lines (<=60 chars ending with punctuation).
function isCollapseJunk(j: string): boolean {
  if (j.trim() === '') return true
  for (const line of j.split('\n')) {
    const t = line.trim()
    if (t === '') continue
    if (isHrLine(t)) continue
    if (t.length <= 60 && /[。！？；：.!?;:）】〉"'…—]$/.test(t)) continue
    return false
  }
  return true
}

// collapseConsecutiveDuplicates removes the largest block that appears twice
// near the end of text, keeping the first copy. The two copies may be separated
// by a small amount of separator-like junk (whitespace, markdown "---" lines,
// short thinking sentences) which is dropped along with the second copy. Only
// >=minBlock identical blocks trigger, so normal prose is never affected.
// Trailing whitespace of the whole text is preserved.
export function collapseConsecutiveDuplicates(text: string, minBlock = 400): string {
  const gapMax = 300
  let s = text
  let trailing = ''
  const trimmed = s.replace(/[ \t\r\n]+$/, '')
  if (trimmed !== s) {
    trailing = s.slice(trimmed.length)
    s = trimmed
  }
  for (let i = 0; i < 5; i++) {
    if (s.length < minBlock * 2) break
    let best = 0
    // L is where the second copy starts; B = s.slice(L) is the suffix. Try the
    // longest second copy first (largest B). For each L we look for a previous
    // copy of B ending within gapMax chars of L (so the first copy may have a
    // small junk suffix before the second copy begins).
    for (let L = minBlock; L <= s.length - minBlock; L++) {
      const B = s.slice(L)
      const lenB = s.length - L
      let regionStart = L - lenB - gapMax
      if (regionStart < 0) regionStart = 0
      const region = s.slice(regionStart, L)
      const idx = region.lastIndexOf(B)
      if (idx < 0) continue
      // E0 is the end of the first copy; the junk sits in s.slice(E0, L).
      const E0 = regionStart + idx + lenB
      if (!isCollapseJunk(s.slice(E0, L))) continue
      best = E0
      break
    }
    if (best === 0) break
    s = s.slice(0, best)
  }
  return s + trailing
}

// salvageRawContent 尝试从全量原始输出中挽救被意外包裹在 <think> 或思维标签中的有效分析正文。
export function salvageRawContent(raw: string): string {
  if (!raw || !raw.trim()) return ''
  let s = raw
  // 1. 如果有 </think> 且后面有非空白内容，优先取其后
  const idx = s.lastIndexOf('</think>')
  if (idx >= 0) {
    const after = s.slice(idx + 8).trim()
    if (after.length > 50) {
      return after
    }
  }
  // 2. 剥离所有的 <think> 和 </think> 等标签
  s = s.replace(/<think[\s\S]*?>/gi, '').replace(/<\/think>/gi, '')
  s = s.replace(/<thinking[\s\S]*?>/gi, '').replace(/<\/thinking>/gi, '')
  s = s.trim()

  // 3. 寻找 Markdown 研报正文标志性开头
  const markers = ['# ', '## ', '一、', '1. ', '【', '综上所述', '结论先行']
  for (const marker of markers) {
    const pos = s.indexOf(marker)
    if (pos >= 0) {
      return s.slice(pos).trim()
    }
  }
  return s
}

// stripLeadingPlanningNoise 剥离正文开头意外泄露的 Plan-Execute 思考/规划杂音句子。
// 例如："下一步需要调用 XX 工具... 下一步需要查询... ### 正文" -> "### 正文"
export function stripLeadingPlanningNoise(text: string): string {
  if (!text) return ''
  let s = text
  const noisePrefixes = [
    '下一步需要查询', '下一步需要调用', '下一步我将调用', '下一步调用', '我将调用工具',
  ]

  for (let round = 0; round < 10; round++) {
    let matched = false
    const trimmed = s.replace(/^[ \t\r\n]+/, '')
    for (const prefix of noisePrefixes) {
      if (trimmed.startsWith(prefix)) {
        let endIdx = -1
        for (let i = 0; i < trimmed.length; i++) {
          const ch = trimmed[i]
          if (ch === '\n' || ch === '。' || ch === '！' || ch === '!') {
            endIdx = i + 1
            break
          }
        }
        if (endIdx > 0) {
          s = trimmed.slice(endIdx).replace(/^[ \t\r\n]+/, '')
          matched = true
          break
        }
      }
    }
    if (!matched) break
  }
  return s
}

// stripClosedAgentAction 移除成对闭合的 ```agent-action ... ``` 动作代码块，避免暴露 JSON 污染最终文本。
export function stripClosedAgentAction(text: string): string {
  if (!text) return ''
  return text.replace(/```agent-action\s*[\s\S]*?```/g, '')
}

// stripUnclosedAgentAction 移除末尾因 token 耗尽而未闭合的残缺 ```agent-action 代码块。
export function stripUnclosedAgentAction(text: string): string {
  if (!text) return ''
  const marker = '```agent-action'
  const lastIdx = text.lastIndexOf(marker)
  if (lastIdx < 0) return text

  const suffix = text.slice(lastIdx + marker.length)
  if (!suffix.includes('```')) {
    // 未闭合，直接截断 marker 之前的内容
    return text.slice(0, lastIdx).replace(/[ \t\r\n]+$/, '')
  }
  return text
}

// stripAllAgentActionBlocks 移除所有 ```agent-action 代码块（包括闭合与未闭合的残卷）。
export function stripAllAgentActionBlocks(text: string): string {
  if (!text) return ''
  return stripUnclosedAgentAction(stripClosedAgentAction(text))
}

// cleanFinalContent is applied once when streaming finishes: strip reasoning
// blocks first, then collapse a repeated answer if the model emitted one.
export function cleanFinalContent(raw: string): string {
  let cleaned = collapseConsecutiveDuplicates(cleanThinkBlocks(raw), 400)
  // 防空兜底：若过滤后为空，但原始文本包含大量字符，智能挽救正文内容
  if (!cleaned.trim() && raw && raw.trim().length > 100) {
    const salvaged = salvageRawContent(raw)
    if (salvaged) {
      cleaned = collapseConsecutiveDuplicates(salvaged, 400)
    }
  }
  // 剥离开头规划杂音与所有 agent-action 代码块
  cleaned = stripLeadingPlanningNoise(cleaned)
  cleaned = stripAllAgentActionBlocks(cleaned)
  return cleaned
}
