import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { QueryClientProvider, QueryClient } from '@tanstack/react-query'

export const AppProvider = ({ children }: { children: React.ReactNode }) => {
  const transport = createConnectTransport({
    baseUrl: import.meta.env.VITE_BFF_URL,
  })
  const queryClient = new QueryClient()

  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  )
}
