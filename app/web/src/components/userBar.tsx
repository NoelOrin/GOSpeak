import clsx from "clsx";
import { useState } from "react";
import Avatar from "@/components/common/avatar";
import MicControl from "@/components/chat/micControl";
import SpeakerControl from "@/components/chat/speakerControl";
import UserCard from "@/components/modal/userCard";
import { setMasterMuted, setMasterVolume } from "@/handler_audio";
import { guestCaps, isGuest } from "@/stores/guestStore";
import { useSocketStore, socketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import {
	useVoiceChatStore,
	useVoiceChatActions,
} from "@/stores/voiceChatStore";

interface UserBarPropsType {
	className?: string;
	/** 打开设置弹窗回调 */
	onOpenSettings?: () => void;
}

export default function UserBar({
	className,
	onOpenSettings,
}: UserBarPropsType) {
	const user = useUserStore((s) => s.user);
	const displayName = user?.display_name || user?.name || "?";
	const guestSpeakBlocked = isGuest() && !guestCaps().speak;
	const speechRestricted = useSocketStore((s) => s.speechRestricted);
	const speechRestrictionInfo = useSocketStore((s) => s.speechRestrictionInfo);
	const isSpeechRestricted = speechRestricted || guestSpeakBlocked;
	const speechRestrictionReason = speechRestrictionInfo?.reason;

	const data = useVoiceChatStore();
	const { setOutputVolume, setInputVolume, setIsInputMute, setIsOutMute } =
		useVoiceChatActions();

	const [showCard, setShowCard] = useState(false);

	return (
		<div
			className={clsx(
				"flex items-center justify-between px-1.5 pb-1.5 select-none",
				className,
			)}
		>
			<div className="border-border relative flex w-full items-center justify-between rounded-lg border p-2">
				{/* 用户信息按钮 + 弹出卡片 */}
				<div className="relative">
					<button
						className="flex h-11 min-h-11 cursor-pointer items-center space-x-2"
						type="button"
						onClick={() => setShowCard(!showCard)}
					>
						<Avatar
							src={user?.avatar}
							name={displayName}
							alt={user?.name}
							className="size-10"
						/>
						<div className="flex flex-col items-start">
							<div className="text-sm font-bold">{displayName}</div>
							<div className="text-faint-foreground text-xs">
								{user?.role ?? ""}
							</div>
						</div>
					</button>

					{/* 弹出卡片 */}
					{showCard && (
						<>
							{/* 遮罩层，点击关闭 */}
							<div
								className="fixed inset-0 z-40"
								role="button"
								tabIndex={0}
								onKeyDown={(e) => {
									if (e.key === "Enter" || e.key === " ") setShowCard(false);
								}}
								onClick={() => setShowCard(false)}
							/>
							<div className="absolute bottom-full left-0 z-50 mb-2">
								<UserCard
									onClose={() => setShowCard(false)}
									onOpenSettings={() => {
										setShowCard(false);
										onOpenSettings?.();
									}}
								/>
							</div>
						</>
					)}
				</div>

				<div className="flex items-center gap-1">
					<SpeakerControl
						volume={data.outputVolume}
						onChange={(v) => {
							setOutputVolume(v);
							setMasterVolume(v / 100);
						}}
						isMute={data.isOutMute}
						onCheck={(checked) => {
							setIsOutMute(checked);
							setMasterMuted(checked);
						}}
					/>
					<MicControl
						volume={data.inputVolume}
						onChange={setInputVolume}
						isMute={data.isInputMute}
						disabled={isSpeechRestricted}
						disabledTip={
							guestSpeakBlocked
								? "该域未开放访客发言"
								: speechRestrictionReason
									? `已被禁言：${speechRestrictionReason}`
									: "已被禁言，仅收听模式"
						}
						onCheck={(checked) => {
							setIsInputMute(checked);
							const roomName = socketStore.currentRoom();
							const myName = user?.name;
							if (roomName && myName) {
								socketStore.emitMicState(roomName, myName, checked);
							}
						}}
					/>
				</div>
			</div>
			{isSpeechRestricted && (
				<div className="text-warning mt-1 px-3 text-2xs">
					当前为仅收听模式，已禁止发布麦克风
					{speechRestrictionReason ? `：${speechRestrictionReason}` : ""}
				</div>
			)}
		</div>
	);
}
