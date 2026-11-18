import { useState } from 'react'
import { Box, Button, Chip, Stack, TextField, Typography } from '@mui/material'
import { DiffSide, type ReviewComment } from '../connect/codexel_pb.js'
import { Markdown } from './Markdown.tsx'

const CommentBody = ({ text }: { text: string }) => (
  <Box sx={{ '& p': { my: 0.5 }, '& pre': { overflowX: 'auto' }, fontSize: 14, wordBreak: 'break-word' }}>
    <Markdown text={text} />
  </Box>
)

// CommentComposer is the input of the comment. Cmd/Ctrl+Enter submits and Escape cancels. onSubmit reports its error,
// and the body is kept to retry if it fails.
export const CommentComposer = ({
  initial = '',
  label,
  onSubmit,
  onCancel,
}: {
  initial?: string
  label: string
  onSubmit: (body: string) => Promise<void>
  onCancel?: () => void
}) => {
  const [body, setBody] = useState(initial)
  const [sending, setSending] = useState(false)
  const submit = async () => {
    if (!body.trim()) return
    setSending(true)
    await onSubmit(body).then(
      () => setBody(''),
      () => {},
    )
    setSending(false)
  }

  return (
    <Box sx={{ p: 1, fontFamily: 'initial' }}>
      <TextField
        autoFocus
        fullWidth
        multiline
        minRows={2}
        size="small"
        placeholder="Markdown"
        value={body}
        onChange={(e) => setBody(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            submit()
          } else if (e.key === 'Escape' && onCancel) {
            e.preventDefault()
            onCancel()
          }
        }}
      />
      <Stack direction="row" spacing={1} sx={{ mt: 1, justifyContent: 'flex-end' }}>
        {onCancel && (
          <Button size="small" onClick={onCancel}>
            Cancel
          </Button>
        )}
        <Button size="small" variant="contained" disabled={sending || !body.trim()} onClick={submit}>
          {label}
        </Button>
      </Stack>
    </Box>
  )
}

const lineLabel = (c: ReviewComment) =>
  c.startLine === 0 ? '' : `${c.side === DiffSide.OLD ? 'old ' : ''}L${c.startLine}${c.endLine > c.startLine ? `-${c.endLine}` : ''}`

// CommentThread shows the comment and the replies. The operations are hidden if the handlers are undefined.
export const CommentThread = ({
  comment,
  showLocation,
  onReply,
  onResolve,
  onEdit,
  onDelete,
}: {
  comment: ReviewComment
  // showLocation shows the path and the lines. It is used if the comment is not shown at the lines.
  showLocation?: boolean
  onReply?: (body: string) => Promise<void>
  onResolve?: (resolved: boolean) => void
  onEdit?: (body: string) => Promise<void>
  onDelete?: () => void
}) => {
  const [replying, setReplying] = useState(false)
  const [editing, setEditing] = useState(false)
  const [open, setOpen] = useState(!comment.resolved)

  return (
    <Box
      data-comment={comment.id}
      sx={{
        m: 1,
        border: 1,
        borderColor: comment.resolved ? 'divider' : 'primary.main',
        borderRadius: 1,
        bgcolor: 'background.paper',
        fontFamily: 'initial',
        whiteSpace: 'normal',
        maxWidth: 960,
        position: 'sticky',
        left: 8,
      }}
    >
      <Stack direction="row" spacing={1} sx={{ px: 1, py: 0.5, alignItems: 'center', borderBottom: open ? 1 : 0, borderColor: 'divider' }}>
        <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
          {comment.author}
        </Typography>
        {showLocation && comment.changeId && (
          <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
            {comment.path || 'commit message'} {lineLabel(comment)}
          </Typography>
        )}
        <Typography variant="caption" color="text.secondary">
          round {comment.round} · {new Date(Number(comment.createdAt) * 1000).toLocaleString()}
        </Typography>
        {comment.drifted && <Chip label="drifted" size="small" color="warning" variant="outlined" title="The commented lines are not found in the latest round" />}
        {comment.resolved && <Chip label="resolved" size="small" color="success" variant="outlined" />}
        {comment.replies.length > 0 && !open && (
          <Typography variant="caption" color="text.secondary">
            {comment.replies.length} repl{comment.replies.length === 1 ? 'y' : 'ies'}
          </Typography>
        )}
        <Box sx={{ flex: 1 }} />
        <Button size="small" sx={{ minWidth: 0, py: 0 }} onClick={() => setOpen((o) => !o)}>
          {open ? 'Hide' : 'Show'}
        </Button>
      </Stack>
      {open && (
        <>
          {editing && onEdit ? (
            <CommentComposer
              initial={comment.body}
              label="Save"
              onSubmit={async (body) => {
                await onEdit(body)
                setEditing(false)
              }}
              onCancel={() => setEditing(false)}
            />
          ) : (
            <Box sx={{ px: 1.5, py: 0.5 }}>
              {comment.drifted && comment.anchor && (
                <Box component="pre" sx={{ m: 0, p: 0.5, fontSize: 12, bgcolor: 'action.hover', overflowX: 'auto' }}>
                  {comment.anchor}
                </Box>
              )}
              <CommentBody text={comment.body} />
            </Box>
          )}
          {comment.replies.map((r) => (
            <Box key={r.id} sx={{ px: 1.5, py: 0.5, borderTop: 1, borderColor: 'divider' }}>
              <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
                {r.author}{' '}
                <Typography component="span" variant="caption" color="text.secondary" sx={{ fontWeight: 'normal' }}>
                  {new Date(Number(r.createdAt) * 1000).toLocaleString()}
                </Typography>
              </Typography>
              <CommentBody text={r.body} />
            </Box>
          ))}
          {replying && onReply ? (
            <CommentComposer
              label="Reply"
              onSubmit={async (body) => {
                await onReply(body)
                setReplying(false)
              }}
              onCancel={() => setReplying(false)}
            />
          ) : (
            <Stack direction="row" spacing={1} sx={{ px: 1, pb: 0.5 }}>
              {onReply && (
                <Button size="small" onClick={() => setReplying(true)}>
                  Reply
                </Button>
              )}
              {onResolve && (
                <Button size="small" onClick={() => onResolve(!comment.resolved)}>
                  {comment.resolved ? 'Unresolve' : 'Resolve'}
                </Button>
              )}
              {onEdit && !editing && (
                <Button size="small" onClick={() => setEditing(true)}>
                  Edit
                </Button>
              )}
              {onDelete && (
                <Button size="small" color="error" onClick={onDelete}>
                  Delete
                </Button>
              )}
            </Stack>
          )}
        </>
      )}
    </Box>
  )
}
