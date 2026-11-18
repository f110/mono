import { useMemo } from 'react'
import { Box, Chip, Stack, Typography } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import { DiffLineKind, FileStatus, type DiffLine, type FileDiff, type Hunk } from '../connect/codexel_pb.js'
import { lineRanges, splitRows, type Range } from '../lib/diffLines.ts'
import { languageOf, tokenize } from '../lib/highlight.ts'

const statusLabel: Record<FileStatus, string> = {
  [FileStatus.UNSPECIFIED]: '',
  [FileStatus.MODIFIED]: 'M',
  [FileStatus.ADDED]: 'A',
  [FileStatus.DELETED]: 'D',
  [FileStatus.RENAMED]: 'R',
  [FileStatus.COPIED]: 'C',
}

export const FileStatusChip = ({ status }: { status: FileStatus }) => (
  <Chip
    label={statusLabel[status]}
    size="small"
    variant="outlined"
    color={status === FileStatus.ADDED ? 'success' : status === FileStatus.DELETED ? 'error' : 'default'}
    sx={{ height: 18, minWidth: 24, fontFamily: 'monospace' }}
  />
)

const marker: Record<DiffLineKind, string> = {
  [DiffLineKind.UNSPECIFIED]: ' ',
  [DiffLineKind.CONTEXT]: ' ',
  [DiffLineKind.ADDED]: '+',
  [DiffLineKind.REMOVED]: '-',
}

const LineNumber = ({ n }: { n: number }) => (
  <Box component="span" sx={{ width: 48, flexShrink: 0, textAlign: 'right', pr: 1, color: 'text.disabled', userSelect: 'none' }}>
    {n > 0 ? n : ''}
  </Box>
)

// LineContent renders the highlighted tokens of the line. The tokens are split at the boundaries of the changed ranges
// of the word diff to emphasize them.
const LineContent = ({ line, lang, ranges, emphasis }: { line: DiffLine; lang: string | undefined; ranges: Range[] | undefined; emphasis: string }) => {
  const segments: { text: string; className?: string; changed: boolean }[] = []
  let pos = 0
  for (const token of tokenize(line.text, lang)) {
    let offset = 0
    while (offset < token.text.length) {
      const at = pos + offset
      const changed = ranges?.some((r) => r.start <= at && at < r.end) ?? false
      // The next boundary is the end of the token or the next start/end of the ranges.
      let end = token.text.length
      for (const r of ranges ?? []) {
        for (const b of [r.start, r.end]) {
          if (b > at && b - pos < end) {
            end = b - pos
          }
        }
      }
      segments.push({ text: token.text.slice(offset, end), className: token.className, changed })
      offset = end
    }
    pos += token.text.length
  }

  return (
    <Box component="span" sx={{ whiteSpace: 'pre' }}>
      {segments.map((s, i) => (
        <span key={i} className={s.className} style={s.changed ? { backgroundColor: emphasis, borderRadius: 2 } : undefined}>
          {s.text}
        </span>
      ))}
      {line.noNewline && (
        <Box component="span" sx={{ color: 'text.disabled' }} title="No newline at end of file">
          {' ⏎̸'}
        </Box>
      )}
    </Box>
  )
}

const HunkHeader = ({ hunk }: { hunk: Hunk }) => {
  const theme = useTheme()
  return (
    <Box data-hunk sx={{ px: 1, color: 'text.secondary', bgcolor: alpha(theme.palette.info.main, 0.08), scrollMarginTop: 48 }}>
      @@ -{hunk.oldStart},{hunk.oldLines} +{hunk.newStart},{hunk.newLines} @@ {hunk.section}
    </Box>
  )
}

// FileDiffView shows the diff of the file in the unified or the split format.
export const FileDiffView = ({ file, split }: { file: FileDiff; split: boolean }) => {
  const theme = useTheme()
  const lang = languageOf(file.path)
  const ranges = useMemo(() => file.hunks.map((h) => lineRanges(h)), [file])
  const background: Record<DiffLineKind, string | undefined> = {
    [DiffLineKind.UNSPECIFIED]: undefined,
    [DiffLineKind.CONTEXT]: undefined,
    [DiffLineKind.ADDED]: alpha(theme.palette.success.main, 0.12),
    [DiffLineKind.REMOVED]: alpha(theme.palette.error.main, 0.12),
  }
  const emphasis: Record<DiffLineKind, string> = {
    [DiffLineKind.UNSPECIFIED]: 'transparent',
    [DiffLineKind.CONTEXT]: 'transparent',
    [DiffLineKind.ADDED]: alpha(theme.palette.success.main, 0.35),
    [DiffLineKind.REMOVED]: alpha(theme.palette.error.main, 0.35),
  }

  const cell = (line: DiffLine | undefined, hunkIndex: number, side: 'old' | 'new') => (
    <Stack direction="row" sx={{ flex: 1, minWidth: 0, bgcolor: line ? background[line.kind] : 'action.hover' }}>
      {line && (
        <>
          <LineNumber n={side === 'old' ? line.oldLine : line.newLine} />
          <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
            {marker[line.kind]}
          </Box>
          <LineContent line={line} lang={lang} ranges={ranges[hunkIndex].get(line)} emphasis={emphasis[line.kind]} />
        </>
      )}
    </Stack>
  )

  return (
    <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mb: 2 }}>
      <Stack direction="row" spacing={1} sx={{ p: 1, alignItems: 'center', bgcolor: 'background.paper', position: 'sticky', top: 0, zIndex: 1, borderBottom: 1, borderColor: 'divider' }}>
        <FileStatusChip status={file.status} />
        <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1, minWidth: 0 }} noWrap title={file.path}>
          {file.oldPath ? `${file.oldPath} → ${file.path}` : file.path}
        </Typography>
        <Typography variant="caption" sx={{ color: 'success.main' }}>
          +{file.added}
        </Typography>
        <Typography variant="caption" sx={{ color: 'error.main' }}>
          -{file.removed}
        </Typography>
      </Stack>
      {file.binary && (
        <Typography variant="body2" color="text.secondary" sx={{ p: 1 }}>
          Binary file
        </Typography>
      )}
      <Box sx={{ fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5, overflowX: 'auto' }}>
        {file.hunks.map((h, i) => (
          <Box key={i}>
            <HunkHeader hunk={h} />
            {split
              ? splitRows(h).map((row, j) => (
                  <Stack key={j} direction="row">
                    {cell(row.left, i, 'old')}
                    <Box sx={{ width: '1px', bgcolor: 'divider', flexShrink: 0 }} />
                    {cell(row.right, i, 'new')}
                  </Stack>
                ))
              : h.lines.map((l, j) => (
                  <Stack key={j} direction="row" sx={{ bgcolor: background[l.kind] }}>
                    <LineNumber n={l.oldLine} />
                    <LineNumber n={l.newLine} />
                    <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
                      {marker[l.kind]}
                    </Box>
                    <LineContent line={l} lang={lang} ranges={ranges[i].get(l)} emphasis={emphasis[l.kind]} />
                  </Stack>
                ))}
          </Box>
        ))}
      </Box>
    </Box>
  )
}
