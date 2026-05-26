import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'
import type { GithubEvent } from '../model/msg_pb'

export function useListGithubEvents(): GithubEvent[] {
  const res = useQuery(
    BFF.method.listGithubEvents,
    create(BFF.method.listGithubEvents.input),
  )
  if (!res.data) {
    return []
  }
  return res.data.events
}
