import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const transport = createConnectTransport({
  baseUrl: window.location.origin,
  fetch: async (input, init) => {
    const res = await fetch(input, init)
    if (res.status === 401) {
      window.location.reload()
    }
    return res
  },
})
const queryClient = new QueryClient()

export const AppProvider = ({ children }: { children: React.ReactNode }) => (
  <TransportProvider transport={transport}>
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  </TransportProvider>
)
