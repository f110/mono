import { useEffect, useMemo, useRef } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Box, Button, Stack, TextField, Typography } from '@mui/material'
import { FileService } from '../connect/codexel_pb.js'
import type { TreeNode } from '../lib/fileTree.ts'
import { languageOf, tokenize } from '../lib/highlight.ts'

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
          j/k: move, h/l: fold/unfold, Enter: open, /: filter, 1 or Esc: log
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

// FileView shows the content of the file at the revision with the syntax highlighting.
export const FileView = ({ repoPath, revision, file, onHistory }: { repoPath: string; revision: string; file: string; onHistory: () => void }) => {
  const { data, error } = useQuery(FileService.method.getFile, { path: repoPath, revision, file }, { staleTime: Infinity })
  const lang = languageOf(file)
  const lines = useMemo(() => (data ? decoder.decode(data.content).split('\n') : []), [data])
  const binary = useMemo(() => data?.content.slice(0, 8000).includes(0) ?? false, [data])

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
