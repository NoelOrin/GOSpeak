import { Portal } from "solid-js/web";
import clsx from "clsx";
import CirclePlus from "lucide-solid/icons/circle-plus";
import LogOut from "lucide-solid/icons/log-out";
import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import { useNavigate } from "@tanstack/solid-router";
import {
	createEffect,
	createMemo,
	createSignal,
	For,
	onCleanup,
	onMount,
	Show,
} from "solid-js";
import CreateRoomModal from "@/components/modal/createRoomModal";
import EditRoomModal from "@/components/modal/editRoomModal";
import type { RoomRecord } from "@/api/room";
import { deleteRoom } from "@/api/room";
import { chatStore } from "@/stores/chatStore";
import domainStore from "@/stores/domainStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";
import { showToast } from "solid-notifications";
import userStore from "@/stores/userStore";
import { hasPermission } from "@/utils/permissions";
import Divider from "../common/divider";
import ConfirmModal from "@/components/common/ConfirmModal";
import MemberItemButton from "./components/memberItemButton";
import PasswordModal from "./components/passwordModal";
import UserInfoPopover from "./components/userInfoPopover";
import {
	canDeleteRoomItem,
	canEditRoomItem,
	toEditRoomRecord,
} from "./roomListUtils";

function apiErrorMessage(error: unknown): string {
	const response = (error as { response?: { data?: { msg?: string } } })
		?.response?.data?.msg;
	if (response) return response;
	if (error instanceof Error) return error.message;
	return "删除房间失败";
}

