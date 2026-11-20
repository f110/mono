const fromPaletteKey = 'codexelFromPalette'

// pressKey dispatches the key as if it is pressed. It is used by the command palette to run the action handled by the
// key, so the key is not ignored by ignoreKey even while the dialog of the palette is closing.
export const pressKey = (key: string) => {
  const e = new KeyboardEvent('keydown', { key })
  Object.defineProperty(e, fromPaletteKey, { value: true })
  window.dispatchEvent(e)
}

// ignoreKey reports whether the shortcut key should be ignored: the key for the input, the key with the modifier and the
// key while any dialog or menu is open.
export const ignoreKey = (e: KeyboardEvent): boolean => {
  if (fromPaletteKey in e) {
    return false
  }
  const target = e.target as HTMLElement | null
  const editing = target !== null && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)
  return editing || e.metaKey || e.ctrlKey || e.altKey || document.querySelector('[role="dialog"], [role="menu"]') !== null
}

// isSubmitKey reports whether the key submits the multi-line input: Cmd/Ctrl+Enter except while the IME is composing.
// The key which commits the composition may have isComposing false on some browsers, so keyCode 229 is also checked.
// See docs/ui-conventions.md.
export const isSubmitKey = (e: KeyboardEvent): boolean => e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !e.isComposing && e.keyCode !== 229
