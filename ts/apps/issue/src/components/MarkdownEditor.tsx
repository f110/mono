import { useState } from 'react'
import { Box, Tab, Tabs, TextField } from '@mui/material'
import { Markdown } from './Markdown.tsx'

export const MarkdownEditor = ({
  value,
  onChange,
  minRows = 6,
}: {
  value: string
  onChange: (value: string) => void
  minRows?: number
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
        <TextField value={value} onChange={(e) => onChange(e.target.value)} multiline minRows={minRows} fullWidth sx={{ mt: 1 }} />
      ) : (
        <Box sx={{ minHeight: minRows * 24, p: 1 }}>
          <Markdown text={value} />
        </Box>
      )}
    </Box>
  )
}
