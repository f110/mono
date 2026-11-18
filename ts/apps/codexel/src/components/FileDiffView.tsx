import { useEffect, useMemo, useState } from 'react'
import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { Box, Button, Chip, Stack, Typography } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { DiffLineKind, DiffLineSchema, FileService, FileStatus, type DiffLine, type FileDiff, type Hunk } from '../connect/codexel_pb.js'
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

const imageTypes: Record<string, string> = {
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  ico: 'image/x-icon',
}

const extension = (path: string) => path.split('.').pop()?.toLowerCase() ?? ''
const isMarkdown = (path: string) => ['md', 'markdown'].includes(extension(path))

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

// Gap is the unchanged lines which the diff doesn't show. The lines are the numbers in the new file. end is undefined
// for the lines after the last hunk because the length of the file is unknown until it is read.
type Gap = { start: number; end: number | undefined; delta: number }

// gaps returns the gap before each hunk and the one after the last hunk. delta converts the line number in the new
// file into the one in the old file.
const gaps = (hunks: Hunk[]): Gap[] => {
  const result: Gap[] = []
  let newNext = 1
  let oldNext = 1
  for (const h of hunks) {
    // The hunk which has no lines in the file starts after the line of start.
    const newFirst = h.newLines === 0 ? h.newStart + 1 : h.newStart
    result.push({ start: newNext, end: newFirst - 1, delta: oldNext - newNext })
    newNext = newFirst + h.newLines
    oldNext = (h.oldLines === 0 ? h.oldStart + 1 : h.oldStart) + h.oldLines
  }
  result.push({ start: newNext, end: undefined, delta: oldNext - newNext })
  return result
}

const decoder = new TextDecoder()

// useFile reads the file at the revision when enabled is true.
const useFile = (repoPath: string, revision: string | undefined, file: string, enabled: boolean) =>
  useQuery(FileService.method.getFile, { path: repoPath, revision: revision ?? '', file }, { enabled: enabled && revision !== undefined, staleTime: Infinity })

const ImageDiff = ({ file, repoPath, revision, oldRevision }: { file: FileDiff; repoPath: string; revision: string; oldRevision: string | undefined }) => {
  const type = imageTypes[extension(file.path)]
  const newImage = useFile(repoPath, revision, file.path, file.status !== FileStatus.DELETED)
  const oldImage = useFile(repoPath, oldRevision, file.oldPath || file.path, file.status !== FileStatus.ADDED)
  const [urls, setUrls] = useState<{ old?: string; new?: string }>({})

  useEffect(() => {
    const toUrl = (content: Uint8Array | undefined) => (content ? URL.createObjectURL(new Blob([content as BlobPart], { type })) : undefined)
    const next = { old: toUrl(oldImage.data?.content), new: toUrl(newImage.data?.content) }
    setUrls(next)
    return () => {
      for (const u of [next.old, next.new]) {
        if (u) URL.revokeObjectURL(u)
      }
    }
  }, [oldImage.data, newImage.data, type])

  return (
    <Stack direction="row" spacing={2} sx={{ p: 1 }}>
      {[
        { label: 'Before', url: urls.old },
        { label: 'After', url: urls.new },
      ].map((v) => (
        <Box key={v.label} sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="caption" color="text.secondary">
            {v.label}
          </Typography>
          {v.url ? <Box component="img" src={v.url} sx={{ display: 'block', maxWidth: '100%', border: 1, borderColor: 'divider' }} /> : <Typography color="text.disabled">-</Typography>}
        </Box>
      ))}
    </Stack>
  )
}

