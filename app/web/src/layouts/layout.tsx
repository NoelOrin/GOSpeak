import { useLocation, useNavigate } from "@tanstack/solid-router";
import { Slot, Split } from "cui-solid";
import ArrowLeft from "lucide-solid/icons/arrow-left";
import Headphones from "lucide-solid/icons/headphones";
import Home from "lucide-solid/icons/home";
import MessageSquare from "lucide-solid/icons/message-square";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import Users from "lucide-solid/icons/users";
import {
	createEffect,
	createMemo,
	createSignal,
	type JSX,
	onCleanup,
	Show,
} from "solid-js";
import DynamicRender from "@/components/common/dynamicRender";
import SettingModal from "@/components/modal/settting/settingModal";
import MemberSidebar from "@/components/room/components/memberSidebar";
import RoomDetail from "@/components/room/roomDetail";
import TextRoomPanel from "@/components/textRoom/TextRoomPanel";
import UserBar from "@/components/userBar";
import { useIsMobile } from "@/hooks/useBreakpoint";
import { useTitle } from "@/hooks/useTitle";
import DomainWorkspace from "@/layouts/common/domainWorkspace";
import Header from "@/layouts/common/header";
import Main from "@/layouts/common/main";
import Sidebar from "@/layouts/common/sidebar";
import { chatStore } from "@/stores/chatStore";
import domainStore from "@/stores/domainStore";
import { socketStore } from "@/stores/socketStore";
import { hasManageAccess } from "@/utils/permissions";

type MobileStageTab = "voice" | "text" | "members";

