import clsx from "clsx";
import { createSignal, Show } from "solid-js";
import Avatar from "@/components/common/avatar";
import { setMasterMuted, setMasterVolume } from "@/handler_audio";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import VoiceChatStore from "@/stores/voiceChatStore";
import MicControl from "./chat/micControl";
import SpeakerControl from "./chat/speakerControl";
import UserCard from "./modal/userCard";

interface UserBarPropsType {
	class?: string;
	/** 打开设置弹窗回调 */
	onOpenSettings?: () => void;
}

const UserBar = ({ ...props }: UserBarPropsType) => {
	const {
		data,
		setOutputVolume,
		setInputVolume,
		setIsInputMute,
		setIsOutMute,
	} = VoiceChatStore;

	const user = () => userStore.user();
	const displayName = () => user()?.display_name || user()?.name || "?";
	const isSpeechRestricted = () => socketStore.speechRestricted();
	const speechRestrictionReason = () =>
		socketStore.speechRestrictionInfo()?.reason;

	const [showCard, setShowCard] = createSignal(false);

	return (
		<div
			class={clsx(
				"flex justify-between items-center px-1.5 pb-1.5 dark:text-white select-none",
				props.class,
			)}
		>
			<div class="relative flex justify-between items-center p-2 border border-color rounded-xl w-full">
				{/* 用户信息按钮 + 弹出卡片 */}
				<div class="relative">
					<button
						class="flex items-center space-x-2 cursor-pointer"
						type="button"
						onClick={() => setShowCard(!showCard())}
					>
						<Avatar
							src={user()?.avatar}
							name={displayName()}
							alt={user()?.name}
							class="size-10"
						/>
						<div class="flex flex-col items-start">
							<div class="font-bold text-[14px]">{displayName()}</div>
							<div class="text-xs text-base-content/50">
								{user()?.role ?? ""}
							</div>
						</div>
					</button>

					{/* 弹出卡片 */}
					<Show when={showCard()}>
						{/* 遮罩层，点击关闭 */}
						<div
							class="fixed inset-0 z-40" role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") setShowCard(false) }}
							onClick={() => setShowCard(false)}
						/>
						<div class="absolute bottom-full left-0 mb-2 z-50">
							<UserCard
								onClose={() => setShowCard(false)}
								onOpenSettings={() => {
									setShowCard(false);
									props.onOpenSettings?.();
								}}
							/>
						</div>
					</Show>
				</div>

				<div class="flex space-x-3">
					<SpeakerControl
						volume={() => data.outputVolume}
						onChange={(v) => {
							setOutputVolume(v);
							setMasterVolume(v / 100);
						}}
						isMute={() => data.isOutMute}
						onCheck={(checked) => {
							setIsOutMute(checked);
							setMasterMuted(checked);
						}}
					/>
					<MicControl
						volume={() => data.inputVolume}
						onChange={setInputVolume}
						isMute={() => data.isInputMute}
						disabled={isSpeechRestricted}
						disabledTip={
							speechRestrictionReason()
								? `已被禁言：${speechRestrictionReason()}`
								: "已被禁言，仅收听模式"
						}
						onCheck={(checked) => {
							setIsInputMute(checked);
							const roomName = socketStore.currentRoom();
							const myName = userStore.user()?.name;
							if (roomName && myName) {
								socketStore.emitMicState(roomName, myName, checked);
							}
						}}
					/>
				</div>
			</div>
			<Show when={isSpeechRestricted()}>
				<div class="mt-1 px-3 text-[11px] text-warning">
					当前为仅收听模式，已禁止发布麦克风
					{speechRestrictionReason() ? `：${speechRestrictionReason()}` : ""}
				</div>
			</Show>
		</div>
	);
};

export default UserBar;
