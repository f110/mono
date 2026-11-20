import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { useRouter } from '@tanstack/react-router'
import { Box, Button, IconButton, Stack, Tab, Tabs, TextField, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import CloseIcon from '@mui/icons-material/Close'
import { RepositoryService } from '../connect/codexel_pb.js'
import { fileLocation, parentDirs, visibleNodes, withFileLocation, type FileRevision, type TreeNode } from '../lib/fileTree.ts'
import { languageOf, tokenize } from '../lib/highlight.ts'
import { ignoreKey } from '../lib/keys.ts'

// FileTreePanel shows the files of the revision. The keys are handled by the parent. filter shows the matching files
// flatly instead of the tree.
export const FileTreePanel = ({
  nodes,
  selected,
  filter,
  filterRef,
  onFilterChange,
  onSelect,
}: {
  nodes: TreeNode[]
  selected: number
  filter: string
  filterRef: React.RefObject<HTMLInputElement | null>
  onFilterChange: (filter: string) => void
  onSelect: (index: number) => void
}) => {
  const selectedRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  return (
    <Box sx={{ py: 0.5 }}>
      <Box sx={{ px: 1 }}>
        <TextField
          inputRef={filterRef}
          size="small"
          fullWidth
          placeholder="Filter (/ to focus)"
          value={filter}
          onChange={(e) => onFilterChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape' || e.key === 'Enter') {
              filterRef.current?.blur()
            }
          }}
        />
        <Typography variant="caption" color="text.secondary">
          j/k: move, h/l: fold/unfold, Enter: open, H/L: previous/next tab, x: close the tab, /: filter, 1 or Esc: log
        </Typography>
      </Box>
      {nodes.map((n, i) => (
        <Box
          key={n.path}
          ref={i === selected ? selectedRef : undefined}
          onClick={() => onSelect(i)}
          sx={{
            pl: 1 + n.depth * 2,
            fontFamily: 'monospace',
            fontSize: 13,
            lineHeight: '24px',
            cursor: 'pointer',
            whiteSpace: 'nowrap',
            bgcolor: i === selected ? 'action.selected' : undefined,
            color: n.dir ? 'text.secondary' : 'text.primary',
          }}
        >
          {n.dir ? `${n.name}/` : filter ? n.path : n.name}
        </Box>
      ))}
    </Box>
  )
}

const decoder = new TextDecoder()

// FileView shows the content of the file at the revision with the syntax highlighting. scrollRef is scrolled to
// scrollTop after the content is loaded.
export const FileView = ({
  repoPath,
  revision,
  file,
  scrollRef,
  scrollTop,
  onHistory,
}: {
  repoPath: string
  revision: string
  file: string
  scrollRef: React.RefObject<HTMLElement | null>
  scrollTop: number
  onHistory: () => void
}) => {
  const { data, error } = useQuery(RepositoryService.method.getFile, { path: repoPath, revision, file }, { staleTime: Infinity })
  const lang = languageOf(file)
  const lines = useMemo(() => (data ? decoder.decode(data.content).split('\n') : []), [data])
  const binary = useMemo(() => data?.content.slice(0, 8000).includes(0) ?? false, [data])
  useLayoutEffect(() => {
    if (data && scrollRef.current) {
      scrollRef.current.scrollTop = scrollTop
    }
  }, [data, scrollRef, scrollTop])

  return (
    <Box sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1 }}>
        <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1 }}>
          {file}
        </Typography>
        <Button size="small" onClick={onHistory}>
          History
        </Button>
      </Stack>
      {error && <Typography color="error">{error.message}</Typography>}
      {binary ? (
        <Typography color="text.secondary">Binary file</Typography>
      ) : (
        <Box sx={{ fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5, border: 1, borderColor: 'divider', borderRadius: 1, overflowX: 'auto' }}>
          {lines.map((l, i) => (
            <Stack key={i} direction="row">
              <Box component="span" sx={{ width: 48, flexShrink: 0, textAlign: 'right', pr: 1, color: 'text.disabled', userSelect: 'none' }}>
                {i + 1}
              </Box>
              <Box component="span" sx={{ whiteSpace: 'pre' }}>
                {tokenize(l, lang).map((t, j) => (
                  <span key={j} className={t.className}>
                    {t.text}
                  </span>
                ))}
              </Box>
            </Stack>
          ))}
        </Box>
      )}
    </Box>
  )
}

