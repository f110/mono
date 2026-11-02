import { Outlet, Link as RouterLink, useRouterState } from '@tanstack/react-router'
import { AppBar, Box, Button, Container, CssBaseline, Link, Toolbar, Typography } from '@mui/material'
import { useQuery } from '@connectrpc/connect-query'
import { Issue } from '../connect/issue_pb'

export const Layout = () => {
  const { data } = useQuery(Issue.method.whoAmI, {})
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  return (
    <>
      <CssBaseline />
      <AppBar position="static">
        <Toolbar>
          <Link component={RouterLink} to="/" color="inherit" underline="none" sx={{ flexGrow: 1 }}>
            <Typography variant="h6">Issue Tracker</Typography>
          </Link>
          <Typography variant="body2" sx={{ mr: 2 }}>
            {data?.actor?.name}
          </Typography>
          <Button component={RouterLink} to="/new" color="inherit" variant="outlined">
            新規作成
          </Button>
        </Toolbar>
      </AppBar>
      <Container maxWidth={pathname === '/' ? false : 'lg'}>
        <Box sx={{ py: 3 }}>
          <Outlet />
        </Box>
      </Container>
    </>
  )
}
