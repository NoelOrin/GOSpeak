import AudioDeviceStore, {
	type useAudioDeviceStore,
} from "@/stores/audioDeviceStore";
import { Section, Toggle } from "../shared";

export const NetworkSection = ({
	store,
}: {
	store: ReturnType<typeof useAudioDeviceStore.getState>;
}) => (
	<Section title="网络传输优化">
		<Toggle
			label="立体声"
			desc="以双声道发布音频"
			checked={store.stereo}
			onChange={AudioDeviceStore.setStereo}
		/>
		<Toggle
			label="DTX 不连续传输"
			desc="静音时停止发包，节省带宽"
			checked={store.dtx}
			onChange={AudioDeviceStore.setDtx}
		/>
		<Toggle
			label="RED 冗余音频"
			desc="发送冗余数据抗丢包，轻微增加带宽"
			checked={store.red}
			onChange={AudioDeviceStore.setRed}
		/>
	</Section>
);
