import { createConnectQueryKey, useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { BFF } from '../connect/bff_pb'

// useCompleteTask forces a task's Job into a terminal state. It is only usable
// in the local kwok dev environment (ServerConfig.emulateJobControl).
export function useCompleteTask(): ReturnType<typeof useMutation> {
  const queryClient = useQueryClient()

  return useMutation(BFF.method.completeTask, {
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
