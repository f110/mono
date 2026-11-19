import { Chip } from '@mui/material'
import { ReviewState } from '../connect/codexel_pb.js'
import { stateChip } from '../lib/review.ts'

// ApprovedBadge marks the commit which is approved in the review.
export const ApprovedBadge = () => (
  <Chip label={stateChip[ReviewState.APPROVED].label} size="small" color={stateChip[ReviewState.APPROVED].color} sx={{ height: 18 }} />
)
