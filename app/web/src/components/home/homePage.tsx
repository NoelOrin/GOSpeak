import { useNavigate } from "@tanstack/solid-router";
import Headphones from "lucide-solid/icons/headphones";
import MessageSquare from "lucide-solid/icons/message-square";
import { createMemo, For, onMount, Show } from "solid-js";
import QuickActions from "@/components/dashboard/quick-actions";
import { socketStore } from "@/stores/socketStore";
import type { RoomInfo } from "@/socket/types";

const HomePage = () => {
	const navigate = useNavigate();

	onMount(() => {
		socketStore.connect();
		socketStore.listRooms();
	});

	const recentRooms = createMemo(() =>
		[...socketStore.rooms()].sort((a, b) => b.count - a.count).slice(0, 5),
	);

	const openRoom = (room: RoomInfo) => {
		socketStore.selectRoom(room);
		const domainUUID = room.domain_uuid;
		if (domainUUID) {
			void navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID },
			});
			return;
		}
		void navigate({ to: "/discover" });
	};

	return (
		<div class="flex h-full min-h-0 min-w-0 flex-col overflow-y-auto bg-base-100">
			<div class="min-w-0 border-b border-base-300 px-4 pb-3 pt-4">
				<h2 class="text-base font-semibold">快捷入口</h2>
				<p class="mt-1 text-xs leading-5 text-base-content/60">
					创建房间、进入最近频道，快速开始语音或文字会话。
				</p>
			</div>

			<div class="min-w-0 px-2 py-1">
				<QuickActions compact />
			</div>

			<section class="px-2 pb-4">
				<div class="flex items-center justify-between px-2 pb-1 pt-3">
					<span class="text-xs font-semibold text-base-content/60">
						最近房间
					</span>
					<span class="text-[10px] text-base-content/40">
						{socketStore.rooms().length} 个在线
					</span>
				</div>
				<Show
					when={recentRooms().length > 0}
					fallback={
						<div class="px-2 py-4 text-center text-xs text-base-content/40">
							暂无在线房间
						</div>
					}
				>
					<div class="space-y-1">
						<For each={recentRooms()}>
							{(room) => (
								<button
									type="button"
									class="flex w-full items-center gap-3 rounded-lg border border-transparent px-3 py-2 text-left transition-colors hover:border-base-300 hover:bg-base-200"
									onClick={() => openRoom(room)}
								>
									<span class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-base-200 text-base-content/70">
										{room.type === "text" ? (
											<MessageSquare size={16} />
										) : (
											<Headphones size={16} />
										)}
									</span>
									<span class="min-w-0 flex-1">
										<span class="block truncate text-sm font-medium">
											{room.name}
										</span>
										<span class="block text-xs text-base-content/45">
											{room.count}/{room.limit} 人
										</span>
									</span>
								</button>
							)}
						</For>
					</div>
				</Show>
			</section>
		</div>
	);
};
export default HomePage;
