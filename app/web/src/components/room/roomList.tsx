import { toast, Button, ScrollArea, Skeleton } from "@gospeak/components";
import { useQueryClient } from "@tanstack/react-query";
import { CirclePlus, RotateCw } from "lucide-react";
import { useEffect, useState } from "react";
import type { RoomRecord } from "@/api/room";
import { deleteRoom } from "@/api/room";
import Divider from "@/components/common/divider";
import ConfirmModal from "@/components/common/ConfirmModal";
import TooltipIconButton from "@/components/common/tooltipIconButton";
import DomainMenu from "@/components/domain/DomainMenu";
import CreateRoomModal from "@/components/modal/createRoomModal";
import EditRoomModal from "@/components/modal/editRoomModal";
import UserInfoPopover from "@/components/room/components/userInfoPopover";
import { RoomItem } from "@/components/room/components/roomItem";
import {
	toEditRoomRecord,
	visibleRoomsForDomain,
} from "@/components/room/roomListUtils";
import { useDomainStore } from "@/stores/domainStore";
import {
	type RoomInfo,
	socketStore,
	useSocketStore,
} from "@/stores/socketStore";
import { hasPermission } from "@/utils/permissions";
import { useNavigate } from "@tanstack/react-router";
import { apiErrorMessage } from "@/utils/apiError";
import { toastActionError } from "@/utils/toast";

function RoomItemSkeleton() {
	return (
		<div className="flex w-full shrink-0 flex-col">
			<div className="flex items-center justify-between border-0 px-1.5 py-2">
				<div className="flex w-full items-center space-x-1">
					<Skeleton className="h-4 flex-1 rounded-sm" />
				</div>
				<div className="flex items-center text-xs">
					<Skeleton className="h-4 w-8 rounded-sm" />
				</div>
			</div>
		</div>
	);
}

function RoomListFallback() {
	const connecting = useSocketStore((s) => s.connecting);
	return (
		<div className="flex flex-col gap-3 px-2 py-4" aria-live="polite">
			{connecting ? (
				<>
					<RoomItemSkeleton />
					<div className="text-muted-foreground text-center text-xs">
						正在连接服务器...
					</div>
				</>
			) : (
				<div className="text-muted-foreground flex flex-col items-center gap-2 py-2 text-xs">
					<span>连接失败，请检查服务器状态</span>
					<Button
						variant="ghost"
						size="sm"
						onClick={() => socketStore.connect()}
					>
						重新连接
					</Button>
				</div>
			)}
		</div>
	);
}

function RoomListHeader({ onOpenCreate }: { onOpenCreate: () => void }) {
	const currentDomainUUID = socketStore.currentDomainUUID();
	const domainCache = useDomainStore((s) => s.domainCache);
	const currentDomain = domainCache[currentDomainUUID ?? ""];
	return (
		<div className="mt-2 flex justify-between">
			<h3 className="min-w-0 flex-1 truncate font-bold">
				{currentDomain?.name || "选择域"}
			</h3>
			<div className="flex items-center gap-0.5">
				{currentDomain && hasPermission("room:create") && (
					<TooltipIconButton label="新建房间" onClick={onOpenCreate}>
						<CirclePlus size={16} />
					</TooltipIconButton>
				)}
				<TooltipIconButton
					label="刷新房间列表"
					onClick={() => socketStore.listRooms()}
				>
					<RotateCw size={16} />
				</TooltipIconButton>
				{currentDomain && <DomainMenu domainUUID={currentDomain.uuid} />}
			</div>
		</div>
	);
}

