import { useCallback, useEffect, type RefObject } from 'react'
import { useRouter } from '@tanstack/react-router'

const scrollParent = (el: Element | null): Element | null => {
  for (let p = el?.parentElement; p; p = p.parentElement) {
    const { overflowY } = getComputedStyle(p)
    if (overflowY === 'auto' || overflowY === 'scroll') {
      return p
    }
  }
  return null
}

// useScrollHistory returns the function to scroll to the target in the scroll container of ref. The scroll positions
// before and after the scroll are kept in the history, so the browser back and forward restore them.
export const useScrollHistory = (ref: RefObject<Element | null>): ((target: Element | null | undefined) => void) => {
  const { history } = useRouter()

  useEffect(
    () =>
      history.subscribe(({ location, action }) => {
        const top = (location.state as { scrollTop?: number }).scrollTop
        const container = scrollParent(ref.current)
        if (action.type !== 'PUSH' && action.type !== 'REPLACE' && top !== undefined && container) {
          container.scrollTop = top
        }
      }),
    [history, ref],
  )

  return useCallback(
    (target: Element | null | undefined) => {
      const container = scrollParent(ref.current)
      if (!target || !container) {
        return
      }
      const { href, state } = history.location
      history.replace(href, { ...state, scrollTop: container.scrollTop })
      history.flush()
      target.scrollIntoView({ block: 'start' })
      history.push(href, { scrollTop: container.scrollTop })
    },
    [history, ref],
  )
}
