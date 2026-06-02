import { type Timestamp, timestampMs } from '@bufbuild/protobuf/wkt'
import type { Duration } from '@bufbuild/protobuf/wkt'
import { useEffect, useState } from 'react'
import { formatDuration, formatDurationMs } from '../utils/duration.ts'

type Args = {
  startAt?: Timestamp
  finishedAt?: Timestamp
  duration?: Duration
}

export function useLiveDuration({
  startAt,
  finishedAt,
  duration,
}: Args): string {
  const running = !!startAt && !finishedAt
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!running) {
      return
    }
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [running])

  if (running) {
    return formatDurationMs(now - timestampMs(startAt))
  }
  return formatDuration(duration)
}
