import { Box } from '@mui/material'
import 'github-markdown-css/github-markdown-light.css'
import 'highlight.js/styles/github.css'
import ReactMarkdown from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import rehypeSanitize from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'

type Node = {
  type: string
  value?: string
  url?: string
  children?: Node[]
}

const ticketReference = /#(\d+)\b/g

// remarkTicketLink replaces #123 with the link to the ticket. The code and the existing link are not changed.
function remarkTicketLink() {
  const walk = (node: Node) => {
    if (!node.children || node.type === 'link' || node.type === 'inlineCode' || node.type === 'code') {
      return
    }
    node.children = node.children.flatMap((child): Node[] => {
      if (child.type !== 'text' || !child.value) {
        walk(child)
        return [child]
      }
      const nodes: Node[] = []
      let last = 0
      for (const m of child.value.matchAll(ticketReference)) {
        if (m.index > last) {
          nodes.push({ type: 'text', value: child.value.slice(last, m.index) })
        }
        nodes.push({ type: 'link', url: `/${m[1]}`, children: [{ type: 'text', value: m[0] }] })
        last = m.index + m[0].length
      }
      if (last < child.value.length) {
        nodes.push({ type: 'text', value: child.value.slice(last) })
      }
      return nodes
    })
  }
  return (tree: Node) => walk(tree)
}

export const Markdown = ({ text }: { text: string }) => (
  <Box className="markdown-body">
    <ReactMarkdown remarkPlugins={[remarkGfm, remarkTicketLink]} rehypePlugins={[rehypeSanitize, rehypeHighlight]}>
      {text}
    </ReactMarkdown>
  </Box>
)
