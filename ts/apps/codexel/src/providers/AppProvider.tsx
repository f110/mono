import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

export const transport = createConnectTransport({ baseUrl: window.location.origin })
const queryClient = new QueryClient()
const theme = createTheme({ colorSchemes: { dark: true } })

export const AppProvider = ({ children }: { children: React.ReactNode }) => (
  <TransportProvider transport={transport}>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {children}
      </ThemeProvider>
    </QueryClientProvider>
  </TransportProvider>
)
