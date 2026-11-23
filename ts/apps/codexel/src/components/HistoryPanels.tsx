import { useEffect, useRef } from 'react'
import { Box, Button, Chip, Stack, Typography } from '@mui/material'
import type { EvologEntry, Operation } from '../connect/codexel_pb.js'

const formatTime = (unix: bigint) => new Date(Number(unix) * 1000).toLocaleString()

const useScrollToSelected = (selected: number) => {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    ref.current?.scrollIntoView({ block: 'nearest' })
  }, [selected])
  return ref
}

// OperationsPanel lists the operations from the newest. The operations to revert and to restore are shown in the selected
// row. The keys are handled by the parent.
export const OperationsPanel = ({
  operations,
  selected,
  onSelect,
  onRevert,
  onRestore,
}: {
  operations: Operation[]
  selected: number
  onSelect: (index: number) => void
  onRevert: (index: number) => void
  onRestore: (index: number) => void
}) => {
  const selectedRef = useScrollToSelected(selected)
  return (
    <Box sx={{ py: 0.5 }}>
      <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>
        j/k or ↓/↑: move, 1 or Esc: log
      </Typography>
      {operations.map((op, i) => (
        <Stack
          key={op.id}
          ref={i === selected ? selectedRef : undefined}
          direction="row"
          spacing={1}
          onClick={() => onSelect(i)}
          sx={{ px: 1, py: 0.5, alignItems: 'center', cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined }}
        >
          <Typography variant="body2" sx={{ fontFamily: 'monospace', color: 'secondary.main', flexShrink: 0 }}>
            {op.id}
          </Typography>
          {op.current && <Chip label="current" size="small" color="primary" variant="outlined" />}
          <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
            {op.description}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
            {formatTime(op.time)}
          </Typography>
          {i === selected && (
            <>
              <Button size="small" sx={{ minWidth: 0, py: 0, flexShrink: 0 }} onClick={() => onRevert(i)}>
                Revert
              </Button>
              <Button size="small" color="warning" sx={{ minWidth: 0, py: 0, flexShrink: 0 }} onClick={() => onRestore(i)}>
                Restore…
              </Button>
            </>
          )}
        </Stack>
      ))}
    </Box>
  )
}

// EvologPanel lists the versions of the revision from the newest. The keys are handled by the parent.
export const EvologPanel = ({ entries, selected, onSelect }: { entries: EvologEntry[]; selected: number; onSelect: (index: number) => void }) => {
  const selectedRef = useScrollToSelected(selected)
  return (
    <Box sx={{ py: 0.5 }}>
      <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>
        j/k or ↓/↑: move, 1 or Esc: log
      </Typography>
      {entries.map((e, i) => (
        <Stack
          key={e.commitId}
          ref={i === selected ? selectedRef : undefined}
          direction="row"
          spacing={1}
          onClick={() => onSelect(i)}
          sx={{ px: 1, py: 0.5, alignItems: 'center', cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined }}
        >
          <Typography variant="body2" sx={{ fontFamily: 'monospace', color: 'secondary.main', flexShrink: 0 }}>
            {e.commitId.slice(0, 8)}
          </Typography>
          <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
            {e.operation}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
            {e.files.length} file(s) · {formatTime(e.timestamp)}
          </Typography>
        </Stack>
      ))}
    </Box>
  )
}
