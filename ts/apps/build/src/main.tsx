import dayjs from 'dayjs'
import duration from 'dayjs/plugin/duration'
import relativeTime from 'dayjs/plugin/relativeTime'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { RouterProvider } from '@tanstack/react-router'
import { AppProvider } from './providers/AppProvider.tsx'
import { router } from './route.tsx'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'

dayjs.extend(duration)
dayjs.extend(relativeTime)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppProvider>
      <RouterProvider router={router} />
      <TanStackRouterDevtools router={router} />
      <ReactQueryDevtools initialIsOpen={false} />
    </AppProvider>
  </StrictMode>,
)
