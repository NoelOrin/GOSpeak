import { Portal } from "solid-js/web";
import clsx from "clsx";
import Hash from "lucide-solid/icons/hash";
import Lock from "lucide-solid/icons/lock";
import LogOut from "lucide-solid/icons/log-out";
import Mic from "lucide-solid/icons/mic";
import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import { createEffect, createSignal, For, onCleanup, Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import domainStore from "@/stores/domainStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import { hasPermission } from "@/utils/permissions";
import MemberItemButton from "./memberItemButton";
import PasswordModal from "./passwordModal";
import { canDeleteRoomItem, canEditRoomItem } from "../roomListUtils";

// 子项
export interface RoomItemPropsType {
	room: RoomInfo;
	isActive?: boolean;
	selectedMember: () => { identity: string; x: number; y: number } | null;
	onSelectMember: (identity: string, x: number, y: number) => void;
	onCloseMember: () => void;
	onEditRoom: (room: RoomInfo) => void;
	onDeleteRoom: (room: RoomInfo) => void;
}
export const RoomItem = (props: RoomItemPropsType) => {
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
	const [clickHint, setClickHint] = createSignal<{
		x: number;
		y: number;
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
		setClickHint(null);
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
		`${props.room.name}${props.room.hasPassword ? " · 需要密码" : ""}`;

	const showTip = (e: MouseEvent) => {
		if (!truncated()) return;
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		setTip({ text: tipText(), left: rect.left, top: rect.bottom + 6 });
	};

	const showDoubleClickHint = (event: MouseEvent) => {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		setClickHint({ x: rect.left + rect.width / 2, y: rect.bottom + 6 });
	};

	const hideTip = () => setTip(null);

	// 滚动容器滚动时 tooltip 位置错位，捕获阶段监听滚动即时清除
	createEffect(() => {
		if (!tip() && !clickHint()) return;
		const onScroll = () => {
			hideTip();
			setClickHint(null);
		};
		window.addEventListener("scroll", onScroll, true);
		onCleanup(() => window.removeEventListener("scroll", onScroll, true));
	});

	const handleJoin = () => {
		setClickHint(null);
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
		if (props.room.type === "text") {
			chatStore.leaveTextRoom();
			return;
		}
		void socketStore
			.leaveRoom(props.room.name, props.room.domain_uuid)
			.catch(() => {});
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
			<div class="flex flex-col w-full shrink-0 overflow-hidden">
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
						onDblClick={() => {
							setClickHint(null);
							handleJoin();
						}}
					>
						<div class="flex min-w-0 flex-1 items-center space-x-1">
							<span>
								<Show
									when={props.room.type === "text"}
									fallback={<Mic size={16} strokeWidth={2.5} />}
								>
									<Hash size={16} strokeWidth={2.5} />
								</Show>
							</span>
							<span
								ref={nameRef}
								class="min-w-0 flex-1 truncate whitespace-nowrap text-left text-[14px] leading-none"
							>
								{props.room.name}
							</span>
							<Show when={props.room.hasPassword}>
								<span class="shrink-0 text-base-content/50">
									<Lock size={14} />
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
				<Show when={clickHint()}>
					{(hint) => (
						<div
							class="pointer-events-none fixed z-[9999] whitespace-nowrap rounded-md bg-base-content px-2 py-1 text-xs font-medium text-base-100 shadow-lg"
							style={{
								left: `${hint().x}px`,
								top: `${hint().y}px`,
								transform: "translateX(-50%)",
							}}
						>
							双击进入
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