// FileDiffView shows the diff of the file in the unified or the split format. revision is the commit of the new side
// and oldRevision is the one of the old side (the parent). The unchanged lines and the images are read from them.
export const FileDiffView = ({
  file,
  split,
  preview,
  repoPath,
  revision,
  oldRevision,
  onHistory,
}: {
  file: FileDiff
  split: boolean
  preview: boolean
  repoPath: string
  revision: string
  oldRevision: string | undefined
  onHistory: (file: string) => void
}) => {
  const theme = useTheme()
  const lang = languageOf(file.path)
  const ranges = useMemo(() => file.hunks.map((h) => lineRanges(h)), [file])
  const fileGaps = useMemo(() => gaps(file.hunks), [file])
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const expandable = !file.binary && file.status !== FileStatus.ADDED && file.status !== FileStatus.DELETED && file.hunks.length > 0
  const showPreview = preview && isMarkdown(file.path) && file.status !== FileStatus.DELETED
  const content = useFile(repoPath, revision, file.path, (expandable && expanded.size > 0) || showPreview)
  const lines = useMemo(() => {
    if (!content.data) return undefined
    const text = decoder.decode(content.data.content)
    const result = text.split('\n')
    if (text.endsWith('\n')) result.pop()
    return result
  }, [content.data])
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

  const cell = (line: DiffLine | undefined, ranges: Range[] | undefined, side: 'old' | 'new') => (
    <Stack direction="row" sx={{ flex: 1, minWidth: 0, bgcolor: line ? background[line.kind] : 'action.hover' }}>
      {line && (
        <>
          <LineNumber n={side === 'old' ? line.oldLine : line.newLine} />
          <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
            {marker[line.kind]}
          </Box>
          <LineContent line={line} lang={lang} ranges={ranges} emphasis={emphasis[line.kind]} />
        </>
      )}
    </Stack>
  )

  const row = (line: DiffLine, ranges: Range[] | undefined, key: string | number) => (
    <Stack key={key} direction="row" sx={{ bgcolor: background[line.kind] }}>
      <LineNumber n={line.oldLine} />
      <LineNumber n={line.newLine} />
      <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
        {marker[line.kind]}
      </Box>
      <LineContent line={line} lang={lang} ranges={ranges} emphasis={emphasis[line.kind]} />
    </Stack>
  )

  const renderGap = (index: number) => {
    const gap = fileGaps[index]
    if (!expandable || (gap.end !== undefined && gap.end < gap.start)) {
      return null
    }
    if (!expanded.has(index) || !lines) {
      const count = gap.end === undefined ? undefined : gap.end - gap.start + 1
      return (
        <Button
          size="small"
          fullWidth
          onClick={() => setExpanded((s) => new Set(s).add(index))}
          sx={{ justifyContent: 'flex-start', borderRadius: 0, fontSize: 11, py: 0, color: 'text.secondary', bgcolor: 'action.hover' }}
        >
          {content.isLoading && expanded.has(index) ? 'Loading…' : count === undefined ? '↕ Show the rest of the file' : `↕ Show ${count} hidden line(s)`}
        </Button>
      )
    }
    const end = gap.end ?? lines.length
    const context: DiffLine[] = []
    for (let n = gap.start; n <= end; n++) {
      context.push(create(DiffLineSchema, { kind: DiffLineKind.CONTEXT, text: lines[n - 1] ?? '', oldLine: n + gap.delta, newLine: n }))
    }
    return context.map((l, j) =>
      split ? (
        <Stack key={`g${j}`} direction="row">
          {cell(l, undefined, 'old')}
          <Box sx={{ width: '1px', bgcolor: 'divider', flexShrink: 0 }} />
          {cell(l, undefined, 'new')}
        </Stack>
      ) : (
        row(l, undefined, `g${j}`)
      ),
    )
  }

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
        <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => onHistory(file.path)}>
          History
        </Button>
      </Stack>
      {file.binary && imageTypes[extension(file.path)] && <ImageDiff file={file} repoPath={repoPath} revision={revision} oldRevision={oldRevision} />}
      {file.binary && !imageTypes[extension(file.path)] && (
        <Typography variant="body2" color="text.secondary" sx={{ p: 1 }}>
          Binary file
        </Typography>
      )}
      {showPreview ? (
        <Box className="markdown-body" sx={{ p: 2, '& pre': { overflowX: 'auto' }, '& img': { maxWidth: '100%' } }}>
          {lines ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{lines.join('\n')}</ReactMarkdown> : 'Loading…'}
        </Box>
      ) : (
        <Box sx={{ fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5, overflowX: 'auto' }}>
          {file.hunks.map((h, i) => (
            <Box key={i}>
              {renderGap(i)}
              <HunkHeader hunk={h} />
              {split
                ? splitRows(h).map((r, j) => (
                    <Stack key={j} direction="row">
                      {cell(r.left, r.left ? ranges[i].get(r.left) : undefined, 'old')}
                      <Box sx={{ width: '1px', bgcolor: 'divider', flexShrink: 0 }} />
                      {cell(r.right, r.right ? ranges[i].get(r.right) : undefined, 'new')}
                    </Stack>
                  ))
                : h.lines.map((l, j) => row(l, ranges[i].get(l), j))}
            </Box>
          ))}
          {renderGap(file.hunks.length)}
        </Box>
      )}
    </Box>
  )
}
