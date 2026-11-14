import { type ReactNode, useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { Link as RouterLink, useParams } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  IconButton,
  Link,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import EditIcon from '@mui/icons-material/Edit'
import {
  type AgentSession,
  type Backlink,
  type Activity,
  type Comment,
  type CommitLink,
  type PullRequestLink,
  type RelatedTicket,
  ActivityKind,
  Issue,
  RelationType,
  TicketStatus,
  TicketType,
} from '../connect/issue_pb'
import { Markdown } from '../components/Markdown.tsx'
import { CopyTicketNumberButton, CopyTicketURLButton } from '../components/CopyButton.tsx'
import { EditHistory } from '../components/EditHistory.tsx'
import { PostCard } from '../components/PostCard.tsx'
import { RelativeTime } from '../components/RelativeTime.tsx'
import { MarkdownEditor } from '../components/MarkdownEditor.tsx'
import { useMasters } from '../hooks/useMasters.ts'
import {
  activityLabels,
  codingAgentLabels,
  priorities,
  priorityLabels,
  relationLabels,
  relationTypes,
  statuses,
  statusColors,
  statusLabels,
  typeLabels,
  types,
} from '../labels.ts'

const statusNames: Record<string, string> = {
  open: statusLabels[TicketStatus.OPEN],
  in_progress: statusLabels[TicketStatus.IN_PROGRESS],
  done: statusLabels[TicketStatus.DONE],
  wont_fix: statusLabels[TicketStatus.WONT_FIX],
  need_investigation: statusLabels[TicketStatus.NEED_INVESTIGATION],
}

const typeNames: Record<string, string> = {
  bug: typeLabels[TicketType.BUG],
  feature: typeLabels[TicketType.FEATURE],
  task: typeLabels[TicketType.TASK],
  operation: typeLabels[TicketType.OPERATION],
}

const relationNames: Record<string, string> = {
  parent_of: relationLabels[RelationType.PARENT_OF][0],
  child_of: relationLabels[RelationType.PARENT_OF][1],
  blocks: relationLabels[RelationType.BLOCKS][0],
  blocked_by: relationLabels[RelationType.BLOCKS][1],
  duplicate_of: relationLabels[RelationType.DUPLICATE_OF][0],
  duplicated_by: relationLabels[RelationType.DUPLICATE_OF][1],
  relates_to: relationLabels[RelationType.RELATES_TO][0],
}

const TicketLink = ({ number }: { number: number }) => (
  <Link component={RouterLink} to="/$number" params={{ number: String(number) }}>
    #{number}
  </Link>
)

function useInvalidate(): () => void {
  const queryClient = useQueryClient()
  return () => void queryClient.invalidateQueries()
}

export const TicketPage = () => {
  const { number: param } = useParams({ from: '/$number' })
  const number = Number(param)
  const { actorName } = useMasters()
  const invalidate = useInvalidate()
  const { data, error } = useQuery(Issue.method.getTicket, { number })
  const { data: activities } = useQuery(Issue.method.listActivities, { ticket: number })
  const update = useMutation(Issue.method.updateTicket, { onSuccess: invalidate })
  const [editingTitle, setEditingTitle] = useState<string | null>(null)
  const [editingBody, setEditingBody] = useState<string | null>(null)

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  const ticket = data?.ticket
  if (!ticket) {
    return null
  }

  return (
    <Stack spacing={2}>
      {update.error && <Alert severity="error">{update.error.message}</Alert>}
      {editingTitle === null ? (
        <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
          <Chip label={statusLabels[ticket.status]} sx={statusColors[ticket.status]} />
          <Typography variant="h5">
            {ticket.title} <Typography component="span" variant="h5" color="text.secondary">#{ticket.number}</Typography>
          </Typography>
          <CopyTicketNumberButton number={ticket.number} />
          <CopyTicketURLButton number={ticket.number} />
          <Tooltip title="タイトルを編集">
            <IconButton size="small" aria-label="タイトルを編集" onClick={() => setEditingTitle(ticket.title)}>
              <EditIcon sx={{ fontSize: '0.875rem' }} />
            </IconButton>
          </Tooltip>
        </Stack>
      ) : (
        <Stack
          component="form"
          direction="row"
          spacing={1}
          sx={{ alignItems: 'center', '& .MuiButton-root': { flexShrink: 0, whiteSpace: 'nowrap' } }}
          onSubmit={(e) => {
            e.preventDefault()
            update.mutate({ number, title: editingTitle })
            setEditingTitle(null)
          }}
        >
          <TextField value={editingTitle} onChange={(e) => setEditingTitle(e.target.value)} fullWidth size="small" autoFocus />
          <Button type="submit" variant="contained" disabled={editingTitle === ''}>
            保存
          </Button>
          <Button onClick={() => setEditingTitle(null)}>キャンセル</Button>
        </Stack>
      )}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'minmax(0, 1fr) 280px' },
          gridTemplateRows: { md: 'auto 1fr' },
          gridTemplateAreas: { xs: '"fields" "main" "side"', md: '"main fields" "main side"' },
          columnGap: 3,
          rowGap: 2,
        }}
      >
        <Stack spacing={2} sx={{ gridArea: 'main' }}>
          <PostCard
            author={actorName(ticket.reporterId)}
            time={<RelativeTime time={ticket.createdAt} />}
            history={
              (activities?.activities ?? []).some((a) => a.kind === ActivityKind.BODY) && (
                <EditHistory target={{ ticket: number }} current={ticket.body} authorId={ticket.reporterId} createdAt={ticket.createdAt} />
              )
            }
            reporter
            actions={editingBody === null ? [{ label: '編集', onClick: () => setEditingBody(ticket.body) }] : []}
          >
            {editingBody === null ? (
              <Markdown text={ticket.body} onChange={(body) => update.mutate({ number, body })} />
            ) : (
              <Stack
                component="form"
                spacing={1}
                onSubmit={(e) => {
                  e.preventDefault()
                  update.mutate({ number, body: editingBody })
                  setEditingBody(null)
                }}
              >
                <MarkdownEditor value={editingBody} onChange={setEditingBody} />
                <Stack direction="row" spacing={1}>
                  <Button type="submit" variant="contained">
                    保存
                  </Button>
                  <Button onClick={() => setEditingBody(null)}>キャンセル</Button>
                </Stack>
              </Stack>
            )}
          </PostCard>
          <Timeline activities={activities?.activities ?? []} comments={data.comments} reporterId={ticket.reporterId} />
          <CommentForm number={number} />
        </Stack>
        <Box sx={{ gridArea: 'fields' }}>
          <Fields
            status={ticket.status}
            type={ticket.type}
            priority={ticket.priority}
            componentId={ticket.componentId}
            assigneeId={ticket.assigneeId}
            onChange={(fields) => update.mutate({ number, ...fields })}
          />
        </Box>
        <Stack spacing={2} sx={{ gridArea: 'side' }}>
          <Divider />
          <Relations number={number} relations={data.relations} />
          <Divider />
          <Backlinks backlinks={data.backlinks} />
          <Divider />
          <Commits commits={data.commits} />
          <Divider />
          <PullRequests pullRequests={data.pullRequests} />
          <Divider />
          <AgentSessions sessions={data.agentSessions} />
        </Stack>
      </Box>
    </Stack>
  )
}

