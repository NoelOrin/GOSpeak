import {
	Button,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import {
	normalizeScreenShareQuality,
	SCREEN_SHARE_QUALITY_ORDER,
	SCREEN_SHARE_QUALITY_PRESETS,
} from "@gospeak/sfu-client";
import { useEffect } from "react";
import AudioDeviceStore, {
	useAudioDeviceStore,
} from "@/stores/audioDeviceStore";
import ScreenShareSettingsStore, {
	useScreenShareSettingsStore,
} from "@/stores/screenShareSettingsStore";
import { Field, Page, Section } from "../shared";
import type { SettingTabConfig } from "../types";
import { DevicesSection } from "./devices";
import { NetworkSection } from "./network";
import { QualitySection } from "./quality";
import { useMicTest } from "./useMicTest";

function AudioForm() {
	const store = useAudioDeviceStore();
	const screenShareQuality = useScreenShareSettingsStore((s) => s.quality);
	const { testing, level, toggle } = useMicTest({
		deviceId: store.selectedAudioInput,
		echoCancellation: store.echoCancellation,
		noiseSuppression: store.noiseSuppression,
		autoGainControl: store.autoGainControl,
	});

	useEffect(() => {
		void AudioDeviceStore.fetchAudioDevices();
	}, []);

	return (
		<Page
			title="音频"
			desc="设备、采集质量与传输参数。设备可在房间内热切换，采集/传输参数需重新进房生效。"
		>
			<DevicesSection
				store={store}
				testing={testing}
				level={level}
				toggle={toggle}
			/>

			<QualitySection store={store} />

			<NetworkSection store={store} />

			<Section
				title="屏幕共享"
				desc="改档即时生效，影响他人看到的清晰度与上传带宽。"
			>
				<Field
					label="共享画质"
					hint="共享过程中改档不会中断共享，也不会重新弹出窗口选择。"
				>
					<Select
						value={screenShareQuality}
						onValueChange={(value) =>
							ScreenShareSettingsStore.setQuality(
								normalizeScreenShareQuality(value),
							)
						}
					>
						<SelectTrigger>
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
				</Field>
			</Section>

			<div className="pt-2">
				<Button
					type="button"
					variant="outline"
					onClick={() => AudioDeviceStore.resetAudioDefaults()}
				>
					恢复音频默认值
				</Button>
			</div>

			{store.inputMode === "music" && (
				<div className="border-info/30 bg-info/10 text-info flex items-center rounded-sm px-3 py-2 text-sm">
					音乐模式已关闭常见语音处理；若出现回声，请改回「语音通话」。
				</div>
			)}
		</Page>
	);
}

const audio: SettingTabConfig = {
	id: "audio",
	label: "音频",
	icon: "mic",
	component: AudioForm,
};

export default audio;
