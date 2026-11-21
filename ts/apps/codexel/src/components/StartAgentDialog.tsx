import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Alert, Autocomplete, Button, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, Stack, TextField } from '@mui/material'
import { AgentService, RepositoryService, type Agent } from '../connect/codexel_pb.js'

// StartAgentDialog starts the agent. If reviewId is given, the agent reviews the review by the prompt in the repository,
// so the working directory is not asked.
export const StartAgentDialog = ({
  open,
  onClose,
  onStarted,
  reviewId,
}: {
  open: boolean
  onClose: () => void
  onStarted: (agent: Agent) => void
  reviewId?: string
}) => {
  const { data } = useQuery(AgentService.method.listWorkspaces, {}, { enabled: open })
  const repositories = useQuery(RepositoryService.method.list, {}, { enabled: open && reviewId === undefined })
  const start = useMutation(AgentService.method.start)
  const [workspaceId, setWorkspaceId] = useState('')
  const [kind, setKind] = useState('claude')
  const [cwd, setCwd] = useState('')

  const submit = () => {
    start.mutate(
      { workspaceId, kind, cwd, reviewId },
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
      <DialogTitle>{reviewId === undefined ? 'Start agent' : 'Start review agent'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          {start.error && <Alert severity="error">{start.error.message}</Alert>}
          {reviewId !== undefined && (
            <Alert severity="info">
              The agent reviews the changes by the prompt in .codexel/review.md at trunk() and comments on them. The unresolved comments are passed to
              the agent which created the changes when the review is finished.
            </Alert>
          )}
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
          {reviewId === undefined && (
            <Autocomplete
              freeSolo
              options={repositories.data?.repositories.map((r) => r.path) ?? []}
              inputValue={cwd}
              onInputChange={(_, v) => setCwd(v)}
              renderInput={(params) => (
                <TextField {...params} label="Working directory" helperText="The opened repositories are suggested. The default of herdr if empty" />
              )}
            />
          )}
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
