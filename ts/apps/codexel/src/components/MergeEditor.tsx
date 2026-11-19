import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Button, ButtonGroup, Stack, TextField, Typography } from '@mui/material'
import { RepositoryService, ResolveSide, type ConflictedCommit, type ConflictedFile } from '../connect/codexel_pb.js'
import { composeResolution, parseConflict } from '../lib/conflict.ts'

const sideNames = ['A', 'B', 'C', 'D', 'E', 'F']

// MergeEditor resolves the conflicted file. The conflict of two sides can be resolved by taking one side in any
// revision. Editing the result is possible only in the working copy because it writes the file.
export const MergeEditor = ({
  repoPath,
  commit,
  file,
  onResolve,
  onSave,
  onEdit,
}: {
  repoPath: string
  commit: ConflictedCommit
  file: ConflictedFile
  onResolve: (side: ResolveSide) => void
  onSave: (content: string) => void
  onEdit: () => void
}) => {
  const { data, error } = useQuery(RepositoryService.method.getConflict, { path: repoPath, revision: commit.commitId, file: file.path })
  const segments = useMemo(() => (data ? parseConflict(data.content) : []), [data])
  const conflicts = segments.filter((s) => s.kind === 'conflict')
  const [resolutions, setResolutions] = useState<string[]>([])

  useEffect(() => {
    setResolutions(conflicts.map(() => ''))
    // The resolutions are reset only when the file is loaded.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [segments])

  const take = (index: number, text: string) => setResolutions((r) => r.map((v, i) => (i === index ? text : v)))

  return (
    <Box sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1 }}>
        <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1 }}>
          {file.path}
        </Typography>
        {file.sides === 2 && (
          <ButtonGroup size="small">
            <Button onClick={() => onResolve(ResolveSide.OURS)}>Keep A</Button>
            <Button onClick={() => onResolve(ResolveSide.THEIRS)}>Keep B</Button>
          </ButtonGroup>
        )}
      </Stack>
      {error && <Alert severity="error">{error.message}</Alert>}
      {!commit.workingCopy && (
        <Alert
          severity="info"
          sx={{ mb: 1 }}
          action={
            <Button size="small" onClick={onEdit}>
              Edit this revision
            </Button>
          }
        >
          The result can be edited only in the working copy.
        </Alert>
      )}
      {conflicts.map((c, i) => (
        <Box key={i} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mb: 2 }}>
          <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>
            Conflict {i + 1} of {conflicts.length}
          </Typography>
          <Stack direction="row" spacing={1} sx={{ p: 1 }}>
            {c.kind === 'conflict' &&
              c.sides.map((s, j) => (
                <Box key={j} sx={{ flex: 1, minWidth: 0 }}>
                  <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                    <Typography variant="caption" noWrap sx={{ flex: 1 }} title={s.label}>
                      [{sideNames[j]}] {s.label}
                    </Typography>
                    <Button size="small" disabled={!commit.workingCopy} onClick={() => take(i, s.text)}>
                      Take
                    </Button>
                  </Stack>
                  <Box component="pre" sx={{ m: 0, p: 1, bgcolor: 'action.hover', fontSize: 12, overflowX: 'auto', minHeight: 24 }}>
                    {s.text}
                  </Box>
                </Box>
              ))}
          </Stack>
          {c.kind === 'conflict' && (
            <Stack direction="row" spacing={1} sx={{ px: 1 }}>
              <Button size="small" disabled={!commit.workingCopy} onClick={() => take(i, c.sides.map((s) => s.text).join('\n'))}>
                Take all
              </Button>
            </Stack>
          )}
          <TextField
            multiline
            fullWidth
            minRows={2}
            disabled={!commit.workingCopy}
            placeholder="Result"
            value={resolutions[i] ?? ''}
            onChange={(e) => take(i, e.target.value)}
            slotProps={{ input: { sx: { fontFamily: 'monospace', fontSize: 12 } } }}
            sx={{ p: 1 }}
          />
        </Box>
      ))}
      {commit.workingCopy && conflicts.length > 0 && (
        <Button variant="contained" onClick={() => onSave(composeResolution(segments, resolutions))}>
          Save the result to the working copy
        </Button>
      )}
    </Box>
  )
}
