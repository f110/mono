import { createFileRoute } from '@tanstack/react-router'
import { EventPage } from '../pages/events/$eventId'

export const Route = createFileRoute('/events/$eventId')({
  component: () => <EventPage />,
  notFoundComponent: () => <p>The event is not found</p>,
})
