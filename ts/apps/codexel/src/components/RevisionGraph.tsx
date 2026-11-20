import { useEffect, useRef, type MouseEvent } from 'react'
import { Box, Chip, Stack, Typography } from '@mui/material'
import { useNavigate } from '@tanstack/react-router'
import { ReviewState, type Agent, type Commit, type LogRow, type Review } from '../connect/codexel_pb.js'
import { stateChip } from '../lib/review.ts'
import { GraphGutter, cellWidth } from './GraphGutter.tsx'

const nodeHeight = 28
const connectorHeight = 14

const relativeTime = (unix: bigint): string => {
  const sec = Math.floor(Date.now() / 1000) - Number(unix)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return `${Math.floor(sec / 86400)}d ago`
}

const ChangeId = ({ commit }: { commit: Commit }) => (
  <Typography component="span" variant="body2" sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', flexShrink: 0 }}>
    <Box component="span" sx={{ color: 'secondary.main', fontWeight: 'bold' }}>
      {commit.changeIdPrefix}
    </Box>
    <Box component="span" sx={{ color: 'text.disabled' }}>
      {commit.changeId.slice(commit.changeIdPrefix.length, 8)}
    </Box>
  </Typography>
)

// AgentBadge shows the agent which created the commit by the Agent and the Agent-Session trailers. It links to the
// agent while the agent is running in herdr.
const AgentBadge = ({ commit, agents }: { commit: Commit; agents: Agent[] }) => {
  const navigate = useNavigate()
  const kind = commit.trailers.find((t) => t.key === 'Agent')?.value
  const sessionId = commit.trailers.find((t) => t.key === 'Agent-Session')?.value
  if (!kind || !sessionId) {
    return null
  }
  const agent = agents.find((a) => a.kind === kind && a.sessionId === sessionId)
  return (
    <Chip
      label={kind}
      title={`${kind} / ${sessionId}`}
      size="small"
      color="info"
      variant={agent ? 'filled' : 'outlined'}
      sx={{ height: 18 }}
      onClick={
        agent
          ? (e: MouseEvent) => {
              e.stopPropagation()
              navigate({ to: '/agents', search: { pane: agent.paneId } })
            }
          : undefined
      }
    />
  )
}

// ReviewBadge shows the state of the latest review of the change and the number of the reviews. It links to the latest
// review. The approved review is hidden if the commit is not approved because it is rewritten after the approval.
const ReviewBadge = ({ reviews, approved }: { reviews: Review[]; approved: boolean }) => {
  const navigate = useNavigate()
  const latest = reviews[0]
  if (!latest || (latest.state === ReviewState.APPROVED && !approved)) {
    return null
  }
  return (
    <Chip
      label={`review: ${stateChip[latest.state].label}${reviews.length > 1 ? ` (${reviews.length})` : ''}`}
      size="small"
      color={stateChip[latest.state].color}
      variant={latest.state === ReviewState.APPROVED ? 'filled' : 'outlined'}
      sx={{ height: 18 }}
      onClick={(e: MouseEvent) => {
        e.stopPropagation()
        navigate({ to: '/reviews/$reviewId', params: { reviewId: latest.id } })
      }}
    />
  )
}

// PullRequestBadge links to the pull request of the change submitted by Codexel.
const PullRequestBadge = ({ url }: { url: string | undefined }) => {
  if (!url) {
    return null
  }
  return (
    <Chip
      label={`PR #${url.split('/').pop()}`}
      title={url}
      size="small"
      color="secondary"
      variant="outlined"
      sx={{ height: 18 }}
      component="a"
      href={url}
      target="_blank"
      rel="noreferrer"
      clickable
      onClick={(e: MouseEvent) => e.stopPropagation()}
    />
  )
}

