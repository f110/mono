import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { AgentService, type Agent } from '../connect/codexel_pb.js'
import { transport } from '../providers/AppProvider.tsx'

const retryInterval = 3000

// useAgents returns the agents and keeps them up to date by WatchAgents. It reconnects when the stream ends.
export const useAgents = (): { agents: Agent[] | undefined; error: Error | undefined } => {
  const [agents, setAgents] = useState<Agent[]>()
  const [error, setError] = useState<Error>()

  useEffect(() => {
    const abort = new AbortController()
    const client = createClient(AgentService, transport)
    const watch = async () => {
      while (!abort.signal.aborted) {
        try {
          for await (const res of client.watch({}, { signal: abort.signal })) {
            setAgents(res.agents)
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
  }, [])

  return { agents, error }
}
