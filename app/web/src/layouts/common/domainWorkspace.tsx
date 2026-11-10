import { Hash, Headphones, Lock, Mic, X } from "lucide-react";
import { useEffect, useRef } from "react";
import { ErrorBoundary } from "@/components/common/errorBoundary";
import MemberSidebar from "@/components/room/components/memberSidebar";
import { visibleRoomsForDomain } from "@/components/room/roomListUtils";
import RoomDetail from "@/components/room/roomDetail";
import TextRoomPanel from "@/components/textRoom/TextRoomPanel";
import ChannelHeader from "@/layouts/common/channelHeader";
import type { RoomInfo } from "@/socket/types";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { socketStore, useSocketStore } from "@/stores/socketStore";
import { useTextRoomUiStore } from "@/stores/textRoomUiStore";
import {
	clampChatHeight,
	MIN_CHAT_HEIGHT,
	useWorkspaceUiStore,
} from "@/stores/workspaceUiStore";

interface DomainWorkspaceProps {
	isDomain: boolean;
	mobileHideMembers?: boolean;
}

/**
 * 域工作区（Oopz 式上下分栏）。
 *
 * - 主区上下分：上方语音舞台、下方文字聊天；中间可拖拽分隔条，
 *   高度持久化到 localStorage。
 * - 聊天区是主区的独立兄弟区块，不嵌进任何语音房间视图：切换语音频道
 *   只换上方内容，聊天区与其高度保持不动。
 * - 成员列表是工作区的独立兄弟节点，从页面顶部开始，保持原有组件、
 *   原有宽度与原有浅色（由根容器底色提供，不额外包裹）。
 * - 深色只作用于主工作区（ds-surface），全局主题 token 不变。
 */
