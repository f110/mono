import { useEffect, useRef } from 'react'
import { Box, Chip, Stack, Typography } from '@mui/material'
import type { Commit, LogRow } from '../connect/codexel_pb.js'
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
  <Typography component="span" variant="body2" sx={{ fontFamily: 'monospace', flexShrink: 0 }}>
    <Box component="span" sx={{ color: 'secondary.main', fontWeight: 'bold' }}>
      {commit.changeIdPrefix}
    </Box>
    <Box component="span" sx={{ color: 'text.disabled' }}>
      {commit.changeId.slice(commit.changeIdPrefix.length, 8)}
    </Box>
  </Typography>
)

const CommitLine = ({ commit }: { commit: Commit }) => (
  <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0, flex: 1 }}>
    <ChangeId commit={commit} />
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
export const RevisionGraph = ({ rows, selected, onSelect }: { rows: LogRow[]; selected: number; onSelect: (index: number) => void }) => {
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
            sx={{ cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined, '&:hover': { bgcolor: 'action.hover' }, px: 1 }}
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
                    <CommitLine commit={commit} />
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
