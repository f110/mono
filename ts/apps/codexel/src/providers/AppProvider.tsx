import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

export const transport = createConnectTransport({ baseUrl: window.location.origin })
const queryClient = new QueryClient()
// theme holds the styles shared by all screens. Do not override them in each component.
const theme = createTheme({
  typography: {
    button: { textTransform: 'none' },
  },
  components: {
    MuiTabs: { styleOverrides: { root: { minHeight: 36 } } },
    MuiTab: { styleOverrides: { root: { minHeight: 36 } } },
    MuiChip: { styleOverrides: { sizeSmall: { height: 18 } } },
  },
})

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
