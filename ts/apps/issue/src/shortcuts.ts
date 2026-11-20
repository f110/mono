import { useEffect, useRef } from 'react'

// ShortcutHelp is the row of the help of the shortcuts.
export type ShortcutHelp = { label: string; description: string }

// Shortcut is the shortcut key handled by useShortcuts. keys are the values of KeyboardEvent.key.
export type Shortcut = ShortcutHelp & { keys: string[] }

export const globalShortcuts = {
  help: { keys: ['?'], label: '?', description: 'ショートカットのヘルプを表示' },
  new: { keys: ['n'], label: 'n', description: 'チケットを新規作成' },
} satisfies Record<string, Shortcut>

export const listShortcuts = {
  down: { keys: ['j', 'ArrowDown'], label: 'j / ↓', description: 'カーソルを次のチケットに移動' },
  up: { keys: ['k', 'ArrowUp'], label: 'k / ↑', description: 'カーソルを前のチケットに移動' },
  open: { keys: ['Enter', 'o'], label: 'Enter / o', description: 'カーソルのチケットを開く' },
  search: { keys: ['/'], label: '/', description: 'フィルタの入力欄にフォーカス' },
  nextPage: { keys: [']'], label: ']', description: '次のページ' },
  prevPage: { keys: ['['], label: '[', description: '前のページ' },
} satisfies Record<string, Shortcut>

export const ticketShortcuts = {
  comment: { keys: ['c'], label: 'c', description: 'コメントの入力欄にフォーカス' },
  editBody: { keys: ['e'], label: 'e', description: '本文を編集' },
  editTitle: { keys: ['t'], label: 't', description: 'タイトルを編集' },
  status: { keys: ['s'], label: 's', description: 'Status を選択' },
  assignee: { keys: ['a'], label: 'a', description: 'Assignee を選択' },
  priority: { keys: ['p'], label: 'p', description: 'Priority を選択' },
  back: { keys: ['u'], label: 'u', description: '一覧に戻る' },
} satisfies Record<string, Shortcut>

// inputShortcuts are handled by the input itself and only shown in the help.
export const inputShortcuts: ShortcutHelp[] = [
  { label: 'Cmd/Ctrl+Enter', description: '本文・コメントを送信' },
  { label: 'Esc', description: '入力欄からフォーカスを外す' },
]

// ignoreKey reports whether the shortcut key should be ignored: the key for the input, the key handled by the focused
// element, the key with the modifier and the key while any dialog or menu is open.
export const ignoreKey = (e: KeyboardEvent): boolean => {
  const target = e.target as HTMLElement | null
  const editing =
    target !== null && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)
  const activating = e.key === 'Enter' && target !== document.body
  return (
    e.defaultPrevented ||
    e.isComposing ||
    editing ||
    activating ||
    e.metaKey ||
    e.ctrlKey ||
    e.altKey ||
    document.querySelector('[role="dialog"], [role="menu"], [role="listbox"]') !== null
  )
}

// useShortcuts calls the handler of the shortcut whose key is pressed. shortcuts must not change between renders.
export function useShortcuts<K extends string>(shortcuts: Record<K, Shortcut>, handlers: Record<K, () => void>) {
  const ref = useRef(handlers)
  useEffect(() => {
    ref.current = handlers
  })
  useEffect(() => {
    const names = Object.keys(shortcuts) as K[]
    const onKeyDown = (e: KeyboardEvent) => {
      if (ignoreKey(e)) {
        return
      }
      const name = names.find((n) => shortcuts[n].keys.includes(e.key))
      if (name === undefined) {
        return
      }
      e.preventDefault()
      ref.current[name]()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [shortcuts])
}
