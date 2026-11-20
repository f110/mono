import { useState } from 'react'
import { Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField } from '@mui/material'
import { isSubmitKey } from '../lib/keys.ts'

// DescriptionDialog edits the description of the revision. Cmd/Ctrl+Enter submits it.
export const DescriptionDialog = ({
  title,
  initial,
  onSubmit,
  onClose,
}: {
  title: string
  initial: string
  onSubmit: (description: string) => void
  onClose: () => void
}) => {
  const [description, setDescription] = useState(initial)

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <TextField
          multiline
          fullWidth
          minRows={8}
          autoFocus
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          onKeyDown={(e) => {
            if (isSubmitKey(e.nativeEvent)) {
              e.preventDefault()
              onSubmit(description)
            }
          }}
          placeholder="Cmd/Ctrl+Enter to save"
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
          sx={{ mt: 1 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={() => onSubmit(description)}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  )
}
