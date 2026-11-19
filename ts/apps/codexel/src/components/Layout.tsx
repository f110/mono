import { useQuery } from '@connectrpc/connect-query'
import { Alert, AppBar, Box, Button, Toolbar, Typography } from '@mui/material'
import { Link, Outlet } from '@tanstack/react-router'
import { SystemService } from '../connect/codexel_pb.js'

export const Layout = () => {
  const { data } = useQuery(SystemService.method.getStatus, {})
  const missingBinaries = data?.missingBinaries ?? []

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
    </Box>
  )
}
