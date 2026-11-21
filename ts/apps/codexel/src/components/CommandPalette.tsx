import { useState } from 'react'
import { Box, Dialog, List, ListItemButton, ListItemText, TextField, Typography } from '@mui/material'
import { isInputSubmitKey } from '../lib/keys.ts'

export type Action = { label: string; shortcut?: string; run: () => void }

// CommandPalette runs the action picked by filtering the labels. ↑/↓ moves and Enter runs it.
export const CommandPalette = ({ actions, onClose }: { actions: Action[]; onClose: () => void }) => {
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState(0)
  const words = filter
    .toLowerCase()
    .split(/\s+/)
    .filter((w) => w !== '')
  const matched = actions.filter((a) => words.every((w) => a.label.toLowerCase().includes(w)))
  const index = Math.min(selected, Math.max(matched.length - 1, 0))

  const run = (a: Action | undefined) => {
    if (!a) return
    onClose()
    a.run()
  }

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm" slotProps={{ paper: { sx: { alignSelf: 'flex-start', mt: 10 } } }}>
      <TextField
        autoFocus
        fullWidth
        placeholder="Type a command"
        value={filter}
        onChange={(e) => {
          setFilter(e.target.value)
          setSelected(0)
        }}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            setSelected(Math.min(index + 1, matched.length - 1))
          } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            setSelected(Math.max(index - 1, 0))
          } else if (isInputSubmitKey(e.nativeEvent)) {
            e.preventDefault()
            run(matched[index])
          }
        }}
        sx={{ p: 1 }}
      />
      <List dense sx={{ maxHeight: 400, overflow: 'auto' }}>
        {matched.map((a, i) => (
          <ListItemButton key={a.label} selected={i === index} onClick={() => run(a)}>
            <ListItemText primary={a.label} />
            {a.shortcut && (
              <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                {a.shortcut}
              </Typography>
            )}
          </ListItemButton>
        ))}
        {matched.length === 0 && (
          <Box sx={{ p: 2 }}>
            <Typography color="text.secondary">No commands</Typography>
          </Box>
        )}
      </List>
    </Dialog>
  )
}
