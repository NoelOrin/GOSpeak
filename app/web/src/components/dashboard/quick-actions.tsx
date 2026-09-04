import { useNavigate } from "@tanstack/solid-router";
import ArrowRight from "lucide-solid/icons/arrow-right";
import CirclePlus from "lucide-solid/icons/circle-plus";
import Radio from "lucide-solid/icons/radio";
import { createSignal } from "solid-js";
import { showToast } from "solid-notifications";
import CreateRoomModal from "@/components/modal/createRoomModal";
import { socketStore } from "@/stores/socketStore";

interface QuickActionsProps {
	compact?: boolean;
}

const QuickActions = (props: QuickActionsProps) => {
	const navigate = useNavigate();
	let createRoomModalRef!: HTMLDialogElement;
	const [createRoomDomain, setCreateRoomDomain] = createSignal("");

	const handleCreateRoom = () => {
		const domainUUID = socketStore.currentDomainUUID();
		if (!domainUUID) {
			showToast("请先选择域", { type: "warning" });
			return;
		}
		setCreateRoomDomain(domainUUID);
		createRoomModalRef?.showModal?.();
	};

	const closeCreateRoomModal = () => {
		createRoomModalRef?.close?.();
	};

	const handleJoinRandom = () => {
		const domainUUID = socketStore.currentDomainUUID();
		if (!domainUUID) {
			showToast("请先选择域", { type: "warning" });
			return;
		}
		const rooms = socketStore
			.rooms()
			.filter(
				(room) =>
					room.count < room.limit && (room.domain_uuid || "") === domainUUID,
			);
		if (rooms.length === 0) {
			showToast("当前域没有可加入的房间", { type: "warning" });
			return;
		}
		const target = [...rooms].sort((a, b) => b.count - a.count)[0];
		socketStore.selectRoom(target);
		const targetDomainUUID = target.domain_uuid;
		if (targetDomainUUID)
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: targetDomainUUID },
			});
		else navigate({ to: "/discover" });
	};

	return (
		<>
			<section class="min-w-0 rounded-lg bg-transparent px-2 py-2">
				<div class="mb-5 min-w-0">
					<h2 class="text-lg font-semibold">快捷入口</h2>
					<p class="text-sm text-base-content/60">
						{props.compact ? "常用操作" : "把常用操作放在首页，减少跳转成本"}
					</p>
				</div>
				<div
					class={
						props.compact
							? "grid min-w-0 gap-2"
							: "grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4"
					}
				>
					<button
						type="button"
						class={`btn min-w-0 justify-between overflow-hidden rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={handleCreateRoom}
					>
						<div class="min-w-0 flex-1">
							<div class="truncate font-medium">快速创建房间</div>
							<div class="mt-1 truncate text-xs text-base-content/60">
								填写配置后创建新房间
							</div>
						</div>
						<CirclePlus size={20} class="shrink-0" />
					</button>
					<button
						type="button"
						class={`btn min-w-0 justify-between overflow-hidden rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={handleJoinRandom}
					>
						<div class="min-w-0 flex-1">
							<div class="truncate font-medium">随机加入房间</div>
							<div class="mt-1 truncate text-xs text-base-content/60">
								优先选择当前最活跃房间
							</div>
						</div>
						<Radio size={20} class="shrink-0" />
					</button>
					<button
						type="button"
						class={`btn min-w-0 justify-between overflow-hidden rounded-lg border-0 bg-base-200 px-4 text-left normal-case hover:bg-base-300 ${props.compact ? "h-18" : "h-24"}`}
						onClick={() => {
							const uuid = socketStore.currentDomainUUID();
							if (uuid)
								navigate({
									to: "/domain/$domainUUID",
									params: { domainUUID: uuid },
								});
							else navigate({ to: "/discover" });
						}}
					>
						<div class="min-w-0 flex-1">
							<div class="truncate font-medium">前往域</div>
							<div class="mt-1 truncate text-xs text-base-content/60">
								查看全部房间与在线成员
							</div>
						</div>
						<ArrowRight size={20} class="shrink-0" />
					</button>
				</div>
			</section>
			<CreateRoomModal
				ref={createRoomModalRef}
				domainUUID={createRoomDomain()}
				onClose={closeCreateRoomModal}
			/>
		</>
	);
};

export default QuickActions;
