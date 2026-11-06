import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Link as RouterLink, useNavigate, useSearch } from '@tanstack/react-router'
import { Alert, Button, Divider, IconButton, Link, MenuItem, Stack, TextField, Typography } from '@mui/material'
import { type Template, Issue, Priority, RelationType, TicketType } from '../connect/issue_pb'
import { MarkdownEditor } from '../components/MarkdownEditor.tsx'
import { useMasters } from '../hooks/useMasters.ts'
import { priorities, priorityLabels, relationLabels, relationTypes, typeLabels, types } from '../labels.ts'

type NewRelation = {
  type: RelationType
  to: number
}

export const NewTicketPage = () => {
  const { template: templateId } = useSearch({ from: '/new' })
  const { data, error } = useQuery(Issue.method.listTemplates, {}, { enabled: templateId !== undefined })

  if (templateId === undefined) {
    return <NewTicketForm />
  }
  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (!data) {
    return null
  }
  const template = data.templates.find((t) => t.id === templateId)
  if (!template) {
    return <Alert severity="error">Template が見つかりません</Alert>
  }
  return <NewTicketForm key={template.id} template={template} />
}

const NewTicketForm = ({ template }: { template?: Template }) => {
  const navigate = useNavigate()
  const { operators, components } = useMasters()
  const [title, setTitle] = useState(template?.title ?? '')
  const [body, setBody] = useState(template?.body ?? '')
  const [type, setType] = useState(TicketType.UNSPECIFIED)
  const [priority, setPriority] = useState(Priority.UNSPECIFIED)
  const [componentId, setComponentId] = useState(template?.componentId ?? 0)
  const [assigneeId, setAssigneeId] = useState(0)
  const [relations, setRelations] = useState<NewRelation[]>([])
  const [relationType, setRelationType] = useState(RelationType.RELATES_TO)
  const [relationTo, setRelationTo] = useState('')
  const create = useMutation(Issue.method.createTicket)
  const relate = useMutation(Issue.method.addRelation)

  const submit = async () => {
    const res = await create.mutateAsync({
      title,
      body,
      type,
      priority,
      ...(componentId ? { componentId } : {}),
      ...(assigneeId ? { assigneeId } : {}),
    })
    const number = res.ticket?.number ?? 0
    for (const r of relations) {
      await relate.mutateAsync({ relation: { from: number, type: r.type, to: r.to } })
    }
    void navigate({ to: '/$number', params: { number: String(number) } })
  }

  return (
    <Stack spacing={2}>
      <Typography variant="h5">新規作成</Typography>
      {(create.error || relate.error) && <Alert severity="error">{(create.error ?? relate.error)?.message}</Alert>}
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
        <Stack
          component="form"
          spacing={2}
          sx={{ flex: 1, minWidth: 0 }}
          onSubmit={(e) => {
            e.preventDefault()
            void submit()
          }}
        >
          <TextField label="タイトル" value={title} onChange={(e) => setTitle(e.target.value)} required fullWidth />
          <MarkdownEditor value={body} onChange={setBody} />
          <Stack direction="row">
            <Button type="submit" variant="contained" disabled={create.isPending || relate.isPending || title === ''}>
              作成
            </Button>
          </Stack>
        </Stack>
        <Stack spacing={2} sx={{ width: { md: 280 } }}>
          <TextField select size="small" label="Type" value={type} onChange={(e) => setType(Number(e.target.value))}>
            {types.map((t) => (
              <MenuItem key={t} value={t}>
                {typeLabels[t]}
              </MenuItem>
            ))}
          </TextField>
          <TextField select size="small" label="Priority" value={priority} onChange={(e) => setPriority(Number(e.target.value))}>
            {priorities.map((p) => (
              <MenuItem key={p} value={p}>
                {priorityLabels[p]}
              </MenuItem>
            ))}
          </TextField>
          <TextField select size="small" label="Component" value={componentId} onChange={(e) => setComponentId(Number(e.target.value))}>
            <MenuItem value={0}>未分類</MenuItem>
            {components.map((c) => (
              <MenuItem key={c.id} value={c.id}>
                {c.name}
              </MenuItem>
            ))}
          </TextField>
          <TextField select size="small" label="Assignee" value={assigneeId} onChange={(e) => setAssigneeId(Number(e.target.value))}>
            <MenuItem value={0}>未割り当て</MenuItem>
            {operators.map((a) => (
              <MenuItem key={a.id} value={a.id}>
                {a.name}
              </MenuItem>
            ))}
          </TextField>
          <Divider />
          <Stack spacing={1}>
            <Typography variant="subtitle2">Relations</Typography>
            {relations.map((r, i) => (
              <Stack key={`${r.type}-${r.to}`} direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {relationLabels[r.type][0]}{' '}
                  <Link component={RouterLink} to="/$number" params={{ number: String(r.to) }}>
                    #{r.to}
                  </Link>
                </Typography>
                <IconButton size="small" aria-label="削除" onClick={() => setRelations(relations.filter((_, j) => j !== i))}>
                  ×
                </IconButton>
              </Stack>
            ))}
            <Stack direction="row" spacing={1}>
              <TextField select size="small" value={relationType} onChange={(e) => setRelationType(Number(e.target.value))} sx={{ flex: 1 }}>
                {relationTypes.map((t) => (
                  <MenuItem key={t} value={t}>
                    {relationLabels[t][0]}
                  </MenuItem>
                ))}
              </TextField>
              <TextField size="small" placeholder="番号" value={relationTo} onChange={(e) => setRelationTo(e.target.value)} sx={{ width: 72 }} />
              <Button
                size="small"
                disabled={!/^\d+$/.test(relationTo)}
                onClick={() => {
                  setRelations([...relations, { type: relationType, to: Number(relationTo) }])
                  setRelationTo('')
                }}
              >
                追加
              </Button>
            </Stack>
          </Stack>
        </Stack>
      </Stack>
    </Stack>
  )
}
