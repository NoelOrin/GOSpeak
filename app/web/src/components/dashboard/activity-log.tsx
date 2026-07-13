import dayjs from "dayjs";
import { For, Show } from "solid-js";
import type { ActivityEvent } from "@/stores/socketStore";

interface ActivityLogProps {
	items: ActivityEvent[];
}

const activityText = (item: ActivityEvent) => {
	switch (item.type) {
		case "member_joined":
			return `${item.identity || "成员"} 加入了 ${item.room}`;
		case "member_left":
			return `${item.identity || "成员"} 离开了 ${item.room}`;
		case "room_joined":
			return `你已进入 ${item.room}`;
		case "room_left":
			return `你已离开 ${item.room}`;
	}
};

const ActivityLog = (props: ActivityLogProps) => {
	return (
		<section class="rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
			<div class="mb-4 flex items-center justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">最近活动</h2>
					<p class="text-sm text-base-content/60">展示最近的房间和成员变更</p>
				</div>
				<div class="badge badge-ghost">{props.items.length} 条</div>
			</div>
			<Show
				when={props.items.length > 0}
				fallback={
					<div class="rounded-lg bg-base-200/70 px-4 py-8 text-center text-sm text-base-content/50">
						暂无活动记录
					</div>
				}
			>
				<div class="space-y-3">
					<For each={props.items}>
						{(item) => (
							<div class="flex items-start gap-3 rounded-lg border border-base-300/70 px-3 py-3">
								<div class="mt-1 h-2.5 w-2.5 rounded-full bg-primary" />
								<div class="min-w-0 flex-1">
									<div class="text-sm text-base-content">
										{activityText(item)}
									</div>
									<div class="mt-1 text-xs text-base-content/50">
										{dayjs(item.timestamp).format("HH:mm:ss")}
									</div>
								</div>
							</div>
						)}
					</For>
				</div>
			</Show>
		</section>
	);
};

export default ActivityLog;