const Fields = ({
  status,
  type,
  priority,
  componentId,
  assigneeId,
  onChange,
}: {
  status: number
  type: number
  priority: number
  componentId: number
  assigneeId: number
  onChange: (fields: { status?: number; type?: number; priority?: number; componentId?: number; assigneeId?: number }) => void
}) => {
  const { operators, components } = useMasters()

  return (
    <Stack spacing={2}>
      <TextField select size="small" label="Status" value={status} onChange={(e) => onChange({ status: Number(e.target.value) })}>
        {statuses.map((s) => (
          <MenuItem key={s} value={s}>
            {statusLabels[s]}
          </MenuItem>
        ))}
      </TextField>
      <TextField select size="small" label="Type" value={type} onChange={(e) => onChange({ type: Number(e.target.value) })}>
        {types.map((t) => (
          <MenuItem key={t} value={t}>
            {typeLabels[t]}
          </MenuItem>
        ))}
      </TextField>
      <TextField select size="small" label="Priority" value={priority} onChange={(e) => onChange({ priority: Number(e.target.value) })}>
        {priorities.map((p) => (
          <MenuItem key={p} value={p}>
            {priorityLabels[p]}
          </MenuItem>
        ))}
      </TextField>
      <TextField select size="small" label="Component" value={componentId} onChange={(e) => onChange({ componentId: Number(e.target.value) })}>
        <MenuItem value={0}>未分類</MenuItem>
        {components.map((c) => (
          <MenuItem key={c.id} value={c.id}>
            {c.name}
          </MenuItem>
        ))}
      </TextField>
      <TextField select size="small" label="Assignee" value={assigneeId} onChange={(e) => onChange({ assigneeId: Number(e.target.value) })}>
        <MenuItem value={0}>未割り当て</MenuItem>
        {operators.map((a) => (
          <MenuItem key={a.id} value={a.id}>
            {a.name}
          </MenuItem>
        ))}
      </TextField>
    </Stack>
  )
}