export default function RoomList() {
	const queryClient = useQueryClient();
	const _navigate = useNavigate();
	const [selectedMember, setSelectedMember] = useState<{
		identity: string;
		x: number;
		y: number;
	} | null>(null);
	const [editingRoom, setEditingRoom] = useState<RoomInfo | null>(null);
	const [createRoomOpen, setCreateRoomOpen] = useState(false);
	const [deletingRoom, setDeletingRoom] = useState<RoomInfo | null>(null);
	const [deleting, setDeleting] = useState(false);
	const [deleteError, setDeleteError] = useState("");

	// 进入时加载房间列表
	useEffect(() => {
		socketStore.connect();
	}, []);

	const rooms = useSocketStore((s) => s.rooms);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const currentDomainUUID = socketStore.currentDomainUUID();
	const visibleRooms = visibleRoomsForDomain(rooms, currentDomainUUID);

	const openCreateRoomModal = () => {
		const domainUUID = socketStore.currentDomainUUID();
		if (!domainUUID) {
			toast.warning("请先选择域");
			return;
		}
		setCreateRoomOpen(true);
	};

	const openDeleteRoom = (room: RoomInfo) => {
		setDeleteError("");
		setDeletingRoom(room);
	};

	const closeDeleteModal = () => {
		setDeletingRoom(null);
		setDeleteError("");
	};

	const handleDeleteRoom = async () => {
		const room = deletingRoom;
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		try {
			await deleteRoom(room.id);
			closeDeleteModal();
			toast.success("房间已删除");
			socketStore.listRooms();
			queryClient.invalidateQueries({ queryKey: ["my-domains-detailed"] });
		} catch (error) {
			const message = apiErrorMessage(error);
			setDeleteError(message);
			toastActionError(error, "删除房间失败");
		} finally {
			setDeleting(false);
		}
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

	return (
		<>
			<div className="box-border flex h-full min-h-0 flex-col px-2 select-none">
				<RoomListHeader onOpenCreate={openCreateRoomModal} />
				<Divider className="mx-1 my-1" />
				{/* pr-1.5 与滚动条同宽：滚动条浮在这段留白上，不压住右侧人数，
				    宽度恒定所以出现/隐藏滚动条都不会让行左右跳动 */}
				<ScrollArea className="min-h-0 flex-1" viewportClassName="pb-1 pr-1.5">
					<div className="flex flex-col space-y-1">
						{socketStore.connected() ? (
							visibleRooms.length > 0 ? (
								visibleRooms.map((room) => (
									<RoomItem
										key={`${room.domain_uuid}:${room.name}`}
										room={room}
										isActive={
											currentRoom === room.name &&
											(room.domain_uuid || "") === (currentDomainUUID || "")
										}
										selectedMember={selectedMember}
										onSelectMember={(identity, x, y) =>
											setSelectedMember({ identity, x, y })
										}
										onCloseMember={() => setSelectedMember(null)}
										onEditRoom={setEditingRoom}
										onDeleteRoom={openDeleteRoom}
									/>
								))
							) : (
								<div className="text-faint-foreground flex h-20 items-center justify-center text-xs">
									暂无房间
								</div>
							)
						) : (
							<RoomListFallback />
						)}
					</div>
				</ScrollArea>
			</div>
			<CreateRoomModal
				open={createRoomOpen}
				domainUUID={currentDomainUUID ?? ""}
				onClose={() => setCreateRoomOpen(false)}
			/>
			{editingRoom && (
				<EditRoomModal
					open
					room={toEditRoomRecord(editingRoom)}
					onClose={() => setEditingRoom(null)}
					onSaved={handleRoomSaved}
				/>
			)}
			{deletingRoom && (
				<ConfirmModal
					open
					title="删除房间"
					message={
						<span>
							确认删除房间 {deletingRoom.name}？删除后不可恢复。
							{deleteError && (
								<span className="text-destructive mt-2 block">
									{deleteError}
								</span>
							)}
						</span>
					}
					confirmText="删除"
					loading={deleting}
					onClose={closeDeleteModal}
					onConfirm={handleDeleteRoom}
				/>
			)}
			{selectedMember && (
				<UserInfoPopover
					identity={selectedMember.identity}
					pos={{ x: selectedMember.x, y: selectedMember.y }}
					onClose={() => setSelectedMember(null)}
				/>
			)}
		</>
	);
}
