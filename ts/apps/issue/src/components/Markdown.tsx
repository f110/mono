import { Box } from '@mui/material'
import 'github-markdown-css/github-markdown-light.css'
import 'highlight.js/styles/github.css'
import ReactMarkdown, { type Components } from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import rehypeSanitize from 'rehype-sanitize'
import remarkBreaks from 'remark-breaks'
import remarkGfm from 'remark-gfm'
import type { MentionedTicket } from '../connect/issue_pb'
import { TicketStatusIcon } from './TicketStatusIcon.tsx'

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

type HastNode = {
  type: string
  tagName?: string
  properties?: Record<string, unknown>
  position?: unknown
  children?: HastNode[]
}

// rehypeTaskList adds the class of github-markdown-css to the checkbox of the task list
// and copies the position of the list item to the checkbox for finding the marker in the source.
function rehypeTaskList() {
  const walk = (node: HastNode) => {
    for (const child of node.children ?? []) {
      const className = child.properties?.className
      if (child.tagName === 'li' && Array.isArray(className) && className.includes('task-list-item')) {
        const first = child.children?.find((c) => c.type === 'element')
        const input = first?.tagName === 'p' ? first.children?.[0] : first
        if (input?.tagName === 'input' && input.properties) {
          input.properties.className = ['task-list-item-checkbox']
          input.position = child.position
        }
      }
      walk(child)
    }
  }
  return (tree: HastNode) => walk(tree)
}

// rehypeExternalLink makes the link to the other origin open in a new tab.
function rehypeExternalLink() {
  const isExternal = (href: unknown) => {
    if (typeof href !== 'string') {
      return false
    }
    try {
      return new URL(href, window.location.href).origin !== window.location.origin
    } catch {
      return false
    }
  }
  const walk = (node: HastNode) => {
    for (const child of node.children ?? []) {
      if (child.tagName === 'a' && child.properties && isExternal(child.properties.href)) {
        child.properties.target = '_blank'
        child.properties.rel = ['noreferrer']
      }
      walk(child)
    }
  }
  return (tree: HastNode) => walk(tree)
}

const taskMarker = /^(?:[-*+]|\d+[.)])[ \t]+\[([ xX])\]/

// toggleTask toggles the task of the list item which starts at offset. It returns null if the task is not found.
function toggleTask(text: string, offset: number): string | null {
  const m = taskMarker.exec(text.slice(offset))
  if (!m) {
    return null
  }
  const i = offset + m[0].length - 2
  return text.slice(0, i) + (m[1] === ' ' ? 'x' : ' ') + text.slice(i + 1)
}

// Markdown renders text. The link of "#123" is expanded to the status and the title if the ticket is in tickets.
export const Markdown = ({ text, tickets, onChange }: { text: string; tickets?: MentionedTicket[]; onChange?: (text: string) => void }) => {
  const components: Components = {
    a: ({ node, href, children, ...props }) => {
      const m = /^\/(\d+)$/.exec(href ?? '')
      const label = node?.children.length === 1 && node.children[0].type === 'text' ? node.children[0].value : undefined
      const ticket = m && label === `#${m[1]}` ? tickets?.find((t) => t.ticket === Number(m[1])) : undefined
      if (!ticket) {
        return (
          <a href={href} {...props}>
            {children}
          </a>
        )
      }
      return (
        <a href={href} {...props}>
          <TicketStatusIcon status={ticket.status} /> {ticket.title} {children}
        </a>
      )
    },
  }
  if (onChange) {
    components.input = ({ node, ...props }) => {
      const offset = node?.position?.start.offset
      if (props.type !== 'checkbox' || offset === undefined) {
        return <input {...props} />
      }
      return (
        <input
          {...props}
          disabled={false}
          onChange={() => {
            const toggled = toggleTask(text, offset)
            if (toggled !== null) {
              onChange(toggled)
            }
          }}
        />
      )
    }
  }

  return (
    <Box
      className="markdown-body"
      sx={{
        '&.markdown-body': {
          '& h1, & h2': { pb: 0, borderBottom: 'none' },
          '& h1': { fontSize: '1.5em' },
          '& h2': { fontSize: '1.25em' },
          '& h3': { fontSize: '1em' },
          '& h4': { fontSize: '.875em' },
          '& h5': { fontSize: '.85em', color: '#59636e' },
        },
      }}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkBreaks, remarkTicketLink]}
        rehypePlugins={[rehypeSanitize, rehypeHighlight, rehypeTaskList, rehypeExternalLink]}
        components={components}
      >
        {text}
      </ReactMarkdown>
    </Box>
  )
}
