import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Box, Button, IconButton, Stack, Tab, Tabs, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { OpenRepositoryDialog } from '../components/OpenRepositoryDialog.tsx'
import { RepositoryView } from '../components/RepositoryView.tsx'
import { useAgents } from '../hooks/useAgents.ts'
import { RepositoryService } from '../connect/codexel_pb.js'

export const RepositoriesPage = () => {
  const { path } = useSearch({ from: '/repositories' })
  const navigate = useNavigate({ from: '/repositories' })
  const { data, refetch } = useQuery(RepositoryService.method.list, {})
  // useAgents reads the list of the repositories again when the agents change.
  useAgents()
  const closeRepository = useMutation(RepositoryService.method.close)
  const [openDialog, setOpenDialog] = useState(false)
  const repositories = data?.repositories ?? []
  const current = path ?? repositories[0]?.path
  const select = (p: string | undefined) => navigate({ search: { path: p } })

  const close = (p: string) => {
    closeRepository.mutate(
      { path: p },
      {
        onSuccess: () => {
          refetch()
          if (p === current) {
            select(undefined)
          }
        },
      },
    )
  }

  return (
    <Stack sx={{ height: '100%' }}>
      <Stack direction="row" sx={{ borderBottom: 1, borderColor: 'divider', alignItems: 'center' }}>
        <Tabs
          value={repositories.some((r) => r.path === current) ? current : false}
          onChange={(_, v) => select(v)}
          variant="scrollable"
          sx={{ flex: 1 }}
        >
          {repositories.map((r) => (
            <Tab
              key={r.path}
              value={r.path}
              title={r.path}
              label={
                <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                  <span>{r.name}</span>
                  <IconButton
                    component="span"
                    size="small"
                    sx={{ p: 0 }}
                    onClick={(e) => {
                      e.stopPropagation()
                      close(r.path)
                    }}
                  >
                    <CloseIcon fontSize="inherit" />
                  </IconButton>
                </Stack>
              }
            />
          ))}
        </Tabs>
        <Button sx={{ mx: 1 }} onClick={() => setOpenDialog(true)}>
          Open
        </Button>
      </Stack>
      <Box sx={{ flex: 1, minHeight: 0 }}>
        {current ? (
          <RepositoryView
            key={current}
            path={current}
            onOpenRepository={(p) => {
              refetch()
              select(p)
            }}
          />
        ) : (
          <Typography color="text.secondary" sx={{ p: 2 }}>
            Open a repository
          </Typography>
        )}
      </Box>
      <OpenRepositoryDialog
        open={openDialog}
        onClose={() => setOpenDialog(false)}
        onOpened={(r) => {
          refetch()
          select(r.path)
        }}
      />
    </Stack>
  )
}
