import { useEffect, useState, type RefObject } from 'react'
import { ignoreKey } from '../lib/keys.ts'

const splitKey = 'codexel.diff.split'

// useDiffNavigation handles the keys of the diff in the container: | toggles the split view, m toggles the preview of
// Markdown and ] / [ move to the next / previous hunk. The setting of the split view is kept in the local storage.
export const useDiffNavigation = (container: RefObject<HTMLElement | null>): { split: boolean; preview: boolean } => {
  const [split, setSplit] = useState(() => localStorage.getItem(splitKey) === 'true')
  const [preview, setPreview] = useState(false)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (ignoreKey(e)) {
        return
      }
      const el = container.current
      if (e.key === '|') {
        setSplit((s) => {
          localStorage.setItem(splitKey, String(!s))
          return !s
        })
      } else if (e.key === 'm') {
        setPreview((p) => !p)
      } else if ((e.key === ']' || e.key === '[') && el) {
        const top = el.getBoundingClientRect().top
        const hunks = [...el.querySelectorAll<HTMLElement>('[data-hunk]')]
        const offsets = hunks.map((h) => h.getBoundingClientRect().top - top)
        // The hunk at the top is the current one. scrollMarginTop of the hunk leaves the space for the sticky header.
        const above = hunks.filter((_, i) => offsets[i] < 40)
        const target = e.key === ']' ? hunks.find((_, i) => offsets[i] > 60) : above[above.length - 1]
        target?.scrollIntoView({ block: 'start' })
      } else {
        return
      }
      e.preventDefault()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [container])

  return { split, preview }
}
