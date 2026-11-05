import { Outlet, Link as RouterLink, useRouterState } from '@tanstack/react-router'
import { List, ListItemButton, ListItemText, Paper, Stack } from '@mui/material'

const sections = [
  { to: '/admin/components', label: 'Component' },
  { to: '/admin/templates', label: 'Template' },
] as const

export const AdminLayout = () => {
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  return (
    <Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
      <Paper sx={{ width: { md: 220 }, alignSelf: 'flex-start', flexShrink: 0 }}>
        <List dense disablePadding>
          {sections.map((s) => (
            <ListItemButton key={s.to} component={RouterLink} to={s.to} selected={pathname.startsWith(s.to)}>
              <ListItemText primary={s.label} />
            </ListItemButton>
          ))}
        </List>
      </Paper>
      <Stack sx={{ flex: 1, minWidth: 0 }}>
        <Outlet />
      </Stack>
    </Stack>
  )
}
