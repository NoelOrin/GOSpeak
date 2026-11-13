import { useLocation, useNavigate } from "@tanstack/react-router";
import {
	ArrowLeft,
	Headphones,
	Home,
	MessageSquare,
	Settings,
	ShieldCheck,
	Users,
} from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { ErrorBoundary } from "@/components/common/errorBoundary";
import { useGlobalShortcuts } from "@/hooks/useGlobalShortcuts";
import DomainWorkspace from "@/layouts/common/domainWorkspace";
import Header from "@/layouts/common/header";
import Main from "@/layouts/common/main";
import Sidebar from "@/layouts/common/sidebar";
import RoomDetail from "@/components/room/roomDetail";
import VoiceStatusBar from "@/components/room/VoiceStatusBar";
import { VoiceSessionProvider } from "@/components/room/VoiceSessionProvider";
import MemberSidebar from "@/components/room/components/memberSidebar";
import TextRoomPanel from "@/components/textRoom/TextRoomPanel";
import UserBar from "@/components/userBar";
import SettingModal from "@/components/modal/settingModal/settingModal";
import NotificationModal from "@/components/modal/notificationModal";
import DeviceChangeNotice from "@/components/modal/deviceChangeNotice";
import { useChatStore } from "@/stores/chatStore";
import { useWorkspaceUiStore } from "@/stores/workspaceUiStore";
import { useDomainStore } from "@/stores/domainStore";
import { useSocketStore } from "@/stores/socketStore";
import { useOverlayRelay } from "@/hooks/useOverlayRelay";
import DynamicRender from "@/components/common/dynamicRender";
import ManageNav from "@/components/manage/manageNav";
import { firstAccessibleManagePath } from "@/components/manage/manageNav";
import { hasManageAccess } from "@/utils/permissions";

type MobileStageTab = "voice" | "text" | "members";

const MIN_SPLIT_WIDTH = 333;
const MAX_SPLIT_WIDTH = 525;

const clampSplitWidth = (w: number) =>
	Math.max(MIN_SPLIT_WIDTH, Math.min(MAX_SPLIT_WIDTH, w));

// 注意：子组件一律定义在模块层。若定义在 Layout 函数体内，每次渲染都会生成
// 新组件身份导致整棵子树重挂，与 store 更新互激形成无限循环。
interface LayoutState {
	isDomain: boolean;
	isManage: boolean;
	isHome: boolean;
	isMobile: boolean;
	splitWidth: string;
	onOpenSettings: () => void;
	onOpenNotifications: () => void;
	splitDragRef: React.RefObject<HTMLDivElement | null>;
	children: ReactNode;
}

function MobileBottomNav({ isDomain, isManage, isHome }: LayoutState) {
	const navigate = useNavigate();
	return (
		<nav className="border-border bg-card relative z-20 flex shrink-0 border-t">
			<button
				type="button"
				className={`flex flex-1 cursor-pointer flex-col items-center justify-center gap-0.5 py-1.5 text-2xs transition-colors ${isHome ? "text-primary" : "text-faint-foreground"}`}
				onClick={() => navigate({ to: "/discover" })}
			>
				<Home size={20} />
				<span>首页</span>
			</button>
			<button
				type="button"
				className={`flex flex-1 cursor-pointer flex-col items-center justify-center gap-0.5 py-1.5 text-2xs transition-colors ${isDomain ? "text-primary" : "text-faint-foreground"}`}
				onClick={() => {
					const uuid = useDomainStore.getState().currentDomainUUID;
					if (uuid)
						navigate({
							to: "/domain/$domainUUID",
							params: { domainUUID: uuid },
						});
					else navigate({ to: "/discover" });
				}}
			>
				<Headphones size={20} />
				<span>域</span>
			</button>
			<button
				type="button"
				className="text-faint-foreground flex flex-1 cursor-pointer flex-col items-center justify-center gap-0.5 py-1.5 text-2xs transition-colors"
				onClick={() => useLayoutSettings.open()}
			>
				<Settings size={20} />
				<span>设置</span>
			</button>
			{hasManageAccess() && (
				<button
					type="button"
					className={`flex flex-1 cursor-pointer flex-col items-center justify-center gap-0.5 py-1.5 text-2xs transition-colors ${isManage ? "text-primary" : "text-faint-foreground"}`}
					onClick={() =>
						navigate({
							to: `/manage/${firstAccessibleManagePath() ?? "users"}`,
						})
					}
				>
					<ShieldCheck size={20} />
					<span>管理</span>
				</button>
			)}
		</nav>
	);
}

