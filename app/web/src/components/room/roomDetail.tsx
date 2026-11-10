import { Slider } from "@gospeak/components";
import { clsx } from "clsx";
import { Ellipsis, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import Avatar from "@/components/common/avatar";
import ConfirmModal from "@/components/common/ConfirmModal";
import PasswordModal from "@/components/room/components/passwordModal";
import { setMutedByIdentity, setVolumeByIdentity } from "@/handler_audio";
import { useSpeakingStore } from "@/handler_audio/speakingStore";
import { useScreenShareStore } from "@/stores/screenShareStore";
import {
	type MemberInfo,
	socketStore,
	useSocketStore,
} from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import {
	useVoiceChatActions,
	useVoiceChatStore,
} from "@/stores/voiceChatStore";
import {
	isRoomPasswordError,
	localizedApiMessage,
} from "@/utils/errorMessages";
import { hasPermission } from "@/utils/permissions";
import ScreenShareView from "./ScreenShareView";
import { useVoiceSessionContext } from "./VoiceSessionProvider";

/** Discord 语音频道视图：参与者卡片网格 + 底部断开按钮 */
export default function RoomDetail() {
	// 语音会话由常驻 Provider 持有，切到文字频道不会中断通话
	const { selectedRoom, phase, phaseLabel, isJoined, isLoading, error, retry } =
		useVoiceSessionContext();

	const members = useSocketStore((s) => s.rooms);
	const currentRoomName = useSocketStore((s) => s.currentRoom);
	const myName = useUserStore((s) => s.user?.name);
	const currentDomainUUID = socketStore.currentDomainUUID();

	const roomMembers =
		members.find(
			(r) =>
				r.name === currentRoomName &&
				(r.domain_uuid || "") === (currentDomainUUID || ""),
		)?.members ?? [];

	const sortedMembers = [...roomMembers].sort((a, b) => {
		if (a.identity === myName) return -1;
		if (b.identity === myName) return 1;
		return 0;
	});

	const containerRef = useRef<HTMLDivElement>(null);

	// 屏幕共享：有画面时占满主区，成员卡片暂时让位
	const remoteVideos = useScreenShareStore((s) => s.remote);
	const localSharing = useScreenShareStore((s) => s.localSharing);
	const localTrack = useScreenShareStore((s) => s.localTrack);
	const sharedVideo =
		remoteVideos.find((item) => item.source === "screen") ?? remoteVideos[0];
	const sharedTrack = sharedVideo?.track ?? (localSharing ? localTrack : null);
	const sharedLabel = sharedVideo
		? `${sharedVideo.identity} 正在共享屏幕`
		: localSharing
			? "你正在共享屏幕"
			: "";

	// 全屏目标是共享画面 <video> 本身，按钮浮在视频右下角
	const shareVideoRef = useRef<HTMLVideoElement>(null);
	const [shareFullscreen, setShareFullscreen] = useState(false);

	useEffect(() => {
		const onChange = () => {
			setShareFullscreen(document.fullscreenElement === shareVideoRef.current);
		};
		document.addEventListener("fullscreenchange", onChange);
		return () => document.removeEventListener("fullscreenchange", onChange);
	}, []);

	const toggleShareFullscreen = () => {
		const el = shareVideoRef.current;
		if (!el) return;
		if (document.fullscreenElement) {
			void document.exitFullscreen().catch(() => {});
			return;
		}
		void el.requestFullscreen?.().catch(() => {});
	};

	// 密码错时允许重新输入：此前"重试"只是用同一个错误密码再发一次，必然再失败
	const [reenterPassword, setReenterPassword] = useState(false);
	const failedMessage = error
		? localizedApiMessage({ response: { data: { msg: error } } }, error)
		: "";
	const needsPassword = isRoomPasswordError({
		response: { data: { msg: error ?? "" } },
	});

	return (
		<div className="bg-card relative flex h-full w-full min-w-0 flex-col select-none">
			{!selectedRoom ? (
				<div className="text-muted-foreground flex h-full flex-col items-center justify-center gap-2 text-sm">
					<Volume2 size={32} strokeWidth={1.5} />
					<span>请从左侧选择一个语音频道</span>
				</div>
			) : !isJoined ? (
				/* 未加入时状态直接作为面板内容，而不是盖在成员网格上的浮层卡片：
				   房间页此时还没有成员可看，浮层只会多一层阴影和一块背景。 */
				<JoinStatus
					roomName={selectedRoom.name}
					failed={phase === "failed"}
					message={failedMessage || phaseLabel}
					loadingLabel={isLoading ? phaseLabel : "准备加入..."}
					needsPassword={needsPassword}
					onRetry={retry}
					onReenterPassword={() => setReenterPassword(true)}
				/>
			) : sharedTrack ? (
				<div className="bg-sunken relative flex min-h-0 flex-1 items-center justify-center p-3">
					{/* 直接点画面全屏，不额外加浮层按钮/包裹元素 */}
					<ScreenShareView
						ref={shareVideoRef}
						track={sharedTrack}
						onClick={toggleShareFullscreen}
						fullscreen={shareFullscreen}
						onToggleFullscreen={toggleShareFullscreen}
						className={`rounded-lg ${
							shareFullscreen ? "cursor-zoom-out" : "cursor-zoom-in"
						}`}
					/>
					<div className="bg-card border-border text-foreground absolute top-3 left-3 rounded-md border px-2 py-1 text-xs shadow-sm">
						{sharedLabel}
					</div>
				</div>
			) : (
				<div className="ds-scroll min-h-0 flex-1 overflow-y-auto p-4">
					<div
						className="box-border grid h-full min-h-[180px] w-full place-content-center gap-4"
						style={{
							gridTemplateColumns:
								"repeat(auto-fit, minmax(min(100%, 240px), 1fr))",
						}}
						ref={containerRef}
					>
						{roomMembers.length > 0 ? (
							sortedMembers.map((member) => (
								<MemberCard key={member.identity} member={member} />
							))
						) : (
							<div className="text-muted-foreground col-span-full flex h-32 items-center justify-center text-sm">
								已连接，等待成员加入
							</div>
						)}
					</div>
				</div>
			)}

			{/* 断开统一在左下语音状态条；麦克风/扬声器在左下用户栏，
			    文字聊天在频道标题栏，主区内不再重复放置控制按钮 */}

			{reenterPassword && selectedRoom && (
				<PasswordModal
					room={selectedRoom}
					onClose={() => {
						setReenterPassword(false);
						// 新密码已写入 selectedRoomInfo，重新取 token 才会带上它
						retry();
					}}
				/>
			)}
		</div>
	);
}

/**
 * 加入中的状态视图：房间名 + 进度 / 失败原因 + 重试。
 *
 * 直接铺在房间面板里（0.4.0 的加载样式），不做卡片、不加阴影 —— 它本来就是
 * 这个面板唯一的内容，而不是盖在内容上的对话框。
 */
function JoinStatus({
	roomName,
	failed,
	message,
	loadingLabel,
	needsPassword,
	onRetry,
	onReenterPassword,
}: {
	roomName: string;
	failed: boolean;
	message: string;
	loadingLabel: string;
	needsPassword: boolean;
	onRetry: () => void;
	onReenterPassword: () => void;
}) {
	return (
		<div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
			<div className="text-foreground text-lg font-bold">{roomName}</div>
			{failed ? (
				<>
					<div className="text-destructive max-w-md text-sm">{message}</div>
					<div className="flex items-center gap-2">
						{needsPassword && (
							<button
								type="button"
								className="bg-primary text-primary-foreground inline-flex h-9 cursor-pointer items-center rounded-sm px-4 text-sm font-medium hover:opacity-90"
								onClick={onReenterPassword}
							>
								重新输入密码
							</button>
						)}
						<button
							type="button"
							className={
								needsPassword
									? "border-border hover:bg-muted inline-flex h-9 cursor-pointer items-center rounded-sm border px-4 text-sm transition-colors"
									: "bg-primary text-primary-foreground inline-flex h-9 cursor-pointer items-center rounded-sm px-4 text-sm font-medium hover:opacity-90"
							}
							onClick={onRetry}
						>
							重试
						</button>
					</div>
				</>
			) : (
				<>
					<span className="border-primary size-5 animate-spin rounded-full border-2 border-t-transparent" />
					<div className="text-muted-foreground text-sm">{loadingLabel}</div>
				</>
			)}
		</div>
	);
}

function MemberCard({ member }: { member: MemberInfo }) {
	const isMe = member.identity === useUserStore.getState().user?.name;
	const user = useUserStore((s) => s.user);
	const displayName = isMe
		? user?.display_name || member.displayName || member.name || member.identity
		: member.displayName || member.name || member.identity;

	// selector 只返回原始值：若写成 `otherMemberState[id] ?? { ... }`，
	// 每次渲染都会产生新对象，useSyncExternalStore 判定快照变化 →
	// 无限重渲染（React: Maximum update depth exceeded）。
	const volume = useVoiceChatStore(
		(s) => s.otherMemberState[member.identity]?.outputVolume ?? 100,
	);
	const isMuted = useVoiceChatStore(
		(s) => s.otherMemberState[member.identity]?.isMute ?? false,
	);
	const speakingIdentities = useSpeakingStore((s) => s.speakingIdentities);
	const isSpeaking = speakingIdentities.includes(member.identity);
	const { setMemberOutputVolume, setMemberMute } = useVoiceChatActions();

	useEffect(() => {
		setVolumeByIdentity(member.identity, volume / 100);
		setMutedByIdentity(member.identity, isMuted);
	}, [member.identity, volume, isMuted]);

	const [menuOpen, setMenuOpen] = useState(false);
	const [confirmKick, setConfirmKick] = useState(false);

	return (
		<div
			className={clsx(
				"bg-muted relative flex aspect-video w-full min-w-0 items-center justify-center overflow-hidden rounded-lg transition-shadow",
				isSpeaking && "ring-2 ring-success",
			)}
		>
			<Avatar
				src={member.avatar}
				name={displayName}
				alt={member.name}
				className="size-20"
				textClass="text-2xl"
			/>

			<div className="bg-sunken absolute inset-x-0 bottom-0 flex items-center justify-between gap-2 px-3 py-1.5">
				<span className="text-foreground truncate text-sm font-medium">
					{displayName}
				</span>
				{isMuted && (
					<span className="text-destructive shrink-0 text-2xs">已静音</span>
				)}
			</div>

			{!isMe && (
				<div className="absolute top-2 right-2">
					<button
						type="button"
						className="bg-sunken text-foreground hover:text-foreground flex size-7 cursor-pointer items-center justify-center rounded-sm transition-colors"
						onClick={() => setMenuOpen(!menuOpen)}
						aria-label="成员操作"
					>
						<Ellipsis size={16} />
					</button>
					{menuOpen && (
						<>
							<div
								className="fixed inset-0 z-40"
								onClick={() => setMenuOpen(false)}
								aria-hidden="true"
							/>
							<div className="absolute top-8 right-0 z-50 w-40 overflow-hidden rounded-lg border shadow-xl">
								<div className="bg-card border-border border">
									<button
										type="button"
										className="text-foreground hover:bg-muted block w-full cursor-pointer px-3 py-2 text-left text-sm"
										onClick={() => {
											setMemberMute(member.identity, !isMuted);
											setMutedByIdentity(member.identity, !isMuted);
											setMenuOpen(false);
										}}
									>
										{isMuted ? "取消静音" : "静音"}
									</button>
									{hasPermission("signal:kick") && (
										<button
											type="button"
											className="hover:text-destructive hover:bg-muted block w-full cursor-pointer px-3 py-2 text-left text-sm"
											onClick={() => {
												setMenuOpen(false);
												setConfirmKick(true);
											}}
										>
											踢出
										</button>
									)}
								</div>
							</div>
						</>
					)}
				</div>
			)}

			{!isMe && (
				<div className="bg-sunken absolute top-2 left-2 flex w-40 items-center gap-1.5 rounded-sm px-2 py-1">
					<button
						type="button"
						className={clsx(
							"flex shrink-0 cursor-pointer items-center transition-colors",
							isMuted
								? "text-destructive hover:text-destructive"
								: "text-muted-foreground hover:text-foreground",
						)}
						onClick={() => {
							setMemberMute(member.identity, !isMuted);
							setMutedByIdentity(member.identity, !isMuted);
						}}
						aria-label={isMuted ? "取消静音" : "静音"}
					>
						{isMuted ? <VolumeX size={14} /> : <Volume2 size={14} />}
					</button>
					<Slider
						value={[volume]}
						max={100}
						step={1}
						disabled={isMuted}
						className="w-full"
						aria-label={`${displayName} 音量`}
						onValueChange={(value) => {
							const val = value[0] ?? 0;
							setMemberOutputVolume(member.identity, val);
							setVolumeByIdentity(member.identity, val / 100);
						}}
					/>
					<button
						type="button"
						className="text-muted-foreground hover:text-foreground shrink-0 cursor-pointer text-2xs tabular-nums transition-colors"
						onClick={() => {
							setMemberOutputVolume(member.identity, 100);
							setVolumeByIdentity(member.identity, 1);
						}}
						aria-label={`${displayName} 音量 ${volume}%，点击恢复 100%`}
					>
						{volume}%
					</button>
				</div>
			)}

			<ConfirmModal
				open={confirmKick}
				title="移出语音房间"
				message={
					<span>确认把 {displayName} 移出当前语音房间？对方可以重新加入。</span>
				}
				confirmText="移出"
				onClose={() => setConfirmKick(false)}
				onConfirm={() => {
					const room = socketStore.currentRoom();
					if (room) socketStore.kickMember(room, member.identity);
					setConfirmKick(false);
				}}
			/>
		</div>
	);
}
