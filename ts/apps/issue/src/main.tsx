import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { AppProvider } from './providers/AppProvider.tsx'
import { router } from './route.tsx'

dayjs.extend(relativeTime)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppProvider>
      <RouterProvider router={router} />
    </AppProvider>
  </StrictMode>,
)
