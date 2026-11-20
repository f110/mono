import AdjustIcon from '@mui/icons-material/Adjust'
import BlockIcon from '@mui/icons-material/Block'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import { TicketStatus } from '../connect/issue_pb'
import { statusColors, statusLabels } from '../labels.ts'

const icons = {
  [TicketStatus.OPEN]: AdjustIcon,
  [TicketStatus.NEED_INVESTIGATION]: AdjustIcon,
  [TicketStatus.IN_PROGRESS]: AdjustIcon,
  [TicketStatus.IN_REVIEW]: AdjustIcon,
  [TicketStatus.DONE]: CheckCircleOutlineIcon,
  [TicketStatus.WONT_FIX]: BlockIcon,
}

// TicketStatusIcon shows the status of the ticket as the icon in the size of the surrounding text. It shows nothing if
// the status is unspecified.
export const TicketStatusIcon = ({ status }: { status: TicketStatus }) => {
  if (status === TicketStatus.UNSPECIFIED) {
    return null
  }
  const Icon = icons[status]
  return (
    <Icon
      titleAccess={statusLabels[status]}
      sx={{ fontSize: 'inherit', verticalAlign: '-0.125em', color: statusColors[status].backgroundColor ?? 'text.secondary' }}
    />
  )
}