function MobileStageTabs({
	stageTab,
	setStageTab,
}: {
	stageTab: MobileStageTab;
	setStageTab: (t: MobileStageTab) => void;
}) {
	return (
		<div className="bg-muted/50 mx-2 mt-1 mb-1 flex gap-1 rounded-lg p-1">
			{(
				[
					["voice", "语音", Headphones],
					["text", "文字", MessageSquare],
					["members", "成员", Users],
				] as const
			).map(([tab, label, Icon]) => (
				<button
					key={tab}
					type="button"
					className={`flex h-9 flex-1 cursor-pointer items-center justify-center gap-1.5 rounded-lg text-xs font-medium transition-colors ${
						stageTab === tab
							? "bg-card text-foreground shadow-sm"
							: "text-faint-foreground"
					}`}
					onClick={() => setStageTab(tab)}
				>
					<Icon size={14} />
					{label}
				</button>
			))}
		</div>
	);
}

function MobileLayout(
	state: LayoutState & {
		mobileStage: boolean;
		stageTab: MobileStageTab;
		setStageTab: (t: MobileStageTab) => void;
		leaveMobileStage: () => void;
	},
) {
	const {
		isDomain,
		isManage,
		onOpenSettings,
		children,
		mobileStage,
		stageTab,
		setStageTab,
		leaveMobileStage,
	} = state;

	const currentRoom = useSocketStore((s) => s.currentRoom);
	const selectedRoomInfo = useSocketStore((s) => s.selectedRoomInfo);
	const textRoomName = useChatStore((s) => s.textRoomName);
	const membersCount = useSocketStore((s) => s.rooms).find(
		(r) =>
			r.name === currentRoom &&
			(r.domain_uuid || "") ===
				(useDomainStore.getState().currentDomainUUID || ""),
	)?.members?.length;

	const stageTitle = () => {
		if (stageTab === "text") return textRoomName || "文字房";
		return selectedRoomInfo?.name || currentRoom || "语音房";
	};

	return (
		<div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
			{/* 域舞台：选中房间后全屏 */}
			{isDomain && mobileStage ? (
				<div className="bg-muted flex min-h-0 flex-1 flex-col overflow-hidden">
					<div className="border-border bg-card flex h-11 shrink-0 items-center gap-1 border-b px-1">
						<button
							type="button"
							className="hover:bg-muted flex size-9 cursor-pointer items-center justify-center rounded-sm transition-colors"
							onClick={leaveMobileStage}
							aria-label="返回列表"
						>
							<ArrowLeft size={20} />
						</button>
						<div className="min-w-0 flex-1 truncate text-sm font-semibold">
							{stageTitle()}
						</div>
						<span className="text-faint-foreground shrink-0 pr-2 text-xs">
							{membersCount ?? 0} 人
						</span>
					</div>
					<MobileStageTabs stageTab={stageTab} setStageTab={setStageTab} />
					<div className="min-h-0 flex-1 overflow-hidden">
						{stageTab === "voice" && (
							<div className="h-full">
								<ErrorBoundary
									name="mobile-voice-stage"
									resetKeys={[currentRoom, stageTab]}
									title="语音舞台加载失败"
								>
									<RoomDetail />
								</ErrorBoundary>
							</div>
						)}
						{stageTab === "text" && (
							<div className="bg-card h-full">
								<ErrorBoundary
									name="mobile-text-room"
									resetKeys={[textRoomName, stageTab]}
									title="文字频道加载失败"
								>
									<TextRoomPanel />
								</ErrorBoundary>
							</div>
						)}
						{stageTab === "members" && (
							<div className="bg-card h-full">
								<ErrorBoundary
									name="mobile-member-sidebar"
									resetKeys={[stageTab]}
									title="成员列表加载失败"
								>
									<MemberSidebar fullWidth />
								</ErrorBoundary>
							</div>
						)}
					</div>
					<VoiceStatusBar />
					<UserBar
						className="border-border border-t"
						onOpenSettings={onOpenSettings}
					/>
				</div>
			) : (
				<div className="flex min-h-0 flex-1 flex-col overflow-hidden">
					<div className="border-border min-h-0 flex-1 overflow-hidden border-t">
						{isDomain && (
							<div className="bg-card flex h-full flex-col">
								<div className="min-h-0 flex-1 overflow-hidden">
									<DynamicRender />
								</div>
							</div>
						)}
						{isManage && (
							<div className="bg-muted flex h-full min-h-0 flex-col">
								<div className="border-border bg-card max-h-40 shrink-0 overflow-y-auto border-b">
									<ManageNavMobile />
								</div>
								<div className="min-h-0 flex-1 overflow-hidden">{children}</div>
							</div>
						)}
						{!isDomain && !isManage && (
							<div className="bg-muted h-full overflow-y-auto">{children}</div>
						)}
					</div>
					<VoiceStatusBar />
					<UserBar
						className="border-border border-t"
						onOpenSettings={onOpenSettings}
					/>
				</div>
			)}

			{!(isDomain && mobileStage) && <MobileBottomNav {...state} />}
		</div>
	);
}

