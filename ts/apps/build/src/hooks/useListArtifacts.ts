import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'

export interface ListArtifactsParams {
  taskId?: number
  repositoryId?: number
  revision?: string
}

// useListArtifacts fetches the artifacts for a single task (taskId) or for a
// commit (repositoryId + revision). The query stays disabled until one of the
// two selectors is fully specified.
export function useListArtifacts(params: ListArtifactsParams) {
  const enabled =
    params.taskId !== undefined ||
    (params.repositoryId !== undefined && !!params.revision)
  return useQuery(
    BFF.method.listArtifacts,
    {
      taskId: params.taskId,
      repositoryId: params.repositoryId,
      revision: params.revision,
    },
    { enabled },
  )
}
