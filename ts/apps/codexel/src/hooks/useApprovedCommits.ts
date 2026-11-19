import { useEffect } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { ReviewService } from '../connect/codexel_pb.js'

// useApprovedCommits returns the commit IDs which are approved in the reviews of the repository. They are read again
// when operation changes because the commits are rewritten (e.g. rebased).
export const useApprovedCommits = (path: string | undefined, operation: string | undefined): Set<string> => {
  const { data, refetch } = useQuery(ReviewService.method.listApprovedCommits, { path }, { enabled: !!path })
  useEffect(() => {
    if (path) {
      refetch()
    }
  }, [path, operation, refetch])
  return new Set(data?.commitIds ?? [])
}
