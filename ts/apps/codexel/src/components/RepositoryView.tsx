import { useCallback, useEffect, useRef, useState } from 'react'
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
  Divider,
  IconButton,
  Menu,
  MenuItem,
  Snackbar,
  Stack,
  Tab,
  Tabs,
  TextField,
  ToggleButton,
  Typography,
  ToggleButtonGroup,
} from '@mui/material'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import MoreHorizIcon from '@mui/icons-material/MoreHoriz'
import {
  RebaseSource,
  RebaseTarget,
  RepositoryService,
  ReviewService,
  type Bookmark,
  type Commit,
  type RepositoryWorkspace,
} from '../connect/codexel_pb.js'
import { useAgents } from '../hooks/useAgents.ts'
import { useApprovedCommits } from '../hooks/useApprovedCommits.ts'
import { useDiffNavigation } from '../hooks/useDiffNavigation.ts'
import { useOperation } from '../hooks/useOperation.ts'
import { usePullRequests } from '../hooks/usePullRequests.ts'
import { useReviews } from '../hooks/useReviews.ts'
import { fileLocation, fileRevset, type FileRevision } from '../lib/fileTree.ts'
import { ignoreKey, pressKey } from '../lib/keys.ts'
import { computeSlide } from '../lib/slide.ts'
import { transport } from '../providers/AppProvider.tsx'
import { BookmarkDialog } from './BookmarkDialog.tsx'
import { BookmarksPanel, bookmarkLabel } from './BookmarksPanel.tsx'
import { CommandPalette } from './CommandPalette.tsx'
import { DescriptionDialog } from './DescriptionDialog.tsx'
import { DiffControls } from './DiffControls.tsx'
import { DivergenceDialog } from './DivergenceDialog.tsx'
import { FileDiffView } from './FileDiffView.tsx'
import { FileBrowser, FileSidebar, type OpenRequest } from './FileTree.tsx'
import { EvologPanel, OperationsPanel } from './HistoryPanels.tsx'
import { PullRequestDialog } from './PullRequestDialog.tsx'
import { RevisionDetail } from './RevisionDetail.tsx'
import { RevisionGraph } from './RevisionGraph.tsx'
import { SplitDialog } from './SplitDialog.tsx'
import { WorkspaceNameDialog, WorkspacesPanel } from './WorkspacesPanel.tsx'

// revisionKey identifies the revision across the operations. The change ID doesn't change by rewriting the revision but
// it is ambiguous for the divergent revisions.
const revisionKey = (c: Commit) => (c.divergent || c.hidden ? c.commitId : c.changeId)

const repositoryClient = createClient(RepositoryService, transport)
const reviewClient = createClient(ReviewService, transport)

// operationLimit is the number of the operations shown in the operation log.
const operationLimit = 200

// sidebarKey is the key of the local storage which keeps whether the sidebar of the files is open.
const sidebarKey = 'codexel.files.sidebar'

type View = 'log' | 'bookmarks' | 'oplog' | 'evolog' | 'files' | 'workspaces'

type Editing ={ kind: 'describe'; commit: Commit; initial: string } | { kind: 'commit'; initial: string }

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

// rebaseSources and rebaseTargets are the choices of the rebase mode with the keys to choose them.
const rebaseSources: [string, RebaseSource][] = [
  ['r', RebaseSource.REVISIONS],
  ['s', RebaseSource.DESCENDANTS],
  ['b', RebaseSource.BRANCH],
]

const rebaseTargets: [string, RebaseTarget][] = [
  ['o', RebaseTarget.ONTO],
  ['a', RebaseTarget.INSERT_AFTER],
  ['i', RebaseTarget.INSERT_BEFORE],
]

