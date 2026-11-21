import { useState } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import CheckIcon from '@mui/icons-material/Check'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import MoreHorizIcon from '@mui/icons-material/MoreHoriz'
import RateReviewIcon from '@mui/icons-material/RateReview'
import { Alert, Box, Chip, Divider, IconButton, Link, Stack, Tooltip, Typography } from '@mui/material'
import { useNavigate } from '@tanstack/react-router'
import { RepositoryService, SystemService, type Commit, type Review } from '../connect/codexel_pb.js'
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

// linkIssues returns text with #N linked to the ticket N of the Issue tracker.
const linkIssues = (text: string, issueUrl: string) =>
  text.split(/(#\d+\b)/).map((s, i) =>
    i % 2 === 1 ? (
      <Link key={i} href={`${issueUrl}/${s.slice(1)}`} target="_blank" rel="noreferrer">
        {s}
      </Link>
    ) : (
      s
    ),
  )

// Description shows the description of the commit. If issueUrl is not empty, #N in the trailers is linked to the ticket.
// The trailers are the last paragraph like jj.
const Description = ({ description, hasTrailers, issueUrl }: { description: string; hasTrailers: boolean; issueUrl: string }) => {
  const text = description.trimEnd()
  const i = text.lastIndexOf('\n\n')
  if (!issueUrl || !hasTrailers || i === -1) {
    return description
  }
  return (
    <>
      {text.slice(0, i + 2)}
      {linkIssues(text.slice(i + 2), issueUrl)}
    </>
  )
}

// CopyButton copies the text to the clipboard. The icon shows the result for a while after clicking.
const CopyButton = ({ text, label }: { text: string; label: string }) => {
  const [result, setResult] = useState<'copied' | 'failed'>()
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setResult('copied')
    } catch {
      setResult('failed')
    }
    setTimeout(() => setResult(undefined), 1500)
  }
  return (
    <Tooltip title={result === 'copied' ? 'Copied' : result === 'failed' ? 'Failed to copy' : label}>
      <IconButton size="small" aria-label={label} onClick={copy}>
        {result === 'copied' ? (
          <CheckIcon fontSize="inherit" color="success" />
        ) : result === 'failed' ? (
          <ErrorOutlineIcon fontSize="inherit" color="error" />
        ) : (
          <ContentCopyIcon fontSize="inherit" />
        )}
      </IconButton>
    </Tooltip>
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
  onReview,
  onMenu,
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
  // onReview starts the review. The button is not shown if it is undefined.
  onReview?: () => void
  // onMenu opens the menu of the operations on the revision anchored to the button. The button is not shown if it is
  // undefined.
  onMenu?: (anchor: HTMLElement) => void
}) => {
  const { data, error } = useQuery(RepositoryService.method.getRevision, { path, revision: commit.commitId }, { staleTime: Infinity })
  const { data: status } = useQuery(SystemService.method.getStatus, {})

  return (
    <Box sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
          {commit.changeId}
        </Typography>
        <CopyButton text={commit.changeId} label="Copy the change ID" />
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
        {(onReview || onMenu) && (
          <Stack direction="row" sx={{ ml: 'auto !important' }}>
            {onReview && (
              <Tooltip title="Review (v)">
                <IconButton size="small" aria-label="Review" onClick={onReview}>
                  <RateReviewIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
            {onMenu && (
              <IconButton size="small" aria-label="Operations" onClick={(e) => onMenu(e.currentTarget)}>
                <MoreHorizIcon fontSize="small" />
              </IconButton>
            )}
          </Stack>
        )}
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {commit.author} &lt;{commit.email}&gt; · {new Date(Number(commit.timestamp) * 1000).toLocaleString()}
      </Typography>
      {reviews && reviews.length > 0 && <Reviews reviews={reviews} />}
      {error && <Alert severity="error">{error.message}</Alert>}
      <Box component="pre" sx={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 13, my: 1 }}>
        {data?.description ? (
          <Description description={data.description} hasTrailers={commit.trailers.length > 0} issueUrl={status?.issueUrl ?? ''} />
        ) : (
          '(no description set)'
        )}
      </Box>
      <Divider sx={{ mb: 2 }} />
      {data?.files.length === 0 && data.conflicts.length === 0 && <Typography color="text.secondary">No changes</Typography>}
      {data && (data.files.length > 0 || data.conflicts.length > 0) && (
        <ChangedFiles files={data.files} conflicts={data.conflicts} findFile={(p) => document.querySelector(`[data-file="${CSS.escape(p)}"]`)} />
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
