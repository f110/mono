import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { AgentService, type TranscriptEntry } from '../connect/codexel_pb.js'
import { transport } from '../providers/AppProvider.tsx'

const retryInterval = 3000

export type Transcript = {
  title: string
  lastPrompt: string
  entries: TranscriptEntry[]
}

// useTranscript follows the transcript of the session of Claude Code by WatchTranscript. It reconnects when the stream
// ends.
export const useTranscript = (sessionId: string): { transcript: Transcript | undefined; error: Error | undefined } => {
  const [transcript, setTranscript] = useState<Transcript>()
  const [error, setError] = useState<Error>()

  useEffect(() => {
    if (!sessionId) {
      return
    }
    const abort = new AbortController()
    const client = createClient(AgentService, transport)
    const watch = async () => {
      while (!abort.signal.aborted) {
        // The stream sends all entries at first, so the entries are collected again on every connection.
        let entries: TranscriptEntry[] = []
        try {
          for await (const res of client.watchTranscript({ sessionId }, { signal: abort.signal })) {
            entries = [...entries, ...res.entries]
            setTranscript({ title: res.title, lastPrompt: res.lastPrompt, entries })
            setError(undefined)
          }
        } catch (e) {
          if (abort.signal.aborted) {
            return
          }
          setError(e instanceof Error ? e : new Error(String(e)))
        }
        await new Promise((resolve) => setTimeout(resolve, retryInterval))
      }
    }
    watch()
    return () => abort.abort()
  }, [sessionId])

  return { transcript, error }
}
