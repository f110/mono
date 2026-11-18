import { useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Divider, Stack, TextField, Typography } from '@mui/material'
import { WorkspaceService, type Commit } from '../connect/codexel_pb.js'

// WorkspaceDialog manages the workspaces of the repository. The workspace is opened as another repository.
export const WorkspaceDialog = ({
  repoPath,
  selected,
  onOpen,
  onAdd,
  onForget,
  onRename,
  onUpdateStale,
  onClose,
}: {
  repoPath: string
  // selected is the revision for the working copy of the new workspace.
  selected: Commit | undefined
  onOpen: (root: string) => void
  onAdd: (name: string) => void
  onForget: (name: string) => void
  onRename: (name: string) => void
  onUpdateStale: () => void
  onClose: () => void
}) => {
  const { data } = useQuery(WorkspaceService.method.listRepositoryWorkspaces, { path: repoPath })
  const [newName, setNewName] = useState('')
  const [rename, setRename] = useState('')
  const workspaces = data?.workspaces ?? []

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Workspaces</DialogTitle>
      <DialogContent>
        {workspaces.map((w) => {
          const current = w.root === repoPath
          return (
            <Stack key={w.name} direction="row" spacing={1} sx={{ alignItems: 'center', py: 0.5 }}>
              <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                {w.name}
              </Typography>
              {current && <Chip label="current" size="small" color="primary" variant="outlined" sx={{ height: 18 }} />}
              <Typography variant="caption" color="text.secondary" noWrap sx={{ flex: 1, minWidth: 0 }} title={w.root}>
                {w.root}
              </Typography>
              {!current && (
                <>
                  <Button size="small" onClick={() => onOpen(w.root)}>
                    Open
                  </Button>
                  <Button size="small" color="warning" onClick={() => onForget(w.name)}>
                    Forget
                  </Button>
                </>
              )}
            </Stack>
          )
        })}
        <Divider sx={{ my: 1 }} />
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', py: 0.5 }}>
          <TextField size="small" label="New workspace" value={newName} onChange={(e) => setNewName(e.target.value)} sx={{ flex: 1 }} />
          <Button disabled={!newName || !selected} onClick={() => onAdd(newName)}>
            Add on {selected?.changeId.slice(0, 8)}
          </Button>
        </Stack>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', py: 0.5 }}>
          <TextField size="small" label="Rename the current workspace" value={rename} onChange={(e) => setRename(e.target.value)} sx={{ flex: 1 }} />
          <Button disabled={!rename} onClick={() => onRename(rename)}>
            Rename
          </Button>
        </Stack>
        <Button size="small" onClick={onUpdateStale} sx={{ mt: 1 }}>
          Update the stale working copy
        </Button>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  )
}