const Relations = ({ number, relations }: { number: number; relations: RelatedTicket[] }) => {
  const invalidate = useInvalidate()
  const add = useMutation(Issue.method.addRelation, { onSuccess: invalidate })
  const remove = useMutation(Issue.method.removeRelation, { onSuccess: invalidate })
  const [type, setType] = useState(RelationType.RELATES_TO)
  const [to, setTo] = useState('')

  return (
    <Stack spacing={1}>
      <Typography variant="subtitle2">Relations</Typography>
      {(add.error || remove.error) && <Alert severity="error">{(add.error ?? remove.error)?.message}</Alert>}
      {relations.map(({ relation: r, title }) => {
        if (!r) {
          return null
        }
        const forward = r.from === number
        const other = forward ? r.to : r.from
        return (
          <Stack key={`${r.from}-${r.type}-${r.to}`} direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <Typography variant="body2" sx={{ flex: 1 }}>
              {relationLabels[r.type][forward ? 0 : 1]}{' '}
              <Tooltip title={title}>
                <span>
                  <TicketLink number={other} />
                </span>
              </Tooltip>
            </Typography>
            <IconButton
              size="small"
              aria-label="削除"
              onClick={() => {
                if (window.confirm(`#${other} との関連を削除しますか？`)) {
                  remove.mutate({ relation: { from: r.from, type: r.type, to: r.to } })
                }
              }}
            >
              ×
            </IconButton>
          </Stack>
        )
      })}
      <Stack
        component="form"
        direction="row"
        spacing={1}
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate({ relation: { from: number, type, to: Number(to) } })
          setTo('')
        }}
      >
        <TextField select size="small" value={type} onChange={(e) => setType(Number(e.target.value))} sx={{ flex: 1 }}>
          {relationTypes.map((t) => (
            <MenuItem key={t} value={t}>
              {relationLabels[t][0]}
            </MenuItem>
          ))}
        </TextField>
        <TextField size="small" placeholder="番号" value={to} onChange={(e) => setTo(e.target.value)} sx={{ width: 72 }} />
        <Button type="submit" size="small" disabled={!/^\d+$/.test(to)}>
          追加
        </Button>
      </Stack>
    </Stack>
  )
}

const Backlinks = ({ backlinks }: { backlinks: Backlink[] }) => (
  <Stack spacing={1}>
    <Typography variant="subtitle2">Backlinks</Typography>
    {backlinks.length === 0 && (
      <Typography variant="body2" color="text.secondary">
        なし
      </Typography>
    )}
    {backlinks.map((b) => (
      <Typography key={b.ticket} variant="body2" noWrap>
        <TicketLink number={b.ticket} /> {b.title}
      </Typography>
    ))}
  </Stack>
)

const ExternalLink = ({ url, children }: { url: string; children: ReactNode }) =>
  url === '' ? (
    children
  ) : (
    <Link href={url} target="_blank" rel="noreferrer">
      {children}
    </Link>
  )

const Commits = ({ commits }: { commits: CommitLink[] }) => (
  <Stack spacing={1}>
    <Typography variant="subtitle2">Commits</Typography>
    {commits.length === 0 && (
      <Typography variant="body2" color="text.secondary">
        なし
      </Typography>
    )}
    {commits.map((c) => (
      <Typography key={c.sha} variant="body2" noWrap>
        <ExternalLink url={c.url}>
          <Box component="span" sx={{ fontFamily: 'monospace' }}>
            {c.sha.slice(0, 7)}
          </Box>
        </ExternalLink>{' '}
        {c.subject}
      </Typography>
    ))}
  </Stack>
)

const PullRequests = ({ pullRequests }: { pullRequests: PullRequestLink[] }) => (
  <Stack spacing={1}>
    <Typography variant="subtitle2">Pull requests</Typography>
    {pullRequests.length === 0 && (
      <Typography variant="body2" color="text.secondary">
        なし
      </Typography>
    )}
    {pullRequests.map((p) => (
      <Typography key={p.url === '' ? p.number : p.url} variant="body2" noWrap>
        <ExternalLink url={p.url}>#{p.number}</ExternalLink> {p.title}
      </Typography>
    ))}
  </Stack>
)

const AgentSessions = ({ sessions }: { sessions: AgentSession[] }) => (
  <Stack spacing={1}>
    <Typography variant="subtitle2">Agent sessions</Typography>
    {sessions.length === 0 && (
      <Typography variant="body2" color="text.secondary">
        なし
      </Typography>
    )}
    {sessions.map((s) => (
      <Box key={`${s.hostname}/${s.sessionId}`}>
        <Typography variant="body2" noWrap>
          {codingAgentLabels[s.agent]} on {s.hostname}
        </Typography>
        <Typography variant="body2" color="text.secondary" noWrap sx={{ fontFamily: 'monospace' }}>
          {s.sessionId}
        </Typography>
      </Box>
    ))}
  </Stack>
)

