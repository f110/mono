import { type Ref, useState } from 'react'
import { Box, Tab, Tabs, TextField } from '@mui/material'
import { Markdown } from './Markdown.tsx'

export const MarkdownEditor = ({
  value,
  onChange,
  minRows = 6,
  autoFocus,
  inputRef,
}: {
  value: string
  onChange: (value: string) => void
  minRows?: number
  autoFocus?: boolean
  inputRef?: Ref<HTMLTextAreaElement>
}) => {
  const [tab, setTab] = useState<'write' | 'preview'>('write')

  return (
    <Box>
      <Tabs
        value={tab}
        onChange={(_, v: 'write' | 'preview') => setTab(v)}
        sx={{ minHeight: 36, '& .MuiTab-root': { minHeight: 36, minWidth: 0, px: 2, py: 0.5 } }}
      >
        <Tab label="編集" value="write" />
        <Tab label="プレビュー" value="preview" />
      </Tabs>
      {tab === 'write' ? (
        <TextField
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== 'Enter' || !(e.metaKey || e.ctrlKey) || e.nativeEvent.isComposing || e.nativeEvent.keyCode === 229) {
              return
            }
            e.preventDefault()
            const form = (e.target as HTMLTextAreaElement).form
            const submitter = form?.querySelector<HTMLButtonElement>('button[type=submit]')
            if (form && submitter && !submitter.disabled) {
              form.requestSubmit(submitter)
            }
          }}
          placeholder="Cmd/Ctrl+Enter で送信"
          autoFocus={autoFocus}
          inputRef={inputRef}
          multiline
          minRows={minRows}
          fullWidth
          sx={{ mt: 1 }}
        />
      ) : (
        <Box sx={{ minHeight: minRows * 24, p: 1 }}>
          <Markdown text={value} />
        </Box>
      )}
    </Box>
  )
}
