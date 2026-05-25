import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/link/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/link/"!</div>
}
