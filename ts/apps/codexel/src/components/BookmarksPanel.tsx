import { useEffect, useRef } from 'react'
import { Box, Chip, Stack, Typography } from '@mui/material'
import type { Bookmark } from '../connect/codexel_pb.js'

// syncState describes the tracked remote bookmark relative to the local one. ahead is the commits only in the remote
// bookmark and behind is the commits only in the local one.
const syncState = (b: Bookmark): { label: string; color: 'default' | 'success' | 'warning' | 'error' | 'info' } => {
  if (b.conflict) return { label: 'conflict', color: 'error' }
  if (b.remote === '') return { label: b.synced ? 'local' : 'local (unsynced)', color: 'default' }
  if (!b.tracked) return { label: 'untracked', color: 'default' }
  if (b.ahead > 0 && b.behind > 0) return { label: `diverged: ${b.behind} to push, ${b.ahead} new on remote`, color: 'warning' }
  if (b.behind > 0) return { label: `${b.behind} to push`, color: 'info' }
  if (b.ahead > 0) return { label: `${b.ahead} new on remote`, color: 'warning' }
  return { label: 'synced', color: 'success' }
}

export const bookmarkLabel = (b: Bookmark) => (b.remote ? `${b.name}@${b.remote}` : b.name)

// BookmarksPanel lists the local and the remote bookmarks. The keys are handled by the parent.
export const BookmarksPanel = ({ bookmarks, selected, onSelect }: { bookmarks: Bookmark[]; selected: number; onSelect: (index: number) => void }) => {
  const selectedRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  return (
    <Box sx={{ py: 0.5 }}>
      <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>
        j/k: move, d: delete, f: forget, t: track/untrack, 1: log
      </Typography>
      {bookmarks.map((b, i) => {
        const state = syncState(b)
        return (
          <Stack
            key={bookmarkLabel(b)}
            ref={i === selected ? selectedRef : undefined}
            direction="row"
            spacing={1}
            onClick={() => onSelect(i)}
            sx={{ px: 1, height: 28, alignItems: 'center', cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined }}
          >
            <Typography variant="body2" sx={{ fontFamily: 'monospace', pl: b.remote ? 2 : 0 }} noWrap>
              {bookmarkLabel(b)}
            </Typography>
            <Chip label={state.label} size="small" color={state.color} variant="outlined" sx={{ height: 18 }} />
            <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
              {b.target.slice(0, 8)}
            </Typography>
            <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
              {b.description}
            </Typography>
          </Stack>
        )
      })}
    </Box>
  )
}
