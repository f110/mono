import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { QueryClientProvider, QueryClient } from '@tanstack/react-query'

const transport = createConnectTransport({ baseUrl: window.location.origin + '/bff' })

export const AppProvider = ({ children }: { children: React.ReactNode }) => {
  const queryClient = new QueryClient()

  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  )
}
