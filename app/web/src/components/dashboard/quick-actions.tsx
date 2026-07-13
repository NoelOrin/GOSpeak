import { useNavigate } from "@tanstack/solid-router";
import ArrowRight from "lucide-solid/icons/arrow-right";
import CirclePlus from "lucide-solid/icons/circle-plus";
import Radio from "lucide-solid/icons/radio";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { createMemo } from "solid-js";
import { showToast } from "solid-notifications";
import CreateRoomModal from "@/components/modal/createRoomModal";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";

interface QuickActionsProps {
	compact?: boolean;
}

const QuickActions = (props: QuickActionsProps) => {
	const navigate = useNavigate();
	const isAdmin = createMemo(() => userStore.user()?.role === "admin");
	let createRoomModalRef!: HTMLDialogElement;

	const handleCreateRoom = () => {
		createRoomModalRef?.showModal?.();
	};

	const closeCreateRoomModal = () => {
		createRoomModalRef?.close?.();
	};

	const handleJoinRandom = () => {
		const rooms = socketStore.rooms().filter((room) => room.count < room.limit);
		if (rooms.length === 0) {
			showToast("当前没有可加入的房间", { type: "warning" });
			return;
		}
		const target = [...rooms].sort((a, b) => b.count - a.count)[0];
		socketStore.selectRoom(target);
		navigate({ to: "/channel" });
	};

	return (
		<>
			<section class="rounded-lg bg-transparent px-2 py-2">
				<div class="mb-5">
					<h2 class="text-lg font-semibold">快捷入口</h2>
					<p class="text-sm text-base-content/60">
						{props.compact ? "常用操作" : "把常用操作放在首页，减少跳转成本"}
					</p>
				</div>
				<div
					class={
						props.compact
							? "grid gap-2"
							: "grid gap-3 sm:grid-cols-2 xl:grid-cols-4"
					}
				>
					<button
						type="button"
						class={`btn justify-between rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={handleCreateRoom}
					>
						<div>
							<div class="font-medium">快速创建房间</div>
							<div class="mt-1 text-xs text-base-content/60">
								填写配置后创建新房间
							</div>
						</div>
						<CirclePlus size={20} />
					</button>
					<button
						type="button"
						class={`btn justify-between rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={handleJoinRandom}
					>
						<div>
							<div class="font-medium">随机加入房间</div>
							<div class="mt-1 text-xs text-base-content/60">
								优先选择当前最活跃房间
							</div>
						</div>
						<Radio size={20} />
					</button>
					<button
						type="button"
						class={`btn justify-between rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={() => navigate({ to: "/channel" })}
					>
						<div>
							<div class="font-medium">前往频道列表</div>
							<div class="mt-1 text-xs text-base-content/60">
								查看全部房间与在线成员
							</div>
						</div>
						<ArrowRight size={20} />
					</button>
					{isAdmin() ? (
						<button
							type="button"
							class={`btn justify-between rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
							onClick={() => navigate({ to: "/manage/permission" })}
						>
							<div>
								<div class="font-medium">权限管理</div>
								<div class="mt-1 text-xs text-base-content/60">
									进入权限与禁言管理页面
								</div>
							</div>
							<ShieldCheck size={20} />
						</button>
					) : null}
				</div>
			</section>
			<CreateRoomModal
				ref={createRoomModalRef}
				onClose={closeCreateRoomModal}
			/>
		</>
	);
};

export default QuickActions;
