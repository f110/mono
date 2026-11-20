import { ReviewState } from '../connect/codexel_pb.js'

export const stateChip: Record<ReviewState, { label: string; color: 'default' | 'primary' | 'warning' | 'success' }> = {
  [ReviewState.UNSPECIFIED]: { label: '', color: 'default' },
  [ReviewState.OPEN]: { label: 'open', color: 'primary' },
  [ReviewState.FINISHED]: { label: 'finished', color: 'warning' },
  [ReviewState.APPROVED]: { label: 'approved', color: 'success' },
  [ReviewState.DISCARDED]: { label: 'discarded', color: 'default' },
}
