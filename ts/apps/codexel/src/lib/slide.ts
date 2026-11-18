import { RebaseTarget, type Commit } from '../connect/codexel_pb.js'

export type Slide = { ok: true; destination: string; target: RebaseTarget } | { ok: false; reason: string }

// computeSlide returns the destination to swap the revision with the parent (down) or the child (up). The children are
// found in commits, so the child out of the shown revset is not considered.
export const computeSlide = (commits: Commit[], index: number, direction: 'down' | 'up'): Slide => {
  const src = commits[index]
  if (!src) return { ok: false, reason: 'No revision selected' }
  if (src.immutable) return { ok: false, reason: 'Cannot slide the immutable revision' }
  if (src.parents.length > 1) return { ok: false, reason: 'Cannot slide the merge commit. Use rebase' }

  const byId = new Map(commits.map((c) => [c.commitId, c]))
  if (direction === 'down') {
    if (src.parents.length === 0) return { ok: false, reason: 'The revision has no parent' }
    const parent = byId.get(src.parents[0])
    if (!parent) return { ok: false, reason: 'The parent is not shown' }
    if (parent.immutable) return { ok: false, reason: 'Cannot slide past the immutable parent' }
    if (parent.parents.length > 1) return { ok: false, reason: 'The parent is the merge commit. Use rebase' }
    return { ok: true, destination: parent.commitId, target: RebaseTarget.INSERT_BEFORE }
  }

  const children = commits.filter((c) => c.parents.includes(src.commitId))
  if (children.length === 0) return { ok: false, reason: 'The revision has no child' }
  if (children.length > 1) return { ok: false, reason: 'The revision has multiple children. Use rebase' }
  const child = children[0]
  if (child.parents.length > 1) return { ok: false, reason: 'The child is the merge commit. Use rebase' }
  return { ok: true, destination: child.commitId, target: RebaseTarget.INSERT_AFTER }
}
