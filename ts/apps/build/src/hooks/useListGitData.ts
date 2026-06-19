import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF, type GitDataRepository } from '../connect/bff_pb'

export function useListGitData(): GitDataRepository[] {
  const res = useQuery(
    BFF.method.listGitData,
    create(BFF.method.listGitData.input),
  )
  if (!res.data) {
    return []
  }
  return res.data.repositories
}
