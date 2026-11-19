import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  MenuItem,
  Snackbar,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { useParams } from '@tanstack/react-router'
import { ChangedFiles } from '../components/ChangedFiles.tsx'
import { FileDiffView, type LineAnnotation } from '../components/FileDiffView.tsx'
import { CommentComposer, CommentThread } from '../components/ReviewComments.tsx'
import { DiffSide, ReviewService, ReviewState, type Review, type ReviewComment } from '../connect/codexel_pb.js'
import { useDiffNavigation } from '../hooks/useDiffNavigation.ts'
import { lineSide, type LineSide } from '../lib/diffLines.ts'
import { ignoreKey } from '../lib/keys.ts'
import { stateChip } from '../lib/review.ts'
import { transport } from '../providers/AppProvider.tsx'

const client = createClient(ReviewService, transport)

// reviewRefreshInterval is the interval to read the review again. The agent replies to the comments and starts the next
// round by MCP, which is not notified.
const reviewRefreshInterval = 5000

const sides: Record<LineSide, DiffSide> = { old: DiffSide.OLD, new: DiffSide.NEW }

const shortcuts: [string, string][] = [
  ['j / k', 'Move the cursor to the next / previous line'],
  ['c', 'Comment on the line of the cursor'],
  ['a', 'Toggle viewed on the file of the cursor'],
  ['Shift+F', 'Finish the review'],
  ['] / [', 'Next / previous hunk'],
  ['|', 'Toggle the split view'],
  ['m', 'Toggle the preview of Markdown'],
  ['Click / Shift+Click', 'Select the line / the range to comment'],
  ['Cmd/Ctrl+Enter', 'Submit the comment'],
  ['Esc', 'Cancel the comment / clear the cursor'],
  ['?', 'Show this help'],
]

// LinePosition is the line in the file of the change.
type LinePosition = { changeId: string; path: string; side: LineSide; line: number }

// Selection is the lines selected to comment. origin is the line clicked first to extend the range by Shift.
type Selection = { changeId: string; path: string; side: LineSide; start: number; end: number; origin: number }

// Actions are the operations of the review. They report the error and read the review again.
type Actions = {
  add: (changeId: string, path: string, side: DiffSide, startLine: number, endLine: number, body: string) => Promise<void>
  setViewed: (changeId: string, path: string, viewed: boolean) => void
  thread: (c: ReviewComment) => {
    onReply: (body: string) => Promise<void>
    onResolve: (resolved: boolean) => void
    onEdit?: (body: string) => Promise<void>
    onDelete?: () => void
  }
}

const isViewed = (review: Review, changeId: string, path: string) => review.viewed.some((v) => v.changeId === changeId && v.path === path && v.viewed)

