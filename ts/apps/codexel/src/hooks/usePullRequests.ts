import { useQuery } from '@connectrpc/connect-query'
import { PullRequestService } from '../connect/codexel_pb.js'

// usePullRequests returns the URLs of the pull requests submitted by Codexel by the change ID.
export const usePullRequests = (path: string | undefined): { pullRequests: Map<string, string>; refetch: () => void } => {
  const { data, refetch } = useQuery(PullRequestService.method.list, { path }, { enabled: !!path })
  const pullRequests = new Map<string, string>()
  for (const pr of data?.pullRequests ?? []) {
    pullRequests.set(pr.changeId, pr.url)
  }
  return { pullRequests, refetch }
}
