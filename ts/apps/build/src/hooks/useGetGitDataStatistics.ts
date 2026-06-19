import { useQuery } from '@connectrpc/connect-query'
import { BFF, type ResponseGetGitDataStatistics } from '../connect/bff_pb'

// useGetGitDataStatistics fetches per-repository statistics lazily. The query
// only runs while `enabled` is true (e.g. when the table row is expanded) so
// the potentially expensive commit count is not computed for every repository
// on the list page.
export function useGetGitDataStatistics(
  repo: string,
  enabled: boolean,
): { data?: ResponseGetGitDataStatistics; isLoading: boolean } {
  const res = useQuery(BFF.method.getGitDataStatistics, { repo }, { enabled })
  return { data: res.data, isLoading: res.isLoading }
}
