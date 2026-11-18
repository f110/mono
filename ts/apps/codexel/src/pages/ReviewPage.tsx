import { useCallback, useMemo, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Button, Chip, Divider, MenuItem, Snackbar, Stack, TextField, Typography } from '@mui/material'
import { useParams } from '@tanstack/react-router'
import { FileDiffView, type LineAnnotation } from '../components/FileDiffView.tsx'
import { CommentComposer, CommentThread } from '../components/ReviewComments.tsx'
import { DiffSide, ReviewService, ReviewState, type Review, type ReviewComment } from '../connect/codexel_pb.js'
import { useDiffNavigation } from '../hooks/useDiffNavigation.ts'
import { lineSide, type LineSide } from '../lib/diffLines.ts'
import { stateChip } from '../lib/review.ts'
import { transport } from '../providers/AppProvider.tsx'

const client = createClient(ReviewService, transport)

// reviewRefreshInterval is the interval to read the review again. The agent replies to the comments and starts the next
// round by MCP, which is not notified.
const reviewRefreshInterval = 5000

const sides: Record<LineSide, DiffSide> = { old: DiffSide.OLD, new: DiffSide.NEW }

// Selection is the lines selected to comment. origin is the line clicked first to extend the range by Shift.
type Selection = { changeId: string; path: string; side: LineSide; start: number; end: number; origin: number }

// Actions are the operations of the comments. They report the error and read the review again.
type Actions = {
  add: (changeId: string, path: string, side: DiffSide, startLine: number, endLine: number, body: string) => Promise<void>
  thread: (c: ReviewComment) => {
    onReply: (body: string) => Promise<void>
    onResolve: (resolved: boolean) => void
    onEdit?: (body: string) => Promise<void>
    onDelete?: () => void
  }
}

// ChangeReview shows the diff of the change between the rounds and the comments on it. The comments are shown at the
// lines only if the diff is the one of the latest round from the parent because the lines of the comments are the
// lines of it.
const ChangeReview = ({
  review,
  changeId,
  fromRound,
  toRound,
  inline,
  commentable,
  split,
  preview,
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
  actions: Actions
}) => {
  const { data, error } = useQuery(ReviewService.method.roundDiff, { reviewId: review.id, changeId, fromRound, toRound }, { staleTime: Infinity })
  const [selection, setSelection] = useState<Selection>()
  const [fileComposer, setFileComposer] = useState<string>()
  const comments = review.comments.filter((c) => c.changeId === changeId)
  const files = data?.files ?? []
  const otherComments = comments.filter((c) => !files.some((f) => f.path === c.path))

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (!data) {
    return null
  }
  const commit = data.commit
  return (
    <Box sx={{ mb: 4 }}>
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
      <Box component="pre" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 13, mt: 0 }}>
        {data.description}
      </Box>
      {otherComments.map((c) => (
        <CommentThread key={c.id} comment={c} showLocation {...actions.thread(c)} />
      ))}
      {files.length === 0 && <Typography color="text.secondary">No changes</Typography>}
      {files.map((f) => {
        const fileComments = comments.filter((c) => c.path === f.path)
        const visible = new Set(f.hunks.flatMap((h) => h.lines.map((l) => lineSide(l))).map((v) => `${sides[v.side]}:${v.line}`))
        const atLine = (c: ReviewComment) => inline && c.startLine > 0 && !c.drifted && visible.has(`${c.side}:${c.endLine}`)
        const annotation: LineAnnotation | undefined = inline
          ? {
              onSelect: (side, line, extend) => {
                if (!commentable) return
                setSelection((s) =>
                  extend && s && s.path === f.path && s.side === side
                    ? { ...s, start: Math.min(s.origin, line), end: Math.max(s.origin, line) }
                    : { changeId, path: f.path, side, start: line, end: line, origin: line },
                )
              },
              selected: (side, line) => selection?.path === f.path && selection.side === side && selection.start <= line && line <= selection.end,
              after: (side, line) => (
                <>
                  {fileComments
                    .filter((c) => atLine(c) && c.side === sides[side] && c.endLine === line)
                    .map((c) => (
                      <CommentThread key={c.id} comment={c} {...actions.thread(c)} />
                    ))}
                  {selection?.path === f.path && selection.side === side && selection.end === line && (
                    <Box sx={{ m: 1, maxWidth: 960, border: 1, borderColor: 'divider', borderRadius: 1, position: 'sticky', left: 8 }}>
                      <CommentComposer
                        label={`Comment on L${selection.start}${selection.end > selection.start ? `-${selection.end}` : ''}`}
                        onSubmit={async (body) => {
                          await actions.add(changeId, f.path, sides[side], selection.start, selection.end, body)
                          setSelection(undefined)
                        }}
                        onCancel={() => setSelection(undefined)}
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
            split={split}
            preview={preview}
            repoPath={review.repository}
            revision={commit?.commitId ?? ''}
            oldRevision={commit?.parents.length === 1 ? commit.parents[0] : undefined}
            annotation={annotation}
            actions={
              commentable && (
                <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => setFileComposer(f.path)}>
                  Comment
                </Button>
              )
            }
          >
            {fileComments
              .filter((c) => !atLine(c))
              .map((c) => (
                <CommentThread key={c.id} comment={c} showLocation={c.startLine > 0} {...actions.thread(c)} />
              ))}
            {fileComposer === f.path && (
              <CommentComposer
                label="Comment on the file"
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
    </Box>
  )
}

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

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (!review) {
    return null
  }
  const latest = review.rounds.length
  const toRound = selectedToRound || latest
  const round = review.rounds[toRound - 1]
  const inline = fromRound === 0 && toRound === latest
  const commentable = inline && review.state !== ReviewState.APPROVED
  const reviewComments = review.comments.filter((c) => !c.changeId)
  const unresolved = review.comments.filter((c) => !c.resolved).length

  const finish = () =>
    run(async () => {
      const res = await client.finish({ reviewId })
      const names = res.notifiedAgents.map((a) => `${a.kind} (${a.workspaceLabel || a.paneId})`)
      setMessage({ text: names.length > 0 ? `Notified ${names.join(', ')}` : 'No running agent created these changes', error: false })
    }).catch(() => {})

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
        <Button variant="contained" disabled={review.state !== ReviewState.OPEN} onClick={finish}>
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
            actions={actions}
          />
        ) : (
          <Alert key={changeId} severity="warning" sx={{ mb: 2 }}>
            {changeId.slice(0, 8)} doesn't exist in round {toRound}
          </Alert>
        ),
      )}

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
