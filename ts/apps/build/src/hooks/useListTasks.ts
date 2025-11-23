import { useQuery } from '@connectrpc/connect-query'
import { BFF, type BFFTask } from '../connect/bff_pb'

export function useListTasks(repositoryId: number | undefined): BFFTask[] {
  const res = useQuery(BFF.method.listTasks, { repositoryId: repositoryId })
  if (!res.data) {
    return []
  }
  return res.data.tasks
}
