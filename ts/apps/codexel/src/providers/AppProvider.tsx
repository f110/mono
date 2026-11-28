import * as React from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { createConnectTransport } from '@connectrpc/connect-web'
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

declare module '@mui/material/styles' {
  interface TypographyVariants {
    monospace: string
  }
  interface TypographyVariantsOptions {
    monospace?: string
  }
}

export const transport = createConnectTransport({ baseUrl: window.location.origin })
const queryClient = new QueryClient()
// theme holds the styles shared by all screens. Do not override them in each component.
// typography.monospace is the font of `sx={{ fontFamily: 'monospace' }}`. The generic monospace is resolved to Osaka-Mono, which has no
// bold, under lang="ja".
const theme = createTheme({
  typography: {
    button: { textTransform: 'none' },
    monospace: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
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
