import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'

export function useGetServerInfo(): string[] {
  const res = useQuery(
    BFF.method.getServerInfo,
    create(BFF.method.getServerInfo.input),
  )
  if (!res.data) {
    return []
  }
  return res.data.supportedBazelVersions
}
