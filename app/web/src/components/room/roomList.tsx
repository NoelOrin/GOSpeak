import clsx from "clsx";
import CirclePlus from "lucide-solid/icons/circle-plus";
import { createSignal, For, onMount, Show } from "solid-js";
import CreateRoomModal from "@/components/modal/createRoomModal";
import { chatStore } from "@/stores/chatStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";
import { hasPermission } from "@/utils/permissions";
import Divider from "../common/divider";
import MemberItemButton from "./components/memberItemButton";
import PasswordModal from "./components/passwordModal";
import UserInfoPopover from "./components/userInfoPopover";

// 子项
interface RoomItemPropsType {
	room: RoomInfo;
	isActive?: boolean;
	selectedMember: () => { identity: string; x: number; y: number } | null;
	onSelectMember: (identity: string, x: number, y: number) => void;
	onCloseMember: () => void;
}

const RoomItem = (props: RoomItemPropsType) => {
	const isSelected = () =>
		socketStore.selectedRoomInfo()?.name === props.room.name &&
		socketStore.selectedRoomInfo()?.guild_uuid === props.room.guild_uuid;
	const [showPasswordModal, setShowPasswordModal] = createSignal(false);

	const handleJoin = () => {
		if (props.room.hasPassword) {
			setShowPasswordModal(true);
		} else if (props.room.type === "text") {
			chatStore.joinTextRoom({ uuid: props.room.uuid, name: props.room.name });
		} else {
			socketStore.selectRoom(props.room);
		}
	};

	return (
		<div class="flex flex-col w-full overflow-hidden">
			<div
				class={clsx(
					"tooltip tooltip-right w-full",
					isSelected() && !props.isActive ? "tooltip-open" : "",
				)}
				data-tip={props.room.hasPassword ? "🔒 输入密码加入" : "进入房间"}
			>
				<button
					class={clsx(
						"justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm w-full",
						props.isActive ? "btn-active" : "",
						isSelected() && !props.isActive ? "bg-base-200" : "",
					)}
					onClick={() => {
						if (window.matchMedia("(max-width: 767px)").matches) {
							handleJoin();
						}
					}}
					onDblClick={() => {
						handleJoin();
					}}
				>
					<div class="flex items-center space-x-1">
						<span>
							<svg
								width="16"
								height="16"
								viewBox="0 0 48 48"
								fill="none"
								xmlns="http://www.w3.org/2000/svg"
							>
								<rect
									x="17"
									y="4"
									width="14"
									height="27"
									rx="7"
									fill="none"
									stroke="currentColor"
									stroke-width="3"
									stroke-linejoin="round"
								/>
								<path
									d="M9 23C9 31.2843 15.7157 38 24 38C32.2843 38 39 31.2843 39 23"
									stroke="currentColor"
									stroke-width="3"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
								<path
									d="M24 38V44"
									stroke="currentColor"
									stroke-width="3"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
							</svg>
						</span>
						<span class="text-[14px] leading-0">
							{props.room.type === "text" ? "# " : ""}
							{props.room.name}
						</span>
						<Show when={props.room.hasPassword}>
							<span class="text-base-content/50">
								<svg
									width="14"
									height="14"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								>
									<rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
									<path d="M7 11V7a5 5 0 0 1 10 0v4" />
								</svg>
							</span>
						</Show>
					</div>

					<div class="flex text-[12px]">
						<div>
							{props.room.count}/{props.room.limit}
						</div>
					</div>
				</button>
			</div>

			<div class="flex flex-col w-full">
				<For each={props.room.members}>
					{(member) => (
						<MemberItemButton
							member={member}
							onSelectMember={props.onSelectMember}
						/>
					)}
				</For>
			</div>
			<Show when={showPasswordModal()}>
				<PasswordModal
					room={props.room}
					onClose={() => setShowPasswordModal(false)}
				/>
			</Show>
		</div>
	);
};

