import { Fragment, useEffect, useMemo, useState, type MouseEvent, type ReactNode } from 'react'
import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { Box, Button, Chip, Stack, Typography } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import { DiffLineKind, DiffLineSchema, FileStatus, RepositoryService, type DiffLine, type FileDiff, type Hunk } from '../connect/codexel_pb.js'
import { lineRanges, lineSide, splitRows, type LineSide, type Range } from '../lib/diffLines.ts'
import { languageOf, tokenize } from '../lib/highlight.ts'
import { Markdown } from './Markdown.tsx'

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
    sx={{ minWidth: 24, fontFamily: 'monospace' }}
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
export const isMarkdown = (path: string) => ['md', 'markdown'].includes(extension(path))

const LineNumber = ({ n, onClick }: { n: number; onClick?: (e: MouseEvent) => void }) => (
  <Box
    component="span"
    onClick={onClick}
    sx={{
      width: 48,
      flexShrink: 0,
      textAlign: 'right',
      pr: 1,
      color: 'text.disabled',
      userSelect: 'none',
      ...(onClick && { cursor: 'pointer', '&:hover': { color: 'primary.main' } }),
    }}
  >
    {n > 0 ? n : ''}
  </Box>
)

// LineAnnotation attaches the elements (e.g. the comments) to the lines. The lines are identified by the side and the
// line number in the file of the side.
export type LineAnnotation = {
  // onSelect is called when the line number is clicked. extend is true if Shift is pressed to select the range.
  onSelect: (side: LineSide, line: number, extend: boolean) => void
  selected: (side: LineSide, line: number) => boolean
  // focused is the line of the cursor of the keyboard.
  focused: (side: LineSide, line: number) => boolean
  // after returns the element shown after the line.
  after: (side: LineSide, line: number) => ReactNode
}