export default function DomainWorkspace({
	isDomain,
	mobileHideMembers = false,
}: DomainWorkspaceProps) {
	const chatOpen = useWorkspaceUiStore((s) => s.chatOpen);
	const chatHeight = useWorkspaceUiStore((s) => s.chatHeight);
	const setChatHeight = useWorkspaceUiStore((s) => s.setChatHeight);
	const membersOpen = useWorkspaceUiStore((s) => s.membersOpen);
	const setChatOpen = useWorkspaceUiStore((s) => s.setChatOpen);

	const selectedRoom = useSocketStore((s) => s.selectedRoomInfo);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const textRoom = useChatStore((s) => s.textRoom);
	const textRoomName = useChatStore((s) => s.textRoomName);

	// 切换文字频道时清掉上一个房间的搜索 / 线程 / 回复状态
	useEffect(() => {
		useTextRoomUiStore.getState().resetForRoom();
	}, [textRoom]);

	const voiceActive = !!selectedRoom || !!currentRoom;
	// 聊天区非常驻：必须已进入文字频道，且用户没有手动收起
	const showChatPanel = chatOpen && !!textRoom && !mobileHideMembers;

	const stackRef = useRef<HTMLDivElement>(null);
	const splitRef = useRef<HTMLDivElement>(null);
	// 拖拽回调只注册一次，用 ref 读取当前高度，避免每次高度变化都重绑监听器
	const chatHeightRef = useRef(chatHeight);
	chatHeightRef.current = chatHeight;

	// 挂载 / 容器尺寸变化时，把保存的高度收敛到「聊天够用、语音不被压死」的区间
	useEffect(() => {
		if (!showChatPanel) return;
		const el = stackRef.current;
		if (!el) return;
		const sync = () => {
			const next = clampChatHeight(chatHeightRef.current, el.clientHeight);
			if (Math.round(next) !== Math.round(chatHeightRef.current)) {
				chatHeightRef.current = next;
				setChatHeight(next);
			}
		};
		sync();
		if (typeof ResizeObserver === "undefined") return;
		const ro = new ResizeObserver(sync);
		ro.observe(el);
		return () => ro.disconnect();
	}, [showChatPanel, setChatHeight]);

	// 拖动分隔条调整上下比例：向上拖 = 聊天变高
	useEffect(() => {
		if (!showChatPanel) return;
		const handle = splitRef.current;
		if (!handle) return;

		let onMove: ((e: MouseEvent) => void) | null = null;
		let onUp: (() => void) | null = null;
		let restoreBody: (() => void) | null = null;

		const stop = () => {
			if (onMove) document.removeEventListener("mousemove", onMove);
			if (onUp) document.removeEventListener("mouseup", onUp);
			onMove = null;
			onUp = null;
			restoreBody?.();
			restoreBody = null;
		};

		const onDown = (e: MouseEvent) => {
			if (e.button !== 0) return;
			e.preventDefault();
			stop();

			const startY = e.clientY;
			const startHeight = chatHeightRef.current;
			const prevSelect = document.body.style.userSelect;
			const prevCursor = document.body.style.cursor;
			document.body.style.userSelect = "none";
			document.body.style.cursor = "row-resize";
			restoreBody = () => {
				document.body.style.userSelect = prevSelect;
				document.body.style.cursor = prevCursor;
			};

			onMove = (ev: MouseEvent) => {
				const next = clampChatHeight(
					startHeight + (startY - ev.clientY),
					stackRef.current?.clientHeight ?? 0,
				);
				chatHeightRef.current = next;
				setChatHeight(next);
			};
			onUp = () => stop();
			document.addEventListener("mousemove", onMove);
			document.addEventListener("mouseup", onUp);
		};

		handle.addEventListener("mousedown", onDown);
		return () => {
			handle.removeEventListener("mousedown", onDown);
			stop();
		};
	}, [showChatPanel, setChatHeight]);

	const headerKind = voiceActive
		? ("voice" as const)
		: textRoom
			? ("text" as const)
			: ("empty" as const);
	const headerName = voiceActive
		? (selectedRoom?.name ?? currentRoom ?? "语音频道")
		: textRoom
			? (textRoomName ?? "文字频道")
			: "选择一个频道";

	return (
		<div
			className="bg-card border-border flex h-full w-full flex-1 border-t"
			style={{ display: isDomain ? undefined : "none" }}
		>
			{/* 主工作区：深色 Discord 表面 */}
			<div className="ds-surface flex min-w-0 flex-1 flex-col">
				<ChannelHeader
					kind={headerKind}
					name={headerName}
					description={textRoom ? `# ${textRoomName ?? "文字频道"}` : undefined}
				/>

				<div ref={stackRef} className="flex min-h-0 flex-1 flex-col">
					{/* 语音区：占上方，切换频道只换这里 */}
					<div className="relative flex min-h-0 flex-1 flex-col">
						{voiceActive ? (
							<ErrorBoundary
								name="voice-stage"
								resetKeys={[selectedRoom?.name ?? currentRoom]}
								title="语音舞台加载失败"
							>
								<RoomDetail />
							</ErrorBoundary>
						) : (
							<VoiceEmptyState />
						)}
					</div>

					{/* 文字聊天：主区下方的独立区块，可拖拽调高、可折叠 */}
					{showChatPanel && (
						<>
							<div
								ref={splitRef}
								role="separator"
								aria-orientation="horizontal"
								aria-valuenow={Math.round(chatHeight)}
								aria-valuemin={MIN_CHAT_HEIGHT}
								aria-label="拖动调整文字聊天高度"
								title="拖动调整文字聊天高度（↑/↓ 微调，Shift 加速）"
								tabIndex={0}
								onKeyDown={(e) => {
									const step = e.shiftKey ? 60 : 20;
									const delta =
										e.key === "ArrowUp"
											? step
											: e.key === "ArrowDown"
												? -step
												: 0;
									if (!delta) return;
									e.preventDefault();
									const next = clampChatHeight(
										chatHeightRef.current + delta,
										stackRef.current?.clientHeight ?? 0,
									);
									chatHeightRef.current = next;
									setChatHeight(next);
								}}
								className="group relative h-2 shrink-0 cursor-row-resize select-none focus-visible:outline-none"
							>
								<div className="bg-border group-hover:bg-primary pointer-events-none absolute inset-x-0 top-1/2 h-px -translate-y-1/2 transition-colors" />
							</div>
							<section
								className="bg-card flex shrink-0 flex-col overflow-hidden"
								style={{ height: chatHeight }}
								aria-label="文字聊天"
							>
								<div className="border-border flex h-10 shrink-0 items-center justify-between border-b px-4">
									<span className="text-foreground flex min-w-0 items-center gap-1.5 text-sm font-semibold">
										<Hash
											size={16}
											className="text-muted-foreground shrink-0"
										/>
										<span className="truncate">
											{textRoomName ?? "文字聊天"}
										</span>
									</span>
									<button
										type="button"
										className="ds-icon-btn size-7"
										onClick={() => setChatOpen(false)}
										title="收起文字聊天"
										aria-label="收起文字聊天"
									>
										<X size={16} />
									</button>
								</div>
								<div className="min-h-0 flex-1 overflow-hidden">
									<ErrorBoundary
										name="chat-panel"
										resetKeys={[textRoom]}
										title="聊天加载失败"
									>
										<TextRoomPanel />
									</ErrorBoundary>
								</div>
							</section>
						</>
					)}
				</div>
			</div>

			{/* 成员列表：原组件、原位置、原宽度，从顶部开始 */}
			{membersOpen && !mobileHideMembers && (
				<ErrorBoundary name="member-sidebar" title="成员列表加载失败">
					<MemberSidebar />
				</ErrorBoundary>
			)}
		</div>
	);
}

