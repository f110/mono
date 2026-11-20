import { useEffect, useRef, useState } from 'react'
import { Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Stack, TextField, Typography } from '@mui/material'
import type { RepositoryWorkspace } from '../connect/codexel_pb.js'

// WorkspacesPanel lists the workspaces of the repository. The workspace is opened as another repository. The operations
// on the workspace are shown in the selected row. The keys are handled by the parent.
export const WorkspacesPanel = ({
  repoPath,
  workspaces,
  selected,
  onSelect,
  onOpen,
  onRemove,
  onForget,
  onRename,
  onUpdateStale,
}: {
  repoPath: string
  workspaces: RepositoryWorkspace[]
  selected: number
  onSelect: (index: number) => void
  onOpen: (w: RepositoryWorkspace) => void
  onRemove: (w: RepositoryWorkspace) => void
  onForget: (w: RepositoryWorkspace) => void
  onRename: () => void
  onUpdateStale: () => void
}) => {
  const selectedRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  return (
    <Box sx={{ py: 0.5 }}>
      <Stack direction="row" sx={{ px: 1, alignItems: 'center' }}>
        <Typography variant="caption" color="text.secondary" sx={{ flex: 1 }}>
          j/k: move, o: open, d: remove, f: forget, 1: log
        </Typography>
        <Button size="small" sx={{ textTransform: 'none' }} onClick={onRename}>
          Rename the current
        </Button>
        <Button size="small" sx={{ textTransform: 'none' }} onClick={onUpdateStale}>
          Update the stale
        </Button>
      </Stack>
      {workspaces.map((w, i) => {
        const current = w.root === repoPath
        return (
          <Stack
            key={w.name}
            ref={i === selected ? selectedRef : undefined}
            direction="row"
            spacing={1}
            onClick={() => onSelect(i)}
            sx={{ px: 1, height: 28, alignItems: 'center', cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined }}
          >
            <Typography variant="body2" sx={{ fontFamily: 'monospace' }} noWrap>
              {w.name}
            </Typography>
            {current && <Chip label="current" size="small" color="primary" variant="outlined" sx={{ height: 18 }} />}
            <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
              {w.target.slice(0, 8)}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap sx={{ flex: 1, minWidth: 0 }} title={w.root}>
              {w.root}
            </Typography>
            {i === selected && !current && (
              <>
                <Button size="small" title="o" sx={{ minWidth: 0, py: 0, flexShrink: 0 }} onClick={() => onOpen(w)}>
                  Open
                </Button>
                <Button size="small" color="error" title="d" sx={{ minWidth: 0, py: 0, flexShrink: 0 }} onClick={() => onRemove(w)}>
                  Remove
                </Button>
                <Button size="small" color="warning" title="f" sx={{ minWidth: 0, py: 0, flexShrink: 0 }} onClick={() => onForget(w)}>
                  Forget
                </Button>
              </>
            )}
          </Stack>
        )
      })}
    </Box>
  )
}

// WorkspaceNameDialog asks the name of the workspace to add or to rename.
export const WorkspaceNameDialog = ({
  title,
  submitLabel,
  onSubmit,
  onClose,
}: {
  title: string
  submitLabel: string
  onSubmit: (name: string) => void
  onClose: () => void
}) => {
  const [name, setName] = useState('')

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <TextField
          autoFocus
          fullWidth
          size="small"
          label="Workspace"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && name) {
              onSubmit(name)
            }
          }}
          sx={{ mt: 1 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!name} onClick={() => onSubmit(name)}>
          {submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
