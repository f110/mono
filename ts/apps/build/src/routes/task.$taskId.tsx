import { createFileRoute } from '@tanstack/react-router'
import { TaskPage } from '../pages/task'

export const Route = createFileRoute('/task/$taskId')({
  component: () => <TaskPage />,
  notFoundComponent: () => <p>The task is not found</p>,
})
