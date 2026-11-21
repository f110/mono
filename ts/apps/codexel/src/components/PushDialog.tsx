import { useState } from 'react'
import { Button, Checkbox, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, Stack, Typography } from '@mui/material'
import type { Bookmark } from '../connect/codexel_pb.js'

// pushState describes what the push does to the tracked remote bookmarks of the local bookmark.
const pushState = (b: Bookmark, remotes: Bookmark[]) => {
  if (remotes.length === 0) return 'new'
  if (b.target === '') return 'delete'
  return remotes.map((r) => `${r.remote}: ${r.behind} to push, ${r.ahead} new on remote`).join(', ')
}

// PushDialog picks the local bookmarks to push. The candidates are the local bookmarks which are not on the tracked
// remote bookmarks yet, including the deleted ones.
export const PushDialog = ({
  bookmarks,
  onSubmit,
  onClose,
}: {
  bookmarks: Bookmark[]
  onSubmit: (names: string[]) => void
  onClose: () => void
}) => {
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const candidates = bookmarks
    .filter((b) => b.remote === '' && !b.conflict)
    .map((b) => ({ bookmark: b, remotes: bookmarks.filter((r) => r.remote !== '' && r.tracked && r.name === b.name) }))
    .filter(({ bookmark, remotes }) => remotes.length === 0 || !bookmark.synced)

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Push the bookmarks</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          The checked bookmarks are pushed to the remote.
        </Typography>
        <Stack sx={{ my: 1 }}>
          {candidates.map(({ bookmark: b, remotes }) => (
            <FormControlLabel
              key={b.name}
              control={
                <Checkbox
                  size="small"
                  checked={selected.has(b.name)}
                  onChange={(e) =>
                    setSelected((s) => {
                      const next = new Set(s)
                      if (e.target.checked) next.add(b.name)
                      else next.delete(b.name)
                      return next
                    })
                  }
                />
              }
              label={
                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {b.name}: {pushState(b, remotes)}
                </Typography>
              }
            />
          ))}
          {candidates.length === 0 && <Typography variant="body2">No bookmarks to push.</Typography>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={selected.size === 0} onClick={() => onSubmit([...selected])}>
          Push
        </Button>
      </DialogActions>
    </Dialog>
  )
}
