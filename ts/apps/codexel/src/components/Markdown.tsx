import { isValidElement, useId } from 'react'
import { Alert, Box } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

// Mermaid renders the diagram. mermaid is loaded only when the diagram is shown because it is large. securityLevel
// strict sanitizes the diagram because it is written by the agents and the files in the repository.
const Mermaid = ({ code }: { code: string }) => {
  const id = useId().replace(/[^a-zA-Z0-9]/g, '')
  const { data, error } = useQuery({
    queryKey: ['mermaid', code],
    queryFn: async () => {
      const { default: mermaid } = await import('mermaid')
      mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' })
      const { svg } = await mermaid.render(`mermaid${id}`, code)
      return svg
    },
    staleTime: Infinity,
    retry: false,
  })

  if (error) {
    return (
      <>
        <Alert severity="warning">{error.message}</Alert>
        <pre>{code}</pre>
      </>
    )
  }
  return <Box sx={{ '& svg': { maxWidth: '100%', height: 'auto' } }} dangerouslySetInnerHTML={{ __html: data ?? '' }} />
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
