import { useRef, useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Link as RouterLink, createLink } from '@tanstack/react-router'
import { Button, ButtonGroup, Divider, ListItemText, ListSubheader, Menu, MenuItem, SvgIcon, Typography } from '@mui/material'
import { Issue } from '../connect/issue_pb'

const MenuItemLink = createLink(MenuItem)

export const NewTicketButton = () => {
  const anchorRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const { data } = useQuery(Issue.method.listTemplates, {}, { enabled: open })
  const { data: components } = useQuery(Issue.method.listComponents, {}, { enabled: open })
  const componentName = (id?: number) => components?.components.find((c) => c.id === id)?.name
  const close = () => setOpen(false)

  return (
    <>
      <ButtonGroup variant="outlined" color="inherit" ref={anchorRef}>
        <Button component={RouterLink} to="/new">
          新規作成
        </Button>
        <Button size="small" aria-label="テンプレートから作成" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(true)}>
          <SvgIcon fontSize="small">
            <path d="M7 10l5 5 5-5z" />
          </SvgIcon>
        </Button>
      </ButtonGroup>
      <Menu
        anchorEl={anchorRef.current}
        open={open}
        onClose={close}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <ListSubheader>テンプレートから作成</ListSubheader>
        {data?.templates.map((t) => (
          <MenuItemLink key={t.id} component="a" to="/new" search={{ template: t.id }} onClick={close}>
            <ListItemText>{t.title}</ListItemText>
            <Typography variant="body2" color="text.secondary" sx={{ ml: 3 }}>
              {componentName(t.componentId)}
            </Typography>
          </MenuItemLink>
        ))}
        {data && data.templates.length === 0 && <MenuItem disabled>テンプレートはありません</MenuItem>}
        <Divider />
        <MenuItemLink component="a" to="/admin/templates" onClick={close}>
          テンプレートを管理
        </MenuItemLink>
      </Menu>
    </>
  )
}