function ManageNavMobile() {
	// 移动端管理导航复用桌面组件（其布局响应式处理横竖排）
	return <ManageNav />;
}

function DesktopLayout(state: LayoutState) {
	const {
		isDomain,
		onOpenSettings,
		onOpenNotifications,
		splitDragRef,
		splitWidth,
		children,
	} = state;

	return (
		<div className="flex h-full flex-1 overflow-hidden">
			<div className="flex h-full flex-1">
				{/* prev：Sidebar + 房间列表（可拖拽调宽，替代 cui-solid Split） */}
				<div
					className="flex h-full shrink-0 flex-col justify-between"
					style={{ width: splitWidth }}
				>
					<div className="flex min-h-0 flex-1">
						<Sidebar
							onOpenSettings={onOpenSettings}
							onOpenNotifications={onOpenNotifications}
						/>
						<div className="border-border box-border min-w-0 min-h-0 flex-1 border-t border-l border-solid">
							<DynamicRender />
						</div>
					</div>
					<VoiceStatusBar />
					<UserBar onOpenSettings={onOpenSettings} />
				</div>
				<div
					ref={splitDragRef}
					className="bg-border hover:bg-primary w-1 shrink-0 cursor-col-resize transition-colors"
					title="拖动调整列表宽度"
				/>
				{/* next：主内容 + 域工作区 */}
				<Main>
					<div
						className="border-border bg-muted h-full w-full flex-1 border-t"
						style={{ display: isDomain ? "none" : undefined }}
					>
						{children}
					</div>
					<DomainWorkspace isDomain={isDomain} />
				</Main>
			</div>
		</div>
	);
}

// 设置弹窗全局单例：布局内任意层级（含 UserBar、底部导航）都能打开
let settingsOpener: (() => void) | null = null;
const useLayoutSettings = {
	open: () => settingsOpener?.(),
};

