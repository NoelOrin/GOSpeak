import { HeadphoneOff, Headphones } from "lucide-react";
import AudioControl from "./audioControl";

interface SpeakerControlProps {
	name?: string;
	className?: string;
	volume: number;
	isMute: boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
}

export default function SpeakerControl({
	name,
	...props
}: SpeakerControlProps) {
	return (
		<AudioControl
			name={name || "output"}
			MutedIcon={HeadphoneOff}
			UnmutedIcon={Headphones}
			muteLabel="扬声器静音"
			unmuteLabel="取消扬声器静音"
			volumeLabel="扬声器音量"
			{...props}
		/>
	);
}
