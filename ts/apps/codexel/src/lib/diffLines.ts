import { diffWordsWithSpace } from 'diff'
import { DiffLineKind, type DiffLine, type Hunk } from '../connect/codexel_pb.js'

// Range is the range of the characters changed in the line.
export type Range = { start: number; end: number }

// maxWordDiffLength is the length of the line to compute the word diff. The longer lines are not compared because the
// diff is slow and not readable.
const maxWordDiffLength = 500
// maxChangedRatio is the ratio of the changed characters to show the word diff. The lines which are mostly changed
// are shown as the whole line changes.
const maxChangedRatio = 0.6

// wordRanges compares the removed and the added line and returns the changed ranges of both.
export const wordRanges = (oldText: string, newText: string): { old: Range[]; new: Range[] } | undefined => {
  if (oldText.length > maxWordDiffLength || newText.length > maxWordDiffLength) {
    return undefined
  }
  const oldRanges: Range[] = []
  const newRanges: Range[] = []
  let oldPos = 0
  let newPos = 0
  let changed = 0
  for (const c of diffWordsWithSpace(oldText, newText)) {
    const len = c.value.length
    if (c.removed) {
      oldRanges.push({ start: oldPos, end: oldPos + len })
      oldPos += len
      changed += len
    } else if (c.added) {
      newRanges.push({ start: newPos, end: newPos + len })
      newPos += len
      changed += len
    } else {
      oldPos += len
      newPos += len
    }
  }
  if (changed / Math.max(1, oldText.length + newText.length) > maxChangedRatio) {
    return undefined
  }
  return { old: oldRanges, new: newRanges }
}

// Row is a row of the split view. The removed and the added lines in a block are put side by side.
export type Row = { left?: DiffLine; right?: DiffLine }

// pairBlocks calls fn for each block of the removed lines followed by the added lines.
const pairBlocks = (lines: DiffLine[], fn: (removed: DiffLine[], added: DiffLine[]) => void, context: (line: DiffLine) => void) => {
  let i = 0
  while (i < lines.length) {
    if (lines[i].kind !== DiffLineKind.REMOVED && lines[i].kind !== DiffLineKind.ADDED) {
      context(lines[i])
      i++
      continue
    }
    const removed: DiffLine[] = []
    const added: DiffLine[] = []
    while (i < lines.length && lines[i].kind === DiffLineKind.REMOVED) removed.push(lines[i++])
    while (i < lines.length && lines[i].kind === DiffLineKind.ADDED) added.push(lines[i++])
    fn(removed, added)
  }
}

export const splitRows = (hunk: Hunk): Row[] => {
  const rows: Row[] = []
  pairBlocks(
    hunk.lines,
    (removed, added) => {
      for (let j = 0; j < Math.max(removed.length, added.length); j++) {
        rows.push({ left: removed[j], right: added[j] })
      }
    },
    (line) => rows.push({ left: line, right: line }),
  )
  return rows
}

// lineRanges returns the changed ranges of the lines by the word diff. The i-th removed line in a block is compared
// with the i-th added line.
export const lineRanges = (hunk: Hunk): Map<DiffLine, Range[]> => {
  const ranges = new Map<DiffLine, Range[]>()
  pairBlocks(
    hunk.lines,
    (removed, added) => {
      for (let j = 0; j < Math.min(removed.length, added.length); j++) {
        const r = wordRanges(removed[j].text, added[j].text)
        if (r) {
          ranges.set(removed[j], r.old)
          ranges.set(added[j], r.new)
        }
      }
    },
    () => {},
  )
  return ranges
}
