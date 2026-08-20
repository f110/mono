import { createFileRoute } from '@tanstack/react-router'
import { ArtifactsPage } from '../pages/artifacts'

export const Route = createFileRoute('/artifacts')({
  component: () => <ArtifactsPage />,
})
