import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'
import type { ExternalReleaseTrigger } from '../model/msg_pb'

export function useListExternalReleaseTriggers(): ExternalReleaseTrigger[] {
  const res = useQuery(
    BFF.method.listExternalReleaseTriggers,
    create(BFF.method.listExternalReleaseTriggers.input),
  )
  if (!res.data) {
    return []
  }
  return res.data.triggers
}
