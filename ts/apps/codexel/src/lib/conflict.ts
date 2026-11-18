// Side is a side of the conflict. label is the description of the commit in the marker.
export type Side = { label: string; text: string }

// Segment is the part of the conflicted file. The conflict has the sides and the base.
export type Segment = { kind: 'text'; text: string } | { kind: 'conflict'; sides: Side[]; base: Side[] }

// parseConflict parses the file with the conflict markers of the snapshot style of jj:
//
//	<<<<<<< conflict 1 of 1
//	+++++++ <side>
//	------- <base>
//	+++++++ <side>
//	>>>>>>> conflict 1 of 1 ends
//
// jj makes the markers longer if the content has the same characters, so the length of the start marker is used.
export const parseConflict = (content: string): Segment[] => {
  const segments: Segment[] = []
  const lines = content.split('\n')
  let text: string[] = []
  let i = 0
  while (i < lines.length) {
    const start = /^(<{7,}) conflict/.exec(lines[i])
    if (!start) {
      text.push(lines[i])
      i++
      continue
    }
    const n = start[1].length
    const end = new RegExp(`^>{${n}} `)
    const sideMarker = new RegExp(`^\\+{${n}} ?(.*)$`)
    const baseMarker = new RegExp(`^-{${n}} ?(.*)$`)
    const sides: Side[] = []
    const base: Side[] = []
    let current: { side: Side; lines: string[] } | undefined
    const flush = () => {
      if (current) {
        current.side.text = current.lines.join('\n')
      }
    }
    i++
    while (i < lines.length && !end.test(lines[i])) {
      const s = sideMarker.exec(lines[i])
      const b = baseMarker.exec(lines[i])
      if (s || b) {
        flush()
        const side = { label: (s ?? b)![1], text: '' }
        ;(s ? sides : base).push(side)
        current = { side, lines: [] }
      } else {
        current?.lines.push(lines[i])
      }
      i++
    }
    flush()
    i++
    if (text.length > 0) {
      segments.push({ kind: 'text', text: text.join('\n') })
      text = []
    }
    segments.push({ kind: 'conflict', sides, base })
  }
  if (text.length > 0) {
    segments.push({ kind: 'text', text: text.join('\n') })
  }
  return segments
}

// composeResolution joins the text and the resolutions of the conflicts. resolutions[i] is the text of the i-th
// conflict.
export const composeResolution = (segments: Segment[], resolutions: string[]): string => {
  let conflict = 0
  return segments.map((s) => (s.kind === 'text' ? s.text : resolutions[conflict++] ?? '')).join('\n')
}
