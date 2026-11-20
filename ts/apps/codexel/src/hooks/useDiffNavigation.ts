import { useCallback, useEffect, useState, type RefObject } from 'react'
import { ignoreKey } from '../lib/keys.ts'

const splitKey = 'codexel.diff.split'

export type DiffNavigation = {
  split: boolean
  preview: boolean
  toggleSplit: () => void
  togglePreview: () => void
  // moveHunk scrolls the container to the next / previous hunk.
  moveHunk: (direction: 'next' | 'previous') => void
}

// useDiffNavigation handles the keys of the diff in the container: | toggles the split view, m toggles the preview of
// Markdown and ] / [ move to the next / previous hunk. The setting of the split view is kept in the local storage.
export const useDiffNavigation = (container: RefObject<HTMLElement | null>): DiffNavigation => {
  const [split, setSplit] = useState(() => localStorage.getItem(splitKey) === 'true')
  const [preview, setPreview] = useState(false)

  const toggleSplit = useCallback(
    () =>
      setSplit((s) => {
        localStorage.setItem(splitKey, String(!s))
        return !s
      }),
    [],
  )
  const togglePreview = useCallback(() => setPreview((p) => !p), [])
  const moveHunk = useCallback(
    (direction: 'next' | 'previous') => {
      const el = container.current
      if (!el) {
        return
      }
      const top = el.getBoundingClientRect().top
      const hunks = [...el.querySelectorAll<HTMLElement>('[data-hunk]')]
      // The offset is from the position where scrollIntoView puts the hunk. scrollMarginTop of the hunk leaves the space for
      // the sticky headers. The hunk at the position is the current one.
      const offsets = hunks.map((h) => h.getBoundingClientRect().top - top - parseFloat(getComputedStyle(h).scrollMarginTop))
      const above = hunks.filter((_, i) => offsets[i] < -8)
      const target = direction === 'next' ? hunks.find((_, i) => offsets[i] > 12) : above[above.length - 1]
      target?.scrollIntoView({ block: 'start' })
    },
    [container],
  )

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (ignoreKey(e)) {
        return
      }
      if (e.key === '|') {
        toggleSplit()
      } else if (e.key === 'm') {
        togglePreview()
      } else if (e.key === ']' || e.key === '[') {
        moveHunk(e.key === ']' ? 'next' : 'previous')
      } else {
        return
      }
      e.preventDefault()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [toggleSplit, togglePreview, moveHunk])

  return { split, preview, toggleSplit, togglePreview, moveHunk }
}
