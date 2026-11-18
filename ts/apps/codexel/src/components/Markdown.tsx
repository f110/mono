import { isValidElement, useEffect, useId, useState } from 'react'
import { Alert, Box, useColorScheme } from '@mui/material'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

// Mermaid renders the diagram. mermaid is loaded only when the diagram is shown because it is large. securityLevel
// strict sanitizes the diagram because it is written by the agents and the files in the repository.
const Mermaid = ({ code }: { code: string }) => {
  const id = useId().replace(/[^a-zA-Z0-9]/g, '')
  const { mode, systemMode } = useColorScheme()
  const dark = (mode === 'system' ? systemMode : mode) === 'dark'
  const [result, setResult] = useState<{ svg?: string; error?: string }>({})

  useEffect(() => {
    let canceled = false
    import('mermaid')
      .then(async ({ default: mermaid }) => {
        mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: dark ? 'dark' : 'default' })
        const { svg } = await mermaid.render(`mermaid${id}`, code)
        if (!canceled) setResult({ svg })
      })
      .catch((e: unknown) => {
        if (!canceled) setResult({ error: e instanceof Error ? e.message : String(e) })
      })
    return () => {
      canceled = true
    }
  }, [code, dark, id])

  if (result.error) {
    return (
      <>
        <Alert severity="warning">{result.error}</Alert>
        <pre>{code}</pre>
      </>
    )
  }
  return <Box sx={{ '& svg': { maxWidth: '100%', height: 'auto' } }} dangerouslySetInnerHTML={{ __html: result.svg ?? '' }} />
}

// Markdown renders GitHub Flavored Markdown. The code blocks of mermaid are rendered as the diagrams.
export const Markdown = ({ text }: { text: string }) => (
  <ReactMarkdown
    remarkPlugins={[remarkGfm]}
    components={{
      // The diagram is not wrapped by pre.
      pre: ({ children }) =>
        isValidElement<{ className?: string }>(children) && children.props.className === 'language-mermaid' ? children : <pre>{children}</pre>,
      code: ({ className, children }) =>
        className === 'language-mermaid' ? (
          <Mermaid code={String(children).replace(/\n$/, '')} />
        ) : (
          <code className={className}>
            {children}
          </code>
        ),
    }}
  >
    {text}
  </ReactMarkdown>
)
