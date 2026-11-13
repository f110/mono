import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router'
import { AdminLayout } from './components/AdminLayout.tsx'
import { Layout } from './components/Layout.tsx'
import { ComponentsPage } from './pages/ComponentsPage.tsx'
import { NewTicketPage } from './pages/NewTicketPage.tsx'
import { TemplatesPage } from './pages/TemplatesPage.tsx'
import { TicketListPage } from './pages/TicketListPage.tsx'
import { TicketPage } from './pages/TicketPage.tsx'

const rootRoute = createRootRoute({
  component: Layout,
  notFoundComponent: () => <p>Not Found</p>,
})

const listRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  validateSearch: (
    search: Record<string, unknown>,
  ): { filter?: string; status?: string[]; component?: string; order_by?: string; pages?: string[] } => ({
    filter: typeof search.filter === 'string' ? search.filter : undefined,
    status: Array.isArray(search.status) && search.status.every((v) => typeof v === 'string') ? search.status : undefined,
    component: typeof search.component === 'string' ? search.component : undefined,
    order_by: typeof search.order_by === 'string' ? search.order_by : undefined,
    pages: Array.isArray(search.pages) && search.pages.every((v) => typeof v === 'string') ? search.pages : undefined,
  }),
  component: TicketListPage,
})

const newRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/new',
  validateSearch: (search: Record<string, unknown>): { template?: number } => ({
    template: typeof search.template === 'number' ? search.template : undefined,
  }),
  component: NewTicketPage,
})

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin',
  component: AdminLayout,
})

const adminIndexRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/admin/components' })
  },
})

const componentsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/components',
  component: ComponentsPage,
})

const templatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/templates',
  component: TemplatesPage,
})

const ticketRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/$number',
  component: TicketPage,
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    listRoute,
    newRoute,
    adminRoute.addChildren([adminIndexRoute, componentsRoute, templatesRoute]),
    ticketRoute,
  ]),
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
