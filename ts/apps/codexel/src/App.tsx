import { CssBaseline, ThemeProvider, Typography, createTheme } from '@mui/material'

const theme = createTheme({ colorSchemes: { dark: true } })

export function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Typography variant="h6">Codexel</Typography>
    </ThemeProvider>
  )
}
