import { useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Radio, Stack, Typography } from '@mui/material'
import { RepositoryService, type Commit } from '../connect/codexel_pb.js'
import { FileDiffView } from './FileDiffView.tsx'

// DivergenceDialog resolves the divergent change. The versions of the change are compared and one of them is kept by
// abandoning the others.
export const DivergenceDialog = ({
  repoPath,
  commit,
  onKeep,
  onClose,
}: {
  repoPath: string
  commit: Commit
  onKeep: (keep: Commit, abandon: Commit[]) => void
  onClose: () => void
}) => {
  const { data } = useQuery(RepositoryService.method.log, { path: repoPath, revset: `change_id(${commit.changeId})` })
  const versions = (data?.rows ?? []).map((r) => r.commit!)
  const [from, setFrom] = useState(0)
  const [to, setTo] = useState(1)
  const diff = useQuery(
    RepositoryService.method.diffRange,
    { path: repoPath, from: versions[from]?.commitId ?? '', to: versions[to]?.commitId ?? '' },
    { enabled: versions.length > 1 && from !== to },
  )

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="lg">
      <DialogTitle>Divergent change {commit.changeId.slice(0, 8)}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          Compare the versions (from / to) and keep one. The other versions are abandoned.
        </Typography>
        {versions.map((v, i) => (
          <Stack key={v.commitId} direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <Radio size="small" checked={from === i} onChange={() => setFrom(i)} title="from" />
            <Radio size="small" checked={to === i} onChange={() => setTo(i)} title="to" color="success" />
            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
              {v.commitId.slice(0, 12)}
            </Typography>
            <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
              {v.description || '(no description set)'} · {v.author}
            </Typography>
            <Button
              size="small"
              onClick={() =>
                onKeep(
                  v,
                  versions.filter((o) => o.commitId !== v.commitId),
                )
              }
            >
              Keep this
            </Button>
          </Stack>
        ))}
        <Box sx={{ mt: 2 }}>
          {diff.data?.files.length === 0 && <Typography color="text.secondary">No difference</Typography>}
          {diff.data?.files.map((f) => (
            <FileDiffView
              key={f.path}
              file={f}
              split={false}
              preview={false}
              repoPath={repoPath}
              revision={versions[to].commitId}
              oldRevision={versions[from].commitId}
              onHistory={() => {}}
            />
          ))}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  )
}
