import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { Tooltip } from '@mui/material'
import dayjs from 'dayjs'

// RelativeTime shows the time relative to now and the absolute time on hover.
export const RelativeTime = ({ time }: { time?: Timestamp }) => {
  if (!time) {
    return null
  }
  const t = dayjs(timestampDate(time))
  return (
    <Tooltip title={t.format('YYYY-MM-DD HH:mm:ss')}>
      <time dateTime={t.toISOString()}>{t.fromNow()}</time>
    </Tooltip>
  )
}
