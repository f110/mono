import { IconButton, Stack, ToggleButton } from '@mui/material'
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown'
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp'
import type { DiffNavigation } from '../hooks/useDiffNavigation.ts'

// DiffControls is the buttons of the diff: the split view, the preview of Markdown and the move to the next / previous hunk.
export const DiffControls = ({ navigation }: { navigation: DiffNavigation }) => (
  <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
    <ToggleButton size="small" value="split" selected={navigation.split} onChange={navigation.toggleSplit} title="|" sx={{ py: 0.25 }}>
      Split
    </ToggleButton>
    <ToggleButton size="small" value="preview" selected={navigation.preview} onChange={navigation.togglePreview} title="m" sx={{ py: 0.25 }}>
      Preview
    </ToggleButton>
    <IconButton size="small" title="Previous hunk ([)" onClick={() => navigation.moveHunk('previous')}>
      <KeyboardArrowUpIcon fontSize="small" />
    </IconButton>
    <IconButton size="small" title="Next hunk (])" onClick={() => navigation.moveHunk('next')}>
      <KeyboardArrowDownIcon fontSize="small" />
    </IconButton>
  </Stack>
)
