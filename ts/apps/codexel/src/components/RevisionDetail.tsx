import { useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Chip, Divider, Stack, Typography } from '@mui/material'
import { RevisionService, type Commit } from '../connect/codexel_pb.js'
import { FileDiffView } from './FileDiffView.tsx'

// RevisionDetail shows the whole description and the diff of the revision. The revision is identified by the commit ID,
// so the result doesn't change while the commit exists.
export const RevisionDetail = ({ path, commit }: { path: string; commit: Commit }) => {
  const { data, error } = useQuery(RevisionService.method.getRevision, { path, revision: commit.commitId }, { staleTime: Infinity })

  return (
    <Box sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
          {commit.changeId}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          {commit.commitId}
        </Typography>
        {commit.immutable && <Chip label="immutable" size="small" variant="outlined" />}
        {commit.currentWorkingCopy && <Chip label="@" size="small" color="primary" variant="outlined" />}
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {commit.author} &lt;{commit.email}&gt; · {new Date(Number(commit.timestamp) * 1000).toLocaleString()}
      </Typography>
      {error && <Alert severity="error">{error.message}</Alert>}
      <Box component="pre" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 13, my: 1 }}>
        {data?.description || '(no description set)'}
      </Box>
      <Divider sx={{ mb: 2 }} />
      {data?.files.length === 0 && <Typography color="text.secondary">No changes</Typography>}
      {data?.files.map((f) => <FileDiffView key={f.path} file={f} />)}
    </Box>
  )
}
