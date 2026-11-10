import { MicOff } from "lucide-react";
import type { MemberInfo } from "@/socket/types";
import Avatar from "../../common/avatar";

interface MemberItemButtonPropsType {
	member: MemberInfo;
	onSelectMember: (identity: string, x: number, y: number) => void;
}

export default function MemberItemButton({
	member,
	onSelectMember,
}: MemberItemButtonPropsType) {
	const name = member.displayName || member.name || member.identity;
	return (
		<button
			type="button"
			className="text-muted-foreground hover:bg-muted/50 flex w-full cursor-pointer items-center gap-1.5 rounded-sm px-4 py-1 text-sm transition-colors"
			onClick={(e) => onSelectMember(member.identity, e.clientX, e.clientY)}
		>
			<span className="relative shrink-0">
				<Avatar
					className="size-6"
					textClass="text-xs"
					src={member.avatar}
					name={name}
				/>
				<span
					className="border-card bg-success absolute -right-0.5 -bottom-0.5 size-2 rounded-full border"
					aria-hidden="true"
				/>
			</span>
			<span className="flex-1 truncate text-left">{name}</span>
			{(member.isMicMuted || member.isMuted) && (
				<MicOff size={12} className="text-faint-foreground shrink-0" />
			)}
		</button>
	);
}
