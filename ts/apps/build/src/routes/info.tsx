import { createFileRoute } from '@tanstack/react-router'
import { InfoPage } from '../pages/info'

export const Route = createFileRoute('/info')({
  component: () => <InfoPage />,
})
