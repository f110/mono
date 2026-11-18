import { useEffect } from 'react'
import { AppBar, Box, Button, IconButton, Toolbar, Typography, useColorScheme } from '@mui/material'
import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import { Link, Outlet } from '@tanstack/react-router'
import { ignoreKey } from '../lib/keys.ts'

export const Layout = () => {
  const { mode, systemMode, setMode } = useColorScheme()
  const dark = (mode === 'system' ? systemMode : mode) === 'dark'
  const toggle = () => setMode(dark ? 'light' : 'dark')

  // t toggles the theme like lightjj. MUI keeps the mode in the local storage.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 't' || ignoreKey(e)) {
        return
      }
      e.preventDefault()
      setMode(dark ? 'light' : 'dark')
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [dark, setMode])

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
          <Box sx={{ flex: 1 }} />
          <IconButton color="inherit" onClick={toggle} title="Toggle the theme (t)">
            {dark ? <LightModeIcon /> : <DarkModeIcon />}
          </IconButton>
        </Toolbar>
      </AppBar>
      <Box component="main" sx={{ flex: 1, minHeight: 0 }}>
        <Outlet />
      </Box>
    </Box>
  )
}
