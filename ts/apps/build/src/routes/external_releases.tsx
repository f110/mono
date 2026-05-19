import { createFileRoute } from '@tanstack/react-router'
import { ExternalReleaseTriggersPage } from '../pages/external_releases'

export const Route = createFileRoute('/external_releases')({
  component: () => <ExternalReleaseTriggersPage />,
})