const Timeline = ({ activities, comments, reporterId }: { activities: Activity[]; comments: Comment[]; reporterId: number }) => {
  const { actorName, componentName } = useMasters()

  const value = (a: Activity, v: string) => {
    if (v === '') {
      return 'なし'
    }
    switch (a.kind) {
      case ActivityKind.STATUS:
        return statusNames[v] ?? v
      case ActivityKind.TYPE:
        return typeNames[v] ?? v
      case ActivityKind.COMPONENT:
        return componentName(Number(v))
      case ActivityKind.ASSIGNEE:
        return actorName(Number(v))
    }
    return v
  }

  return (
    <Stack spacing={1}>
      {activities.map((a) => {
        if (a.kind === ActivityKind.COMMENT) {
          const comment = comments.find((c) => c.id === a.refId)
          return comment ? <CommentCard key={a.id} comment={comment} reporter={comment.actorId === reporterId} /> : null
        }
        let detail: ReactNode = null
        switch (a.kind) {
          case ActivityKind.TITLE:
          case ActivityKind.STATUS:
          case ActivityKind.TYPE:
          case ActivityKind.PRIORITY:
          case ActivityKind.COMPONENT:
          case ActivityKind.ASSIGNEE:
            detail = `${value(a, a.fromValue)} → ${value(a, a.toValue)}`
            break
          case ActivityKind.RELATION_ADDED:
          case ActivityKind.RELATION_REMOVED: {
            const [name, other] = a.toValue.split(':')
            detail = (
              <>
                {relationNames[name] ?? name} <TicketLink number={Number(other)} />
              </>
            )
            break
          }
          case ActivityKind.MENTIONED:
          case ActivityKind.MENTION_REMOVED:
            detail = <TicketLink number={Number(a.toValue)} />
            break
        }
        return (
          <Typography key={a.id} variant="body2" color="text.secondary">
            {actorName(a.actorId)} {activityLabels[a.kind]} {detail} <RelativeTime time={a.createdAt} />
          </Typography>
        )
      })}
    </Stack>
  )
}

const CommentCard = ({ comment, reporter }: { comment: Comment; reporter: boolean }) => {
  const { actorName } = useMasters()
  const invalidate = useInvalidate()
  const edit = useMutation(Issue.method.editComment, { onSuccess: invalidate })
  const remove = useMutation(Issue.method.deleteComment, { onSuccess: invalidate })
  const [editing, setEditing] = useState<string | null>(null)

  return (
    <PostCard
      author={actorName(comment.actorId)}
      time={<RelativeTime time={comment.createdAt} />}
      history={
        comment.updatedAt !== undefined && (
          <EditHistory target={{ comment: comment.id }} current={comment.body} authorId={comment.actorId} createdAt={comment.createdAt} />
        )
      }
      reporter={reporter}
      actions={
        editing === null
          ? [
              { label: '編集', onClick: () => setEditing(comment.body) },
              {
                label: '削除',
                danger: true,
                onClick: () => {
                  if (window.confirm('コメントを削除しますか？')) {
                    remove.mutate({ id: comment.id })
                  }
                },
              },
            ]
          : []
      }
    >
      {(edit.error || remove.error) && <Alert severity="error">{(edit.error ?? remove.error)?.message}</Alert>}
      {editing === null ? (
        <Markdown text={comment.body} onChange={(body) => edit.mutate({ id: comment.id, body })} />
      ) : (
        <Stack
          component="form"
          spacing={1}
          onSubmit={(e) => {
            e.preventDefault()
            edit.mutate({ id: comment.id, body: editing })
            setEditing(null)
          }}
        >
          <MarkdownEditor value={editing} onChange={setEditing} minRows={3} />
          <Stack direction="row" spacing={1}>
            <Button type="submit" variant="contained">
              保存
            </Button>
            <Button onClick={() => setEditing(null)}>キャンセル</Button>
          </Stack>
        </Stack>
      )}
    </PostCard>
  )
}


const CommentForm = ({ number }: { number: number }) => {
  const invalidate = useInvalidate()
  const [body, setBody] = useState('')
  const add = useMutation(Issue.method.addComment, {
    onSuccess: () => {
      setBody('')
      invalidate()
    },
  })

  return (
    <Box
      component="form"
      onSubmit={(e) => {
        e.preventDefault()
        add.mutate({ ticket: number, body })
      }}
    >
      {add.error && <Alert severity="error">{add.error.message}</Alert>}
      <MarkdownEditor value={body} onChange={setBody} minRows={3} />
      <Stack direction="row" sx={{ mt: 1 }}>
        <Button type="submit" variant="contained" disabled={add.isPending || body === ''}>
          コメント
        </Button>
      </Stack>
    </Box>
  )
}