// ChangeReview shows the diff of the change between the rounds and the comments on it. The comments are shown at the
// lines only if the diff is the one of the latest round from the parent because the lines of the comments are the
// lines of it. The viewed files and the generated files are collapsed until they are opened.
const ChangeReview = ({
  review,
  changeId,
  fromRound,
  toRound,
  inline,
  commentable,
  split,
  preview,
  selection,
  onSelect,
  focus,
  actions,
}: {
  review: Review
  changeId: string
  fromRound: number
  toRound: number
  inline: boolean
  commentable: boolean
  split: boolean
  preview: boolean
  selection: Selection | undefined
  onSelect: (s: Selection | undefined) => void
  focus: LinePosition | undefined
  actions: Actions
}) => {
  const { data, error } = useQuery(ReviewService.method.roundDiff, { reviewId: review.id, changeId, fromRound, toRound }, { staleTime: Infinity })
  const ref = useRef<HTMLDivElement>(null)
  const [fileComposer, setFileComposer] = useState<string>()
  // opened is the files opened or closed explicitly. It overrides the default of the viewed and the generated files.
  const [opened, setOpened] = useState<Record<string, boolean>>({})
  const comments = review.comments.filter((c) => c.changeId === changeId)
  const files = data?.files ?? []
  // The commit message is shown as the first file. The path of it is empty.
  const entries = data?.message ? [data.message, ...files] : files
  const otherComments = comments.filter((c) => !entries.some((f) => f.path === c.path))

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (!data) {
    return null
  }
  const commit = data.commit
  return (
    // scrollMarginTop of the files leaves the space for the sticky header of the change.
    <Box ref={ref} data-change={changeId} sx={{ mb: 4, '& [data-file]': { scrollMarginTop: 56 } }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline', position: 'sticky', top: 0, zIndex: 2, bgcolor: 'background.default', py: 1 }}>
        <Typography variant="h6" sx={{ flex: 1, minWidth: 0 }} noWrap>
          {commit?.description || '(no description set)'}
        </Typography>
        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
          {changeId.slice(0, 8)}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          {commit?.commitId}
        </Typography>
      </Stack>
      {files.length > 0 && (
        <ChangedFiles files={files} onSelect={(p) => ref.current?.querySelector(`[data-file="${CSS.escape(p)}"]`)?.scrollIntoView({ block: 'start' })} />
      )}
      {otherComments.map((c) => (
        <CommentThread key={c.id} comment={c} showLocation {...actions.thread(c)} />
      ))}
      {entries.map((f) => {
        const isMessage = f.path === ''
        const fileComments = comments.filter((c) => c.path === f.path)
        const visible = new Set(f.hunks.flatMap((h) => h.lines.map((l) => lineSide(l))).map((v) => `${sides[v.side]}:${v.line}`))
        const atLine = (c: ReviewComment) => inline && c.startLine > 0 && !c.drifted && visible.has(`${c.side}:${c.endLine}`)
        const viewed = isViewed(review, changeId, f.path)
        const generated = data.generated.includes(f.path)
        const collapsed = !(opened[f.path] ?? !(viewed || generated))
        const selected = selection?.changeId === changeId && selection.path === f.path ? selection : undefined
        const annotation: LineAnnotation | undefined = inline
          ? {
              onSelect: (side, line, extend) => {
                if (!commentable) return
                onSelect(
                  extend && selected && selected.side === side
                    ? { ...selected, start: Math.min(selected.origin, line), end: Math.max(selected.origin, line) }
                    : { changeId, path: f.path, side, start: line, end: line, origin: line },
                )
              },
              selected: (side, line) => selected?.side === side && selected.start <= line && line <= selected.end,
              focused: (side, line) => focus?.changeId === changeId && focus.path === f.path && focus.side === side && focus.line === line,
              after: (side, line) => (
                <>
                  {fileComments
                    .filter((c) => atLine(c) && c.side === sides[side] && c.endLine === line)
                    .map((c) => (
                      <CommentThread key={c.id} comment={c} {...actions.thread(c)} />
                    ))}
                  {selected?.side === side && selected.end === line && (
                    <Box sx={{ m: 1, maxWidth: 960, border: 1, borderColor: 'divider', borderRadius: 1, position: 'sticky', left: 8 }}>
                      <CommentComposer
                        label={`Comment on L${selected.start}${selected.end > selected.start ? `-${selected.end}` : ''}`}
                        onSubmit={async (body) => {
                          await actions.add(changeId, f.path, sides[side], selected.start, selected.end, body)
                          onSelect(undefined)
                        }}
                        onCancel={() => onSelect(undefined)}
                      />
                    </Box>
                  )}
                </>
              ),
            }
          : undefined
        return (
          <FileDiffView
            key={f.path}
            file={f}
            title={isMessage ? 'Commit message' : undefined}
            split={split}
            preview={preview}
            repoPath={review.repository}
            revision={commit?.commitId ?? ''}
            oldRevision={commit?.parents.length === 1 ? commit.parents[0] : undefined}
            annotation={annotation}
            collapsed={collapsed}
            actions={
              <>
                {generated && <Chip label="generated" size="small" variant="outlined" title="linguist-generated in .gitattributes" />}
                {fileComments.length > 0 && collapsed && (
                  <Typography variant="caption" color="text.secondary">
                    {fileComments.length} comment(s)
                  </Typography>
                )}
                {commentable && (
                  <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => setFileComposer(f.path)}>
                    Comment
                  </Button>
                )}
                <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => setOpened((o) => ({ ...o, [f.path]: collapsed }))}>
                  {collapsed ? 'Expand' : 'Collapse'}
                </Button>
                {!isMessage && (
                  <FormControlLabel
                    label="Viewed"
                    sx={{ mr: 0, '& .MuiFormControlLabel-label': { fontSize: 13 } }}
                    control={
                      <Checkbox
                        size="small"
                        sx={{ py: 0 }}
                        checked={viewed}
                        onChange={(e) => {
                          setOpened((o) => {
                            const next = { ...o }
                            delete next[f.path]
                            return next
                          })
                          actions.setViewed(changeId, f.path, e.target.checked)
                        }}
                      />
                    }
                  />
                )}
              </>
            }
          >
            {fileComments
              .filter((c) => !atLine(c))
              .map((c) => (
                <CommentThread key={c.id} comment={c} showLocation={c.startLine > 0} {...actions.thread(c)} />
              ))}
            {fileComposer === f.path && (
              <CommentComposer
                label={isMessage ? 'Comment on the commit message' : 'Comment on the file'}
                onSubmit={async (body) => {
                  await actions.add(changeId, f.path, DiffSide.UNSPECIFIED, 0, 0, body)
                  setFileComposer(undefined)
                }}
                onCancel={() => setFileComposer(undefined)}
              />
            )}
          </FileDiffView>
        )
      })}
      {files.length === 0 && <Typography color="text.secondary">No changes</Typography>}
    </Box>
  )
}

