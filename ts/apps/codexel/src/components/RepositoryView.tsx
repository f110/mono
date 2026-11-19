import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import { useNavigate, useRouter } from '@tanstack/react-router'
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
import {
  RebaseSource,
  RebaseTarget,
  RepositoryService,
  ReviewService,
  type Commit,
} from '../connect/codexel_pb.js'
import { useAgents } from '../hooks/useAgents.ts'
import { useApprovedCommits } from '../hooks/useApprovedCommits.ts'
import { useDiffNavigation } from '../hooks/useDiffNavigation.ts'
import { useOperation } from '../hooks/useOperation.ts'
import { useReviews } from '../hooks/useReviews.ts'
import { fileLocation, fileRevset, type FileRevision } from '../lib/fileTree.ts'
import { ignoreKey, pressKey } from '../lib/keys.ts'
import { computeSlide } from '../lib/slide.ts'
import { transport } from '../providers/AppProvider.tsx'
import { BookmarkDialog } from './BookmarkDialog.tsx'
import { BookmarksPanel, bookmarkLabel } from './BookmarksPanel.tsx'
import { CommandPalette } from './CommandPalette.tsx'
import { DescriptionDialog } from './DescriptionDialog.tsx'
import { DivergenceDialog } from './DivergenceDialog.tsx'
import { FileDiffView } from './FileDiffView.tsx'
import { FileBrowser } from './FileTree.tsx'
import { ConflictsPanel, conflictEntries } from './ConflictsPanel.tsx'
import { EvologPanel, OperationsPanel } from './HistoryPanels.tsx'
import { MergeEditor } from './MergeEditor.tsx'
import { RevisionDetail } from './RevisionDetail.tsx'
import { RevisionGraph } from './RevisionGraph.tsx'
import { SplitDialog } from './SplitDialog.tsx'
import { WorkspaceDialog } from './WorkspaceDialog.tsx'

// revisionKey identifies the revision across the operations. The change ID doesn't change by rewriting the revision but
// it is ambiguous for the divergent revisions.
const revisionKey = (c: Commit) => (c.divergent || c.hidden ? c.commitId : c.changeId)

const repositoryClient = createClient(RepositoryService, transport)
const reviewClient = createClient(ReviewService, transport)

// operationLimit is the number of the operations shown in the operation log.
const operationLimit = 200

type Editing = { kind: 'describe'; commit: Commit; initial: string } | { kind: 'commit'; initial: string }

// Mode is the inline mode to pick the destination by moving the cursor. The sources are fixed when it starts.
type Mode =
  | { kind: 'rebase'; sources: Commit[]; source: RebaseSource; target: RebaseTarget }
  | { kind: 'squash'; sources: Commit[]; combine: boolean }

const sourceLabel: Record<RebaseSource, string> = {
  [RebaseSource.UNSPECIFIED]: '',
  [RebaseSource.REVISIONS]: '-r revisions',
  [RebaseSource.DESCENDANTS]: '-s with descendants',
  [RebaseSource.BRANCH]: '-b branch',
}

const targetLabel: Record<RebaseTarget, string> = {
  [RebaseTarget.UNSPECIFIED]: '',
  [RebaseTarget.ONTO]: 'onto',
  [RebaseTarget.INSERT_AFTER]: 'after',
  [RebaseTarget.INSERT_BEFORE]: 'before',
}

const modeKeys: Record<string, Partial<{ source: RebaseSource; target: RebaseTarget }>> = {
  r: { source: RebaseSource.REVISIONS },
  s: { source: RebaseSource.DESCENDANTS },
  b: { source: RebaseSource.BRANCH },
  o: { target: RebaseTarget.ONTO },
  d: { target: RebaseTarget.ONTO },
  a: { target: RebaseTarget.INSERT_AFTER },
  i: { target: RebaseTarget.INSERT_BEFORE },
}

const modeDescription = (mode: Mode): string =>
  mode.kind === 'rebase'
    ? `Rebase ${mode.sources.length} revision(s) [${sourceLabel[mode.source]}] ${targetLabel[mode.target]} the cursor. ` +
      'j/k: move, r/s/b: source, o/a/i: onto/after/before, Enter: rebase, Esc: cancel'
    : `Squash ${mode.sources.length} revision(s) into the cursor [${mode.combine ? 'combine the descriptions' : 'keep the description of the destination'}]. ` +
      'j/k: move, c: toggle the description, Enter: squash, Esc: cancel'

