import { Box, Stack, Typography } from '@mui/material'
import type { FileDiff } from '../connect/codexel_pb.js'
import { FileStatusChip } from './FileDiffView.tsx'

// ChangedFiles lists the changed files with the numbers of the changed lines. onSelect is called with the path of the
// clicked file.
export const ChangedFiles = ({ files, onSelect }: { files: FileDiff[]; onSelect: (path: string) => void }) => {
  const added = files.reduce((n, f) => n + f.added, 0)
  const removed = files.reduce((n, f) => n + f.removed, 0)
  const max = Math.max(1, ...files.map((f) => f.added + f.removed))

  return (
    <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mb: 2 }}>
      <Stack direction="row" spacing={1} sx={{ px: 1, py: 0.5, borderBottom: 1, borderColor: 'divider', alignItems: 'center' }}>
        <Typography variant="body2" sx={{ flex: 1 }}>
          {files.length} file(s) changed
        </Typography>
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
          onClick={() => onSelect(f.path)}
          sx={{ px: 1, height: 24, alignItems: 'center', cursor: 'pointer', '&:hover': { bgcolor: 'action.hover' } }}
        >
          <FileStatusChip status={f.status} />
          <Typography variant="body2" noWrap sx={{ fontFamily: 'monospace', fontSize: 12, flex: 1, minWidth: 0 }} title={f.path}>
            {f.oldPath ? `${f.oldPath} → ${f.path}` : f.path}
          </Typography>
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
    </Box>
  )
}
