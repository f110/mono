import { useEffect, useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Alert, AppBar, Box, Button, IconButton, Toolbar, Tooltip, Typography } from '@mui/material'
import HelpOutlineIcon from '@mui/icons-material/HelpOutline'
import { Link, Outlet, useRouter } from '@tanstack/react-router'
import { SystemService } from '../connect/codexel_pb.js'
import { ignoreKey, isInput } from '../lib/keys.ts'
import { ShortcutHelpDialog } from './ShortcutHelpDialog.tsx'

// screenKeys are the codes of the keys which move to the screens of the buttons of the app bar with Alt (Option). The code
// is used instead of the key because Option changes the key on macOS.
const screenKeys: Record<string, '/agents' | '/repositories' | '/reviews'> = {
  Digit1: '/agents',
  Digit2: '/repositories',
  Digit3: '/reviews',
}

export const Layout = () => {
  const { data } = useQuery(SystemService.method.getStatus, {})
  const missingBinaries = data?.missingBinaries ?? []
  const [helpOpen, setHelpOpen] = useState(false)
  const router = useRouter()

  // ? opens the help, Esc blurs the input without clearing it and Alt with the keys of screenKeys moves to the screen. See
  // docs/ui-conventions.md. Alt with the key is ignored by the same conditions as the shortcut key except the modifier.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented && !e.isComposing && e.keyCode !== 229 && isInput(e.target)) {
        e.target.blur()
        return
      }
      const to = screenKeys[e.code]
      if (to && e.altKey && !e.metaKey && !e.ctrlKey && !e.shiftKey) {
        const ignored =
          e.isComposing ||
          e.keyCode === 229 ||
          e.defaultPrevented ||
          isInput(e.target) ||
          document.querySelector('[role="dialog"], [role="menu"], [role="listbox"]') !== null
        if (!ignored && router.state.location.pathname !== to) {
          e.preventDefault()
          void router.navigate({ to })
        }
        return
      }
      if (e.key === '?' && !ignoreKey(e)) {
        e.preventDefault()
        setHelpOpen(true)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [router])

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <AppBar position="static" elevation={0}>
        <Toolbar variant="dense" sx={{ gap: 2 }}>
          <Typography variant="h6" component="div">
            Codexel
          </Typography>
          <Button color="inherit" component={Link} to="/agents">
            Agents
          </Button>
          <Button color="inherit" component={Link} to="/repositories">
            Repositories
          </Button>
          <Button color="inherit" component={Link} to="/reviews">
            Reviews
          </Button>
          <Box sx={{ flex: 1 }} />
          <Tooltip title="Keyboard shortcuts (?)">
            <IconButton color="inherit" aria-label="Keyboard shortcuts" onClick={() => setHelpOpen(true)}>
              <HelpOutlineIcon />
            </IconButton>
          </Tooltip>
        </Toolbar>
      </AppBar>
      {missingBinaries.length > 0 && (
        <Alert severity="warning" square>
          Not found in PATH: {missingBinaries.join(', ')}. Restart Codexel after installing them.
        </Alert>
      )}
      <Box component="main" sx={{ flex: 1, minHeight: 0 }}>
        <Outlet />
      </Box>
      <ShortcutHelpDialog open={helpOpen} onClose={() => setHelpOpen(false)} />
    </Box>
  )
}
