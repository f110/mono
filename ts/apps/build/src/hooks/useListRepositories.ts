import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'
import type { Repository } from '../model/msg_pb'

export function useListRepositories(): Repository[] {
  const res = useQuery(
    BFF.method.listRepositories,
    create(BFF.method.listRepositories.input),
  )
  if (!res.data) {
    return []
  }
  return res.data.repositories
}
