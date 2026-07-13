import { For } from "solid-js";
import type { RoomInfo } from "@/stores/socketStore";

interface ServerStatsProps {
	rooms: RoomInfo[];
	totalConnections: number;
}

const ServerStats = (props: ServerStatsProps) => {
	return (
		<section class="rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
			<div class="mb-4">
				<h2 class="text-lg font-semibold">服务端统计</h2>
				<p class="text-sm text-base-content/60">
					按在线人数展示当前最活跃的房间
				</p>
			</div>
			<div class="mb-4 rounded-lg bg-base-200 px-4 py-3 text-sm text-base-content/70">
				总连接数 {props.totalConnections}
			</div>
			<div class="space-y-3">
				<For each={props.rooms}>
					{(room, index) => (
						<div class="rounded-lg border border-base-300/70 px-4 py-3">
							<div class="flex items-center justify-between gap-4">
								<div>
									<div class="text-sm text-base-content/50">
										TOP {index() + 1}
									</div>
									<div class="font-medium">{room.name}</div>
								</div>
								<div class="text-right">
									<div class="text-lg font-semibold">{room.count}</div>
									<div class="text-xs text-base-content/50">
										容量 {room.limit}
									</div>
								</div>
							</div>
							<progress
								class="progress progress-primary mt-3 h-2 w-full"
								max={Math.max(room.limit, 1)}
								value={room.count}
							/>
						</div>
					)}
				</For>
			</div>
		</section>
	);
};

export default ServerStats;
