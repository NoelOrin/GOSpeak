import { createFileRoute } from '@tanstack/solid-router'
import RoomDetail from '@/components/room/roomDetail'


export const Route = createFileRoute('/(app)/channel/')({
  component: RouteComponent,
  staticData: {
    title: '频道',
    icon: 'icon-channel'
  }
})
// cm-split-handler cm-split-handler-v split-pane css TODO
function RouteComponent() {
  return (
    <div class="flex h-full">
          <RoomDetail />
    </div>
  );
}
