import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  Button,
  IconButton,
  Paper,
  SvgIcon,
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
import { type Component, Issue } from '../connect/issue_pb'

export const ComponentsPage = () => {
  const queryClient = useQueryClient()
  const { data, error } = useQuery(Issue.method.listComponents, {})
  const save = useMutation(Issue.method.saveComponent, { onSuccess: () => void queryClient.invalidateQueries() })
  const [name, setName] = useState('')

  return (
    <Stack spacing={2}>
      <Typography variant="h5">Component</Typography>
      {(error || save.error) && <Alert severity="error">{(error ?? save.error)?.message}</Alert>}
      <Stack
        component="form"
        direction="row"
        spacing={1}
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate({ name }, { onSuccess: () => setName('') })
        }}
      >
        <TextField size="small" label="名前" value={name} onChange={(e) => setName(e.target.value)} sx={{ width: 320 }} />
        <Button type="submit" variant="contained" disabled={save.isPending || name.trim() === ''}>
          追加
        </Button>
      </Stack>
      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>名前</TableCell>
              <TableCell sx={{ width: 200 }} />
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.components.map((c) => (
              <ComponentRow key={c.id} component={c} disabled={save.isPending} onSave={(name) => save.mutateAsync({ id: c.id, name })} />
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      {data && data.components.length === 0 && <Typography color="text.secondary">Component はありません</Typography>}
    </Stack>
  )
}

const ComponentRow = ({
  component,
  disabled,
  onSave,
}: {
  component: Component
  disabled: boolean
  onSave: (name: string) => Promise<unknown>
}) => {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(component.name)

  if (!editing) {
    return (
      <TableRow hover>
        <TableCell>{component.name}</TableCell>
        <TableCell align="right">
          <IconButton
            size="small"
            aria-label="名前を変更"
            onClick={() => {
              setName(component.name)
              setEditing(true)
            }}
          >
            <SvgIcon fontSize="small">
              <path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34a.996.996 0 0 0-1.41 0l-1.83 1.83 3.75 3.75z" />
            </SvgIcon>
          </IconButton>
        </TableCell>
      </TableRow>
    )
  }

  return (
    <TableRow>
      <TableCell>
        <TextField size="small" value={name} onChange={(e) => setName(e.target.value)} autoFocus fullWidth />
      </TableCell>
      <TableCell align="right">
        <Button
          size="small"
          variant="contained"
          disabled={disabled || name.trim() === '' || name === component.name}
          onClick={() => void onSave(name).then(() => setEditing(false))}
        >
          保存
        </Button>
        <Button size="small" onClick={() => setEditing(false)}>
          キャンセル
        </Button>
      </TableCell>
    </TableRow>
  )
}