const CommitLine = ({
  commit,
  mark,
  approved,
  reviews,
  pullRequest,
  agents,
}: {
  commit: Commit
  mark: string | undefined
  approved: boolean
  reviews: Review[]
  pullRequest: string | undefined
  agents: Agent[]
}) => (
  <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0, flex: 1 }}>
    {mark && <Chip label={mark} size="small" color="warning" sx={{ height: 18 }} />}
    <ChangeId commit={commit} />
    <AgentBadge commit={commit} agents={agents} />
    <ReviewBadge reviews={reviews} approved={approved} />
    <PullRequestBadge url={pullRequest} />
    {commit.workingCopies.map((w) => (
      <Chip key={`${w}@`} label={`${w}@`} size="small" color="success" variant="outlined" sx={{ height: 18 }} />
    ))}
    {commit.localBookmarks.map((b) => (
      <Chip key={b} label={b} size="small" color="primary" variant="outlined" sx={{ height: 18 }} />
    ))}
    {commit.remoteBookmarks.map((b) => (
      <Chip key={`${b.name}@${b.remote}`} label={`${b.name}@${b.remote}`} size="small" variant="outlined" sx={{ height: 18 }} />
    ))}
    {commit.conflict && <Chip label="conflict" size="small" color="error" sx={{ height: 18 }} />}
    {commit.divergent && <Chip label="divergent" size="small" color="warning" sx={{ height: 18 }} />}
    <Typography
      variant="body2"
      noWrap
      sx={{ flex: 1, minWidth: 0, color: commit.description ? 'text.primary' : 'text.disabled', fontStyle: commit.description ? undefined : 'italic' }}
    >
      {commit.description || (commit.empty ? '(empty) (no description set)' : '(no description set)')}
    </Typography>
    <Typography variant="caption" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
      {commit.author} · {relativeTime(commit.timestamp)}
    </Typography>
  </Stack>
)

// RevisionGraph shows the log of jj with the graph. selected is the index of the selected row.
export const RevisionGraph = ({
  rows,
  selected,
  checked,
  marks,
  approved,
  reviews,
  pullRequests,
  agents,
  onSelect,
  onContextMenu,
}: {
  rows: LogRow[]
  selected: number
  // checked is the commit IDs of the checked revisions.
  checked: Set<string>
  // marks is the labels shown on the revisions by the commit ID (e.g. the source of the rebase).
  marks: Map<string, string>
  // approved is the commit IDs approved in the reviews.
  approved: Set<string>
  // reviews is the reviews by the change ID.
  reviews: Map<string, Review[]>
  // pullRequests is the URLs of the pull requests by the change ID.
  pullRequests: Map<string, string>
  // agents is the agents running in herdr. They are linked from the commits created by them.
  agents: Agent[]
  onSelect: (index: number) => void
  onContextMenu: (index: number, x: number, y: number) => void
}) => {
  const width = Math.max(0, ...rows.flatMap((r) => r.lines.map((l) => [...l.gutter].length))) * cellWidth
  const lines = rows.flatMap((r) => r.lines.map((l) => l.gutter))
  const selectedRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  let lineIndex = 0
  return (
    <Box sx={{ py: 0.5 }}>
      {rows.map((row, i) => {
        const commit = row.commit!
        return (
          <Box
            key={commit.commitId + commit.changeId}
            ref={i === selected ? selectedRef : undefined}
            onClick={() => onSelect(i)}
            onContextMenu={(e) => {
              e.preventDefault()
              onContextMenu(i, e.clientX, e.clientY)
            }}
            sx={{
              cursor: 'pointer',
              bgcolor: i === selected ? 'action.selected' : undefined,
              '&:hover': { bgcolor: 'action.hover' },
              px: 1,
              borderLeft: 3,
              borderColor: checked.has(commit.commitId) ? 'warning.main' : 'transparent',
            }}
          >
            {row.lines.map((line, j) => {
              const index = lineIndex++
              const height = line.node ? nodeHeight : connectorHeight
              return (
                <Stack key={j} direction="row" spacing={1} sx={{ alignItems: 'center', height }}>
                  <GraphGutter
                    gutter={line.gutter}
                    above={lines[index - 1]}
                    below={lines[index + 1]}
                    height={height}
                    width={width}
                    workingCopy={commit.currentWorkingCopy}
                  />
                  {line.node ? (
                    <CommitLine
                      commit={commit}
                      mark={marks.get(commit.commitId)}
                      approved={approved.has(commit.commitId)}
                      reviews={reviews.get(commit.changeId) ?? []}
                      pullRequest={pullRequests.get(commit.changeId)}
                      agents={agents}
                    />
                  ) : (
                    <Typography variant="caption" color="text.disabled">
                      {line.content}
                    </Typography>
                  )}
                </Stack>
              )
            })}
          </Box>
        )
      })}
    </Box>
  )
}
