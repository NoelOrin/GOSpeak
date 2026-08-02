import AudioLines from "lucide-solid/icons/audio-lines";
import ChartColumn from "lucide-solid/icons/chart-column";
import CircleUserRound from "lucide-solid/icons/circle-user-round";
import HouseWifi from "lucide-solid/icons/house-wifi";
import { createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { type ActivityEvent, socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import ActivityLog from "./activity-log";
import OperationGuide from "./operation-guide";
import ServerStats from "./server-stats";
import StatsCard from "./stats-card";

const MAX_ACTIVITY_ITEMS = 8;

const Dashboard = () => {
	const [activityItems, setActivityItems] = createSignal<ActivityEvent[]>([]);

	onMount(() => {
		socketStore.connect();
		socketStore.listRooms();
		const dispose = socketStore.onActivity((event) => {
			setActivityItems((prev) => [event, ...prev].slice(0, MAX_ACTIVITY_ITEMS));
		});
		onCleanup(dispose);
	});

	const totalMembers = createMemo(() =>
		socketStore.rooms().reduce((sum, room) => sum + room.count, 0),
	);

	const connectedLabel = createMemo(() =>
		socketStore.connected() ? "已连接服务器" : "未连接服务器",
	);

	const currentUser = createMemo(() => userStore.user());

	const rankedRooms = createMemo(() =>
		[...socketStore.rooms()].sort((a, b) => b.count - a.count).slice(0, 5),
	);

	return (
		<div class="h-full overflow-y-auto bg-base-200 min-w-0">
			<div class=" flex w-full flex-col gap-6 px-4 py-6 lg:px-6">
				<section class="rounded-lg border border-base-300 bg-base-100 p-6 shadow-sm">
					<div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
						<div>
							<h1 class="text-2xl font-semibold">控制台</h1>
							<p class="mt-2 max-w-2xl text-sm leading-6 text-base-content/65">
								汇总当前连接状态、房间活跃度和常用操作入口，作为默认首页主视图。
							</p>
						</div>
						<div class="flex items-center gap-2">
							<div
								class={`badge ${socketStore.connected() ? "badge-success" : "badge-warning"}`}
							>
								{connectedLabel()}
							</div>
							<div class="badge badge-ghost">
								当前房间 {socketStore.currentRoom() || "无"}
							</div>
						</div>
					</div>
				</section>

				<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
					<StatsCard
						title="房间总览"
						value={socketStore.rooms().length}
						description={`在线成员 ${totalMembers()} 人`}
						icon={<HouseWifi size={20} />}
						accentClass="text-primary"
					/>
					<StatsCard
						title="连接状态"
						value={socketStore.connected() ? "在线" : "离线"}
						description={
							socketStore.connected()
								? "房间列表已同步"
								: "等待建立 socket 连接"
						}
						icon={<AudioLines size={20} />}
						accentClass={
							socketStore.connected() ? "text-success" : "text-warning"
						}
					/>
					<StatsCard
						title="个人状态"
						value={currentUser()?.display_name || currentUser()?.name || "游客"}
						description={
							currentUser()
								? `${currentUser()?.role || "user"} · ${socketStore.connected() ? "在线" : "离线"}`
								: "尚未获取用户信息"
						}
						icon={<CircleUserRound size={20} />}
						accentClass="text-secondary"
					/>
					<StatsCard
						title="活跃房间"
						value={rankedRooms()[0]?.name || "暂无"}
						description={
							rankedRooms()[0]
								? `${rankedRooms()[0].count} 人在线`
								: "等待房间数据"
						}
						icon={<ChartColumn size={20} />}
						accentClass="text-accent"
					/>
				</div>

				<div class="grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
					<ActivityLog items={activityItems()} />
					<ServerStats
						rooms={rankedRooms()}
						totalConnections={totalMembers()}
					/>
				</div>

				<OperationGuide />
			</div>
		</div>
	);
};

export default Dashboard;
