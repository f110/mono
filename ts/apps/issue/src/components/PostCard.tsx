import { type ReactNode, useState } from 'react'
import { Box, IconButton, Menu, MenuItem, Paper, SvgIcon, Typography } from '@mui/material'

export type PostAction = {
  label: string
  danger?: boolean
  onClick: () => void
}

// PostCard is the card of the body or the comment which has the header like GitHub.
export const PostCard = ({
  author,
  time,
  history,
  reporter,
  actions,
  children,
}: {
  author: string
  time: ReactNode
  history?: ReactNode
  reporter: boolean
  actions: PostAction[]
  children: ReactNode
}) => {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)

  return (
    <Paper variant="outlined">
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          py: 0.75,
          pl: 2,
          pr: 1,
          backgroundColor: '#f6f8fa',
          borderBottom: 1,
          borderColor: 'divider',
          borderRadius: '4px 4px 0 0',
        }}
      >
        <Typography variant="subtitle2" sx={{ flex: 1 }}>
          {author}{' '}
          <Typography component="span" variant="body2" color="text.secondary">
            {time}
            {history}
          </Typography>
        </Typography>
        {reporter && (
          <Box
            component="span"
            sx={{ fontSize: '0.75rem', lineHeight: '18px', px: '7px', border: 1, borderColor: 'rgba(0, 0, 0, 0.23)', borderRadius: '9px', color: 'text.secondary' }}
          >
            Reporter
          </Box>
        )}
        {actions.length > 0 && (
          <>
            <IconButton size="small" aria-label="操作" onClick={(e) => setAnchor(e.currentTarget)}>
              <SvgIcon fontSize="small">
                <path d="M6 10c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2m12 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2m-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2" />
              </SvgIcon>
            </IconButton>
            <Menu anchorEl={anchor} open={anchor !== null} onClose={() => setAnchor(null)}>
              {actions.map((a) => (
                <MenuItem
                  key={a.label}
                  sx={a.danger ? { color: 'error.main' } : undefined}
                  onClick={() => {
                    setAnchor(null)
                    a.onClick()
                  }}
                >
                  {a.label}
                </MenuItem>
              ))}
            </Menu>
          </>
        )}
      </Box>
      <Box sx={{ p: 2 }}>{children}</Box>
    </Paper>
  )
}
