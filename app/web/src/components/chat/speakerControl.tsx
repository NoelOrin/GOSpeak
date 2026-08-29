import Volume2 from "lucide-solid/icons/volume-2";
import VolumeX from "lucide-solid/icons/volume-x";
import AudioControl from "./audioControl";

interface SpeakerControlProps {
	name?: string;
	volume: () => number;
	isMute: () => boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
}

const SpeakerControl = ({ name, ...props }: SpeakerControlProps) => {
	return (
		<AudioControl
			name={name || "output"}
			MutedIcon={VolumeX}
			UnmutedIcon={Volume2}
			muteLabel="扬声器静音"
			unmuteLabel="取消扬声器静音"
			volumeLabel="扬声器音量"
			{...props}
		/>
	);
};
export default SpeakerControl;
