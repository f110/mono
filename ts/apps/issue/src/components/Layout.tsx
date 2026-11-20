import { useEffect, useState } from 'react'
import { Outlet, Link as RouterLink, useNavigate, useRouterState } from '@tanstack/react-router'
import { AppBar, Box, Container, CssBaseline, IconButton, Link, SvgIcon, Toolbar, Tooltip, Typography } from '@mui/material'
import { useQuery } from '@connectrpc/connect-query'
import { Issue } from '../connect/issue_pb'
import { globalShortcuts, useShortcuts } from '../shortcuts.ts'
import { NewTicketButton } from './NewTicketButton.tsx'
import { ShortcutHelpDialog } from './ShortcutHelpDialog.tsx'

export const Layout = () => {
  const { data } = useQuery(Issue.method.whoAmI, {})
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const navigate = useNavigate()
  const [helpOpen, setHelpOpen] = useState(false)
  useShortcuts(globalShortcuts, {
    help: () => setHelpOpen(true),
    new: () => void navigate({ to: '/new' }),
  })
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented || e.isComposing) {
        return
      }
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        e.target.blur()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  return (
    <>
      <CssBaseline />
      <AppBar position="static">
        <Toolbar>
          <Link component={RouterLink} to="/" color="inherit" underline="none" sx={{ flexGrow: 1 }}>
            <Typography variant="h6">Issue Tracker</Typography>
          </Link>
          <Typography variant="body2" sx={{ mr: 2, display: { xs: 'none', sm: 'block' } }}>
            {data?.actor?.name}
          </Typography>
          <IconButton component={RouterLink} to="/admin" color="inherit" aria-label="管理" sx={{ mr: 1 }}>
            <SvgIcon>
              <path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z" />
            </SvgIcon>
          </IconButton>
          <Tooltip title="キーボードショートカット (?)">
            <IconButton color="inherit" aria-label="キーボードショートカット" onClick={() => setHelpOpen(true)} sx={{ mr: 1 }}>
              <SvgIcon>
                <path d="M11 18h2v-2h-2zm1-16C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2m0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8m0-14c-2.21 0-4 1.79-4 4h2c0-1.1.9-2 2-2s2 .9 2 2c0 2-3 1.75-3 5h2c0-2.25 3-2.5 3-5 0-2.21-1.79-4-4-4" />
              </SvgIcon>
            </IconButton>
          </Tooltip>
          <NewTicketButton />
        </Toolbar>
      </AppBar>
      <Container maxWidth={pathname === '/' ? false : 'lg'}>
        <Box sx={{ py: 3 }}>
          <Outlet />
        </Box>
      </Container>
      <ShortcutHelpDialog open={helpOpen} onClose={() => setHelpOpen(false)} />
    </>
  )
}
