import { useCallback, useEffect, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Menu,
  MenuItem,
  Snackbar,
  Stack,
  TextField,
} from '@mui/material'
import { RevisionService, type Commit } from '../connect/codexel_pb.js'
import { useOperation } from '../hooks/useOperation.ts'
import { transport } from '../providers/AppProvider.tsx'
import { DescriptionDialog } from './DescriptionDialog.tsx'
import { RevisionDetail } from './RevisionDetail.tsx'
import { RevisionGraph } from './RevisionGraph.tsx'

// isEditing reports whether the key event is for the input. The keyboard shortcuts are ignored while editing.
const isEditing = (e: KeyboardEvent) => {
  const target = e.target as HTMLElement | null
  return target !== null && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)
}

// revisionKey identifies the revision across the operations. The change ID doesn't change by rewriting the revision but
// it is ambiguous for the divergent revisions.
const revisionKey = (c: Commit) => (c.divergent || c.hidden ? c.commitId : c.changeId)

const client = createClient(RevisionService, transport)

type Editing = { kind: 'describe'; commit: Commit; initial: string } | { kind: 'commit'; initial: string }

export const RepositoryView = ({ path }: { path: string }) => {
  const [revset, setRevset] = useState('')
  const [revsetInput, setRevsetInput] = useState('')
  const [selectedKey, setSelectedKey] = useState<string>()
  const [checked, setChecked] = useState<Set<string>>(new Set())
  const [message, setMessage] = useState<{ text: string; error: boolean }>()
  const [editing, setEditing] = useState<Editing>()
  const [abandoning, setAbandoning] = useState<Commit[]>()
  const [menu, setMenu] = useState<{ x: number; y: number; commit: Commit }>()
  const revsetRef = useRef<HTMLInputElement>(null)
  const operation = useOperation(path)
  const { data, error, refetch } = useQuery(RevisionService.method.log, { path, revset })
  const rows = data?.rows ?? []
  const commits = rows.map((r) => r.commit!)
  const foundIndex = commits.findIndex((c) => revisionKey(c) === selectedKey)
  const selected = foundIndex >= 0 ? foundIndex : 0
  const selectedCommit = commits[selected]
  const checkedCommits = commits.filter((c) => checked.has(c.commitId))

  useEffect(() => {
    if (operation) {
      refetch()
    }
  }, [operation, refetch])

  const selectIndex = useCallback((i: number) => {
    const c = commits[Math.max(0, Math.min(i, commits.length - 1))]
    if (c) {
      setSelectedKey(revisionKey(c))
    }
  }, [commits])

  // run runs the operation and shows the message of jj. selectWorkingCopy moves the selection to the new working copy.
  const run = useCallback(
    async (fn: () => Promise<{ output: string }>, selectWorkingCopy: boolean) => {
      try {
        const res = await fn()
        setMessage({ text: res.output, error: false })
        setChecked(new Set())
        const log = await refetch()
        if (selectWorkingCopy) {
          const wc = log.data?.rows.find((r) => r.commit?.currentWorkingCopy)?.commit
          if (wc) {
            setSelectedKey(revisionKey(wc))
          }
        }
      } catch (e) {
        setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
      }
    },
    [refetch],
  )

  // targets returns the checked revisions, or the selected one if nothing is checked.
  const targets = useCallback((): Commit[] => (checkedCommits.length > 0 ? checkedCommits : selectedCommit ? [selectedCommit] : []), [checkedCommits, selectedCommit])

  const newRevision = useCallback(
    (cs: Commit[]) => run(() => client.new({ path, revisions: cs.map((c) => c.commitId) }), true),
    [path, run],
  )
  const edit = useCallback((c: Commit) => run(() => client.edit({ path, revision: c.commitId }), true), [path, run])
  const startDescribe = useCallback(
    async (c: Commit) => {
      const res = await client.getRevision({ path, revision: c.commitId })
      setEditing({ kind: 'describe', commit: c, initial: res.description })
    },
    [path],
  )
  const startCommit = useCallback(async () => {
    const wc = commits.find((c) => c.currentWorkingCopy)
    const res = wc ? await client.getRevision({ path, revision: wc.commitId }) : undefined
    setEditing({ kind: 'commit', initial: res?.description ?? '' })
  }, [commits, path])
  const undo = useCallback(() => run(() => client.undo({ path }), false), [path, run])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isEditing(e) || e.metaKey || e.ctrlKey || e.altKey || editing || abandoning) {
        return
      }
      switch (e.key) {
        case 'j':
          selectIndex(selected + 1)
          break
        case 'k':
          selectIndex(selected - 1)
          break
        case '@':
          selectIndex(commits.findIndex((c) => c.currentWorkingCopy))
          break
        case ' ':
          if (selectedCommit) {
            setChecked((s) => {
              const next = new Set(s)
              if (!next.delete(selectedCommit.commitId)) {
                next.add(selectedCommit.commitId)
              }
              return next
            })
          }
          break
        case '/':
          revsetRef.current?.focus()
          break
        case 'r':
          refetch()
          break
        case 'n':
          newRevision(targets())
          break
        case 'e':
          if (selectedCommit && checked.size === 0) {
            startDescribe(selectedCommit)
          }
          break
        case 'c':
          startCommit()
          break
        case 'u':
          undo()
          break
        case 'Escape':
          setChecked(new Set())
          break
        default:
          return
      }
      e.preventDefault()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [commits, selected, selectedCommit, checked, editing, abandoning, selectIndex, refetch, newRevision, targets, startDescribe, startCommit, undo])

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
              setSelectedKey(undefined)
              revsetRef.current?.blur()
            } else if (e.key === 'Escape') {
              setRevsetInput(revset)
              revsetRef.current?.blur()
            }
          }}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />
      </Box>
      {error && (
        <Alert severity="error" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
          {error.message}
        </Alert>
      )}
      <Stack direction="row" sx={{ flex: 1, minHeight: 0 }}>
        <Box sx={{ width: '45%', flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
          <RevisionGraph
            rows={rows}
            selected={selected}
            checked={checked}
            onSelect={selectIndex}
            onContextMenu={(i, x, y) => {
              selectIndex(i)
              setMenu({ x, y, commit: commits[i] })
            }}
          />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0, overflow: 'auto' }}>
          {selectedCommit && <RevisionDetail key={selectedCommit.commitId} path={path} commit={selectedCommit} />}
        </Box>
      </Stack>

      <Menu open={menu !== undefined} onClose={() => setMenu(undefined)} anchorReference="anchorPosition" anchorPosition={menu ? { top: menu.y, left: menu.x } : undefined}>
        {menu && [
          <MenuItem key="new" onClick={() => { setMenu(undefined); newRevision([menu.commit]) }}>New (n)</MenuItem>,
          <MenuItem key="edit" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); edit(menu.commit) }}>Edit</MenuItem>,
          <MenuItem key="describe" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); startDescribe(menu.commit) }}>Describe (e)</MenuItem>,
          <MenuItem key="abandon" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); setAbandoning(checkedCommits.length > 0 ? checkedCommits : [menu.commit]) }}>Abandon</MenuItem>,
        ]}
      </Menu>

      {editing && (
        <DescriptionDialog
          title={editing.kind === 'describe' ? `Describe ${editing.commit.changeId.slice(0, 8)}` : 'Commit the working copy'}
          initial={editing.initial}
          onClose={() => setEditing(undefined)}
          onSubmit={(description) => {
            const e = editing
            setEditing(undefined)
            if (e.kind === 'describe') {
              run(() => client.describe({ path, revision: e.commit.commitId, description }), false)
            } else {
              run(() => client.commit({ path, description }), true)
            }
          }}
        />
      )}

      <Dialog open={abandoning !== undefined} onClose={() => setAbandoning(undefined)}>
        <DialogTitle>Abandon {abandoning?.length} revision(s)?</DialogTitle>
        <DialogContent>
          {abandoning?.map((c) => (
            <Box key={c.commitId} sx={{ fontFamily: 'monospace', fontSize: 13 }}>
              {c.changeId.slice(0, 8)} {c.description || '(no description set)'}
            </Box>
          ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAbandoning(undefined)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            autoFocus
            onClick={() => {
              const cs = abandoning ?? []
              setAbandoning(undefined)
              run(() => client.abandon({ path, revisions: cs.map((c) => c.commitId) }), true)
            }}
          >
            Abandon
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={message !== undefined} autoHideDuration={message?.error ? null : 6000} onClose={() => setMessage(undefined)}>
        <Alert severity={message?.error ? 'error' : 'info'} onClose={() => setMessage(undefined)} sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 12 }}>
          {message?.text || 'Done'}
        </Alert>
      </Snackbar>
    </Stack>
  )
}
