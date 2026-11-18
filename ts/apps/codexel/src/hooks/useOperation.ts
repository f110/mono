import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { RepositoryService } from '../connect/codexel_pb.js'
import { transport } from '../providers/AppProvider.tsx'

const retryInterval = 3000

// useOperation returns the ID of the current operation of the repository. It changes every time the repository
// changes, so the data of the repository should be read again when it changes.
export const useOperation = (path: string | undefined): string | undefined => {
  const [operation, setOperation] = useState<string>()

  useEffect(() => {
    if (!path) {
      return
    }
    const abort = new AbortController()
    const client = createClient(RepositoryService, transport)
    const watch = async () => {
      while (!abort.signal.aborted) {
        try {
          for await (const res of client.watchRepository({ path }, { signal: abort.signal })) {
            setOperation(res.operationId)
          }
        } catch {
          if (abort.signal.aborted) {
            return
          }
        }
        await new Promise((resolve) => setTimeout(resolve, retryInterval))
      }
    }
    watch()
    return () => abort.abort()
  }, [path])

  return operation
}
