import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Button, IconButton, Stack, Tab, Tabs, TextField, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { OpenRepositoryDialog } from '../components/OpenRepositoryDialog.tsx'
import { RevisionGraph } from '../components/RevisionGraph.tsx'
import { RepositoryService, RevisionService } from '../connect/codexel_pb.js'
import { useOperation } from '../hooks/useOperation.ts'

// isEditing reports whether the key event is for the input. The keyboard shortcuts are ignored while editing.
const isEditing = (e: KeyboardEvent) => {
  const target = e.target as HTMLElement | null
  return target !== null && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)
}

const RepositoryView = ({ path }: { path: string }) => {
  const [revset, setRevset] = useState('')
  const [revsetInput, setRevsetInput] = useState('')
  const [selected, setSelected] = useState(0)
  const revsetRef = useRef<HTMLInputElement>(null)
  const operation = useOperation(path)
  const { data, error, refetch } = useQuery(RevisionService.method.log, { path, revset })
  const rows = data?.rows ?? []

  useEffect(() => {
    if (operation) {
      refetch()
    }
  }, [operation, refetch])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isEditing(e) || e.metaKey || e.ctrlKey || e.altKey) {
        return
      }
      switch (e.key) {
        case 'j':
          setSelected((s) => Math.min(s + 1, rows.length - 1))
          break
        case 'k':
          setSelected((s) => Math.max(s - 1, 0))
          break
        case '@': {
          const i = rows.findIndex((r) => r.commit?.currentWorkingCopy)
          if (i >= 0) {
            setSelected(i)
          }
          break
        }
        case '/':
          e.preventDefault()
          revsetRef.current?.focus()
          break
        case 'r':
          refetch()
          break
        default:
          return
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [rows, refetch])

  return (
    <Stack sx={{ height: '100%' }}>
      <Box sx={{ p: 1 }}>
        <TextField
          inputRef={revsetRef}
          size="small"
          fullWidth
          placeholder="Revset (/ to focus, Enter to apply, Esc to cancel)"
          value={revsetInput}
          onChange={(e) => setRevsetInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              setRevset(revsetInput)
              setSelected(0)
              revsetRef.current?.blur()
            } else if (e.key === 'Escape') {
              setRevsetInput(revset)
              revsetRef.current?.blur()
            }
          }}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />
      </Box>
      {error && <Alert severity="error" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>{error.message}</Alert>}
      <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
        <RevisionGraph rows={rows} selected={selected} onSelect={setSelected} />
      </Box>
    </Stack>
  )
}

export const RepositoriesPage = () => {
  const { path } = useSearch({ from: '/repositories' })
  const navigate = useNavigate({ from: '/repositories' })
  const { data, refetch } = useQuery(RepositoryService.method.listRepositories, {})
  const closeRepository = useMutation(RepositoryService.method.closeRepository)
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
        <Tabs value={repositories.some((r) => r.path === current) ? current : false} onChange={(_, v) => select(v)} variant="scrollable" sx={{ flex: 1 }}>
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
          <RepositoryView key={current} path={current} />
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