// RepositoryView shows the repository. onOpenRepository is called after the other repository (e.g. the workspace) is
// opened.
export const RepositoryView = ({ path, onOpenRepository }: { path: string; onOpenRepository: (path: string) => void }) => {
  const [revset, setRevset] = useState('')
  const [revsetInput, setRevsetInput] = useState('')
  const [selectedKey, setSelectedKey] = useState<string>()
  const [checked, setChecked] = useState<Set<string>>(new Set())
  const [message, setMessage] = useState<{ text: string; error: boolean }>()
  const [editing, setEditing] = useState<Editing>()
  const [abandoning, setAbandoning] = useState<Commit[]>()
  const [mode, setMode] = useState<Mode>()
  const { history } = useRouter()
  const [treeTarget, setTreeTarget] = useState<FileRevision | undefined>(() => fileLocation(history.location.state, path)?.revision)
  const [view, setView] = useState<'log' | 'bookmarks' | 'oplog' | 'evolog' | 'files' | 'conflicts'>(() => (treeTarget ? 'files' : 'log'))
  const [conflictSelected, setConflictSelected] = useState(0)
  const [splitting, setSplitting] = useState<Commit>()
  const [workspaceOpen, setWorkspaceOpen] = useState(false)
  const [divergent, setDivergent] = useState<Commit>()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const { data: aliasData } = useQuery(RepositoryService.method.listAliases, { path }, { enabled: paletteOpen })
  const [restoringFile, setRestoringFile] = useState<{ commit: Commit; file: string }>()
  const [bookmarkSelected, setBookmarkSelected] = useState(0)
  const [settingBookmark, setSettingBookmark] = useState<Commit>()
  const [pushing, setPushing] = useState(false)
  const [menu, setMenu] = useState<{ x: number; y: number; commit: Commit }>()
  const [opSelected, setOpSelected] = useState(0)
  const [opMenu, setOpMenu] = useState<{ x: number; y: number; id: string }>()
  const [restoring, setRestoring] = useState<string>()
  const [evologTarget, setEvologTarget] = useState<Commit>()
  const [evologSelected, setEvologSelected] = useState(0)
  const revsetRef = useRef<HTMLInputElement>(null)
  const detailRef = useRef<HTMLDivElement>(null)
  const { split, preview } = useDiffNavigation(detailRef)
  const operation = useOperation(path)
  const { agents } = useAgents()
  const approved = useApprovedCommits(path, operation)
  const reviews = useReviews(path)
  const { data, error, refetch } = useQuery(RepositoryService.method.log, { path, revset })
  const { data: bookmarkData, refetch: refetchBookmarks } = useQuery(RepositoryService.method.listBookmarks, { path })
  const bookmarks = bookmarkData?.bookmarks ?? []
  const { data: opData, refetch: refetchOperations } = useQuery(RepositoryService.method.listOperations, { path, limit: operationLimit }, { enabled: view === 'oplog' })
  const operations = opData?.operations ?? []
  const { data: evologData } = useQuery(
    RepositoryService.method.evolog,
    { path, revision: evologTarget?.commitId ?? '' },
    { enabled: view === 'evolog' && evologTarget !== undefined },
  )
  const evolog = evologData?.entries ?? []
  const { data: conflictData, refetch: refetchConflicts } = useQuery(RepositoryService.method.listConflicts, { path }, { enabled: view === 'conflicts' })
  const conflicts = useMemo(() => conflictEntries(conflictData?.commits ?? []), [conflictData])
  const restoreFiles = useCallback((r: FileRevision) => {
    setTreeTarget(r)
    setView('files')
  }, [])
  const closeFiles = useCallback(() => setView('log'), [])

  // showHistory shows the revisions which change the file in the log.
  const showHistory = useCallback((file: string) => {
    const r = fileRevset(file)
    setRevsetInput(r)
    setRevset(r)
    setSelectedKey(undefined)
    setView('log')
  }, [])
  const rows = data?.rows ?? []
  const commits = rows.map((r) => r.commit!)
  const foundIndex = commits.findIndex((c) => revisionKey(c) === selectedKey)
  const selected = foundIndex >= 0 ? foundIndex : 0
  const selectedCommit = commits[selected]
  const checkedCommits = commits.filter((c) => checked.has(c.commitId))
  const marks = new Map<string, string>()
  if (mode) {
    for (const c of mode.sources) {
      marks.set(c.commitId, mode.kind)
    }
    if (selectedCommit) {
      marks.set(selectedCommit.commitId, mode.kind === 'rebase' ? targetLabel[mode.target] : 'into')
    }
  }

  useEffect(() => {
    if (operation) {
      refetch()
      refetchBookmarks()
      if (view === 'oplog') {
        refetchOperations()
      }
      if (view === 'conflicts') {
        refetchConflicts()
      }
    }
  }, [operation, view, refetch, refetchBookmarks, refetchOperations, refetchConflicts])

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
    (cs: Commit[]) => run(() => repositoryClient.new({ path, revisions: cs.map((c) => c.commitId) }), true),
    [path, run],
  )
  const edit = useCallback((c: Commit) => run(() => repositoryClient.edit({ path, revision: c.commitId }), true), [path, run])
  const startDescribe = useCallback(
    async (c: Commit) => {
      const res = await repositoryClient.getRevision({ path, revision: c.commitId })
      setEditing({ kind: 'describe', commit: c, initial: res.description })
    },
    [path],
  )
  const startCommit = useCallback(async () => {
    const wc = commits.find((c) => c.currentWorkingCopy)
    const res = wc ? await repositoryClient.getRevision({ path, revision: wc.commitId }) : undefined
    setEditing({ kind: 'commit', initial: res?.description ?? '' })
  }, [commits, path])
  const undo = useCallback(() => run(() => repositoryClient.undo({ path }), false), [path, run])
  const navigate = useNavigate()
  // startReview starts the review of the revisions in order from the oldest because the log shows the newest first.
  const startReview = useCallback(
    async (cs: Commit[]) => {
      try {
        const res = await reviewClient.start({ path, changeIds: cs.map((c) => c.changeId).reverse() })
        navigate({ to: '/reviews/$reviewId', params: { reviewId: res.review!.id } })
      } catch (e) {
        setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
      }
    },
    [path, navigate],
  )

  const slide = useCallback(
    (direction: 'down' | 'up') => {
      const s = computeSlide(commits, selected, direction)
      if (!s.ok) {
        setMessage({ text: s.reason, error: true })
        return
      }
      const src = commits[selected]
      run(() => repositoryClient.rebase({ path, revisions: [src.commitId], source: RebaseSource.REVISIONS, destinations: [s.destination], target: s.target }), false)
    },
    [commits, selected, path, run],
  )

  const executeMode = useCallback(
    async (m: Mode) => {
      const destination = selectedCommit
      if (!destination) {
        return
      }
      setMode(undefined)
      const revisions = m.sources.map((c) => c.commitId)
      if (m.kind === 'rebase') {
        run(() => repositoryClient.rebase({ path, revisions, source: m.source, destinations: [destination.commitId], target: m.target }), false)
        return
      }
      let description = ''
      if (m.combine) {
        const descriptions = await Promise.all([destination, ...m.sources].map((c) => repositoryClient.getRevision({ path, revision: c.commitId })))
        description = descriptions
          .map((d) => d.description.trim())
          .filter((d) => d !== '')
          .join('\n\n')
      }
      run(() => repositoryClient.squash({ path, revisions, into: destination.commitId, description }), false)
    },
    [selectedCommit, path, run],
  )

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      // Cmd/Ctrl+K opens the command palette like lightjj.
      if ((e.metaKey || e.ctrlKey) && e.key === 'k' && !document.querySelector('[role="dialog"]')) {
        e.preventDefault()
        setPaletteOpen(true)
        return
      }
      if (ignoreKey(e)) {
        return
      }
      if (view === 'files') {
        return
      }
      if (view === 'oplog' || view === 'evolog' || view === 'conflicts') {
        const length = { oplog: operations.length, evolog: evolog.length, conflicts: conflicts.length }[view]
        const setIndex = { oplog: setOpSelected, evolog: setEvologSelected, conflicts: setConflictSelected }[view]
        switch (e.key) {
          case 'j':
            setIndex((i) => Math.min(i + 1, length - 1))
            break
          case 'k':
            setIndex((i) => Math.max(i - 1, 0))
            break
          case '1':
          case 'Escape':
            setView('log')
            break
          default:
            return
        }
        e.preventDefault()
        return
      }
      if (view === 'bookmarks') {
        const b = bookmarks[bookmarkSelected]
        switch (e.key) {
          case 'j':
            setBookmarkSelected((i) => Math.min(i + 1, bookmarks.length - 1))
            break
          case 'k':
            setBookmarkSelected((i) => Math.max(i - 1, 0))
            break
          case '1':
            setView('log')
            break
          case 'd':
            if (b && b.remote === '') {
              run(() => repositoryClient.deleteBookmark({ path, name: b.name }), false)
            }
            break
          case 'f':
            if (b && b.remote === '') {
              run(() => repositoryClient.forgetBookmark({ path, name: b.name }), false)
            }
            break
          case 't':
            if (b && b.remote !== '') {
              run(() => (b.tracked ? repositoryClient.untrackBookmark : repositoryClient.trackBookmark)({ path, name: b.name, remote: b.remote }), false)
            }
            break
          case 'u':
            undo()
            break
          default:
            return
        }
        e.preventDefault()
        return
      }
      if (mode) {
        switch (e.key) {
          case 'j':
            selectIndex(selected + 1)
            break
          case 'k':
            selectIndex(selected - 1)
            break
          case 'Enter':
            executeMode(mode)
            break
          case 'Escape':
            setMode(undefined)
            break
          case 'c':
            if (mode.kind === 'squash') {
              setMode({ ...mode, combine: !mode.combine })
            }
            break
          default:
            if (mode.kind === 'rebase' && modeKeys[e.key]) {
              setMode({ ...mode, ...modeKeys[e.key] })
              break
            }
            return
        }
        e.preventDefault()
        return
      }
      switch (e.key) {
        case '2':
          setView('bookmarks')
          break
        case 'F':
          if (selectedCommit) {
            setTreeTarget({ commitId: selectedCommit.commitId, changeId: selectedCommit.changeId })
            setView('files')
          }
          break
        case '3':
          setConflictSelected(0)
          setView('conflicts')
          break
        case '4':
        case 'O':
          setOpSelected(0)
          setView('oplog')
          break
        case '5':
        case 'E':
          if (selectedCommit) {
            setEvologTarget(selectedCommit)
            setEvologSelected(0)
            setView('evolog')
          }
          break
        case 'B':
          if (selectedCommit && checked.size === 0) {
            setSettingBookmark(selectedCommit)
          }
          break
        case 'f':
          run(() => repositoryClient.fetch({ path }), false)
          break
        case 'p':
          setPushing(true)
          break
        case 'R':
          if (targets().length > 0) {
            setMode({ kind: 'rebase', sources: targets(), source: RebaseSource.REVISIONS, target: RebaseTarget.ONTO })
          }
          break
        case 'S':
          if (targets().length > 0) {
            setMode({ kind: 'squash', sources: targets(), combine: false })
          }
          break
        case 'w':
          setWorkspaceOpen(true)
          break
        case 's':
          if (selectedCommit && !selectedCommit.immutable && checked.size === 0) {
            setSplitting(selectedCommit)
          }
          break
        case 'J':
          slide('down')
          break
        case 'K':
          slide('up')
          break
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
        case 'v':
          startReview(targets())
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
  }, [
    view,
    conflicts,
    operations,
    evolog,
    restoring,
    bookmarks,
    bookmarkSelected,
    settingBookmark,
    pushing,
    path,
    run,
    commits,
    selected,
    selectedCommit,
    checked,
    editing,
    abandoning,
    mode,
    selectIndex,
    refetch,
    newRevision,
    targets,
    startDescribe,
    startCommit,
    undo,
    slide,
    executeMode,
    startReview,
  ])

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
      {mode && (
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          {modeDescription(mode)}
        </Alert>
      )}
      {error && (
        <Alert severity="error" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
          {error.message}
        </Alert>
      )}
      <Stack direction="row" sx={{ flex: 1, minHeight: 0, display: view === 'files' ? 'none' : 'flex' }}>
        <Box sx={{ width: '45%', flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
          {view === 'bookmarks' && <BookmarksPanel bookmarks={bookmarks} selected={bookmarkSelected} onSelect={setBookmarkSelected} />}
          {view === 'oplog' && (
            <OperationsPanel
              operations={operations}
              selected={opSelected}
              onSelect={setOpSelected}
              onContextMenu={(i, x, y) => {
                setOpSelected(i)
                setOpMenu({ x, y, id: operations[i].id })
              }}
            />
          )}
          {view === 'evolog' && <EvologPanel entries={evolog} selected={evologSelected} onSelect={setEvologSelected} />}
          {view === 'conflicts' && <ConflictsPanel entries={conflicts} selected={conflictSelected} onSelect={setConflictSelected} />}
          {view === 'log' && (
            <RevisionGraph
              rows={rows}
              selected={selected}
              checked={checked}
              marks={marks}
              approved={approved}
              reviews={reviews}
              agents={agents ?? []}
              onSelect={selectIndex}
              onContextMenu={(i, x, y) => {
                selectIndex(i)
                setMenu({ x, y, commit: commits[i] })
              }}
            />
          )}
        </Box>
        <Box ref={detailRef} sx={{ flex: 1, minWidth: 0, overflow: 'auto' }}>
          {view === 'conflicts' && conflicts[conflictSelected] && (
            <MergeEditor
              key={`${conflicts[conflictSelected].commit.commitId}:${conflicts[conflictSelected].file.path}`}
              repoPath={path}
              commit={conflicts[conflictSelected].commit}
              file={conflicts[conflictSelected].file}
              onResolve={(side) => {
                const e = conflicts[conflictSelected]
                run(() => repositoryClient.resolve({ path, revision: e.commit.commitId, file: e.file.path, side }), false)
              }}
              onSave={(content) => {
                const e = conflicts[conflictSelected]
                run(async () => {
                  await repositoryClient.writeFile({ path, file: e.file.path, content: new TextEncoder().encode(content) })
                  return { output: `Saved ${e.file.path}` }
                }, false)
              }}
              onEdit={() => run(() => repositoryClient.edit({ path, revision: conflicts[conflictSelected].commit.commitId }), true)}
            />
          )}
          {view === 'evolog' && (
            <Box sx={{ p: 2 }}>
              {evolog[evologSelected]?.files.map((f) => (
                <FileDiffView
                  key={f.path}
                  file={f}
                  split={split}
                  preview={preview}
                  repoPath={path}
                  revision={evolog[evologSelected].commitId}
                  oldRevision={undefined}
                  onHistory={showHistory}
                />
              ))}
            </Box>
          )}
          {(view === 'log' || view === 'bookmarks' || view === 'oplog') && selectedCommit && (
            <RevisionDetail
              key={selectedCommit.commitId}
              path={path}
              commit={selectedCommit}
              split={split}
              preview={preview}
              reviews={reviews.get(selectedCommit.changeId)}
              onHistory={showHistory}
              onRestore={(commit, file) => setRestoringFile({ commit, file })}
              onResolveDivergence={setDivergent}
            />
          )}
        </Box>
      </Stack>
      {treeTarget && (
        <FileBrowser repoPath={path} revision={treeTarget} hidden={view !== 'files'} onRestore={restoreFiles} onClose={closeFiles} onHistory={showHistory} />
      )}

      <Menu open={opMenu !== undefined} onClose={() => setOpMenu(undefined)} anchorReference="anchorPosition" anchorPosition={opMenu ? { top: opMenu.y, left: opMenu.x } : undefined}>
        <MenuItem
          onClick={() => {
            const id = opMenu!.id
            setOpMenu(undefined)
            run(() => repositoryClient.revertOperation({ path, id }), false)
          }}
        >
          Revert this operation
        </MenuItem>
        <MenuItem
          onClick={() => {
            setRestoring(opMenu!.id)
            setOpMenu(undefined)
          }}
        >
          Restore to this operation…
        </MenuItem>
      </Menu>

      <Dialog open={restoring !== undefined} onClose={() => setRestoring(undefined)}>
        <DialogTitle>Restore the repository to {restoring}?</DialogTitle>
        <DialogContent>The operations after it are reverted. It can be undone by the operation log.</DialogContent>
        <DialogActions>
          <Button onClick={() => setRestoring(undefined)}>Cancel</Button>
          <Button
            color="warning"
            variant="contained"
            autoFocus
            onClick={() => {
              const id = restoring!
              setRestoring(undefined)
              run(() => repositoryClient.restoreOperation({ path, id }), true)
            }}
          >
            Restore
          </Button>
        </DialogActions>
      </Dialog>

      <Menu open={menu !== undefined} onClose={() => setMenu(undefined)} anchorReference="anchorPosition" anchorPosition={menu ? { top: menu.y, left: menu.x } : undefined}>
        {menu && [
          <MenuItem key="new" onClick={() => { setMenu(undefined); newRevision([menu.commit]) }}>New (n)</MenuItem>,
          <MenuItem key="edit" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); edit(menu.commit) }}>Edit</MenuItem>,
          <MenuItem key="describe" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); startDescribe(menu.commit) }}>Describe (e)</MenuItem>,
          <MenuItem key="abandon" disabled={menu.commit.immutable} onClick={() => { setMenu(undefined); setAbandoning(checkedCommits.length > 0 ? checkedCommits : [menu.commit]) }}>Abandon</MenuItem>,
          <MenuItem key="review" onClick={() => { setMenu(undefined); startReview(checkedCommits.length > 0 ? checkedCommits : [menu.commit]) }}>Review (v)</MenuItem>,
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
              run(() => repositoryClient.describe({ path, revision: e.commit.commitId, description }), false)
            } else {
              run(() => repositoryClient.commit({ path, description }), true)
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
              run(() => repositoryClient.abandon({ path, revisions: cs.map((c) => c.commitId) }), true)
            }}
          >
            Abandon
          </Button>
        </DialogActions>
      </Dialog>

      {settingBookmark && (
        <BookmarkDialog
          title={`Set the bookmark on ${settingBookmark.changeId.slice(0, 8)}`}
          names={bookmarks.filter((b) => b.remote === '').map((b) => b.name)}
          onClose={() => setSettingBookmark(undefined)}
          onSubmit={(name) => {
            const c = settingBookmark
            setSettingBookmark(undefined)
            const exists = bookmarks.some((b) => b.remote === '' && b.name === name)
            run(() => (exists ? repositoryClient.moveBookmark : repositoryClient.setBookmark)({ path, name, revision: c.commitId }), false)
          }}
        />
      )}

      <Dialog open={pushing} onClose={() => setPushing(false)}>
        <DialogTitle>Push to the remote?</DialogTitle>
        <DialogContent>
          The bookmarks of revsets.git-push are pushed.
          {bookmarks
            .filter((b) => b.remote !== '' && b.tracked && !b.synced)
            .map((b) => (
              <Box key={bookmarkLabel(b)} sx={{ fontFamily: 'monospace', fontSize: 13 }}>
                {bookmarkLabel(b)}: {b.behind} to push, {b.ahead} new on remote
              </Box>
            ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPushing(false)}>Cancel</Button>
          <Button
            variant="contained"
            autoFocus
            onClick={() => {
              setPushing(false)
              run(() => repositoryClient.push({ path }), false)
            }}
          >
            Push
          </Button>
        </DialogActions>
      </Dialog>

      {paletteOpen && (
        <CommandPalette
          onClose={() => setPaletteOpen(false)}
          actions={[
            ...(selectedCommit
              ? [
                  { label: 'New revision', shortcut: 'n', run: () => newRevision(targets()) },
                  { label: 'Edit the description', shortcut: 'e', run: () => startDescribe(selectedCommit) },
                  { label: 'Edit the revision (check out)', run: () => edit(selectedCommit) },
                  { label: 'Abandon the revision', run: () => setAbandoning(targets()) },
                  { label: 'Split the revision', shortcut: 's', run: () => setSplitting(selectedCommit) },
                  { label: 'Rebase', shortcut: 'R', run: () => setMode({ kind: 'rebase', sources: targets(), source: RebaseSource.REVISIONS, target: RebaseTarget.ONTO }) },
                  { label: 'Squash', shortcut: 'S', run: () => setMode({ kind: 'squash', sources: targets(), combine: false }) },
                  { label: 'Slide down (swap with the parent)', shortcut: 'J', run: () => slide('down') },
                  { label: 'Slide up (swap with the child)', shortcut: 'K', run: () => slide('up') },
                  { label: 'Set the bookmark', shortcut: 'B', run: () => setSettingBookmark(selectedCommit) },
                  { label: 'Show the evolog', shortcut: 'E', run: () => pressKey('E') },
                  { label: 'Browse the files', shortcut: 'F', run: () => pressKey('F') },
                  { label: 'Start the review of the revisions', shortcut: 'v', run: () => startReview(targets()) },
                ]
              : []),
            { label: 'Commit the working copy', shortcut: 'c', run: () => startCommit() },
            { label: 'Undo', shortcut: 'u', run: () => undo() },
            { label: 'Git fetch', shortcut: 'f', run: () => run(() => repositoryClient.fetch({ path }), false) },
            { label: 'Git push', shortcut: 'p', run: () => setPushing(true) },
            { label: 'Show the log', shortcut: '1', run: () => setView('log') },
            { label: 'Show the bookmarks', shortcut: '2', run: () => setView('bookmarks') },
            { label: 'Show the conflicts', shortcut: '3', run: () => setView('conflicts') },
            { label: 'Show the operation log', shortcut: '4', run: () => setView('oplog') },
            { label: 'Workspaces', shortcut: 'w', run: () => setWorkspaceOpen(true) },
            { label: 'Jump to the working copy', shortcut: '@', run: () => selectIndex(commits.findIndex((c) => c.currentWorkingCopy)) },
            { label: 'Refresh', shortcut: 'r', run: () => refetch() },
            { label: 'Focus the revset', shortcut: '/', run: () => setTimeout(() => revsetRef.current?.focus(), 0) },
            { label: 'Toggle the split view', shortcut: '|', run: () => pressKey('|') },
            { label: 'Toggle the preview of Markdown', shortcut: 'm', run: () => pressKey('m') },
            { label: 'Next hunk', shortcut: ']', run: () => pressKey(']') },
            { label: 'Previous hunk', shortcut: '[', run: () => pressKey('[') },
            { label: 'Toggle the theme', shortcut: 't', run: () => pressKey('t') },
            ...(aliasData?.aliases ?? []).map((a) => ({
              label: `Run alias: ${a.name} (jj ${a.args.join(' ')})`,
              run: () => run(() => repositoryClient.runAlias({ path, name: a.name }), false),
            })),
          ]}
        />
      )}

      {divergent && (
        <DivergenceDialog
          repoPath={path}
          commit={divergent}
          onClose={() => setDivergent(undefined)}
          onKeep={(keep, abandon) => {
            setDivergent(undefined)
            setSelectedKey(keep.changeId)
            run(() => repositoryClient.abandon({ path, revisions: abandon.map((c) => c.commitId) }), false)
          }}
        />
      )}

      {workspaceOpen && (
        <WorkspaceDialog
          repoPath={path}
          selected={selectedCommit}
          onClose={() => setWorkspaceOpen(false)}
          onOpen={async (root) => {
            setWorkspaceOpen(false)
            try {
              const res = await repositoryClient.open({ path: root })
              onOpenRepository(res.repository?.path ?? root)
            } catch (e) {
              setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
            }
          }}
          onAdd={(name) => {
            setWorkspaceOpen(false)
            run(async () => {
              const res = await repositoryClient.addWorkspace({ path, name, revision: selectedCommit!.commitId })
              const opened = await repositoryClient.open({ path: res.root })
              onOpenRepository(opened.repository?.path ?? res.root)
              return res
            }, false)
          }}
          onForget={(name) => {
            setWorkspaceOpen(false)
            run(() => repositoryClient.forgetWorkspace({ path, name }), false)
          }}
          onRename={(name) => {
            setWorkspaceOpen(false)
            run(() => repositoryClient.renameWorkspace({ path, name }), false)
          }}
          onUpdateStale={() => {
            setWorkspaceOpen(false)
            run(() => repositoryClient.updateStale({ path }), false)
          }}
        />
      )}

      {splitting && (
        <SplitDialog
          repoPath={path}
          commit={splitting}
          onClose={() => setSplitting(undefined)}
          onSubmit={(files, description) => {
            const c = splitting
            setSplitting(undefined)
            run(() => repositoryClient.split({ path, revision: c.commitId, description, files }), false)
          }}
        />
      )}

      <Dialog open={restoringFile !== undefined} onClose={() => setRestoringFile(undefined)}>
        <DialogTitle>Discard the changes of {restoringFile?.file}?</DialogTitle>
        <DialogContent>The file is restored to the content of the parent in {restoringFile?.commit.changeId.slice(0, 8)}.</DialogContent>
        <DialogActions>
          <Button onClick={() => setRestoringFile(undefined)}>Cancel</Button>
          <Button
            color="warning"
            variant="contained"
            autoFocus
            onClick={() => {
              const r = restoringFile!
              setRestoringFile(undefined)
              run(() => repositoryClient.restore({ path, revision: r.commit.commitId, files: [r.file] }), false)
            }}
          >
            Restore
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
