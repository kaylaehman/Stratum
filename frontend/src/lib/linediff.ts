// Minimal dependency-free line diff (LCS) used by the config editor's
// review-before-save view. Returns a sequence of ops over the two texts' lines.

export type DiffOp = 'equal' | 'add' | 'remove'

export interface DiffLine {
  op: DiffOp
  /** 1-based line number in the OLD text (undefined for added lines). */
  oldLine?: number
  /** 1-based line number in the NEW text (undefined for removed lines). */
  newLine?: number
  text: string
}

/** diffLines computes a line-level diff between old and new text via LCS. */
export function diffLines(oldText: string, newText: string): DiffLine[] {
  const a = oldText.split('\n')
  const b = newText.split('\n')
  const n = a.length
  const m = b.length

  // LCS length table.
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1])
    }
  }

  const out: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ op: 'equal', oldLine: i + 1, newLine: j + 1, text: a[i] })
      i++
      j++
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ op: 'remove', oldLine: i + 1, text: a[i] })
      i++
    } else {
      out.push({ op: 'add', newLine: j + 1, text: b[j] })
      j++
    }
  }
  while (i < n) {
    out.push({ op: 'remove', oldLine: i + 1, text: a[i] })
    i++
  }
  while (j < m) {
    out.push({ op: 'add', newLine: j + 1, text: b[j] })
    j++
  }
  return out
}

/** diffStats counts added and removed lines in a diff. */
export function diffStats(lines: DiffLine[]): { added: number; removed: number } {
  let added = 0
  let removed = 0
  for (const l of lines) {
    if (l.op === 'add') added++
    else if (l.op === 'remove') removed++
  }
  return { added, removed }
}
