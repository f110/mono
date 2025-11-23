import { Outlet, createRootRoute } from '@tanstack/react-router'
import { SideBar } from '../components/SideBar.tsx'
import CssBaseline from '@mui/material/CssBaseline'
import AppBar from '@mui/material/AppBar'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import Box from '@mui/material/Box'
import Stack from '@mui/material/Stack'
import '../App.css'

export const Route = createRootRoute({
  component: () => (
    <>
      <CssBaseline />
      <Box sx={{}}>
        <Stack>
          <AppBar
            position="fixed"
            sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}
          >
            <Toolbar>
              <Typography variant="h6" noWrap component="div">
                Build Dashboard
              </Typography>
            </Toolbar>
          </AppBar>
          <Toolbar /> {/* For padding of the box at bottom*/}
          <Box sx={{ p: 3, display: 'flex' }}>
            <SideBar />
            <Outlet />
          </Box>
        </Stack>
      </Box>
    </>
  ),
  notFoundComponent: () => <p>Not Found</p>,
})
