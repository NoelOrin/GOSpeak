import CirclePlus from "lucide-solid/icons/circle-plus";
import { useNavigate } from "@tanstack/solid-router";
import { createMemo, createSignal, For, onMount, Show } from "solid-js";
import CreateRoomModal from "@/components/modal/createRoomModal";
import EditRoomModal from "@/components/modal/editRoomModal";
import type { RoomRecord } from "@/api/room";
import { deleteRoom } from "@/api/room";
import domainStore from "@/stores/domainStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";
import { showToast } from "solid-notifications";
import { hasPermission } from "@/utils/permissions";
import Divider from "../common/divider";
import ConfirmModal from "@/components/common/ConfirmModal";
import UserInfoPopover from "./components/userInfoPopover";
import { toEditRoomRecord, visibleRoomsForDomain } from "./roomListUtils";
import { RoomItem } from "./components/roomItem";

function apiErrorMessage(error: unknown): string {
	const response = (error as { response?: { data?: { msg?: string } } })
		?.response?.data?.msg;
	if (response) return response;
	if (error instanceof Error) return error.message;
	return "删除房间失败";
}

const RoomItemSkeleton = () => {
	return (
		<div class="flex flex-col w-full shrink-0">
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
			<h3 class="font-bold">{currentDomain()?.name || "选择域"}</h3>
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
				<Show when={currentDomain() && hasPermission("room:create")}>
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
	const [createRoomDomain, setCreateRoomDomain] = createSignal("");
	const [deletingRoom, setDeletingRoom] = createSignal<RoomInfo | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	const [deleteError, setDeleteError] = createSignal("");
	let deleteDialogRef!: HTMLDialogElement;

	// 进入时加载房间列表
	onMount(() => {
		socketStore.connect();
	});

	const openCreateRoomModal = () => {
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
		visibleRoomsForDomain(socketStore.rooms(), socketStore.currentDomainUUID()),
	);

	return (
		<>
			<div class="box-border flex flex-col px-2 h-full select-none" ref={ref}>
				<RoomListHeader onOpenCreate={openCreateRoomModal} />
				<Divider class="mx-1 my-1" />
				<div class="flex-1 min-h-0 flex flex-col space-y-1 overflow-y-auto scrollbar-none">
					<Show when={socketStore.connected()} fallback={<RoomListFallback />}>
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
			<CreateRoomModal
				ref={createRoomModalRef}
				domainUUID={createRoomDomain()}
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
