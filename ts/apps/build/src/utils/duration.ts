import { type Duration, durationMs } from '@bufbuild/protobuf/wkt'
import dayjs from 'dayjs'

export function formatDuration(d: Duration | undefined): string {
  if (!d) {
    return ''
  }

  const v = dayjs.duration(durationMs(d))
  return (
    (v.hours() > 0 ? v.hours() + 'h' : '') +
    (v.minutes() > 0 ? v.minutes() + 'm' : '') +
    (v.seconds() > 0 ? v.seconds() + 's' : '')
  )
}