function useChannelList(kind: "voice" | "text"): RoomInfo[] {
	const rooms = useSocketStore((s) => s.rooms);
	const domainUUID = socketStore.currentDomainUUID();
	return visibleRoomsForDomain(rooms, domainUUID).filter((room) =>
		kind === "text" ? room.type === "text" : room.type !== "text",
	);
}

/**
 * 空态里的频道直达列表：避免大片留白，同时省掉「去左边双击」这一步。
 * 带密码的房间仍交给左侧列表双击（那边有密码弹窗），这里只做提示。
 */
function ChannelPicker({
	kind,
	list,
	compact = false,
}: {
	kind: "voice" | "text";
	list: RoomInfo[];
	compact?: boolean;
}) {
	if (list.length === 0) return null;

	const join = (room: RoomInfo) => {
		if (room.hasPassword) return;
		if (room.type === "text") {
			void chatStore.joinTextRoom({
				uuid: room.uuid,
				name: room.name,
				domain_uuid: room.domain_uuid,
			});
			return;
		}
		socketStore.selectRoom(room);
	};

	return (
		<div
			className={`flex w-full flex-col ${compact ? "gap-1" : "mt-1 max-w-xs gap-1.5"}`}
		>
			{list.map((room) => (
				<button
					key={`${room.domain_uuid ?? ""}:${room.name}`}
					type="button"
					disabled={room.hasPassword}
					onClick={() => join(room)}
					title={
						room.hasPassword
							? "该频道需要密码，请在左侧列表双击进入"
							: `进入 ${room.name}`
					}
					className="bg-muted border-border hover:bg-muted flex w-full items-center gap-2.5 rounded-lg border px-3 py-2 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-55"
				>
					{kind === "text" ? (
						<Hash size={16} className="text-muted-foreground shrink-0" />
					) : (
						<Mic size={16} className="text-muted-foreground shrink-0" />
					)}
					<span className="text-foreground min-w-0 flex-1 truncate text-sm">
						{room.name}
					</span>
					{room.hasPassword && (
						<Lock size={12} className="text-muted-foreground shrink-0" />
					)}
					<span className="text-muted-foreground shrink-0 text-xs">
						{kind === "text"
							? `${room.count} 人`
							: `${room.count}/${room.limit}`}
					</span>
				</button>
			))}
		</div>
	);
}

function VoiceEmptyState() {
	const list = useChannelList("voice").slice(0, 6);
	return (
		<div className="bg-card flex h-full flex-col items-center justify-center gap-3 px-6 text-center select-none">
			<div className="bg-muted border-border flex size-16 items-center justify-center rounded-full border">
				<Headphones size={32} strokeWidth={1.5} className="text-foreground" />
			</div>
			<h2 className="text-foreground text-lg font-bold">选择一个语音频道</h2>
			<p className="text-muted-foreground max-w-xs text-sm leading-relaxed">
				{list.length > 0
					? "点下面的频道直接进入，切换频道不会中断通话。"
					: "双击左侧列表里的语音频道加入通话。"}
			</p>
			<ChannelPicker kind="voice" list={list} />
		</div>
	);
}
