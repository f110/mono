import { useState } from 'react'
import { useMutation } from '@connectrpc/connect-query'
import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, TextField } from '@mui/material'
import { RepositoryService, type Repository } from '../connect/codexel_pb.js'

export const OpenRepositoryDialog = ({ open, onClose, onOpened }: { open: boolean; onClose: () => void; onOpened: (repo: Repository) => void }) => {
  const openRepository = useMutation(RepositoryService.method.open)
  const [path, setPath] = useState('')

  const submit = (p: string) => {
    openRepository.mutate(
      { path: p },
      {
        onSuccess: (res) => {
          if (res.repository) {
            onOpened(res.repository)
          }
          setPath('')
          onClose()
        },
      },
    )
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Open repository</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          {openRepository.error && <Alert severity="error">{openRepository.error.message}</Alert>}
          <TextField
            label="Path"
            value={path}
            onChange={(e) => setPath(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && path) {
                submit(path)
              }
            }}
            helperText="Any directory in the workspace of jj"
            autoFocus
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={() => submit(path)} disabled={!path || openRepository.isPending}>
          Open
        </Button>
      </DialogActions>
    </Dialog>
  )
}
