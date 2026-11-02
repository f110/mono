import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { Layout } from './components/Layout.tsx'
import { NewTicketPage } from './pages/NewTicketPage.tsx'
import { TicketListPage } from './pages/TicketListPage.tsx'
import { TicketPage } from './pages/TicketPage.tsx'

const rootRoute = createRootRoute({
  component: Layout,
  notFoundComponent: () => <p>Not Found</p>,
})

const listRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  validateSearch: (search: Record<string, unknown>): { filter?: string; component?: string; order_by?: string } => ({
    filter: typeof search.filter === 'string' ? search.filter : undefined,
    component: typeof search.component === 'string' ? search.component : undefined,
    order_by: typeof search.order_by === 'string' ? search.order_by : undefined,
  }),
  component: TicketListPage,
})

const newRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/new',
  component: NewTicketPage,
})

const ticketRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/$number',
  component: TicketPage,
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([listRoute, newRoute, ticketRoute]),
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
