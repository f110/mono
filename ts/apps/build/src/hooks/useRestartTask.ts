import { createConnectQueryKey, useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { BFF } from '../connect/bff_pb'

export function useRestartTask(): ReturnType<typeof useMutation> {
  const queryClient = useQueryClient()

  return useMutation(BFF.method.restartTask, {
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: BFF.method.listTasks,
          cardinality: 'finite',
        }),
      })
    },
  })
}
