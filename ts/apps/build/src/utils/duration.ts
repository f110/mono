import { type Duration, durationMs } from '@bufbuild/protobuf/wkt'
import dayjs from 'dayjs'

export function formatDurationMs(ms: number): string {
  const v = dayjs.duration(ms)
  return (
    (v.hours() > 0 ? v.hours() + 'h' : '') +
    (v.minutes() > 0 ? v.minutes() + 'm' : '') +
    (v.seconds() > 0 ? v.seconds() + 's' : '')
  )
}

export function formatDuration(d: Duration | undefined): string {
  if (!d) {
    return ''
  }
  return formatDurationMs(durationMs(d))
}
