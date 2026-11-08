import clsx from "clsx";
import { Hash, Lock, LogOut, Mic, Pencil, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { useDomainStore } from "@/stores/domainStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";
import { canDeleteRoomItem, canEditRoomItem } from "../roomListUtils";
import MemberItemButton from "./memberItemButton";
import PasswordModal from "./passwordModal";

// 子项
export interface RoomItemPropsType {
	room: RoomInfo;
	isActive?: boolean;
	selectedMember: { identity: string; x: number; y: number } | null;
	onSelectMember: (identity: string, x: number, y: number) => void;
	onCloseMember: () => void;
	onEditRoom: (room: RoomInfo) => void;
	onDeleteRoom: (room: RoomInfo) => void;
}

export function RoomItem({
	room,
	isActive,
	onSelectMember,
	onEditRoom,
	onDeleteRoom,
}: RoomItemPropsType) {
	const unread = useChatStore((s) =>
		room.uuid ? (s.unreadByRoom[room.uuid] ?? 0) : 0,
	);
	const isSelected =
		socketStore.selectedRoomInfo()?.name === room.name &&
		socketStore.selectedRoomInfo()?.domain_uuid === room.domain_uuid;
	const [showPasswordModal, setShowPasswordModal] = useState(false);
	const [truncated, setTruncated] = useState(false);
	const [tip, setTip] = useState<{
		text: string;
		left: number;
		top: number;
	} | null>(null);
	const [clickHint, setClickHint] = useState<{ x: number; y: number } | null>(
		null,
	);
	const [contextMenu, setContextMenu] = useState<{
		x: number;
		y: number;
	} | null>(null);
	const nameRef = useRef<HTMLSpanElement>(null);
	const contextMenuRef = useRef<HTMLDivElement>(null);

	const user = useUserStore((s) => s.user);
	const domainState = useDomainStore((s) => s.domainCache);
	const memberState = useDomainStore((s) => s.memberCache);
	const currentDomainUUID = socketStore.currentDomainUUID();

	const canEdit = canEditRoomItem(
		user,
		currentDomainUUID ? (domainState[currentDomainUUID] ?? null) : null,
		currentDomainUUID
			? memberState[currentDomainUUID]?.find(
					(member) => member.user_uuid === user?.uuid,
				)?.role_name
			: undefined,
		hasPermission("room:update"),
	);

	const canDelete = canDeleteRoomItem(
		user,
		currentDomainUUID ? (domainState[currentDomainUUID] ?? null) : null,
		currentDomainUUID
			? memberState[currentDomainUUID]?.find(
					(member) => member.user_uuid === user?.uuid,
				)?.role_name
			: undefined,
		hasPermission("room:delete"),
	);

	const isInRoom = () => {
		if (room.type === "text") {
			return (
				chatStore.textRoom() === room.uuid ||
				chatStore.textRoomName() === room.name
			);
		}
		const selected = socketStore.selectedRoomInfo();
		const sameDomain = (domainUUID?: string) =>
			(domainUUID || "") === (room.domain_uuid || "");
		return (
			(socketStore.currentRoom() === room.name &&
				sameDomain(socketStore.currentDomainUUID() ?? undefined)) ||
			(!!selected &&
				selected.name === room.name &&
				sameDomain(selected.domain_uuid))
		);
	};

	const openContextMenu = (event: React.MouseEvent) => {
		if (!canEdit && !canDelete && !isInRoom()) return;
		event.preventDefault();
		setClickHint(null);
		const menuWidth = 176;
		const visibleMenuCount = [canEdit, canDelete, isInRoom()].filter(
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

	useEffect(() => {
		if (!contextMenu) return;
		const onPointerDown = (event: MouseEvent) => {
			if (!contextMenuRef.current?.contains(event.target as Node))
				closeContextMenu();
		};
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") closeContextMenu();
		};
		document.addEventListener("mousedown", onPointerDown);
		document.addEventListener("keydown", onKeyDown);
		window.addEventListener("scroll", closeContextMenu, true);
		window.addEventListener("resize", closeContextMenu);
		return () => {
			document.removeEventListener("mousedown", onPointerDown);
			document.removeEventListener("keydown", onKeyDown);
			window.removeEventListener("scroll", closeContextMenu, true);
			window.removeEventListener("resize", closeContextMenu);
		};
	}, [contextMenu]);

	useEffect(() => {
		const el = nameRef.current;
		if (!el) return;
		const check = () => setTruncated(el.scrollWidth > el.clientWidth + 1);
		check();
		const ro = new ResizeObserver(check);
		ro.observe(el);
		return () => ro.disconnect();
	}, [room.name]);

	const tipText = `${room.name}${room.hasPassword ? " · 需要密码" : ""}`;

	const showTip = (e: React.MouseEvent) => {
		if (!truncated) return;
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		setTip({ text: tipText, left: rect.left, top: rect.bottom + 6 });
	};

	const showDoubleClickHint = (event: React.MouseEvent) => {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		setClickHint({ x: rect.left + rect.width / 2, y: rect.bottom + 6 });
	};

	const hideTip = () => setTip(null);

	useEffect(() => {
		if (!tip && !clickHint) return;
		const onScroll = () => {
			hideTip();
			setClickHint(null);
		};
		window.addEventListener("scroll", onScroll, true);
		return () => window.removeEventListener("scroll", onScroll, true);
	}, [tip, clickHint]);

	const handleJoin = () => {
		setClickHint(null);
		closeContextMenu();
		if (room.hasPassword) {
			setShowPasswordModal(true);
		} else if (room.type === "text") {
			chatStore.joinTextRoom({
				uuid: room.uuid,
				name: room.name,
				domain_uuid: room.domain_uuid,
			});
		} else {
			socketStore.selectRoom(room);
		}
	};

	const handleLeave = () => {
		closeContextMenu();
		if (room.type === "text") {
			chatStore.leaveTextRoom();
			return;
		}
		void socketStore.leaveRoom(room.name, room.domain_uuid).catch((error) => {
			toastActionError(error, "离开房间失败，请重试");
		});
		socketStore.clearCurrentRoom();
		socketStore.clearSelectedRoom();
	};

	return (
		<>
			<div className="flex w-full shrink-0 flex-col overflow-hidden">
				<div className="w-full min-w-0">
					<button
						className={clsx(
							"hover:bg-muted flex w-full cursor-pointer items-center justify-between rounded-sm border-0 px-1.5 py-2 text-sm transition-colors",
							isActive ? "bg-muted" : "",
							isSelected && !isActive ? "bg-muted" : "",
						)}
						aria-haspopup="menu"
						aria-expanded={!!contextMenu}
						onContextMenu={openContextMenu}
						onMouseEnter={showTip}
						onMouseLeave={() => {
							hideTip();
							setClickHint(null);
						}}
						onClick={(event) => {
							if (window.matchMedia("(max-width: 767px)").matches) {
								handleJoin();
								return;
							}
							showDoubleClickHint(event);
						}}
						onDoubleClick={() => {
							setClickHint(null);
							handleJoin();
						}}
					>
						<div className="flex min-w-0 flex-1 items-center space-x-1">
							<span>
								{room.type === "text" ? <Hash size={16} /> : <Mic size={16} />}
							</span>
							<span
								ref={nameRef}
								className="min-w-0 flex-1 truncate text-left text-sm leading-none whitespace-nowrap"
							>
								{room.name}
							</span>
							{room.hasPassword && (
								<span className="text-faint-foreground shrink-0">
									<Lock size={14} />
								</span>
							)}
						</div>

						<div className="flex shrink-0 items-center gap-1.5 text-xs">
							{room.type === "text" && unread > 0 && (
								<span
									className="bg-primary text-primary-foreground inline-flex min-w-5 items-center justify-center rounded-full px-1.5 text-2xs font-semibold"
									title={`${unread} 条未读消息`}
								>
									{unread > 99 ? "99+" : unread}
								</span>
							)}
							<div>
								{room.count}/{room.limit}
							</div>
						</div>
					</button>
				</div>

				<div className="flex w-full flex-col">
					{room.members?.map((member) => (
						<MemberItemButton
							key={member.identity}
							member={member}
							onSelectMember={onSelectMember}
						/>
					))}
				</div>
				{showPasswordModal && (
					<PasswordModal
						room={room}
						onClose={() => setShowPasswordModal(false)}
					/>
				)}
			</div>
			{tip && (
				<div
					className="bg-foreground text-background pointer-events-none fixed z-[9999] rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg"
					style={{ left: `${tip.left}px`, top: `${tip.top}px` }}
				>
					{tip.text}
				</div>
			)}
			{clickHint && (
				<div
					className="bg-foreground text-background pointer-events-none fixed z-[9999] -translate-x-1/2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg"
					style={{ left: `${clickHint.x}px`, top: `${clickHint.y}px` }}
				>
					双击进入
				</div>
			)}
			{contextMenu && (
				<div
					ref={contextMenuRef}
					role="menu"
					className="border-border bg-card fixed z-[9999] min-w-44 overflow-hidden rounded-lg border p-1 shadow-xl"
					style={{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }}
				>
					{canEdit && (
						<button
							type="button"
							role="menuitem"
							className="hover:bg-muted focus-visible:bg-muted flex w-full cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors focus:outline-none"
							onClick={() => {
								closeContextMenu();
								onEditRoom(room);
							}}
						>
							<Pencil size={14} />
							<span>编辑房间</span>
						</button>
					)}
					{isInRoom() && (
						<button
							type="button"
							role="menuitem"
							className="hover:bg-muted focus-visible:bg-muted flex w-full cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors focus:outline-none"
							onClick={handleLeave}
						>
							<LogOut size={14} />
							<span>退出房间</span>
						</button>
					)}
					{canDelete && (
						<button
							type="button"
							role="menuitem"
							className="text-destructive hover:bg-destructive/10 focus-visible:bg-destructive/10 flex w-full cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors focus:outline-none"
							onClick={() => {
								closeContextMenu();
								onDeleteRoom(room);
							}}
						>
							<Trash2 size={14} />
							<span>删除房间</span>
						</button>
					)}
				</div>
			)}
		</>
	);
}
