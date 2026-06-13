import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'

export interface ServerInfo {
  supportedBazelVersions: string[]
  schemaVersion: string
}

export function useGetServerInfo(): ServerInfo {
  const res = useQuery(
    BFF.method.getServerInfo,
    create(BFF.method.getServerInfo.input),
  )
  if (!res.data) {
    return { supportedBazelVersions: [], schemaVersion: '' }
  }
  return {
    supportedBazelVersions: res.data.supportedBazelVersions,
    schemaVersion: res.data.schemaVersion,
  }
}