const modeDescription = (mode: Mode): string =>
  mode.kind === 'rebase'
    ? `Rebase ${mode.sources.length} revision(s) [${sourceLabel[mode.source]}] ${targetLabel[mode.target]} the cursor. ` +
      'j/k or ↓/↑: move, r/s/b: source, o/a/i: onto/after/before, Enter: rebase, Esc: cancel'
    : `Squash ${mode.sources.length} revision(s) into the cursor [${mode.combine ? 'combine the descriptions' : 'keep the description of the destination'}]. ` +
      'j/k or ↓/↑: move, c: toggle the description, Enter: squash, Esc: cancel'

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
  const [view, setView] = useState<View>(() => (treeTarget ? 'files' : 'log'))
  const [splitting, setSplitting] = useState<Commit>()
  const [workspaceSelected, setWorkspaceSelected] = useState(0)
  const [addingWorkspace, setAddingWorkspace] = useState<Commit>()
  const [renamingWorkspace, setRenamingWorkspace] = useState(false)
  const [removingWorkspace, setRemovingWorkspace] = useState<RepositoryWorkspace>()
  const [divergent, setDivergent] = useState<Commit>()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [aliasAnchor, setAliasAnchor] = useState<HTMLElement>()
  const { data: aliasData } = useQuery(RepositoryService.method.listAliases, { path }, { enabled: paletteOpen || aliasAnchor !== undefined })
  const [restoringFile, setRestoringFile] = useState<{ commit: Commit; file: string }>()
  const [bookmarkSelected, setBookmarkSelected] = useState(0)
  const [settingBookmark, setSettingBookmark] = useState<Commit>()
  const [pushing, setPushing] = useState(false)
  const [submittingPR, setSubmittingPR] = useState<Commit[]>()
  // menu is the button which opens the menu of the operations on the selected revision.
  const [menu, setMenu] = useState<HTMLElement>()
  const [repoMenuAnchor, setRepoMenuAnchor] = useState<HTMLElement>()
  const [selecting, setSelecting] = useState(false)
  const [opSelected, setOpSelected] = useState(0)
  const [restoring, setRestoring] = useState<string>()
  const [evologTarget, setEvologTarget] = useState<Commit>()
  const [evologSelected, setEvologSelected] = useState(0)
  const revsetRef = useRef<HTMLInputElement>(null)
  const detailRef = useRef<HTMLDivElement>(null)
  const diffNavigation = useDiffNavigation(detailRef)
  const { split, preview } = diffNavigation
  const operation = useOperation(path)
  const { agents } = useAgents()
  const approved = useApprovedCommits(path, operation)
  const reviews = useReviews(path)
  const { pullRequests, refetch: refetchPullRequests } = usePullRequests(path)
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
  const { data: workspaceData, refetch: refetchWorkspaces } = useQuery(
    RepositoryService.method.listWorkspaces,
    { path },
    { enabled: view === 'workspaces' },
  )
  const workspaces = workspaceData?.workspaces ?? []
  const [sidebar, setSidebar] = useState(() => localStorage.getItem(sidebarKey) === 'true')
  const [fileRequest, setFileRequest] = useState<OpenRequest>()
  const { data: workingCopyData, refetch: refetchWorkingCopy } = useQuery(RepositoryService.method.log, { path, revset: '@' }, { enabled: sidebar })
  const workingCopy = workingCopyData?.rows[0]?.commit
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
      if (view === 'workspaces') {
        refetchWorkspaces()
      }
      if (sidebar) {
        refetchWorkingCopy()
      }
    }
  }, [operation, view, sidebar, refetch, refetchBookmarks, refetchOperations, refetchWorkspaces, refetchWorkingCopy])

  const selectIndex = useCallback((i: number) => {
    const c = commits[Math.max(0, Math.min(i, commits.length - 1))]
    if (c) {
      setSelectedKey(revisionKey(c))
    }
  }, [commits])

  const clearChecked = useCallback(() => {
    setChecked(new Set())
    setSelecting(false)
  }, [])

  // run runs the operation and shows the message of jj. selectWorkingCopy moves the selection to the new working copy.
  const run = useCallback(
    async (fn: () => Promise<{ output: string }>, selectWorkingCopy: boolean) => {
      try {
        const res = await fn()
        setMessage({ text: res.output, error: false })
        clearChecked()
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
    [refetch, clearChecked],
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

  const deleteBookmark = useCallback((b: Bookmark) => run(() => repositoryClient.deleteBookmark({ path, name: b.name }), false), [path, run])
  const forgetBookmark = useCallback((b: Bookmark) => run(() => repositoryClient.forgetBookmark({ path, name: b.name }), false), [path, run])
  const toggleTracking = useCallback(
    (b: Bookmark) => run(() => (b.tracked ? repositoryClient.untrackBookmark : repositoryClient.trackBookmark)({ path, name: b.name, remote: b.remote }), false),
    [path, run],
  )
  const revertOperation = useCallback((id: string) => run(() => repositoryClient.revertOperation({ path, id }), false), [path, run])
  const gitFetch = useCallback(() => run(() => repositoryClient.fetch({ path }), false), [path, run])
  const jumpToWorkingCopy = useCallback(() => {
    selectIndex(commits.findIndex((c) => c.currentWorkingCopy))
    setView('log')
  }, [commits, selectIndex])
  const copyChangeId = useCallback(async (c: Commit) => {
    try {
      await navigator.clipboard.writeText(c.changeId)
      setMessage({ text: `Copied ${c.changeId}`, error: false })
    } catch (e) {
      setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
    }
  }, [])
  const showView = useCallback((v: View) => {
    if (v === 'oplog') {
      setOpSelected(0)
    } else if (v === 'workspaces') {
      setWorkspaceSelected(0)
    }
    setView(v)
  }, [])
  const openWorkspace = useCallback(
    async (w: RepositoryWorkspace) => {
      try {
        const res = await repositoryClient.open({ path: w.root })
        onOpenRepository(res.repository?.path ?? w.root)
      } catch (e) {
        setMessage({ text: e instanceof Error ? e.message : String(e), error: true })
      }
    },
    [onOpenRepository],
  )
  const forgetWorkspace = useCallback(
    (w: RepositoryWorkspace) => run(() => repositoryClient.forgetWorkspace({ path, name: w.name }), false),
    [path, run],
  )
  const toggleChecked = useCallback((c: Commit) => {
    setChecked((s) => {
      const next = new Set(s)
      if (!next.delete(c.commitId)) {
        next.add(c.commitId)
      }
      return next
    })
  }, [])
  const startRebase = useCallback(() => {
    if (targets().length > 0) {
      setMode({ kind: 'rebase', sources: targets(), source: RebaseSource.DESCENDANTS, target: RebaseTarget.ONTO })
    }
  }, [targets])
  const startSquash = useCallback(() => {
    if (targets().length > 0) {
      setMode({ kind: 'squash', sources: targets(), combine: false })
    }
  }, [targets])
  const showEvolog = useCallback((c: Commit) => {
    setEvologTarget(c)
    setEvologSelected(0)
    setView('evolog')
  }, [])
  const browseFiles = useCallback((c: Commit) => {
    setTreeTarget({ commitId: c.commitId, changeId: c.changeId })
    setView('files')
  }, [])
  const toggleSidebar = useCallback(
    () =>
      setSidebar((s) => {
        localStorage.setItem(sidebarKey, String(!s))
        return !s
      }),
    [],
  )
  // openSidebarFile opens the file selected in the sidebar in the file browser of the working copy.
  const openSidebarFile = useCallback(
    (file: string) => {
      if (!workingCopy) {
        return
      }
      browseFiles(workingCopy)
      setFileRequest((r) => ({ file, seq: (r?.seq ?? 0) + 1 }))
    },
    [workingCopy, browseFiles],
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
      if (view === 'oplog' || view === 'evolog') {
        const length = { oplog: operations.length, evolog: evolog.length }[view]
        const setIndex = { oplog: setOpSelected, evolog: setEvologSelected }[view]
        switch (e.key) {
          case 'j':
          case 'ArrowDown':
            setIndex((i) => Math.min(i + 1, length - 1))
            break
          case 'k':
          case 'ArrowUp':
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
          case 'ArrowDown':
            setBookmarkSelected((i) => Math.min(i + 1, bookmarks.length - 1))
            break
          case 'k':
          case 'ArrowUp':
            setBookmarkSelected((i) => Math.max(i - 1, 0))
            break
          case '1':
            setView('log')
            break
          case 'd':
            if (b && b.remote === '') {
              deleteBookmark(b)
            }
            break
          case 'f':
            if (b && b.remote === '') {
              forgetBookmark(b)
            }
            break
          case 't':
            if (b && b.remote !== '') {
              toggleTracking(b)
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
      if (view === 'workspaces') {
        const w = workspaces[workspaceSelected]
        const other = w !== undefined && w.root !== path
        switch (e.key) {
          case 'j':
          case 'ArrowDown':
            setWorkspaceSelected((i) => Math.min(i + 1, workspaces.length - 1))
            break
          case 'k':
          case 'ArrowUp':
            setWorkspaceSelected((i) => Math.max(i - 1, 0))
            break
          case '1':
          case 'Escape':
            setView('log')
            break
          case 'o':
            if (other) {
              openWorkspace(w)
            }
            break
          case 'd':
            if (other) {
              setRemovingWorkspace(w)
            }
            break
          case 'f':
            if (other) {
              forgetWorkspace(w)
            }
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
          case 'ArrowDown':
            selectIndex(selected + 1)
            break
          case 'k':
          case 'ArrowUp':
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
            browseFiles(selectedCommit)
          }
          break
        case 'T':
          toggleSidebar()
          break
        case '4':
        case 'O':
          showView('oplog')
          break
        case '5':
        case 'E':
          if (selectedCommit) {
            showEvolog(selectedCommit)
          }
          break
        case 'B':
          if (selectedCommit && checked.size === 0) {
            setSettingBookmark(selectedCommit)
          }
          break
        case 'f':
          gitFetch()
          break
        case 'p':
          setPushing(true)
          break
        case 'P':
          if (targets().length > 0) {
            setSubmittingPR(targets())
          }
          break
        case 'R':
          startRebase()
          break
        case 'S':
          startSquash()
          break
        case 'w':
          showView('workspaces')
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
        case 'ArrowDown':
          selectIndex(selected + 1)
          break
        case 'k':
        case 'ArrowUp':
          selectIndex(selected - 1)
          break
        case '@':
          jumpToWorkingCopy()
          break
        case ' ':
          if (selectedCommit) {
            toggleChecked(selectedCommit)
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
        case 'y':
          if (selectedCommit) {
            copyChangeId(selectedCommit)
          }
          break
        case 'Escape':
          clearChecked()
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
    operations,
    evolog,
    restoring,
    bookmarks,
    bookmarkSelected,
    workspaces,
    workspaceSelected,
    openWorkspace,
    forgetWorkspace,
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
    toggleChecked,
    clearChecked,
    startRebase,
    startSquash,
    showEvolog,
    browseFiles,
    toggleSidebar,
    showView,
    gitFetch,
    jumpToWorkingCopy,
    copyChangeId,
    deleteBookmark,
    forgetBookmark,
    toggleTracking,
  ])

  const single = checked.size === 0
  const revisionMenuItem = (label: string, key: string | undefined, disabled: boolean, fn: () => void, color?: string) => (
    <MenuItem
      key={label}
      disabled={disabled}
      sx={{ color }}
      onClick={() => {
        setMenu(undefined)
        fn()
      }}
    >
      {key ? `${label} (${key})` : label}
    </MenuItem>
  )
  const repositoryMenuItem = (label: string, key: string, fn: () => void) => (
    <MenuItem
      onClick={() => {
        setRepoMenuAnchor(undefined)
        fn()
      }}
    >
      {`${label} (${key})`}
    </MenuItem>
  )
  const revisionMenuItems = (c: Commit) => [
    revisionMenuItem('New', 'n', false, () => newRevision(targets())),
    revisionMenuItem('Edit', undefined, c.immutable || !single, () => edit(c)),
    revisionMenuItem('Describe', 'e', c.immutable || !single, () => startDescribe(c)),
    revisionMenuItem('Split', 's', c.immutable || !single, () => setSplitting(c)),
    <Divider key="d1" />,
    revisionMenuItem('Rebase', 'R', false, startRebase),
    revisionMenuItem('Squash', 'S', false, startSquash),
    revisionMenuItem('Slide down', 'J', !single, () => slide('down')),
    revisionMenuItem('Slide up', 'K', !single, () => slide('up')),
    <Divider key="d2" />,
    revisionMenuItem('Set the bookmark', 'B', !single, () => setSettingBookmark(c)),
    revisionMenuItem('Add a workspace', undefined, !single, () => setAddingWorkspace(c)),
    revisionMenuItem('Show the evolog', 'E', false, () => showEvolog(c)),
    revisionMenuItem('Browse the files', 'F', false, () => browseFiles(c)),
    revisionMenuItem('Copy the change ID', 'y', false, () => copyChangeId(c)),
    <Divider key="d3" />,
    revisionMenuItem('Review', 'v', false, () => startReview(targets())),
    revisionMenuItem('Create the pull request', 'P', false, () => setSubmittingPR(targets())),
    <Divider key="d4" />,
    revisionMenuItem('Abandon', undefined, targets().some((t) => t.immutable), () => setAbandoning(targets()), 'error.main'),
  ]

  return (
    <Stack sx={{ height: '100%' }}>
      <Stack direction="row" spacing={1} sx={{ p: 1, alignItems: 'center' }}>
        <ToggleButton size="small" value="files" selected={sidebar} onChange={toggleSidebar} title="T" aria-label="Files of the working copy">
          <FolderOpenIcon fontSize="small" />
        </ToggleButton>
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
            }
          }}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />
        <IconButton size="small" aria-label="Repository operations" disabled={mode !== undefined} onClick={(e) => setRepoMenuAnchor(e.currentTarget)}>
          <MoreHorizIcon fontSize="small" />
        </IconButton>
      </Stack>
      {mode && (
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          {modeDescription(mode)}
          <Stack direction="row" sx={{ mt: 1, gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
            {mode.kind === 'rebase' ? (
              <>
                <ToggleButtonGroup size="small" exclusive value={mode.source} onChange={(_, v) => v !== null && setMode({ ...mode, source: v })}>
                  {rebaseSources.map(([key, source]) => (
                    <ToggleButton key={source} value={source} title={key} sx={{ textTransform: 'none', py: 0.25 }}>
                      {sourceLabel[source]}
                    </ToggleButton>
                  ))}
                </ToggleButtonGroup>
                <ToggleButtonGroup size="small" exclusive value={mode.target} onChange={(_, v) => v !== null && setMode({ ...mode, target: v })}>
                  {rebaseTargets.map(([key, target]) => (
                    <ToggleButton key={target} value={target} title={key} sx={{ textTransform: 'none', py: 0.25 }}>
                      {targetLabel[target]}
                    </ToggleButton>
                  ))}
                </ToggleButtonGroup>
              </>
            ) : (
              <ToggleButton
                size="small"
                value="combine"
                selected={mode.combine}
                onChange={() => setMode({ ...mode, combine: !mode.combine })}
                title="c"
                sx={{ textTransform: 'none', py: 0.25 }}
              >
                Combine the descriptions
              </ToggleButton>
            )}
            <Button size="small" variant="contained" color="warning" title="Enter" onClick={() => executeMode(mode)}>
              {mode.kind === 'rebase' ? 'Rebase' : 'Squash'}
            </Button>
            <Button size="small" color="inherit" title="Esc" onClick={() => setMode(undefined)}>
              Cancel
            </Button>
          </Stack>
        </Alert>
      )}
      {error && (
        <Alert severity="error" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
          {error.message}
        </Alert>
      )}
      <Stack direction="row" sx={{ flex: 1, minHeight: 0, display: view === 'files' ? 'none' : 'flex' }}>
        {sidebar && workingCopy && <FileSidebar repoPath={path} revision={workingCopy} onOpen={openSidebarFile} />}
        <Stack sx={{ width: '45%', flexShrink: 0, borderRight: 1, borderColor: 'divider' }}>
          <Stack direction="row" sx={{ alignItems: 'center', borderBottom: 1, borderColor: 'divider' }}>
            <Tabs
              value={view === 'log' || view === 'bookmarks' || view === 'oplog' || view === 'workspaces' ? view : false}
              onChange={(_, v) => showView(v)}
              variant="scrollable"
              sx={{ flex: 1, minWidth: 0, minHeight: 36 }}
            >
              <Tab value="log" label="Revisions" sx={{ minHeight: 36 }} />
              <Tab value="bookmarks" label="Bookmarks" sx={{ minHeight: 36 }} />
              <Tab value="oplog" label="Operations" sx={{ minHeight: 36 }} />
              <Tab value="workspaces" label="Workspaces" sx={{ minHeight: 36 }} />
            </Tabs>
            {view === 'log' && selecting && (
              <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
                {checked.size} checked
              </Typography>
            )}
            {view === 'log' && (
              <Button
                size="small"
                sx={{ flexShrink: 0, mx: 0.5, textTransform: 'none' }}
                onClick={() => (selecting ? clearChecked() : setSelecting(true))}
              >
                {selecting ? 'Cancel' : 'Select'}
              </Button>
            )}
          </Stack>
          <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
            {view === 'bookmarks' && (
              <BookmarksPanel
                bookmarks={bookmarks}
                selected={bookmarkSelected}
                onSelect={setBookmarkSelected}
                onDelete={deleteBookmark}
                onForget={forgetBookmark}
                onToggleTracking={toggleTracking}
              />
            )}
            {view === 'oplog' && (
              <OperationsPanel
                operations={operations}
                selected={opSelected}
                onSelect={setOpSelected}
                onRevert={(i) => revertOperation(operations[i].id)}
                onRestore={(i) => setRestoring(operations[i].id)}
              />
            )}
            {view === 'workspaces' && (
              <WorkspacesPanel
                repoPath={path}
                workspaces={workspaces}
                selected={workspaceSelected}
                onSelect={setWorkspaceSelected}
                onOpen={openWorkspace}
                onRemove={setRemovingWorkspace}
                onForget={forgetWorkspace}
                onRename={() => setRenamingWorkspace(true)}
                onUpdateStale={() => run(() => repositoryClient.updateStale({ path }), false)}
              />
            )}
            {view === 'evolog' && <EvologPanel entries={evolog} selected={evologSelected} onSelect={setEvologSelected} />}
            {view === 'log' && (
              <RevisionGraph
                rows={rows}
                selected={selected}
                checked={checked}
                marks={marks}
                approved={approved}
                reviews={reviews}
                pullRequests={pullRequests}
                agents={agents ?? []}
                selecting={selecting}
                onSelect={selectIndex}
                onToggleChecked={(i) => toggleChecked(commits[i])}
              />
            )}
          </Box>
        </Stack>
        <Stack sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" sx={{ minHeight: 36, px: 1, justifyContent: 'flex-end', alignItems: 'center', borderBottom: 1, borderColor: 'divider' }}>
            <DiffControls navigation={diffNavigation} />
          </Stack>
          <Box ref={detailRef} sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
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
                onMenu={view === 'log' && !mode ? setMenu : undefined}
              />
            )}
          </Box>
        </Stack>
      </Stack>
      {treeTarget && (
        <FileBrowser
          repoPath={path}
          revision={treeTarget}
          request={fileRequest}
          hidden={view !== 'files'}
          preview={preview}
          onTogglePreview={diffNavigation.togglePreview}
          onRestore={restoreFiles}
          onClose={closeFiles}
          onHistory={showHistory}
        />
      )}

      <Menu open={repoMenuAnchor !== undefined} anchorEl={repoMenuAnchor} onClose={() => setRepoMenuAnchor(undefined)}>
        {repositoryMenuItem('Commit', 'c', startCommit)}
        {repositoryMenuItem('Undo', 'u', undo)}
        <Divider />
        {repositoryMenuItem('Fetch', 'f', gitFetch)}
        {repositoryMenuItem('Push', 'p', () => setPushing(true))}
        <Divider />
        {repositoryMenuItem('Jump to the working copy', '@', jumpToWorkingCopy)}
        {repositoryMenuItem('Refresh', 'r', () => refetch())}
        <Divider />
        <MenuItem
          onClick={() => {
            setAliasAnchor(repoMenuAnchor)
            setRepoMenuAnchor(undefined)
          }}
        >
          Aliases…
        </MenuItem>
      </Menu>

      <Menu open={aliasAnchor !== undefined} anchorEl={aliasAnchor} onClose={() => setAliasAnchor(undefined)}>
        {aliasData?.aliases.length === 0 && <MenuItem disabled>No aliases</MenuItem>}
        {aliasData?.aliases.map((a) => (
          <MenuItem
            key={a.name}
            onClick={() => {
              setAliasAnchor(undefined)
              run(() => repositoryClient.runAlias({ path, name: a.name }), false)
            }}
          >
            {a.name} (jj {a.args.join(' ')})
          </MenuItem>
        ))}
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

      <Menu open={menu !== undefined && selectedCommit !== undefined} anchorEl={menu} onClose={() => setMenu(undefined)}>
        {menu && selectedCommit && revisionMenuItems(selectedCommit)}
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
                  { label: 'Create the pull request (jj pr submit)', shortcut: 'P', run: () => setSubmittingPR(targets()) },
                  { label: 'Copy the change ID', shortcut: 'y', run: () => copyChangeId(selectedCommit) },
                ]
              : []),
            { label: 'Commit the working copy', shortcut: 'c', run: () => startCommit() },
            { label: 'Undo', shortcut: 'u', run: () => undo() },
            { label: 'Git fetch', shortcut: 'f', run: gitFetch },
            { label: 'Git push', shortcut: 'p', run: () => setPushing(true) },
            { label: 'Show the log', shortcut: '1', run: () => setView('log') },
            { label: 'Show the bookmarks', shortcut: '2', run: () => setView('bookmarks') },
            { label: 'Show the operation log', shortcut: '4', run: () => setView('oplog') },
            { label: 'Show the workspaces', shortcut: 'w', run: () => showView('workspaces') },
            { label: 'Jump to the working copy', shortcut: '@', run: jumpToWorkingCopy },
            { label: 'Toggle the files of the working copy', shortcut: 'T', run: toggleSidebar },
            { label: 'Refresh', shortcut: 'r', run: () => refetch() },
            { label: 'Focus the revset', shortcut: '/', run: () => setTimeout(() => revsetRef.current?.focus(), 0) },
            { label: 'Toggle the split view', shortcut: '|', run: () => pressKey('|') },
            { label: 'Toggle the preview of Markdown', shortcut: 'm', run: () => pressKey('m') },
            { label: 'Next hunk', shortcut: ']', run: () => pressKey(']') },
            { label: 'Previous hunk', shortcut: '[', run: () => pressKey('[') },
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

      {addingWorkspace && (
        <WorkspaceNameDialog
          title={`Add a workspace on ${addingWorkspace.changeId.slice(0, 8)}`}
          submitLabel="Add"
          onClose={() => setAddingWorkspace(undefined)}
          onSubmit={(name) => {
            const revision = addingWorkspace.commitId
            setAddingWorkspace(undefined)
            run(async () => {
              const res = await repositoryClient.addWorkspace({ path, name, revision })
              const opened = await repositoryClient.open({ path: res.root })
              onOpenRepository(opened.repository?.path ?? res.root)
              return res
            }, false)
          }}
        />
      )}

      {renamingWorkspace && (
        <WorkspaceNameDialog
          title="Rename the current workspace"
          submitLabel="Rename"
          onClose={() => setRenamingWorkspace(false)}
          onSubmit={(name) => {
            setRenamingWorkspace(false)
            run(() => repositoryClient.renameWorkspace({ path, name }), false)
          }}
        />
      )}

      <Dialog open={removingWorkspace !== undefined} onClose={() => setRemovingWorkspace(undefined)}>
        <DialogTitle>Remove the workspace {removingWorkspace?.name}?</DialogTitle>
        <DialogContent>
          {removingWorkspace?.root} is deleted. The changes are snapshotted into a revision, but the ignored files are lost. Undo doesn't restore the directory.
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRemovingWorkspace(undefined)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            autoFocus
            onClick={() => {
              const w = removingWorkspace!
              setRemovingWorkspace(undefined)
              run(() => repositoryClient.removeWorkspace({ path, name: w.name }), false)
            }}
          >
            Remove
          </Button>
        </DialogActions>
      </Dialog>

      {submittingPR && (
        <PullRequestDialog
          repoPath={path}
          commits={submittingPR}
          onSubmitted={() => {
            clearChecked()
            refetch()
            refetchBookmarks()
            refetchPullRequests()
          }}
          onClose={() => setSubmittingPR(undefined)}
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
