import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  ListSubheader,
  Stack,
  TextField,
} from '@mui/material'
import { RepositoryService, type Repository } from '../connect/codexel_pb.js'

export const OpenRepositoryDialog = ({
  open,
  onClose,
  onOpened,
}: {
  open: boolean
  onClose: () => void
  onOpened: (repo: Repository) => void
}) => {
  const { data } = useQuery(RepositoryService.method.listCandidates, {}, { enabled: open })
  const openRepository = useMutation(RepositoryService.method.openRepository)
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
          {data && data.repositories.length > 0 && (
            <List dense subheader={<ListSubheader disableGutters>Where the agents work</ListSubheader>}>
              {data.repositories.map((r) => (
                <ListItemButton key={r.path} onClick={() => submit(r.path)}>
                  <ListItemText primary={r.name} secondary={r.path} />
                </ListItemButton>
              ))}
            </List>
          )}
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
