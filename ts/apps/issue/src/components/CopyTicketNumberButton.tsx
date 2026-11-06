import { useState } from 'react'
import { IconButton, Tooltip } from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'

// CopyTicketNumberButton copies the ticket number in the form of `#N` to the clipboard.
export const CopyTicketNumberButton = ({ number }: { number: number }) => {
  const [copied, setCopied] = useState(false)
  const id = `#${number}`
  return (
    <Tooltip title={copied ? 'コピーしました' : `${id} をコピー`} onClose={() => setCopied(false)}>
      <IconButton
        size="small"
        aria-label={`${id} をコピー`}
        onClick={() => void navigator.clipboard.writeText(id).then(() => setCopied(true))}
      >
        <ContentCopyIcon sx={{ fontSize: '0.875rem' }} />
      </IconButton>
    </Tooltip>
  )
}
