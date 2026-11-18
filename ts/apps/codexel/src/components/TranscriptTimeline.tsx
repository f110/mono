import { useLayoutEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Box, Paper, Stack, Typography } from '@mui/material'
import { TranscriptEntryKind, type TranscriptEntry } from '../connect/codexel_pb.js'

const time = (e: TranscriptEntry) => new Date(Number(e.time)).toLocaleTimeString()

const ToolResult = ({ entry }: { entry: TranscriptEntry }) => (
  <Box component="details" sx={{ ml: 2 }}>
    <Box component="summary" sx={{ cursor: 'pointer', color: entry.isError ? 'error.main' : 'text.secondary', fontSize: 12 }}>
      {entry.isError ? 'Error' : 'Result'}
    </Box>
    <Box component="pre" sx={{ m: 0, p: 1, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all', bgcolor: 'action.hover' }}>
      {entry.text}
    </Box>
  </Box>
)

const Entry = ({ entry, result }: { entry: TranscriptEntry; result?: TranscriptEntry }) => {
  switch (entry.kind) {
    case TranscriptEntryKind.USER_PROMPT:
      return (
        <Paper variant="outlined" sx={{ p: 1, bgcolor: 'action.selected' }}>
          <Typography variant="caption" color="text.secondary">
            User · {time(entry)}
          </Typography>
          <Typography component="div" sx={{ whiteSpace: 'pre-wrap' }}>
            {entry.text}
          </Typography>
        </Paper>
      )
    case TranscriptEntryKind.ASSISTANT_TEXT:
      return (
        <Box>
          <Typography variant="caption" color="text.secondary">
            Agent · {time(entry)}
          </Typography>
          <Box sx={{ '& p': { my: 0.5 }, '& pre': { overflow: 'auto' } }}>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{entry.text}</ReactMarkdown>
          </Box>
        </Box>
      )
    case TranscriptEntryKind.TOOL_USE:
      return (
        <Box>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline', fontFamily: 'monospace', fontSize: 13 }}>
            <Typography variant="caption" color="text.secondary">
              {time(entry)}
            </Typography>
            <Box component="span" sx={{ fontWeight: 'bold', flexShrink: 0 }}>
              {entry.toolName}
            </Box>
            <Box component="span" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
              {entry.text}
            </Box>
          </Stack>
          {result && <ToolResult entry={result} />}
        </Box>
      )
    case TranscriptEntryKind.TOOL_RESULT:
      return <ToolResult entry={entry} />
    default:
      return null
  }
}

// TranscriptTimeline shows the prompts, the texts of the agent and the tool calls in order. The result of the tool is
// shown under the call. It follows the end of the timeline only while the view is scrolled to the bottom.
export const TranscriptTimeline = ({ entries }: { entries: TranscriptEntry[] }) => {
  const ref = useRef<HTMLDivElement>(null)
  const atBottom = useRef(true)

  useLayoutEffect(() => {
    const el = ref.current
    if (el && atBottom.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [entries])

  const calls = new Set(entries.filter((e) => e.kind === TranscriptEntryKind.TOOL_USE).map((e) => e.toolUseId))
  const results = new Map(entries.filter((e) => e.kind === TranscriptEntryKind.TOOL_RESULT).map((e) => [e.toolUseId, e]))

  return (
    <Box
      ref={ref}
      onScroll={(e) => {
        const el = e.currentTarget
        atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 4
      }}
      sx={{ height: '100%', overflow: 'auto', p: 1 }}
    >
      <Stack spacing={1}>
        {entries
          .filter((e) => e.kind !== TranscriptEntryKind.TOOL_RESULT || !calls.has(e.toolUseId))
          .map((e, i) => (
            <Entry key={i} entry={e} result={e.kind === TranscriptEntryKind.TOOL_USE ? results.get(e.toolUseId) : undefined} />
          ))}
      </Stack>
    </Box>
  )
}
