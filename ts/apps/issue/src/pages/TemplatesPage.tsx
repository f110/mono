import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  Button,
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { type Template, Issue } from '../connect/issue_pb'
import { MarkdownEditor } from '../components/MarkdownEditor.tsx'
import { useMasters } from '../hooks/useMasters.ts'

export const TemplatesPage = () => {
  const queryClient = useQueryClient()
  const { components, componentName } = useMasters()
  const { data, error } = useQuery(Issue.method.listTemplates, {})
  const onSuccess = () => void queryClient.invalidateQueries()
  const save = useMutation(Issue.method.saveTemplate, { onSuccess })
  const remove = useMutation(Issue.method.deleteTemplate, { onSuccess })
  const [editing, setEditing] = useState<Template | undefined>()
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [componentId, setComponentId] = useState(0)

  const edit = (t?: Template) => {
    setEditing(t)
    setTitle(t?.title ?? '')
    setBody(t?.body ?? '')
    setComponentId(t?.componentId ?? 0)
  }

  return (
    <Stack spacing={2}>
      <Typography variant="h5">Template</Typography>
      {(error || save.error || remove.error) && <Alert severity="error">{(error ?? save.error ?? remove.error)?.message}</Alert>}
      <Stack
        component="form"
        spacing={2}
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate({ id: editing?.id, title, body, componentId }, { onSuccess: () => edit() })
        }}
      >
        <Typography variant="subtitle1">{editing ? `${editing.title} を編集` : 'テンプレートを追加'}</Typography>
        <Stack direction="row" spacing={1}>
          <TextField size="small" label="タイトル" value={title} onChange={(e) => setTitle(e.target.value)} sx={{ width: 320 }} />
          <TextField select size="small" label="Component" value={componentId} onChange={(e) => setComponentId(Number(e.target.value))} sx={{ width: 240 }}>
            <MenuItem value={0}>未分類</MenuItem>
            {components.map((c) => (
              <MenuItem key={c.id} value={c.id}>
                {c.name}
              </MenuItem>
            ))}
          </TextField>
        </Stack>
        <MarkdownEditor value={body} onChange={setBody} />
        <Stack direction="row" spacing={1}>
          <Button type="submit" variant="contained" disabled={save.isPending || title.trim() === ''}>
            {editing ? '保存' : '追加'}
          </Button>
          {editing && <Button onClick={() => edit()}>キャンセル</Button>}
        </Stack>
      </Stack>
      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>タイトル</TableCell>
              <TableCell>Component</TableCell>
              <TableCell sx={{ width: 200 }} />
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.templates.map((t) => (
              <TableRow key={t.id} hover selected={editing?.id === t.id}>
                <TableCell>{t.title}</TableCell>
                <TableCell>{t.componentId ? componentName(t.componentId) : ''}</TableCell>
                <TableCell align="right">
                  <Button size="small" onClick={() => edit(t)}>
                    編集
                  </Button>
                  <Button
                    size="small"
                    color="error"
                    disabled={remove.isPending}
                    onClick={() => {
                      if (window.confirm(`${t.title} を削除しますか？`)) {
                        remove.mutate({ id: t.id }, { onSuccess: () => editing?.id === t.id && edit() })
                      }
                    }}
                  >
                    削除
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      {data && data.templates.length === 0 && <Typography color="text.secondary">Template はありません</Typography>}
    </Stack>
  )
}