// LineContent renders the highlighted tokens of the line. The tokens are split at the boundaries of the changed ranges
// of the word diff to emphasize them.
const LineContent = ({
  line,
  lang,
  ranges,
  emphasis,
}: {
  line: DiffLine
  lang: string | undefined
  ranges: Range[] | undefined
  emphasis: string
}) => {
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
    <Box data-hunk sx={{ px: 1, color: 'text.secondary', bgcolor: alpha(theme.palette.info.main, 0.08) }}>
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
  useQuery(
    RepositoryService.method.getFile,
    { path: repoPath, revision: revision ?? '', file },
    { enabled: enabled && revision !== undefined, staleTime: Infinity },
  )

const ImageDiff = ({
  file,
  repoPath,
  revision,
  oldRevision,
}: {
  file: FileDiff
  repoPath: string
  revision: string
  oldRevision: string | undefined
}) => {
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
          {v.url ? (
            <Box component="img" src={v.url} sx={{ display: 'block', maxWidth: '100%', border: 1, borderColor: 'divider' }} />
          ) : (
            <Typography color="text.disabled">-</Typography>
          )}
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
  onRestore,
  title,
  annotation,
  actions,
  collapsed = false,
  children,
}: {
  file: FileDiff
  split: boolean
  preview: boolean
  repoPath: string
  revision: string
  oldRevision: string | undefined
  // onHistory shows the history of the file. It is undefined if the history can't be shown.
  onHistory?: (file: string) => void
  // onRestore discards the changes of the file. It is undefined if the revision can't be rewritten.
  onRestore?: (file: string) => void
  annotation?: LineAnnotation
  // actions are shown in the header.
  actions?: ReactNode
  // collapsed hides everything except the header.
  collapsed?: boolean
  // title is shown in the header instead of the status and the path if the content is not a file. The hunks of it must
  // have all lines because the content can't be read.
  title?: string
  // children are shown under the header.
  children?: ReactNode
}) => {
  const theme = useTheme()
  const lang = languageOf(file.path)
  const ranges = useMemo(() => file.hunks.map((h) => lineRanges(h)), [file])
  const fileGaps = useMemo(() => gaps(file.hunks), [file])
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set())
  const expandable =
    title === undefined && !file.binary && file.status !== FileStatus.ADDED && file.status !== FileStatus.DELETED && file.hunks.length > 0
  const showPreview = title === undefined && preview && isMarkdown(file.path) && file.status !== FileStatus.DELETED
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
  const selectedBackground = alpha(theme.palette.primary.main, 0.2)

  const onSelect = (line: DiffLine) => {
    if (!annotation) return undefined
    const { side, line: n } = lineSide(line)
    return (e: MouseEvent) => annotation.onSelect(side, n, e.shiftKey)
  }
  const lineBackground = (line: DiffLine) => {
    const { side, line: n } = lineSide(line)
    return annotation?.selected(side, n) ? selectedBackground : background[line.kind]
  }
  // lineProps identifies the line by data-line to move the cursor by the keyboard, and outlines the line of the cursor.
  const lineProps = (line: DiffLine) => {
    const { side, line: n } = lineSide(line)
    return {
      'data-line': annotation ? `${side}:${n}` : undefined,
      boxShadow: annotation?.focused(side, n) ? `inset 2px 0 0 ${theme.palette.primary.main}` : undefined,
    }
  }
  // after returns the elements after the lines. The same line is shown once even if it is on both sides of the split view.
  const after = (...ls: (DiffLine | undefined)[]) => {
    if (!annotation) return null
    const keys = new Set<string>()
    const elements: ReactNode[] = []
    for (const l of ls) {
      if (!l) continue
      const { side, line: n } = lineSide(l)
      if (keys.has(`${side}:${n}`)) continue
      keys.add(`${side}:${n}`)
      elements.push(<Fragment key={`${side}:${n}`}>{annotation.after(side, n)}</Fragment>)
    }
    return elements
  }

  const cell = (line: DiffLine | undefined, ranges: Range[] | undefined, side: 'old' | 'new') => (
    <Stack
      direction="row"
      data-line={line ? lineProps(line)['data-line'] : undefined}
      sx={{ flex: 1, minWidth: 0, bgcolor: line ? lineBackground(line) : 'action.hover', boxShadow: line ? lineProps(line).boxShadow : undefined }}
    >
      {line && (
        <>
          <LineNumber n={side === 'old' ? line.oldLine : line.newLine} onClick={onSelect(line)} />
          <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
            {marker[line.kind]}
          </Box>
          <LineContent line={line} lang={lang} ranges={ranges} emphasis={emphasis[line.kind]} />
        </>
      )}
    </Stack>
  )

  const splitRow = (
    left: DiffLine | undefined,
    right: DiffLine | undefined,
    leftRanges: Range[] | undefined,
    rightRanges: Range[] | undefined,
    key: string | number,
  ) => (
    <Fragment key={key}>
      <Stack direction="row">
        {cell(left, leftRanges, 'old')}
        <Box sx={{ width: '1px', bgcolor: 'divider', flexShrink: 0 }} />
        {cell(right, rightRanges, 'new')}
      </Stack>
      {after(left, right)}
    </Fragment>
  )

  const row = (line: DiffLine, ranges: Range[] | undefined, key: string | number) => (
    <Fragment key={key}>
      <Stack direction="row" data-line={lineProps(line)['data-line']} sx={{ bgcolor: lineBackground(line), boxShadow: lineProps(line).boxShadow }}>
        <LineNumber n={line.oldLine} onClick={onSelect(line)} />
        <LineNumber n={line.newLine} onClick={onSelect(line)} />
        <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
          {marker[line.kind]}
        </Box>
        <LineContent line={line} lang={lang} ranges={ranges} emphasis={emphasis[line.kind]} />
      </Stack>
      {after(line)}
    </Fragment>
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
          {content.isLoading && expanded.has(index)
            ? 'Loading…'
            : count === undefined
              ? '↕ Show the rest of the file'
              : `↕ Show ${count} hidden line(s)`}
        </Button>
      )
    }
    const end = gap.end ?? lines.length
    const context: DiffLine[] = []
    for (let n = gap.start; n <= end; n++) {
      context.push(create(DiffLineSchema, { kind: DiffLineKind.CONTEXT, text: lines[n - 1] ?? '', oldLine: n + gap.delta, newLine: n }))
    }
    return context.map((l, j) => (split ? splitRow(l, l, undefined, undefined, `g${j}`) : row(l, undefined, `g${j}`)))
  }

  // --sticky-top is the height of the sticky elements above the files in the scroll container. The header of the file sticks
  // under them, and scrollMarginTop leaves the space for them and the header.
  return (
    <Box
      data-file={file.path}
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        mb: 2,
        scrollMarginTop: 'calc(var(--sticky-top, 0px) + 8px)',
        '& [data-hunk], & [data-line]': { scrollMarginTop: 'calc(var(--sticky-top, 0px) + 48px)' },
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          p: 1,
          alignItems: 'center',
          bgcolor: 'background.paper',
          position: 'sticky',
          top: 'var(--sticky-top, 0px)',
          zIndex: 1,
          borderBottom: 1,
          borderColor: 'divider',
        }}
      >
        {title === undefined ? (
          <>
            <FileStatusChip status={file.status} />
            <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1, minWidth: 0 }} noWrap title={file.path}>
              {file.oldPath ? `${file.oldPath} → ${file.path}` : file.path}
            </Typography>
          </>
        ) : (
          <Typography variant="body2" sx={{ fontWeight: 'bold', flex: 1, minWidth: 0 }} noWrap>
            {title}
          </Typography>
        )}
        <Typography variant="caption" sx={{ color: 'success.main' }}>
          +{file.added}
        </Typography>
        <Typography variant="caption" sx={{ color: 'error.main' }}>
          -{file.removed}
        </Typography>
        {onHistory && (
          <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => onHistory(file.path)}>
            History
          </Button>
        )}
        {onRestore && (
          <Button size="small" color="warning" sx={{ minWidth: 0, py: 0 }} onClick={() => onRestore(file.path)}>
            Restore
          </Button>
        )}
        {actions}
      </Stack>
      {!collapsed && children}
      {!collapsed && file.binary && imageTypes[extension(file.path)] && (
        <ImageDiff file={file} repoPath={repoPath} revision={revision} oldRevision={oldRevision} />
      )}
      {!collapsed && file.binary && !imageTypes[extension(file.path)] && (
        <Typography variant="body2" color="text.secondary" sx={{ p: 1 }}>
          Binary file
        </Typography>
      )}
      {collapsed ? null : showPreview ? (
        <Box className="markdown-body" sx={{ p: 2, '& pre': { overflowX: 'auto' }, '& img': { maxWidth: '100%' } }}>
          {lines ? <Markdown text={lines.join('\n')} /> : 'Loading…'}
        </Box>
      ) : (
        <Box sx={{ fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5, overflowX: 'auto' }}>
          {file.hunks.map((h, i) => (
            <Box key={i}>
              {renderGap(i)}
              <HunkHeader hunk={h} />
              {split
                ? splitRows(h).map((r, j) =>
                    splitRow(r.left, r.right, r.left ? ranges[i].get(r.left) : undefined, r.right ? ranges[i].get(r.right) : undefined, j),
                  )
                : h.lines.map((l, j) => row(l, ranges[i].get(l), j))}
            </Box>
          ))}
          {renderGap(file.hunks.length)}
        </Box>
      )}
    </Box>
  )
}
