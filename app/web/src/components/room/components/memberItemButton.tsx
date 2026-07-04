import MicOff from "lucide-solid/icons/mic-off";
import { Show } from "solid-js";
import type { MemberInfo } from "@/stores/socketStore";
import Avatar from "../../common/avatar";

interface MemberItemButtonPropsType {
	member: MemberInfo;
	onSelectMember: (identity: string, x: number, y: number) => void;
}

const MemberItemButton = (props: MemberItemButtonPropsType) => {
	return (
		<button
			type="button"
			class="flex items-center gap-1.5 w-full px-4 py-1 text-sm text-base-content/60 cursor-pointer hover:bg-base-200/50 rounded transition-colors"
			onClick={(e) =>
				props.onSelectMember(props.member.identity, e.clientX, e.clientY)
			}
		>
			<Avatar
				class="size-6 shrink-0"
				textClass="text-[12px]"	
				src={props.member.avatar}
				name={
					props.member.displayName || props.member.name || props.member.identity
				}
			/>
			<span class="truncate flex-1 text-left">
				{props.member.displayName || props.member.name || props.member.identity}
			</span>
			<Show when={props.member.isMicMuted || props.member.isMuted}>
				<MicOff size={12} class="text-base-content/40 shrink-0" />
			</Show>
		</button>
	);
};

export default MemberItemButton;
