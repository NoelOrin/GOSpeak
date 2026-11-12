import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
	toast,
} from "@gospeak/components";
import {
	normalizeScreenShareQuality,
	SCREEN_SHARE_QUALITY_ORDER,
	SCREEN_SHARE_QUALITY_PRESETS,
} from "@gospeak/sfu-client";
import { useLocation, useNavigate } from "@tanstack/react-router";
import clsx from "clsx";
import { MonitorUp, PhoneOff, Signal } from "lucide-react";
import { useEffect } from "react";
import { useDomainStore } from "@/stores/domainStore";
import { useScreenShareStore } from "@/stores/screenShareStore";
import ScreenShareSettingsStore, {
	useScreenShareSettingsStore,
} from "@/stores/screenShareSettingsStore";
import { socketStore, useSocketStore } from "@/stores/socketStore";
import { useWorkspaceUiStore } from "@/stores/workspaceUiStore";
import { useVoiceSessionContext } from "./VoiceSessionProvider";

const TONE = {
	success: { dot: "bg-success", text: "text-success" },
	warning: { dot: "bg-warning", text: "text-warning" },
	error: { dot: "bg-destructive", text: "text-destructive" },
	muted: { dot: "bg-foreground/40", text: "text-faint-foreground" },
};

/**
 * 常驻语音连接条（Discord 左下角的 Voice Connected）。
 *
 * 只负责「显示状态 + 点击回到语音频道」：
 * - 麦克风 / 扬声器统一由左下用户栏控制
 * - 断开统一由语音视图底部的红色按钮负责
 * - 文字聊天统一由频道标题栏切换
 * 这样同一个操作不会在页面上出现两套按钮。
 */
export default function VoiceStatusBar({ className }: { className?: string }) {
	const selectedRoom = useSocketStore((s) => s.selectedRoomInfo);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const domainCache = useDomainStore((s) => s.domainCache);
	const {
		phase,
		isJoined,
		isReconnecting,
		isLoading,
		phaseLabel,
		handleManualLeave,
		sfuClient,
	} = useVoiceSessionContext();
	const requestVoiceView = useWorkspaceUiStore((s) => s.requestVoiceView);
	const localSharing = useScreenShareStore((s) => s.localSharing);
	const canShareScreen = !!sfuClient?.setScreenShareEnabled;
	const quality = useScreenShareSettingsStore((s) => s.quality);
	const location = useLocation();
	const navigate = useNavigate();

	useEffect(() => {
		if (!sfuClient?.setScreenShareQuality) return;
		void sfuClient.setScreenShareQuality(quality).catch((error: unknown) => {
			console.warn("[voice] 设置共享画质失败:", error);
		});
	}, [sfuClient, quality]);

	/**
	 * 从任意页面（发现域 / 聊天 / 管理）点状态条，都回到语音房间所在域的房间页。
	 * 只改 stage 不导航的话，离开域路由后点了没有任何反应。
	 */
	const openVoiceRoom = () => {
		requestVoiceView();
		const domainUUID =
			selectedRoom?.domain_uuid ?? socketStore.currentDomainUUID();
		if (!domainUUID) return;
		if (location.pathname.startsWith(`/domain/${domainUUID}`)) return;
		void navigate({
			to: "/domain/$domainUUID",
			params: { domainUUID },
		});
	};

	if (!selectedRoom && !currentRoom) return null;

	const roomName = selectedRoom?.name ?? currentRoom ?? "";
	const domainName = selectedRoom?.domain_uuid
		? domainCache[selectedRoom.domain_uuid]?.name
		: undefined;

	const state = (() => {
		if (phase === "failed") return { tone: "error", label: "连接失败" };
		if (isReconnecting) return { tone: "warning", label: "正在重连…" };
		if (isJoined) return { tone: "success", label: "语音已连接" };
		if (isLoading) return { tone: "warning", label: phaseLabel };
		return { tone: "muted", label: "准备连接…" };
	})();
	const tone = TONE[state.tone as keyof typeof TONE];

	return (
		<div
			className={clsx(
				"bg-muted/60 border-border flex shrink-0 items-center gap-1 border-t px-2 py-1.5 select-none",
				className,
			)}
		>
			<Signal size={16} className={clsx("ml-1 shrink-0", tone.text)} />
			<button
				type="button"
				className="hover:bg-card/70 flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-sm px-1 py-0.5 text-left transition-colors"
				onClick={openVoiceRoom}
				title="返回语音房间"
			>
				<span className={clsx("size-2 shrink-0 rounded-full", tone.dot)} />
				<span className="flex min-w-0 flex-col">
					<span className={clsx("text-xs font-semibold", tone.text)}>
						{state.label}
					</span>
					<span className="text-muted-foreground truncate text-2xs">
						{roomName}
						{domainName ? ` / ${domainName}` : ""}
					</span>
				</span>
			</button>
			{canShareScreen && (
				<>
					<Select
						value={quality}
						onValueChange={(value) =>
							ScreenShareSettingsStore.setQuality(
								normalizeScreenShareQuality(value),
							)
						}
					>
						<SelectTrigger
							className="h-7 w-auto shrink-0 px-2 text-xs"
							title="共享画质"
							aria-label="共享画质"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{SCREEN_SHARE_QUALITY_ORDER.map((item) => (
								<SelectItem key={item} value={item}>
									{SCREEN_SHARE_QUALITY_PRESETS[item].label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<button
						type="button"
						className={clsx(
							"flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-sm transition-colors",
							localSharing
								? "bg-card text-primary"
								: "text-muted-foreground hover:bg-card hover:text-foreground",
						)}
						onClick={() => {
							if (!sfuClient?.setScreenShareEnabled) return;
							// 浏览器选择框被取消时 LiveKit 会抛错，保持原来的共享状态即可；
							// 但真正的采集失败（权限/设备）需要让用户知道
							void sfuClient
								.setScreenShareEnabled(!localSharing)
								.catch((error: unknown) => {
									const name = (error as { name?: string })?.name;
									if (name === "NotAllowedError") return;
									toast.error(
										error instanceof Error && error.message
											? `屏幕共享失败：${error.message}`
											: "屏幕共享失败",
									);
								});
						}}
						title={localSharing ? "停止共享屏幕" : "共享屏幕"}
						aria-label={localSharing ? "停止共享屏幕" : "共享屏幕"}
					>
						<MonitorUp size={16} />
					</button>
				</>
			)}
			<button
				type="button"
				className="text-muted-foreground hover:bg-card hover:text-destructive flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-sm transition-colors"
				onClick={() => void handleManualLeave()}
				title="断开连接"
				aria-label="断开连接"
			>
				<PhoneOff size={16} />
			</button>
		</div>
	);
}