export default function Layout({ children }: { children: ReactNode }) {
	const location = useLocation();
	// 全局快捷键（M 静音等）：挂在布局层，登录后的所有页面都生效
	useGlobalShortcuts();
	// 把主窗口的房间/成员/发言状态接力推给原生覆盖层 HUD（仅桌面端生效）。
	useOverlayRelay();

	const isDomain = location.pathname.startsWith("/domain");
	const isManage = location.pathname.startsWith("/manage");
	const isHome =
		location.pathname === "/" || location.pathname.startsWith("/index");

	const [splitWidth, setSplitWidth] = useState(() => {
		const saved = Number.parseInt(localStorage.getItem("splitWidth") ?? "", 10);
		return `${clampSplitWidth(Number.isFinite(saved) ? saved : MIN_SPLIT_WIDTH)}px`;
	});
	const splitDragRef = useRef<HTMLDivElement>(null);
	// 拖拽回调只注册一次，用 ref 读取当前宽度，避免每次宽度变化都重绑监听器
	const splitWidthRef = useRef(splitWidth);
	splitWidthRef.current = splitWidth;

	// 移动端：列表 vs 舞台；域内 tab
	const [mobileStage, setMobileStage] = useState(false);
	const [stageTab, setStageTab] = useState<MobileStageTab>("voice");
	const [isMobile, setIsMobile] = useState(
		typeof window !== "undefined" &&
			window.matchMedia("(max-width: 767px)").matches,
	);
	const [settingsOpen, setSettingsOpen] = useState(false);
	const [notificationsOpen, setNotificationsOpen] = useState(false);

	useEffect(() => {
		settingsOpener = () => setSettingsOpen(true);
		return () => {
			settingsOpener = null;
		};
	}, []);

	const selectedRoomInfo = useSocketStore((s) => s.selectedRoomInfo);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const textRoom = useChatStore((s) => s.textRoom);
	const workspaceStage = useWorkspaceUiStore((s) => s.stage);
	const hasVoiceFocus = !!selectedRoomInfo || !!currentRoom;
	const hasTextFocus = !!textRoom;
	const hasRoomFocus = hasVoiceFocus || hasTextFocus;
	const focusKey = `${selectedRoomInfo?.name ?? currentRoom ?? ""}|${textRoom ?? ""}`;

	useEffect(() => {
		const mq = window.matchMedia("(max-width: 767px)");
		const onChange = (e: MediaQueryListEvent) => setIsMobile(e.matches);
		mq.addEventListener("change", onChange);
		return () => mq.removeEventListener("change", onChange);
	}, []);

	// 移动端 tab 跟随主舞台：点文字频道就切文字，点语音频道就回语音
	useEffect(() => {
		if (!isMobile || !isDomain) return;
		setStageTab(workspaceStage === "text" ? "text" : "voice");
	}, [workspaceStage, isMobile, isDomain]);

	// 进房自动切舞台；主动收起只收视图，不断开语音连接
	const [mobileStageDismissed, setMobileStageDismissed] = useState(false);
	const prevFocusKey = useRef(focusKey);

	// 从其它页面点左下语音状态条时，重新展开移动端舞台
	const voiceFocusNonce = useWorkspaceUiStore((s) => s.voiceFocusNonce);
	const lastVoiceFocusNonce = useRef(voiceFocusNonce);
	useEffect(() => {
		if (voiceFocusNonce === lastVoiceFocusNonce.current) return;
		lastVoiceFocusNonce.current = voiceFocusNonce;
		setMobileStageDismissed(false);
	}, [voiceFocusNonce]);
	useEffect(() => {
		if (!isMobile || !isDomain) {
			setMobileStage(false);
			return;
		}
		const focusChanged = focusKey !== prevFocusKey.current;
		prevFocusKey.current = focusKey;
		if (focusChanged) setMobileStageDismissed(false);
		const dismissed = mobileStageDismissed && !focusChanged;
		if (hasRoomFocus && !dismissed) {
			setMobileStage(true);
		} else if (!hasRoomFocus) {
			setMobileStage(false);
		}
	}, [isMobile, isDomain, hasRoomFocus, focusKey, mobileStageDismissed]);

	// 中栏可拖拽调宽（替代 cui-solid Split）
	// 依赖 isMobile：桌面/移动切换会挂载/卸载分隔条，需要重新绑定
	useEffect(() => {
		if (isMobile) return;
		const handle = splitDragRef.current;
		if (!handle) return;

		let onMouseMove: ((e: MouseEvent) => void) | null = null;
		let onMouseUp: (() => void) | null = null;
		let cleanupDrag: (() => void) | null = null;

		const stopDrag = () => {
			if (onMouseMove) document.removeEventListener("mousemove", onMouseMove);
			if (onMouseUp) document.removeEventListener("mouseup", onMouseUp);
			onMouseMove = null;
			onMouseUp = null;
			cleanupDrag?.();
			cleanupDrag = null;
		};

		const onMouseDown = (e: MouseEvent) => {
			if (e.button !== 0) return;
			e.preventDefault();
			stopDrag();
			const startX = e.clientX;
			const startWidth =
				Number.parseInt(splitWidthRef.current, 10) || MIN_SPLIT_WIDTH;

			// 拖动时禁止选中文本，并统一鼠标样式
			const prevUserSelect = document.body.style.userSelect;
			const prevCursor = document.body.style.cursor;
			document.body.style.userSelect = "none";
			document.body.style.cursor = "col-resize";
			cleanupDrag = () => {
				document.body.style.userSelect = prevUserSelect;
				document.body.style.cursor = prevCursor;
			};

			onMouseMove = (ev: MouseEvent) => {
				const w = clampSplitWidth(startWidth + (ev.clientX - startX));
				const width = `${w}px`;
				splitWidthRef.current = width;
				setSplitWidth(width);
				localStorage.setItem("splitWidth", width);
			};
			onMouseUp = () => stopDrag();
			document.addEventListener("mousemove", onMouseMove);
			document.addEventListener("mouseup", onMouseUp);
		};

		handle.addEventListener("mousedown", onMouseDown);
		return () => {
			handle.removeEventListener("mousedown", onMouseDown);
			stopDrag();
		};
	}, [isMobile]);

	const leaveMobileStage = () => {
		// 只收起舞台，不清房间状态：语音连接保持，左下状态条仍可回到通话
		setMobileStageDismissed(true);
		setMobileStage(false);
	};

	const state: LayoutState = {
		isDomain,
		isManage,
		isHome,
		isMobile,
		splitWidth,
		onOpenSettings: () => setSettingsOpen(true),
		onOpenNotifications: () => setNotificationsOpen(true),
		splitDragRef,
		children,
	};

	return (
		<VoiceSessionProvider>
			<div className="flex h-full min-h-0 flex-1 flex-col">
				{/* 桌面构建的顶栏由根路由的 DesktopTitleBar 提供，这里只服务 Web 构建；
				    移动端舞台自带标题栏，列表页用精简顶栏 */}
				{!__GOSPEAK_DESKTOP__ && !isMobile && <Header />}
				{!__GOSPEAK_DESKTOP__ && isMobile && !(isDomain && mobileStage) && (
					<Header />
				)}

				{isMobile ? (
					<MobileLayout
						{...state}
						mobileStage={mobileStage}
						stageTab={stageTab}
						setStageTab={setStageTab}
						leaveMobileStage={leaveMobileStage}
					/>
				) : (
					<DesktopLayout {...state} />
				)}

				<SettingModal
					open={settingsOpen}
					onClose={() => setSettingsOpen(false)}
				/>
				<NotificationModal
					open={notificationsOpen}
					onClose={() => setNotificationsOpen(false)}
				/>
				{/* 设备增删时全局询问是否切换（单例，与设置弹窗互不干扰） */}
				<DeviceChangeNotice />
			</div>
		</VoiceSessionProvider>
	);
}