const Layout = ({ children }: { children: JSX.Element }) => {
	const location = useLocation();
	const navigate = useNavigate();
	const isMobile = useIsMobile();
	useTitle();

	const isDomain = () => location().pathname.startsWith("/domain");
	const isManage = () => location().pathname.startsWith("/manage");
	const isHome = () =>
		location().pathname === "/" || location().pathname.startsWith("/index");

	const [splitWidth, setSplitWidth] = createSignal(
		localStorage.getItem("splitWidth") || "333px",
	);

	const MIN_SPLIT_WIDTH = 333;
	const MAX_SPLIT_WIDTH = 525;

	let prevRef!: HTMLDivElement;
	let settingModalRef!: HTMLDialogElement;

	// 移动端：列表 vs 舞台；域内 tab
	const [mobileStage, setMobileStage] = createSignal(false);
	const [stageTab, setStageTab] = createSignal<MobileStageTab>("voice");

	const hasVoiceFocus = createMemo(
		() => !!socketStore.selectedRoomInfo() || !!socketStore.currentRoom(),
	);
	const hasTextFocus = createMemo(() => !!chatStore.textRoom());
	const hasChannelFocus = createMemo(() => hasVoiceFocus() || hasTextFocus());

	// 进房自动切舞台；退房回列表
	createEffect(() => {
		if (!isMobile() || !isDomain()) {
			setMobileStage(false);
			return;
		}
		if (hasChannelFocus()) {
			setMobileStage(true);
			if (hasVoiceFocus() && !hasTextFocus()) setStageTab("voice");
			else if (hasTextFocus() && !hasVoiceFocus()) setStageTab("text");
		}
	});

	createEffect(() => {
		if (!prevRef || isMobile()) return;
		const resizeObserver = new ResizeObserver((entries) => {
			for (const _ of entries) {
				const prevWidth = prevRef.offsetWidth;
				setSplitWidth(`${prevWidth}px`);
				localStorage.setItem("splitWidth", `${prevWidth}px`);
			}
		});
		resizeObserver.observe(prevRef);
		onCleanup(() => {
			resizeObserver.disconnect();
		});
	});

	const openSettings = () => settingModalRef?.showModal?.();

	const leaveMobileStage = () => {
		// 清焦点，否则 effect 会立刻再打开舞台
		socketStore.clearSelectedRoom();
		if (chatStore.textRoom()) chatStore.leaveTextRoom();
		setMobileStage(false);
	};

	const stageTitle = () => {
		if (stageTab() === "text") {
			return chatStore.textRoomName() || "文字频道";
		}
		return (
			socketStore.selectedRoomInfo()?.name ||
			socketStore.currentRoom() ||
			"语音频道"
		);
	};

	const MobileBottomNav = () => {
		const itemClass = (active: boolean) =>
			`flex flex-1 flex-col items-center justify-center gap-0.5 py-1.5 text-[10px] transition-colors ${
				active
					? "text-primary"
					: "text-base-content/50 active:text-base-content"
			}`;

		return (
			<nav class="relative z-20 flex shrink-0 border-t border-base-300 bg-base-100 safe-bottom">
				<button
					type="button"
					class={itemClass(isHome())}
					onClick={() => navigate({ to: "/" })}
				>
					<Home size={20} strokeWidth={2.1} />
					<span>首页</span>
				</button>
				<button
					type="button"
					class={itemClass(isDomain())}
					onClick={() => {
						const uuid = domainStore.state.currentDomainUUID;
						if (uuid)
							navigate({
								to: "/domain/$domainUUID",
								params: { domainUUID: uuid },
							});
						else navigate({ to: "/discover" });
					}}
				>
					<Headphones size={20} strokeWidth={2.1} />
					<span>域</span>
				</button>
				<button type="button" class={itemClass(false)} onClick={openSettings}>
					<Settings size={20} strokeWidth={2.1} />
					<span>设置</span>
				</button>
				<Show when={hasManageAccess()}>
					<button
						type="button"
						class={itemClass(isManage())}
						onClick={() => navigate({ to: "/manage" })}
					>
						<ShieldCheck size={20} strokeWidth={2.1} />
						<span>管理</span>
					</button>
				</Show>
			</nav>
		);
	};

	const MobileStageTabs = () => {
		const tabBtn = (tab: MobileStageTab, label: string, icon: JSX.Element) => (
			<button
				type="button"
				class={`flex flex-1 items-center justify-center gap-1.5 h-9 text-xs font-medium rounded-lg transition-colors ${
					stageTab() === tab
						? "bg-base-100 text-base-content shadow-sm"
						: "text-base-content/50"
				}`}
				onClick={() => setStageTab(tab)}
			>
				{icon}
				{label}
			</button>
		);

		return (
			<div class="flex gap-1 mx-2 mt-1 mb-1 p-1 rounded-xl bg-base-300/50">
				{tabBtn("voice", "语音", <Headphones size={14} />)}
				{tabBtn("text", "文字", <MessageSquare size={14} />)}
				{tabBtn("members", "成员", <Users size={14} />)}
			</div>
		);
	};

	const MobileLayout = () => (
		<div class="flex flex-col flex-1 h-full min-h-0 overflow-hidden">
			{/* 域舞台：选中房间后全屏 */}
			<Show when={isDomain() && mobileStage()}>
				<div class="flex flex-col flex-1 min-h-0 overflow-hidden bg-base-200">
					<div class="flex items-center gap-1 h-11 px-1 border-b border-base-300 bg-base-100 shrink-0">
						<button
							type="button"
							class="btn btn-ghost btn-sm btn-square"
							onClick={leaveMobileStage}
							aria-label="返回列表"
						>
							<ArrowLeft size={18} />
						</button>
						<div class="min-w-0 flex-1 font-semibold text-sm truncate">
							{stageTitle()}
						</div>
						<span class="text-xs text-base-content/50 pr-2 shrink-0">
							{socketStore.members().length} 人
						</span>
					</div>
					<MobileStageTabs />
					<div class="flex-1 min-h-0 overflow-hidden">
						<Show when={stageTab() === "voice"}>
							<div class="h-full">
								<RoomDetail />
							</div>
						</Show>
						<Show when={stageTab() === "text"}>
							<div class="h-full bg-base-100">
								<TextRoomPanel />
							</div>
						</Show>
						<Show when={stageTab() === "members"}>
							<div class="h-full bg-base-100">
								<MemberSidebar fullWidth />
							</div>
						</Show>
					</div>
					<UserBar
						class="border-t border-base-300"
						onOpenSettings={openSettings}
					/>
				</div>
			</Show>

			{/* 列表 / 内容层 */}
			<Show when={!(isDomain() && mobileStage())}>
				<div class="flex flex-col flex-1 min-h-0 overflow-hidden">
					<div class="flex-1 min-h-0 overflow-hidden border-t border-base-300">
						{/* 域：房间列表 */}
						<Show when={isDomain()}>
							<div class="flex flex-col h-full bg-base-100">
								<div class="flex-1 min-h-0 overflow-hidden">
									<DynamicRender />
								</div>
							</div>
						</Show>
						{/* 管理：左侧导航 + 页面 */}
						<Show when={isManage()}>
							<div class="flex flex-col h-full min-h-0 bg-base-200">
								<div class="shrink-0 border-b border-base-300 bg-base-100 max-h-40 overflow-y-auto">
									<DynamicRender />
								</div>
								<div class="flex-1 min-h-0 overflow-hidden">{children}</div>
							</div>
						</Show>
						{/* 首页 / 其他：主内容 */}
						<Show when={!isDomain() && !isManage()}>
							<div class="h-full overflow-y-auto bg-base-200">{children}</div>
						</Show>
					</div>
					<UserBar
						class="border-t border-base-300"
						onOpenSettings={openSettings}
					/>
				</div>
			</Show>

			<Show when={!(isDomain() && mobileStage())}>
				<MobileBottomNav />
			</Show>
		</div>
	);

	const DesktopLayout = () => (
		<div class="flex flex-1 h-full overflow-hidden">
			<div class="flex flex-1 h-full">
				<Split min={MIN_SPLIT_WIDTH} split={splitWidth()} max={MAX_SPLIT_WIDTH}>
					<Slot name="prev">
						<div class="flex flex-col justify-between h-full" ref={prevRef}>
							<div class="flex h-full">
								<Sidebar onOpenSettings={openSettings} />
								<div class="box-border min-w-0 flex-1 border-color border-t border-l border-solid">
									<DynamicRender />
								</div>
							</div>
							<UserBar onOpenSettings={openSettings} />
						</div>
					</Slot>

					<Slot name="next">
						<Main>
							<div
								class="flex-1 border-color border-t w-full h-full bg-base-200"
								style={{ display: isDomain() ? "none" : undefined }}
							>
								{children}
							</div>
							<DomainWorkspace isDomain={isDomain} />
						</Main>
					</Slot>
				</Split>
			</div>
		</div>
	);

	return (
		<div class="flex flex-col flex-1 h-full min-h-0">
			{/* 桌面顶栏；移动端舞台自带标题栏，列表页用精简顶栏 */}
			<Show when={!isMobile()}>
				<Header />
			</Show>
			<Show when={isMobile() && !(isDomain() && mobileStage())}>
				<Header />
			</Show>

			<Show when={isMobile()} fallback={<DesktopLayout />}>
				<MobileLayout />
			</Show>

			<SettingModal
				ref={settingModalRef}
				onClose={() => settingModalRef.close()}
			/>
		</div>
	);
};
export default Layout;
