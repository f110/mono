import { useRef } from 'react'
import { Box, Chip, Stack, Typography } from '@mui/material'
import type { ConflictedFile, FileDiff } from '../connect/codexel_pb.js'
import { FileStatusChip } from './FileDiffView.tsx'
import { useScrollHistory } from '../hooks/useScrollHistory.ts'

// ChangedFiles lists the changed files with the numbers of the changed lines. Clicking the file scrolls to the element
// returned by findFile, and the browser back returns to the position before the scroll. The conflicted files are marked,
// and the conflicted files which aren't changed are listed after the changed files.
export const ChangedFiles = ({
  files,
  conflicts = [],
  findFile,
}: {
  files: FileDiff[]
  conflicts?: ConflictedFile[]
  findFile: (path: string) => Element | null | undefined
}) => {
  const ref = useRef<HTMLDivElement>(null)
  const jump = useScrollHistory(ref)
  const added = files.reduce((n, f) => n + f.added, 0)
  const removed = files.reduce((n, f) => n + f.removed, 0)
  const max = Math.max(1, ...files.map((f) => f.added + f.removed))
  const conflictOf = new Map(conflicts.map((c) => [c.path, c]))
  const changed = new Set(files.map((f) => f.path))
  const conflictOnly = conflicts.filter((c) => !changed.has(c.path))

  return (
    <Box ref={ref} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mb: 2 }}>
      <Stack direction="row" spacing={1} sx={{ px: 1, py: 0.5, borderBottom: 1, borderColor: 'divider', alignItems: 'center' }}>
        <Typography variant="body2">{files.length} file(s) changed</Typography>
        {conflicts.length > 0 && (
          <Typography variant="body2" sx={{ color: 'error.main' }}>
            · {conflicts.length} conflict(s)
          </Typography>
        )}
        <Box sx={{ flex: 1 }} />
        <Typography variant="body2" sx={{ color: 'success.main', fontFamily: 'monospace' }}>
          +{added}
        </Typography>
        <Typography variant="body2" sx={{ color: 'error.main', fontFamily: 'monospace' }}>
          -{removed}
        </Typography>
      </Stack>
      {files.map((f) => (
        <Stack
          key={f.path}
          direction="row"
          spacing={1}
          onClick={() => jump(findFile(f.path))}
          sx={{ px: 1, height: 24, alignItems: 'center', cursor: 'pointer', '&:hover': { bgcolor: 'action.hover' } }}
        >
          <Box sx={{ width: 28, flexShrink: 0 }}>
            <FileStatusChip status={f.status} />
          </Box>
          <Typography variant="body2" noWrap sx={{ fontFamily: 'monospace', fontSize: 12, flex: 1, minWidth: 0 }} title={f.path}>
            {f.oldPath ? `${f.oldPath} → ${f.path}` : f.path}
          </Typography>
          <ConflictChip file={conflictOf.get(f.path)} />
          <Typography variant="caption" sx={{ color: 'success.main', fontFamily: 'monospace', width: 48, textAlign: 'right' }}>
            {f.binary ? '' : `+${f.added}`}
          </Typography>
          <Typography variant="caption" sx={{ color: 'error.main', fontFamily: 'monospace', width: 48, textAlign: 'right' }}>
            {f.binary ? 'bin' : `-${f.removed}`}
          </Typography>
          <Stack direction="row" sx={{ width: 80, height: 8, flexShrink: 0 }}>
            <Box sx={{ width: `${(f.added / max) * 100}%`, bgcolor: 'success.main' }} />
            <Box sx={{ width: `${(f.removed / max) * 100}%`, bgcolor: 'error.main' }} />
          </Stack>
        </Stack>
      ))}
      {conflictOnly.map((c) => (
        <Stack key={c.path} direction="row" spacing={1} sx={{ px: 1, height: 24, alignItems: 'center' }}>
          <Box sx={{ width: 28, flexShrink: 0 }} />
          <Typography variant="body2" noWrap sx={{ fontFamily: 'monospace', fontSize: 12, flex: 1, minWidth: 0 }} title={c.path}>
            {c.path}
          </Typography>
          <ConflictChip file={c} />
          <Box sx={{ width: 192, flexShrink: 0 }} />
        </Stack>
      ))}
    </Box>
  )
}

// ConflictChip shows that the file is conflicted. The number of the sides is shown if it is more than two.
const ConflictChip = ({ file }: { file?: ConflictedFile }) =>
  file ? <Chip label={file.sides > 2 ? `conflict ${file.sides}-way` : 'conflict'} size="small" color="error" sx={{ height: 18 }} /> : null
