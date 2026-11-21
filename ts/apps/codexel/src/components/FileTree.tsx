import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { useRouter } from '@tanstack/react-router'
import { Box, Button, IconButton, Stack, Tab, Tabs, TextField, ToggleButton, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import CloseIcon from '@mui/icons-material/Close'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { RepositoryService } from '../connect/codexel_pb.js'
import { fileLocation, parentDirs, visibleNodes, withFileLocation, type FileRevision, type TreeNode } from '../lib/fileTree.ts'
import { languageOf, tokenize } from '../lib/highlight.ts'
import { FileIcon, FolderIcon } from './FileIcon.tsx'
import { ignoreKey, isInputSubmitKey } from '../lib/keys.ts'
import { isMarkdown } from './FileDiffView.tsx'
import { Markdown } from './Markdown.tsx'

// FileTreePanel shows the files of the revision. The keys are handled by the parent. filter shows the matching files
// flatly instead of the tree. help is the description of the keys shown under the filter.
export const FileTreePanel = ({
  nodes,
  selected,
  filter,
  filterRef,
  placeholder,
  help,
  onFilterChange,
  onSelect,
}: {
  nodes: TreeNode[]
  selected: number
  filter: string
  filterRef: RefObject<HTMLInputElement | null>
  placeholder: string
  help?: string
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
          placeholder={placeholder}
          value={filter}
          onChange={(e) => onFilterChange(e.target.value)}
          onKeyDown={(e) => {
            if (isInputSubmitKey(e.nativeEvent)) {
              filterRef.current?.blur()
            }
          }}
        />
        {help && (
          <Typography variant="caption" color="text.secondary">
            {help}
          </Typography>
        )}
      </Box>
      {nodes.map((n, i) => (
        <Box
          key={n.path}
          ref={i === selected ? selectedRef : undefined}
          onClick={() => onSelect(i)}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 0.5,
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
          {n.dir ? (
            <>
              {n.expanded ? <ExpandMoreIcon sx={{ fontSize: 16 }} /> : <ChevronRightIcon sx={{ fontSize: 16 }} />}
              <FolderIcon open={n.expanded} />
              {n.name}
            </>
          ) : (
            <>
              {!filter && <Box sx={{ width: 16, flexShrink: 0 }} />}
              <FileIcon path={n.path} />
              {filter ? n.path : n.name}
            </>
          )}
        </Box>
      ))}
    </Box>
  )
}

const decoder = new TextDecoder()

// FileView shows the content of the file at the revision with the syntax highlighting. Markdown is rendered while preview
// is on. scrollRef is scrolled to scrollTop after the content is loaded.
export const FileView = ({
  repoPath,
  revision,
  file,
  scrollRef,
  scrollTop,
  preview,
  onTogglePreview,
  onHistory,
}: {
  repoPath: string
  revision: string
  file: string
  scrollRef: RefObject<HTMLElement | null>
  scrollTop: number
  preview: boolean
  onTogglePreview: () => void
  onHistory: () => void
}) => {
  const { data, error } = useQuery(RepositoryService.method.getFile, { path: repoPath, revision, file }, { staleTime: Infinity })
  const lang = languageOf(file)
  const text = useMemo(() => (data ? decoder.decode(data.content) : undefined), [data])
  const lines = useMemo(() => text?.split('\n') ?? [], [text])
  const binary = useMemo(() => data?.content.slice(0, 8000).includes(0) ?? false, [data])
  const markdown = isMarkdown(file)
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
        {markdown && (
          <ToggleButton size="small" value="preview" selected={preview} onChange={onTogglePreview} title="m" sx={{ textTransform: 'none', py: 0.25 }}>
            Preview
          </ToggleButton>
        )}
        <Button size="small" onClick={onHistory}>
          History
        </Button>
      </Stack>
      {error && <Typography color="error">{error.message}</Typography>}
      {binary ? (
        <Typography color="text.secondary">Binary file</Typography>
      ) : markdown && preview ? (
        <Box className="markdown-body" sx={{ '& pre': { overflowX: 'auto' }, '& img': { maxWidth: '100%' } }}>
          {text !== undefined && <Markdown text={text} />}
        </Box>
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

// useFileTree returns the visible nodes of the file tree of the revision. filter shows the matching files flatly instead
// of the tree. toggle expands the directory if open is true, collapses it if false and flips it if undefined.
const useFileTree = (repoPath: string, commitId: string, initialExpanded: string[]) => {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(initialExpanded))
  const [filter, setFilter] = useState('')
  const { data } = useQuery(RepositoryService.method.listFiles, { path: repoPath, revision: commitId }, { staleTime: Infinity })
  const nodes = useMemo(() => {
    const files = data?.files ?? []
    if (filter) {
      const f = filter.toLowerCase()
      return files.filter((p) => p.toLowerCase().includes(f)).map((p) => ({ path: p, name: p, depth: 0, dir: false, expanded: false }))
    }
    return visibleNodes(files, expanded)
  }, [data, filter, expanded])
  const toggle = useCallback(
    (path: string, open?: boolean) =>
      setExpanded((s) => {
        const next = new Set(s)
        if (open ?? !next.has(path)) next.add(path)
        else next.delete(path)
        return next
      }),
    [],
  )
  const expand = useCallback((paths: string[]) => setExpanded((s) => new Set([...s, ...paths])), [])
  return { nodes, filter, setFilter, toggle, expand }
}

// ActiveFile is the file shown in the file browser. seq changes every time the file is shown to restore scrollTop.
type ActiveFile = { file: string; scrollTop: number; seq: number }

// OpenRequest asks the file browser to open the file. seq changes every time the file is requested.
export type OpenRequest = { file: string; seq: number }

// FileBrowser shows the file tree of the revision and the opened files in the tabs. Opening the file pushes the position
// to the history, so the browser back returns to the file and the scroll position opened before. onRestore is called
// with the revision of the position restored by the browser back and forward. The keys are handled only while it is
// not hidden, and the tabs and the preview of Markdown are kept while it is hidden. Markdown is previewed by default, and m
// toggles it to the source. request opens the file every time its seq changes.
export const FileBrowser = ({
  repoPath,
  revision,
  hidden,
  request,
  onRestore,
  onClose,
  onHistory,
}: {
  repoPath: string
  revision: FileRevision
  hidden: boolean
  request?: OpenRequest
  onRestore: (revision: FileRevision) => void
  onClose: () => void
  onHistory: (file: string) => void
}) => {
  const { history } = useRouter()
  const initial = useMemo(() => fileLocation(history.location.state, repoPath), [history, repoPath])
  const [tabs, setTabs] = useState<string[]>(() => (initial ? [initial.file] : []))
  const [active, setActive] = useState<ActiveFile | undefined>(() =>
    initial ? { file: initial.file, scrollTop: initial.scrollTop, seq: 0 } : undefined,
  )
  const [selectedPath, setSelectedPath] = useState<string | undefined>(initial?.file)
  const [preview, setPreview] = useState(true)
  const { nodes, filter, setFilter, toggle, expand } = useFileTree(repoPath, revision.commitId, initial ? parentDirs(initial.file) : [])
  const filterRef = useRef<HTMLInputElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const scrolls = useRef(new Map<string, number>())
  const seq = useRef(1)
  const requested = useRef<number>(undefined)
  const selected = Math.max(
    0,
    nodes.findIndex((n) => n.path === selectedPath),
  )

  // show shows the file in the tab and the tree, and scrolls to scrollTop.
  const show = useCallback(
    (file: string, scrollTop: number) => {
      setTabs((t) => (t.includes(file) ? t : [...t, file]))
      expand(parentDirs(file))
      setSelectedPath(file)
      setActive({ file, scrollTop, seq: seq.current++ })
    },
    [expand],
  )

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

  useEffect(() => {
    if (request && request.seq !== requested.current) {
      requested.current = request.seq
      open(request.file)
    }
  }, [request, open])

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
      const tab = active ? tabs.indexOf(active.file) : -1
      switch (e.key) {
        case 'j':
        case 'ArrowDown':
          setSelectedPath(nodes[Math.min(selected + 1, nodes.length - 1)]?.path)
          break
        case 'k':
        case 'ArrowUp':
          setSelectedPath(nodes[Math.max(selected - 1, 0)]?.path)
          break
        case 'l':
          if (node?.dir) toggle(node.path, true)
          break
        case 'h':
          if (node?.dir) toggle(node.path, false)
          break
        case 'Enter':
          if (node?.dir) toggle(node.path)
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
        case 'm':
          setPreview((p) => !p)
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
  }, [hidden, nodes, selected, toggle, tabs, active, open, close, onClose])

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
          placeholder="Filter (/ to focus)"
          help="j/k or ↓/↑: move, h/l: fold/unfold, Enter: open, H/L: previous/next tab, x: close the tab, m: preview, /: filter, 1 or Esc: log"
          onFilterChange={(f) => {
            setFilter(f)
            setSelectedPath(undefined)
          }}
          onSelect={(i) => {
            const n = nodes[i]
            setSelectedPath(n.path)
            if (n.dir) {
              toggle(n.path)
            } else {
              open(n.path)
            }
          }}
        />
      </Box>
      <Stack sx={{ flex: 1, minWidth: 0 }}>
        {tabs.length > 0 && (
          <Tabs
            value={active?.file ?? false}
            onChange={(_, v) => open(v)}
            variant="scrollable"
            sx={{ borderBottom: 1, borderColor: 'divider', minHeight: 40 }}
          >
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
              preview={preview}
              onTogglePreview={() => setPreview((p) => !p)}
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

// FileSidebar shows the file tree of the revision beside the log. onOpen is called with the file selected in the tree to
// open it in the file browser.
export const FileSidebar = ({ repoPath, revision, onOpen }: { repoPath: string; revision: FileRevision; onOpen: (file: string) => void }) => {
  const { nodes, filter, setFilter, toggle } = useFileTree(repoPath, revision.commitId, [])
  const filterRef = useRef<HTMLInputElement>(null)

  return (
    <Box sx={{ width: 280, flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
      <Typography variant="body2" color="text.secondary" sx={{ px: 1, pt: 1 }}>
        Files at{' '}
        <Box component="span" sx={{ fontFamily: 'monospace', fontWeight: 'bold' }}>
          {revision.changeId.slice(0, 8)}
        </Box>
      </Typography>
      <FileTreePanel
        nodes={nodes}
        selected={-1}
        filter={filter}
        filterRef={filterRef}
        placeholder="Filter"
        onFilterChange={setFilter}
        onSelect={(i) => {
          const n = nodes[i]
          if (n.dir) {
            toggle(n.path)
          } else {
            onOpen(n.path)
          }
        }}
      />
    </Box>
  )
}
