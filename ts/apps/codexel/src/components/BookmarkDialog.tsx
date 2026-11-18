import { useState } from 'react'
import { Autocomplete, Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField } from '@mui/material'

// BookmarkDialog asks the name of the bookmark to set on the revision. The existing local bookmarks are suggested.
export const BookmarkDialog = ({
  title,
  names,
  onSubmit,
  onClose,
}: {
  title: string
  names: string[]
  onSubmit: (name: string) => void
  onClose: () => void
}) => {
  const [name, setName] = useState('')

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Autocomplete
          freeSolo
          options={names}
          inputValue={name}
          onInputChange={(_, v) => setName(v)}
          renderInput={(params) => (
            <TextField
              {...params}
              autoFocus
              label="Bookmark"
              helperText="The existing bookmark is moved to the revision even if it moves backwards"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && name) {
                  onSubmit(name)
                }
              }}
            />
          )}
          sx={{ mt: 1 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!name} onClick={() => onSubmit(name)}>
          Set
        </Button>
      </DialogActions>
    </Dialog>
  )
}
