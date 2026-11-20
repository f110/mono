import type { ReactNode } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Chip, Divider, Stack, Typography } from '@mui/material'
import { useNavigate } from '@tanstack/react-router'
import { RepositoryService, type Commit, type Review } from '../connect/codexel_pb.js'
import { stateChip } from '../lib/review.ts'
import { ChangedFiles } from './ChangedFiles.tsx'
import { FileDiffView } from './FileDiffView.tsx'

// Reviews lists the reviews of the change. Clicking the review opens it.
const Reviews = ({ reviews }: { reviews: Review[] }) => {
  const navigate = useNavigate()
  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap', mt: 0.5 }}>
      <Typography variant="caption" color="text.secondary">
        Reviews
      </Typography>
      {reviews.map((r) => (
        <Chip
          key={r.id}
          label={`${stateChip[r.state].label} · round ${r.rounds.length} · ${new Date(Number(r.createdAt) * 1000).toLocaleString()}`}
          size="small"
          color={stateChip[r.state].color}
          variant="outlined"
          onClick={() => navigate({ to: '/reviews/$reviewId', params: { reviewId: r.id } })}
        />
      ))}
    </Stack>
  )
}

// RevisionDetail shows the whole description and the diff of the revision. The revision is identified by the commit ID,
// so the result doesn't change while the commit exists.
export const RevisionDetail = ({
  path,
  commit,
  split,
  preview,
  reviews,
  onHistory,
  onRestore,
  onResolveDivergence,
  actions,
}: {
  path: string
  commit: Commit
  split: boolean
  preview: boolean
  // reviews is the reviews of the change. They are not shown if it is undefined or empty.
  reviews?: Review[]
  // The operations are not shown if the handlers are undefined.
  onHistory?: (file: string) => void
  onRestore?: (commit: Commit, file: string) => void
  onResolveDivergence?: (commit: Commit) => void
  // actions is the buttons of the operations on the revision shown under the header.
  actions?: ReactNode
}) => {
  const { data, error } = useQuery(RepositoryService.method.getRevision, { path, revision: commit.commitId }, { staleTime: Infinity })

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
        {commit.divergent && (
          <Chip
            label={onResolveDivergence ? 'divergent: resolve…' : 'divergent'}
            size="small"
            color="warning"
            onClick={onResolveDivergence && (() => onResolveDivergence(commit))}
          />
        )}
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {commit.author} &lt;{commit.email}&gt; · {new Date(Number(commit.timestamp) * 1000).toLocaleString()}
      </Typography>
      {actions}
      {reviews && reviews.length > 0 && <Reviews reviews={reviews} />}
      {error && <Alert severity="error">{error.message}</Alert>}
      <Box component="pre" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 13, my: 1 }}>
        {data?.description || '(no description set)'}
      </Box>
      <Divider sx={{ mb: 2 }} />
      {data?.files.length === 0 && <Typography color="text.secondary">No changes</Typography>}
      {data && data.files.length > 0 && (
        <ChangedFiles files={data.files} findFile={(p) => document.querySelector(`[data-file="${CSS.escape(p)}"]`)} />
      )}
      {data?.files.map((f) => (
        <FileDiffView
          key={f.path}
          file={f}
          split={split}
          preview={preview}
          repoPath={path}
          revision={commit.commitId}
          oldRevision={commit.parents.length === 1 ? commit.parents[0] : undefined}
          onHistory={onHistory}
          onRestore={commit.immutable || !onRestore ? undefined : (file) => onRestore(commit, file)}
        />
      ))}
    </Box>
  )
}