// 子项
interface RoomItemPropsType {
	room: RoomInfo;
	isActive?: boolean;
	selectedMember: () => { identity: string; x: number; y: number } | null;
	onSelectMember: (identity: string, x: number, y: number) => void;
	onCloseMember: () => void;
	onEditRoom: (room: RoomInfo) => void;
	onDeleteRoom: (room: RoomInfo) => void;
}
const RoomItem = (props: RoomItemPropsType) => {
	const isSelected = () =>
		socketStore.selectedRoomInfo()?.name === props.room.name &&
		socketStore.selectedRoomInfo()?.domain_uuid === props.room.domain_uuid;
	const [showPasswordModal, setShowPasswordModal] = createSignal(false);
	const [truncated, setTruncated] = createSignal(false);
	const [tip, setTip] = createSignal<{
		text: string;
		left: number;
		top: number;
	} | null>(null);
	const [contextMenu, setContextMenu] = createSignal<{
		x: number;
		y: number;
	} | null>(null);
	let nameRef: HTMLSpanElement | undefined;
	let contextMenuRef: HTMLDivElement | undefined;

	const canEdit = () =>
		canEditRoomItem(
			userStore.user(),
			socketStore.currentDomainUUID()
				? (domainStore.state.domainCache[
						socketStore.currentDomainUUID() ?? ""
					] ?? null)
				: null,
			socketStore.currentDomainUUID()
				? domainStore.state.memberCache[
						socketStore.currentDomainUUID() ?? ""
					]?.find((member) => member.user_uuid === userStore.user()?.uuid)
						?.role_name
				: undefined,
			hasPermission("room:update"),
		);

	const canDelete = () =>
		canDeleteRoomItem(
			userStore.user(),
			socketStore.currentDomainUUID()
				? (domainStore.state.domainCache[
						socketStore.currentDomainUUID() ?? ""
					] ?? null)
				: null,
			socketStore.currentDomainUUID()
				? domainStore.state.memberCache[
						socketStore.currentDomainUUID() ?? ""
					]?.find((member) => member.user_uuid === userStore.user()?.uuid)
						?.role_name
				: undefined,
			hasPermission("room:delete"),
		);

	const isInRoom = () => {
		if (props.room.type === "text") {
			return (
				chatStore.textRoom() === props.room.uuid ||
				chatStore.textRoomName() === props.room.name
			);
		}
		const selected = socketStore.selectedRoomInfo();
		const sameDomain = (domainUUID?: string) =>
			(domainUUID || "") === (props.room.domain_uuid || "");
		return (
			(socketStore.currentRoom() === props.room.name &&
				sameDomain(socketStore.currentDomainUUID() ?? undefined)) ||
			(!!selected &&
				selected.name === props.room.name &&
				sameDomain(selected.domain_uuid))
		);
	};

	const openContextMenu = (event: MouseEvent) => {
		if (!canEdit() && !canDelete() && !isInRoom()) return;
		event.preventDefault();
		const menuWidth = 176;
		const visibleMenuCount = [canEdit(), canDelete(), isInRoom()].filter(
			Boolean,
		).length;
		const menuHeight = visibleMenuCount * 40 + 8;
		const x = Math.max(
			8,
			Math.min(event.clientX, Math.max(8, window.innerWidth - menuWidth - 8)),
		);
		const y = Math.max(
			8,
			Math.min(event.clientY, Math.max(8, window.innerHeight - menuHeight - 8)),
		);
		setContextMenu({ x, y });
	};

	const closeContextMenu = () => setContextMenu(null);

	createEffect(() => {
		if (!contextMenu()) return;
		const onPointerDown = (event: MouseEvent) => {
			if (!contextMenuRef?.contains(event.target as Node)) closeContextMenu();
		};
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") closeContextMenu();
		};
		document.addEventListener("mousedown", onPointerDown);
		document.addEventListener("keydown", onKeyDown);
		window.addEventListener("scroll", closeContextMenu, true);
		window.addEventListener("resize", closeContextMenu);
		onCleanup(() => {
			document.removeEventListener("mousedown", onPointerDown);
			document.removeEventListener("keydown", onKeyDown);
			window.removeEventListener("scroll", closeContextMenu, true);
			window.removeEventListener("resize", closeContextMenu);
		});
	});

	createEffect(() => {
		props.room.name;
		const el = nameRef;
		if (!el) return;
		const check = () => setTruncated(el.scrollWidth > el.clientWidth + 1);
		check();
		const ro = new ResizeObserver(check);
		ro.observe(el);
		onCleanup(() => ro.disconnect());
	});

	const tipText = () =>
		`${props.room.type === "text" ? "# " : ""}${props.room.name}${props.room.hasPassword ? " · 需要密码" : ""}`;

	const showTip = (e: MouseEvent) => {
		if (!truncated()) return;
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		setTip({ text: tipText(), left: rect.left, top: rect.bottom + 6 });
	};

	const hideTip = () => setTip(null);

	// 滚动容器滚动时 tooltip 位置错位，捕获阶段监听滚动即时清除
	createEffect(() => {
		if (!tip()) return;
		const onScroll = () => hideTip();
		window.addEventListener("scroll", onScroll, true);
		onCleanup(() => window.removeEventListener("scroll", onScroll, true));
	});

	const handleJoin = () => {
		closeContextMenu();
		if (props.room.hasPassword) {
			setShowPasswordModal(true);
		} else if (props.room.type === "text") {
			chatStore.joinTextRoom({
				uuid: props.room.uuid,
				name: props.room.name,
				domain_uuid: props.room.domain_uuid,
			});
		} else {
			socketStore.selectRoom(props.room);
		}
	};

	const handleLeave = () => {
		closeContextMenu();
		void socketStore.leaveRoom(props.room.name).catch(() => {});
		if (props.room.type === "text") {
			chatStore.leaveTextRoom();
			return;
		}
		socketStore.clearCurrentRoom();
		socketStore.clearSelectedRoom();
	};

	const handleEdit = () => {
		closeContextMenu();
		props.onEditRoom(props.room);
	};

	const handleDelete = () => {
		closeContextMenu();
		props.onDeleteRoom(props.room);
	};

	return (
		<>
			<div class="flex flex-col w-full overflow-hidden">
				<div class="w-full min-w-0">
					<button
						class={clsx(
							"justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm w-full",
							props.isActive ? "btn-active" : "",
							isSelected() && !props.isActive ? "bg-base-200" : "",
						)}
						aria-haspopup="menu"
						aria-expanded={!!contextMenu()}
						onContextMenu={openContextMenu}
						onMouseEnter={showTip}
						onMouseLeave={hideTip}
						onClick={() => {
							if (window.matchMedia("(max-width: 767px)").matches) {
								handleJoin();
							}
						}}
						onDblClick={() => {
							handleJoin();
						}}
					>
						<div class="flex min-w-0 flex-1 items-center space-x-1">
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
							<span
								ref={nameRef}
								class="min-w-0 flex-1 truncate whitespace-nowrap text-left text-[14px] leading-none"
							>
								{props.room.type === "text" ? "# " : ""}
								{props.room.name}
							</span>
							<Show when={props.room.hasPassword}>
								<span class="shrink-0 text-base-content/50">
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

						<div class="shrink-0 text-[12px]">
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
			<Portal>
				<Show when={tip()}>
					{(t) => (
						<div
							class="pointer-events-none fixed z-[9999] top-[var(--tip-top)] left-[var(--tip-left)] whitespace-nowrap rounded-md bg-base-content px-2 py-1 text-xs font-medium text-base-100 shadow-lg"
							style={{
								"--tip-left": `${t().left}px`,
								"--tip-top": `${t().top}px`,
							}}
						>
							{t().text}
						</div>
					)}
				</Show>
				<Show when={contextMenu()}>
					{(menu) => (
						<div
							ref={contextMenuRef}
							role="menu"
							class="fixed z-[9999] min-w-44 overflow-hidden rounded-lg border border-base-300 bg-base-100 p-1 shadow-xl"
							style={{
								left: `${menu().x}px`,
								top: `${menu().y}px`,
							}}
						>
							<Show when={canEdit()}>
								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 focus-visible:bg-base-200 focus:outline-none"
									onClick={handleEdit}
								>
									<Pencil size={14} />
									<span>编辑房间</span>
								</button>
							</Show>
							<Show when={isInRoom()}>
								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 focus-visible:bg-base-200 focus:outline-none"
									onClick={handleLeave}
								>
									<LogOut size={14} />
									<span>退出房间</span>
								</button>
							</Show>
							<Show when={canDelete()}>
								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-error transition-colors hover:bg-error/10 focus-visible:bg-error/10 focus:outline-none"
									onClick={handleDelete}
								>
									<Trash2 size={14} />
									<span>删除房间</span>
								</button>
							</Show>
						</div>
					)}
				</Show>
			</Portal>
		</>
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
	const navigate = useNavigate();
	const currentDomain = createMemo(
		() => domainStore.state.domainCache[socketStore.currentDomainUUID() ?? ""],
	);
	return (
		<div class="flex justify-between mt-2">
			<h3 class="font-bold">{currentDomain()?.name || "语音域"}</h3>
			<div class="flex items-center gap-1">
				<Show when={currentDomain()}>
					<button
						type="button"
						class="btn btn-xs btn-ghost"
						onClick={() =>
							navigate({
								to: "/manage/domains/$domainUUID",
								params: { domainUUID: currentDomain()?.uuid },
							})
						}
					>
						管理
					</button>
				</Show>
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
	let editRoomModalRef!: HTMLDialogElement;
	const [selectedMember, setSelectedMember] = createSignal<{
		identity: string;
		x: number;
		y: number;
	} | null>(null);
	const [editingRoom, setEditingRoom] = createSignal<RoomInfo | null>(null);
	const [deletingRoom, setDeletingRoom] = createSignal<RoomInfo | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	const [deleteError, setDeleteError] = createSignal("");
	let deleteDialogRef!: HTMLDialogElement;

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

	const openEditRoom = (room: RoomInfo) => {
		setEditingRoom(room);
		queueMicrotask(() => editRoomModalRef?.showModal?.());
	};

	const openDeleteRoom = (room: RoomInfo) => {
		setDeleteError("");
		setDeletingRoom(room);
		queueMicrotask(() => deleteDialogRef?.showModal?.());
	};

	const closeDeleteModal = () => {
		deleteDialogRef?.close();
		setDeletingRoom(null);
		setDeleteError("");
	};

	const handleDeleteRoom = async () => {
		const room = deletingRoom();
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		try {
			await deleteRoom(room.id);
			closeDeleteModal();
			showToast("房间已删除", { type: "success" });
			socketStore.listRooms();
		} catch (error) {
			const message = apiErrorMessage(error);
			setDeleteError(message);
			showToast(message, { type: "error" });
		} finally {
			setDeleting(false);
		}
	};

	const closeEditRoom = () => {
		editRoomModalRef?.close?.();
	};

	const handleRoomSaved = (updated: RoomRecord) => {
		const selected = socketStore.selectedRoomInfo();
		if (selected?.id === updated.id) {
			socketStore.selectRoom({
				...selected,
				id: updated.id,
				uuid: updated.uuid,
				name: updated.name,
				domain_uuid: updated.domain_uuid,
				description: updated.description,
				limit: updated.limit,
				audioOnly: updated.audio_only,
				allowAudience: updated.allow_audience,
				type: updated.type,
			});
		}
		socketStore.listRooms();
	};

	const visibleRooms = createMemo(() =>
		socketStore
			.rooms()
			.filter(
				(room) =>
					(room.domain_uuid || "") === (socketStore.currentDomainUUID() || ""),
			),
	);

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
								when={visibleRooms().length > 0}
								fallback={
									<div class="flex justify-center items-center h-20 text-xs text-base-content/40">
										暂无房间
									</div>
								}
							>
								<For each={visibleRooms()}>
									{(room) => (
										<RoomItem
											room={room}
											isActive={
												socketStore.currentRoom() === room.name &&
												(room.domain_uuid || "") ===
													(socketStore.currentDomainUUID() || "")
											}
											selectedMember={selectedMember}
											onSelectMember={(identity, x, y) =>
												setSelectedMember({ identity, x, y })
											}
											onCloseMember={() => setSelectedMember(null)}
											onEditRoom={openEditRoom}
											onDeleteRoom={openDeleteRoom}
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
			<Show when={editingRoom()}>
				{(room) => (
					<EditRoomModal
						ref={editRoomModalRef}
						room={toEditRoomRecord(room())}
						onClose={closeEditRoom}
						onSaved={handleRoomSaved}
					/>
				)}
			</Show>
			<Show when={deletingRoom()}>
				{(room) => (
					<ConfirmModal
						open
						title="删除房间"
						message={
							<span>
								确认删除房间 {room().name}？删除后不可恢复。
								<Show when={deleteError()}>
									<span class="mt-2 block text-error">{deleteError()}</span>
								</Show>
							</span>
						}
						confirmText="删除"
						confirmClass="btn btn-error"
						loading={deleting()}
						dialogRef={(el) => {
							deleteDialogRef = el;
						}}
						onClose={closeDeleteModal}
						onConfirm={handleDeleteRoom}
					/>
				)}
			</Show>
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
