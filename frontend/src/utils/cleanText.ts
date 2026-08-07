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
      inThink = true
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

// cleanFinalContent is applied once when streaming finishes: strip reasoning
// blocks first, then collapse a repeated answer if the model emitted one.
export function cleanFinalContent(raw: string): string {
  return collapseConsecutiveDuplicates(cleanThinkBlocks(raw), 400)
}
