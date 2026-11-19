import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, Stack, TextField } from '@mui/material'
import { AgentService, type Agent } from '../connect/codexel_pb.js'

export const StartAgentDialog = ({
  open,
  onClose,
  onStarted,
}: {
  open: boolean
  onClose: () => void
  onStarted: (agent: Agent) => void
}) => {
  const { data } = useQuery(AgentService.method.listWorkspaces, {}, { enabled: open })
  const start = useMutation(AgentService.method.start)
  const [workspaceId, setWorkspaceId] = useState('')
  const [kind, setKind] = useState('claude')
  const [cwd, setCwd] = useState('')

  const submit = () => {
    start.mutate(
      { workspaceId, kind, cwd },
      {
        onSuccess: (res) => {
          if (res.agent) {
            onStarted(res.agent)
          }
          onClose()
        },
      },
    )
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Start agent</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          {start.error && <Alert severity="error">{start.error.message}</Alert>}
          <TextField
            select
            label="Workspace"
            value={workspaceId}
            onChange={(e) => setWorkspaceId(e.target.value)}
            slotProps={{ select: { displayEmpty: true }, inputLabel: { shrink: true } }}
            helperText="herdr is started if it is not running when the new workspace is created"
          >
            <MenuItem value="">New workspace</MenuItem>
            {data?.workspaces.map((w) => (
              <MenuItem key={w.workspaceId} value={w.workspaceId}>
                {w.label}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label="Kind"
            value={kind}
            onChange={(e) => setKind(e.target.value)}
            required
            helperText="The kind of the agent supported by herdr (e.g. claude, codex)"
          />
          <TextField label="Working directory" value={cwd} onChange={(e) => setCwd(e.target.value)} helperText="The default of herdr if empty" />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={submit} disabled={!kind || start.isPending}>
          Start
        </Button>
      </DialogActions>
    </Dialog>
  )
}
