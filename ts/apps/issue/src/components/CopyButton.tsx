import { type ReactNode, useState } from 'react'
import { IconButton, Tooltip } from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import LinkIcon from '@mui/icons-material/Link'

// CopyButton copies text to the clipboard.
const CopyButton = ({ text, label, icon }: { text: string; label: string; icon: ReactNode }) => {
  const [copied, setCopied] = useState(false)
  return (
    <Tooltip title={copied ? 'コピーしました' : label} onClose={() => setCopied(false)}>
      <IconButton size="small" aria-label={label} onClick={() => void navigator.clipboard.writeText(text).then(() => setCopied(true))}>
        {icon}
      </IconButton>
    </Tooltip>
  )
}

const iconSx = { fontSize: '0.875rem' }

// CopyTicketNumberButton copies the ticket number in the form of `#N` to the clipboard.
export const CopyTicketNumberButton = ({ number }: { number: number }) => (
  <CopyButton text={`#${number}`} label={`#${number} をコピー`} icon={<ContentCopyIcon sx={iconSx} />} />
)

// CopyTicketURLButton copies the full URL of the ticket to the clipboard.
export const CopyTicketURLButton = ({ number }: { number: number }) => (
  <CopyButton text={`${window.location.origin}/${number}`} label="URLをコピー" icon={<LinkIcon sx={iconSx} />} />
)
