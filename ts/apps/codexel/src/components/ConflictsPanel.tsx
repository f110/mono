import { Box, Chip, Stack, Typography } from '@mui/material'
import type { ConflictedCommit, ConflictedFile } from '../connect/codexel_pb.js'

export type ConflictEntry = { commit: ConflictedCommit; file: ConflictedFile }

export const conflictEntries = (commits: ConflictedCommit[]): ConflictEntry[] => commits.flatMap((c) => c.files.map((f) => ({ commit: c, file: f })))

// ConflictsPanel lists the conflicted files grouped by the commit. The keys are handled by the parent.
export const ConflictsPanel = ({ entries, selected, onSelect }: { entries: ConflictEntry[]; selected: number; onSelect: (index: number) => void }) => (
  <Box sx={{ py: 0.5 }}>
    <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>
      j/k: move, 1 or Esc: log
    </Typography>
    {entries.length === 0 && (
      <Typography color="text.secondary" sx={{ p: 1 }}>
        No conflicts
      </Typography>
    )}
    {entries.map((e, i) => (
      <Box key={`${e.commit.commitId}:${e.file.path}`}>
        {(i === 0 || entries[i - 1].commit.commitId !== e.commit.commitId) && (
          <Stack direction="row" spacing={1} sx={{ px: 1, pt: 1, alignItems: 'center' }}>
            <Typography variant="body2" sx={{ fontFamily: 'monospace', color: 'secondary.main' }}>
              {e.commit.changeId.slice(0, 8)}
            </Typography>
            {e.commit.workingCopy && <Chip label="@" size="small" color="primary" variant="outlined" sx={{ height: 18 }} />}
            <Typography variant="body2" noWrap>
              {e.commit.description || '(no description set)'}
            </Typography>
          </Stack>
        )}
        <Stack
          direction="row"
          spacing={1}
          onClick={() => onSelect(i)}
          sx={{ pl: 3, pr: 1, height: 24, alignItems: 'center', cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : undefined }}
        >
          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12, flex: 1 }} noWrap>
            {e.file.path}
          </Typography>
          {e.file.sides > 2 && <Chip label={`${e.file.sides}-way`} size="small" color="warning" variant="outlined" sx={{ height: 18 }} />}
        </Stack>
      </Box>
    ))}
  </Box>
)
