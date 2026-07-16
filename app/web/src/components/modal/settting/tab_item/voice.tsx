import AudioDeviceStore from "@/stores/audioDeviceStore";
import VoiceChatStore from "@/stores/voiceChatStore";
import { Field, Page, Section, Toggle } from "./shared";
import type { SettingTabConfig } from "./types";

const VoiceForm = () => {
	const {
		data,
		setInputVolume,
		setOutputVolume,
		setIsInputMute,
		setIsOutMute,
	} = VoiceChatStore;

	return (
		<Page title="语音行为" desc="进房默认状态、本机输入/输出音量与提示音。">
			<Section title="进房行为">
				<Toggle
					label="加入频道时静音麦克风"
					desc="进房后默认不推流，可手动开麦"
					checked={AudioDeviceStore.state.muteOnJoin}
					onChange={AudioDeviceStore.setMuteOnJoin}
				/>
				<Toggle
					label="进出房提示音"
					desc="自己与他人加入/离开时播放短提示音"
					checked={AudioDeviceStore.state.notificationSounds}
					onChange={AudioDeviceStore.setNotificationSounds}
				/>
			</Section>

			<Section title="本机音量">
				<Field label={`麦克风输入音量 · ${data.inputVolume}%`}>
					<input
						type="range"
						min="0"
						max="100"
						class="range range-sm w-full"
						value={data.inputVolume}
						onInput={(e) => setInputVolume(Number(e.currentTarget.value))}
					/>
				</Field>
				<Field label={`扬声器输出音量 · ${data.outputVolume}%`}>
					<input
						type="range"
						min="0"
						max="100"
						class="range range-sm w-full"
						value={data.outputVolume}
						onInput={(e) => setOutputVolume(Number(e.currentTarget.value))}
					/>
				</Field>
				<div class="grid gap-2 sm:grid-cols-2">
					<button
						type="button"
						class="btn btn-outline btn-sm"
						onClick={() => setIsInputMute(!data.isInputMute)}
					>
						{data.isInputMute ? "取消麦克风静音" : "静音麦克风"}
					</button>
					<button
						type="button"
						class="btn btn-outline btn-sm"
						onClick={() => setIsOutMute(!data.isOutMute)}
					>
						{data.isOutMute ? "取消扬声器静音" : "静音扬声器"}
					</button>
				</div>
			</Section>
		</Page>
	);
};

const voice: SettingTabConfig = {
	id: "voice",
	label: "语音",
	icon: "volume",
	component: VoiceForm,
};

export default voice;
