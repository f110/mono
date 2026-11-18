import { useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Button, Checkbox, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, Stack, TextField, Typography } from '@mui/material'
import { RevisionService, type Commit } from '../connect/codexel_pb.js'

// SplitDialog picks the files to move into the new revision before the revision.
export const SplitDialog = ({
  repoPath,
  commit,
  onSubmit,
  onClose,
}: {
  repoPath: string
  commit: Commit
  onSubmit: (files: string[], description: string) => void
  onClose: () => void
}) => {
  const { data } = useQuery(RevisionService.method.getRevision, { path: repoPath, revision: commit.commitId }, { staleTime: Infinity })
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [description, setDescription] = useState('')
  const files = data?.files ?? []

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>Split {commit.changeId.slice(0, 8)}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          The checked files move into the new revision before this one. The rest keeps the description.
        </Typography>
        <Stack sx={{ my: 1 }}>
          {files.map((f) => (
            <FormControlLabel
              key={f.path}
              control={
                <Checkbox
                  size="small"
                  checked={selected.has(f.path)}
                  onChange={(e) =>
                    setSelected((s) => {
                      const next = new Set(s)
                      if (e.target.checked) next.add(f.path)
                      else next.delete(f.path)
                      return next
                    })
                  }
                />
              }
              label={
                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {f.path} (+{f.added} -{f.removed})
                </Typography>
              }
            />
          ))}
        </Stack>
        <TextField
          label="Description of the new revision"
          multiline
          fullWidth
          minRows={2}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={selected.size === 0 || selected.size === files.length} onClick={() => onSubmit([...selected], description)}>
          Split
        </Button>
      </DialogActions>
    </Dialog>
  )
}