// ActiveFile is the file shown in the file browser. seq changes every time the file is shown to restore scrollTop.
type ActiveFile = { file: string; scrollTop: number; seq: number }

// FileBrowser shows the file tree of the revision and the opened files in the tabs. Opening the file pushes the position
// to the history, so the browser back returns to the file and the scroll position opened before. onRestore is called
// with the revision of the position restored by the browser back and forward. The keys are handled only while it is
// not hidden, and the tabs are kept while it is hidden.
export const FileBrowser = ({
  repoPath,
  revision,
  hidden,
  onRestore,
  onClose,
  onHistory,
}: {
  repoPath: string
  revision: FileRevision
  hidden: boolean
  onRestore: (revision: FileRevision) => void
  onClose: () => void
  onHistory: (file: string) => void
}) => {
  const { history } = useRouter()
  const initial = useMemo(() => fileLocation(history.location.state, repoPath), [history, repoPath])
  const [tabs, setTabs] = useState<string[]>(() => (initial ? [initial.file] : []))
  const [active, setActive] = useState<ActiveFile | undefined>(() => (initial ? { file: initial.file, scrollTop: initial.scrollTop, seq: 0 } : undefined))
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(initial ? parentDirs(initial.file) : []))
  const [selectedPath, setSelectedPath] = useState<string | undefined>(initial?.file)
  const [filter, setFilter] = useState('')
  const filterRef = useRef<HTMLInputElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const scrolls = useRef(new Map<string, number>())
  const seq = useRef(1)
  const { data } = useQuery(RepositoryService.method.listFiles, { path: repoPath, revision: revision.commitId }, { staleTime: Infinity })
  const nodes = useMemo(() => {
    const files = data?.files ?? []
    if (filter) {
      const f = filter.toLowerCase()
      return files.filter((p) => p.toLowerCase().includes(f)).map((p) => ({ path: p, name: p, depth: 0, dir: false }))
    }
    return visibleNodes(files, expanded)
  }, [data, filter, expanded])
  const selected = Math.max(0, nodes.findIndex((n) => n.path === selectedPath))

  // show shows the file in the tab and the tree, and scrolls to scrollTop.
  const show = useCallback((file: string, scrollTop: number) => {
    setTabs((t) => (t.includes(file) ? t : [...t, file]))
    setExpanded((s) => new Set([...s, ...parentDirs(file)]))
    setSelectedPath(file)
    setActive({ file, scrollTop, seq: seq.current++ })
  }, [])

  // open shows the file and pushes the position to the history. The position of the file shown before is kept in the
  // current entry of the history.
  const open = useCallback(
    (file: string) => {
      if (active?.file === file) {
        return
      }
      const scrollTop = scrolls.current.get(file) ?? 0
      const { href, state } = history.location
      if (active) {
        history.replace(href, withFileLocation(state, { repoPath, revision, file: active.file, scrollTop: scrolls.current.get(active.file) ?? 0 }))
        history.flush()
        history.push(href, withFileLocation({}, { repoPath, revision, file, scrollTop }))
      } else {
        history.replace(href, withFileLocation(state, { repoPath, revision, file, scrollTop }))
      }
      show(file, scrollTop)
    },
    [active, history, repoPath, revision, show],
  )

  const close = useCallback(
    (file: string) => {
      const rest = tabs.filter((t) => t !== file)
      setTabs(rest)
      if (active?.file !== file) {
        return
      }
      const next = rest[Math.min(tabs.indexOf(file), rest.length - 1)]
      if (next) {
        open(next)
      } else {
        setActive(undefined)
      }
    },
    [tabs, active, open],
  )

  useEffect(
    () =>
      history.subscribe(({ location, action }) => {
        if (action.type === 'PUSH' || action.type === 'REPLACE') {
          return
        }
        const l = fileLocation(location.state, repoPath)
        if (l) {
          onRestore(l.revision)
          show(l.file, l.scrollTop)
        }
      }),
    [history, repoPath, onRestore, show],
  )

  useEffect(() => {
    if (!hidden) {
      setActive((a) => a && { ...a, scrollTop: scrolls.current.get(a.file) ?? a.scrollTop, seq: seq.current++ })
    }
  }, [hidden])

  useEffect(() => {
    if (hidden) {
      return
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (ignoreKey(e)) {
        return
      }
      const node = nodes[selected]
      const toggle = (path: string, open: boolean) =>
        setExpanded((s) => {
          const next = new Set(s)
          if (open) next.add(path)
          else next.delete(path)
          return next
        })
      const tab = active ? tabs.indexOf(active.file) : -1
      switch (e.key) {
        case 'j':
          setSelectedPath(nodes[Math.min(selected + 1, nodes.length - 1)]?.path)
          break
        case 'k':
          setSelectedPath(nodes[Math.max(selected - 1, 0)]?.path)
          break
        case 'l':
          if (node?.dir) toggle(node.path, true)
          break
        case 'h':
          if (node?.dir) toggle(node.path, false)
          break
        case 'Enter':
          if (node?.dir) toggle(node.path, !expanded.has(node.path))
          else if (node) open(node.path)
          break
        case 'H':
          if (tab > 0) open(tabs[tab - 1])
          break
        case 'L':
          if (tab >= 0 && tab < tabs.length - 1) open(tabs[tab + 1])
          break
        case 'x':
          if (active) close(active.file)
          break
        case '/':
          filterRef.current?.focus()
          break
        case '1':
        case 'Escape':
          onClose()
          break
        default:
          return
      }
      e.preventDefault()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [hidden, nodes, selected, expanded, tabs, active, open, close, onClose])

  return (
    <Stack direction="row" sx={{ flex: 1, minHeight: 0, display: hidden ? 'none' : 'flex' }}>
      <Box sx={{ width: '30%', flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', px: 1, pt: 1 }}>
          <Button size="small" title="1 / Esc" startIcon={<ArrowBackIcon />} onClick={onClose}>
            Log
          </Button>
          <Typography variant="body2" color="text.secondary">
            Files at{' '}
            <Box component="span" sx={{ fontFamily: 'monospace', fontWeight: 'bold' }}>
              {revision.changeId.slice(0, 8)}
            </Box>
          </Typography>
        </Stack>
        <FileTreePanel
          nodes={nodes}
          selected={selected}
          filter={filter}
          filterRef={filterRef}
          onFilterChange={(f) => {
            setFilter(f)
            setSelectedPath(undefined)
          }}
          onSelect={(i) => {
            const n = nodes[i]
            setSelectedPath(n.path)
            if (n.dir) {
              setExpanded((s) => {
                const next = new Set(s)
                if (!next.delete(n.path)) next.add(n.path)
                return next
              })
            } else {
              open(n.path)
            }
          }}
        />
      </Box>
      <Stack sx={{ flex: 1, minWidth: 0 }}>
        {tabs.length > 0 && (
          <Tabs value={active?.file ?? false} onChange={(_, v) => open(v)} variant="scrollable" sx={{ borderBottom: 1, borderColor: 'divider', minHeight: 40 }}>
            {tabs.map((t) => (
              <Tab
                key={t}
                value={t}
                title={t}
                sx={{ minHeight: 40, textTransform: 'none', fontFamily: 'monospace' }}
                label={
                  <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                    <span>{t.split('/').pop()}</span>
                    <IconButton
                      component="span"
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation()
                        close(t)
                      }}
                    >
                      <CloseIcon fontSize="inherit" />
                    </IconButton>
                  </Stack>
                }
              />
            ))}
          </Tabs>
        )}
        <Box
          ref={contentRef}
          onScroll={(e) => {
            if (active && !hidden) {
              scrolls.current.set(active.file, e.currentTarget.scrollTop)
            }
          }}
          sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}
        >
          {active ? (
            <FileView
              key={`${revision.commitId}:${active.file}:${active.seq}`}
              repoPath={repoPath}
              revision={revision.commitId}
              file={active.file}
              scrollRef={contentRef}
              scrollTop={active.scrollTop}
              onHistory={() => onHistory(active.file)}
            />
          ) : (
            <Typography color="text.secondary" sx={{ p: 2 }}>
              Select the file from the tree
            </Typography>
          )}
        </Box>
      </Stack>
    </Stack>
  )
}