const RoomItemSkeleton = () => {
	return (
		<div class="flex flex-col w-full">
			<div
				aria-hidden="true"
				class="justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm"
			>
				<div class="flex items-center space-x-1 w-full">
					<div class="flex-1 h-4 skeleton"></div>
				</div>
				<div class="flex items-center text-[12px]">
					<div class="w-8 h-4 skeleton"></div>
				</div>
			</div>
		</div>
	);
};

const RoomListFallback = () => (
	<div class="flex flex-col gap-3 px-2 py-4" aria-live="polite">
		<Show
			when={socketStore.connecting()}
			fallback={
				<div class="flex flex-col items-center gap-2 py-2 text-xs text-base-content/70">
					<span>连接失败，请检查服务器状态</span>
					<button
						type="button"
						class="btn btn-sm btn-ghost"
						onClick={() => socketStore.connect()}
					>
						重新连接
					</button>
				</div>
			}
		>
			<RoomItemSkeleton />
			<div class="text-center text-xs text-base-content/70">
				正在连接服务器...
			</div>
		</Show>
	</div>
);

const RoomListHeader = (props: { onOpenCreate: () => void }) => {
	return (
		<div class="flex justify-between mt-2">
			<h3 class="font-bold">服务器</h3>
			<div class="flex items-center gap-1">
				<Show when={hasPermission("room:create")}>
					<button
						type="button"
						class="btn btn-xs btn-ghost btn-square"
						onClick={props.onOpenCreate}
						title="新建房间"
						aria-label="新建房间"
					>
						<CirclePlus size={14} />
					</button>
				</Show>
				<button
					class="btn btn-xs btn-ghost min-w-11"
					onClick={() => socketStore.listRooms()}
				>
					刷新
				</button>
			</div>
		</div>
	);
};

interface RoomListPropsType {
	ref?: HTMLDivElement;
}

const RoomList = ({ ref }: RoomListPropsType) => {
	let createRoomModalRef!: HTMLDialogElement;
	const [selectedMember, setSelectedMember] = createSignal<{
		identity: string;
		x: number;
		y: number;
	} | null>(null);

	// 进入时加载房间列表
	onMount(() => {
		socketStore.connect();
	});

	const openCreateRoomModal = () => {
		createRoomModalRef?.showModal?.();
	};

	const closeCreateRoomModal = () => {
		createRoomModalRef?.close?.();
	};

	return (
		<>
			<div class="box-border flex flex-col px-2 h-full select-none" ref={ref}>
				<RoomListHeader onOpenCreate={openCreateRoomModal} />
				<Divider class="mx-1 my-1" />
				<div class="relative flex-1 min-h-0">
					<div class="box-border absolute inset-0 flex flex-col space-y-1 overflow-y-auto">
						<Show
							when={socketStore.connected()}
							fallback={<RoomListFallback />}
						>
							<Show
								when={socketStore.rooms().length > 0}
								fallback={
									<div class="flex justify-center items-center h-20 text-xs text-base-content/40">
										暂无房间
									</div>
								}
							>
								<For each={socketStore.rooms()}>
									{(room) => (
										<RoomItem
											room={room}
											isActive={socketStore.currentRoom() === room.name}
											selectedMember={selectedMember}
											onSelectMember={(identity, x, y) =>
												setSelectedMember({ identity, x, y })
											}
											onCloseMember={() => setSelectedMember(null)}
										/>
									)}
								</For>
							</Show>
						</Show>
					</div>
				</div>
			</div>
			<CreateRoomModal
				ref={createRoomModalRef}
				onClose={closeCreateRoomModal}
			/>
			<Show when={selectedMember()}>
				{(sel) => (
					<UserInfoPopover
						identity={sel().identity}
						pos={{ x: sel().x, y: sel().y }}
						onClose={() => setSelectedMember(null)}
					/>
				)}
			</Show>
		</>
	);
};

export default RoomList;
