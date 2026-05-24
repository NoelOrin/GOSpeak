import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/room/$roomId')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/room/$roomId"!</div>
}
