import { AppBar, Box, Button, Toolbar, Typography } from '@mui/material'
import { Link, Outlet } from '@tanstack/react-router'

export const Layout = () => (
  <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
    <AppBar position="static" elevation={0}>
      <Toolbar variant="dense" sx={{ gap: 2 }}>
        <Typography variant="h6" component="div">
          Codexel
        </Typography>
        <Button color="inherit" component={Link} to="/agents">
          Agents
        </Button>
      </Toolbar>
    </AppBar>
    <Box component="main" sx={{ flex: 1, minHeight: 0 }}>
      <Outlet />
    </Box>
  </Box>
)
