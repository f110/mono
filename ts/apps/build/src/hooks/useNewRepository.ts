import { createConnectQueryKey, useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { BFF } from '../connect/bff_pb'

export function useNewRepository(): ReturnType<typeof useMutation> {
  const queryClient = useQueryClient()

  return useMutation(BFF.method.saveRepository, {
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: BFF.method.listRepositories,
          cardinality: 'finite',
        }),
      })
    },
  })
}
