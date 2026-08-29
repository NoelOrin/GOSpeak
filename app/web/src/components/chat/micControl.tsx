import Mic from "lucide-solid/icons/mic";
import MicOff from "lucide-solid/icons/mic-off";
import AudioControl from "./audioControl";

interface MicControlProps {
	name?: string;
	volume: () => number;
	isMute: () => boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
	/** 禁用时开关不可点击（如被服务端禁言），鼠标悬停提示 reason。 */
	disabled?: () => boolean;
	disabledTip?: string;
}

const MicControl = ({ name, ...props }: MicControlProps) => {
	return (
		<AudioControl
			name={name || "input"}
			MutedIcon={MicOff}
			UnmutedIcon={Mic}
			muteLabel="麦克风静音"
			unmuteLabel="取消麦克风静音"
			volumeLabel="麦克风音量"
			{...props}
		/>
	);
};
export default MicControl;
