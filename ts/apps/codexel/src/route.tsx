import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router'
import { Layout } from './components/Layout.tsx'
import { AgentsPage } from './pages/AgentsPage.tsx'
import { RepositoriesPage } from './pages/RepositoriesPage.tsx'

const rootRoute = createRootRoute({
  component: Layout,
  notFoundComponent: () => <p>Not Found</p>,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/agents' })
  },
})

const agentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/agents',
  validateSearch: (search: Record<string, unknown>): { pane?: string } => ({
    pane: typeof search.pane === 'string' ? search.pane : undefined,
  }),
  component: AgentsPage,
})

const repositoriesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/repositories',
  validateSearch: (search: Record<string, unknown>): { path?: string } => ({
    path: typeof search.path === 'string' ? search.path : undefined,
  }),
  component: RepositoriesPage,
})

const routeTree = rootRoute.addChildren([indexRoute, agentsRoute, repositoriesRoute])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
