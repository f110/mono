import { Box, Chip, Stack, Typography } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import { DiffLineKind, FileStatus, type FileDiff } from '../connect/codexel_pb.js'

const statusLabel: Record<FileStatus, string> = {
  [FileStatus.UNSPECIFIED]: '',
  [FileStatus.MODIFIED]: 'M',
  [FileStatus.ADDED]: 'A',
  [FileStatus.DELETED]: 'D',
  [FileStatus.RENAMED]: 'R',
  [FileStatus.COPIED]: 'C',
}

export const FileStatusChip = ({ status }: { status: FileStatus }) => (
  <Chip
    label={statusLabel[status]}
    size="small"
    variant="outlined"
    color={status === FileStatus.ADDED ? 'success' : status === FileStatus.DELETED ? 'error' : 'default'}
    sx={{ height: 18, minWidth: 24, fontFamily: 'monospace' }}
  />
)

const LineNumber = ({ n }: { n: number }) => (
  <Box component="span" sx={{ width: 48, flexShrink: 0, textAlign: 'right', pr: 1, color: 'text.disabled', userSelect: 'none' }}>
    {n > 0 ? n : ''}
  </Box>
)

// FileDiffView shows the diff of the file in the unified format.
export const FileDiffView = ({ file }: { file: FileDiff }) => {
  const theme = useTheme()
  const background: Record<DiffLineKind, string | undefined> = {
    [DiffLineKind.UNSPECIFIED]: undefined,
    [DiffLineKind.CONTEXT]: undefined,
    [DiffLineKind.ADDED]: alpha(theme.palette.success.main, 0.15),
    [DiffLineKind.REMOVED]: alpha(theme.palette.error.main, 0.15),
  }
  const marker: Record<DiffLineKind, string> = {
    [DiffLineKind.UNSPECIFIED]: ' ',
    [DiffLineKind.CONTEXT]: ' ',
    [DiffLineKind.ADDED]: '+',
    [DiffLineKind.REMOVED]: '-',
  }

  return (
    <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mb: 2 }}>
      <Stack direction="row" spacing={1} sx={{ p: 1, alignItems: 'center', bgcolor: 'action.hover', position: 'sticky', top: 0, zIndex: 1 }}>
        <FileStatusChip status={file.status} />
        <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1, minWidth: 0 }} noWrap title={file.path}>
          {file.oldPath ? `${file.oldPath} → ${file.path}` : file.path}
        </Typography>
        <Typography variant="caption" sx={{ color: 'success.main' }}>
          +{file.added}
        </Typography>
        <Typography variant="caption" sx={{ color: 'error.main' }}>
          -{file.removed}
        </Typography>
      </Stack>
      {file.binary && (
        <Typography variant="body2" color="text.secondary" sx={{ p: 1 }}>
          Binary file
        </Typography>
      )}
      <Box sx={{ fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5, overflowX: 'auto' }}>
        {file.hunks.map((h, i) => (
          <Box key={i}>
            <Box sx={{ px: 1, color: 'text.secondary', bgcolor: alpha(theme.palette.info.main, 0.08) }}>
              @@ -{h.oldStart},{h.oldLines} +{h.newStart},{h.newLines} @@ {h.section}
            </Box>
            {h.lines.map((l, j) => (
              <Stack key={j} direction="row" sx={{ bgcolor: background[l.kind], whiteSpace: 'pre' }}>
                <LineNumber n={l.oldLine} />
                <LineNumber n={l.newLine} />
                <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.secondary', userSelect: 'none' }}>
                  {marker[l.kind]}
                </Box>
                <Box component="span">
                  {l.text}
                  {l.noNewline && (
                    <Box component="span" sx={{ color: 'text.disabled' }} title="No newline at end of file">
                      {' ⏎̸'}
                    </Box>
                  )}
                </Box>
              </Stack>
            ))}
          </Box>
        ))}
      </Box>
    </Box>
  )
}
