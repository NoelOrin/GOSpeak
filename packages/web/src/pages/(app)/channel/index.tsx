import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/channel/')({
  component: RouteComponent,
  staticData: {
    title: '频道',
    icon: 'icon-channel'
  }
})

function RouteComponent() {
  return null;
}
