import { useEffect, useRef } from 'react'
import Anser from 'anser'
import { Box } from '@mui/material'

const toColor = (c: string): string | undefined => (c ? `rgb(${c})` : undefined)

// AgentOutput renders the terminal output. Only the colors and the decorations of ANSI SGR are interpreted.
export const AgentOutput = ({ text }: { text: string }) => {
  const ref = useRef<HTMLPreElement>(null)
  const entries = Anser.ansiToJson(text.replaceAll('\r', ''), { json: true, remove_empty: true })

  useEffect(() => {
    const el = ref.current
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  }, [text])

  return (
    <Box
      component="pre"
      ref={ref}
      sx={{
        m: 0,
        p: 1,
        height: '100%',
        overflow: 'auto',
        fontFamily: 'monospace',
        fontSize: 13,
        lineHeight: 1.3,
        bgcolor: '#111',
        color: '#ddd',
      }}
    >
      {entries.map((e, i) => (
        <span
          key={i}
          style={{
            color: toColor(e.fg),
            backgroundColor: toColor(e.bg),
            fontWeight: e.decorations.includes('bold') ? 'bold' : undefined,
            opacity: e.decorations.includes('dim') ? 0.6 : undefined,
            fontStyle: e.decorations.includes('italic') ? 'italic' : undefined,
            textDecoration: e.decorations.includes('underline') ? 'underline' : undefined,
          }}
        >
          {e.content}
        </span>
      ))}
    </Box>
  )
}