// positionOf returns the line of the element which has data-line in the file of the change.
const positionOf = (el: HTMLElement): LinePosition | undefined => {
  const changeId = el.closest<HTMLElement>('[data-change]')?.dataset.change
  const path = el.closest<HTMLElement>('[data-file]')?.dataset.file
  const [side, line] = (el.dataset.line ?? '').split(':')
  if (!changeId || path === undefined || (side !== 'old' && side !== 'new')) return undefined
  return { changeId, path, side, line: Number(line) }
}

const samePosition = (a: LinePosition | undefined, b: LinePosition | undefined) =>
  a !== undefined && b !== undefined && a.changeId === b.changeId && a.path === b.path && a.side === b.side && a.line === b.line

// ReviewPage shows the review. The diff is shown per change and the start and the end of the diff are selected from
// the rounds.
export const ReviewPage = () => {
  const { reviewId } = useParams({ from: '/reviews/$reviewId' })
  const { data, error, refetch } = useQuery(ReviewService.method.getReview, { id: reviewId }, { refetchInterval: reviewRefreshInterval })
  const review = data?.review
  const [fromRound, setFromRound] = useState(0)
  const [selectedToRound, setToRound] = useState(0)
  const [message, setMessage] = useState<{ text: string; error: boolean }>()
  const [composing, setComposing] = useState(false)
  const [selection, setSelection] = useState<Selection>()
  const [focus, setFocus] = useState<LinePosition>()
  const [helpOpen, setHelpOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const { split, preview } = useDiffNavigation(ref)

  const run = useCallback(
    async (fn: () => Promise<unknown>) => {
      try {
        await fn()
      } catch (e) {
        setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
        throw e
      } finally {
        await refetch()
      }
    },
    [refetch],
  )
  const actions: Actions = useMemo(
    () => ({
      add: (changeId, path, side, startLine, endLine, body) => run(() => client.addComment({ reviewId, changeId, path, side, startLine, endLine, body })),
      setViewed: (changeId, path, viewed) => run(() => client.setViewed({ reviewId, changeId, path, viewed })).catch(() => {}),
      thread: (c) => ({
        onReply: (body) => run(() => client.reply({ reviewId, commentId: c.id, body })),
        onResolve: (resolved) => run(() => client.resolveComment({ reviewId, commentId: c.id, resolved })).catch(() => {}),
        ...(c.author === 'user' && {
          onEdit: (body: string) => run(() => client.editComment({ reviewId, commentId: c.id, body })),
          onDelete: () => run(() => client.deleteComment({ reviewId, commentId: c.id })).catch(() => {}),
        }),
      }),
    }),
    [reviewId, run],
  )

  const latest = review?.rounds.length ?? 0
  const toRound = selectedToRound || latest
  const inline = fromRound === 0 && toRound === latest
  const commentable = inline && review !== undefined && review.state !== ReviewState.APPROVED
  const finish = useCallback(
    () =>
      run(async () => {
        const res = await client.finish({ reviewId })
        const names = res.notifiedAgents.map((a) => `${a.kind} (${a.workspaceLabel || a.paneId})`)
        setMessage({ text: names.length > 0 ? `Notified ${names.join(', ')}` : 'No running agent created these changes', error: false })
      }).catch(() => {}),
    [reviewId, run],
  )

  // The keys move the cursor over the lines in the order of the document. The lines of the collapsed files are skipped
  // because they are not rendered.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (ignoreKey(e) || !review) {
        return
      }
      switch (e.key) {
        case 'j':
        case 'k': {
          const lines = [...(ref.current?.querySelectorAll<HTMLElement>('[data-line]') ?? [])]
          const positions = lines.map(positionOf)
          const current = positions.findIndex((p) => samePosition(p, focus))
          const step = e.key === 'j' ? 1 : -1
          let i = current < 0 ? (e.key === 'j' ? 0 : lines.length - 1) : current + step
          // The same line appears on both sides of the split view.
          while (i >= 0 && i < lines.length && (positions[i] === undefined || samePosition(positions[i], focus))) {
            i += step
          }
          if (i < 0 || i >= lines.length) break
          setFocus(positions[i])
          lines[i].scrollIntoView({ block: 'nearest' })
          break
        }
        case 'c':
          if (focus && commentable) {
            setSelection({ changeId: focus.changeId, path: focus.path, side: focus.side, start: focus.line, end: focus.line, origin: focus.line })
          }
          break
        case 'a':
          if (focus && focus.path !== '') {
            actions.setViewed(focus.changeId, focus.path, !isViewed(review, focus.changeId, focus.path))
            setFocus(undefined)
          }
          break
        case 'F':
          if (review.state === ReviewState.OPEN) {
            finish()
          }
          break
        case '?':
          setHelpOpen(true)
          break
        case 'Escape':
          if (selection) setSelection(undefined)
          else setFocus(undefined)
          break
        default:
          return
      }
      e.preventDefault()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [review, focus, selection, commentable, actions, finish])

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (!review) {
    return null
  }
  const round = review.rounds[toRound - 1]
  const reviewComments = review.comments.filter((c) => !c.changeId)
  const unresolved = review.comments.filter((c) => !c.resolved).length

  return (
    <Box ref={ref} sx={{ height: '100%', overflow: 'auto', px: 2 }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', py: 1, flexWrap: 'wrap' }}>
        <Typography variant="h5">{review.repository.split('/').pop()}</Typography>
        <Chip label={stateChip[review.state].label} color={stateChip[review.state].color} size="small" />
        <Typography variant="body2" color="text.secondary">
          {review.changeIds.length} change(s) · round {latest} · {unresolved} unresolved
        </Typography>
        <Box sx={{ flex: 1 }} />
        <TextField select size="small" label="From" value={fromRound} onChange={(e) => setFromRound(Number(e.target.value))} sx={{ minWidth: 120 }}>
          <MenuItem value={0}>Base</MenuItem>
          {review.rounds.slice(0, toRound - 1).map((r) => (
            <MenuItem key={r.number} value={r.number}>
              Round {r.number}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          select
          size="small"
          label="To"
          value={toRound}
          onChange={(e) => {
            const to = Number(e.target.value)
            setToRound(to === latest ? 0 : to)
            if (fromRound >= to) setFromRound(0)
          }}
          sx={{ minWidth: 120 }}
        >
          {review.rounds.map((r) => (
            <MenuItem key={r.number} value={r.number}>
              Round {r.number}
              {r.number === latest ? ' (latest)' : ''}
            </MenuItem>
          ))}
        </TextField>
        <Button variant="contained" disabled={review.state !== ReviewState.OPEN} onClick={finish} title="Shift+F">
          Finish
        </Button>
        <Button
          variant="outlined"
          color="success"
          disabled={review.state === ReviewState.APPROVED}
          onClick={() => run(() => client.approve({ reviewId })).catch(() => {})}
        >
          Approve
        </Button>
        <Button size="small" onClick={() => setHelpOpen(true)} title="?">
          Keys
        </Button>
      </Stack>
      {review.state === ReviewState.FINISHED && (
        <Alert severity="info" sx={{ mb: 1 }}>
          Waiting for the agent to address the comments and start the next round.
        </Alert>
      )}

      <Box sx={{ mb: 2 }}>
        {reviewComments.map((c) => (
          <CommentThread key={c.id} comment={c} {...actions.thread(c)} />
        ))}
        {review.state !== ReviewState.APPROVED &&
          (composing ? (
            <CommentComposer
              label="Comment on the review"
              onSubmit={async (body) => {
                await actions.add('', '', DiffSide.UNSPECIFIED, 0, 0, body)
                setComposing(false)
              }}
              onCancel={() => setComposing(false)}
            />
          ) : (
            <Button size="small" onClick={() => setComposing(true)}>
              Comment on the review
            </Button>
          ))}
      </Box>
      <Divider sx={{ mb: 2 }} />

      {review.changeIds.map((changeId) =>
        round?.commits.some((c) => c.changeId === changeId) ? (
          <ChangeReview
            key={`${changeId}:${fromRound}:${toRound}`}
            review={review}
            changeId={changeId}
            fromRound={fromRound}
            toRound={toRound}
            inline={inline}
            commentable={commentable}
            split={split}
            preview={preview}
            selection={selection}
            onSelect={setSelection}
            focus={focus}
            actions={actions}
          />
        ) : (
          <Alert key={changeId} severity="warning" sx={{ mb: 2 }}>
            {changeId.slice(0, 8)} doesn't exist in round {toRound}
          </Alert>
        ),
      )}

      <Dialog open={helpOpen} onClose={() => setHelpOpen(false)}>
        <DialogTitle>Keys</DialogTitle>
        <DialogContent>
          <Table size="small">
            <TableBody>
              {shortcuts.map(([key, action]) => (
                <TableRow key={key}>
                  <TableCell sx={{ fontFamily: 'monospace', whiteSpace: 'nowrap' }}>{key}</TableCell>
                  <TableCell>{action}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>

      <Snackbar
        open={message !== undefined}
        autoHideDuration={message?.error ? null : 6000}
        onClose={() => setMessage(undefined)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={message?.error ? 'error' : 'success'} onClose={() => setMessage(undefined)} sx={{ whiteSpace: 'pre-wrap' }}>
          {message?.text}
        </Alert>
      </Snackbar>
    </Box>
  )
}
