import { createFileRoute } from '@tanstack/react-router'
import { GitDataPage } from '../pages/git_data'

export const Route = createFileRoute('/git_data')({
  component: () => <GitDataPage />,
})
