import { MicOff } from "lucide-react";
import { useState } from "react";
import Avatar from "@/components/common/avatar";
import { useDomainStore } from "@/stores/domainStore";
import { useSocketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import UserInfoPopover from "./userInfoPopover";

// ---------- MemberListItem ----------
interface MemberItemProps {
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMicMuted: boolean;
	isMuted: boolean;
	isMe: boolean;
	onSelect: (identity: string, x: number, y: number) => void;
	isSelected: boolean;
}

function MemberListItem(props: MemberItemProps) {
	const displayName = props.isMe
		? useUserStore.getState().user?.display_name ||
			props.displayName ||
			props.name ||
			props.identity
		: props.displayName || props.name || props.identity;

	return (
		<button
			type="button"
			className={`flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors ${
				props.isSelected ? "bg-border/50" : "hover:bg-border/50"
			}`}
			onClick={(e) => props.onSelect(props.identity, e.clientX, e.clientY)}
		>
			<span className="relative shrink-0">
				<Avatar
					className="size-6"
					textClass="text-2xs"
					src={props.avatar}
					name={props.displayName || props.name || props.identity}
				/>
				<span
					className="border-card bg-success absolute -right-0.5 -bottom-0.5 size-2 rounded-full border"
					aria-hidden="true"
				/>
			</span>
			<span className="flex-1 truncate text-xs font-medium">{displayName}</span>
			{(props.isMicMuted || props.isMuted) && (
				<MicOff size={12} className="text-faint-foreground shrink-0" />
			)}
			{props.isMe && <span className="text-faint-foreground text-2xs">你</span>}
		</button>
	);
}

// ---------- MemberSidebar ----------
export default function MemberSidebar({
	fullWidth = false,
}: {
	fullWidth?: boolean;
}) {
	const [selectedIdentity, setSelectedIdentity] = useState<{
		identity: string;
		x: number;
		y: number;
	} | null>(null);

	const closePopover = () => setSelectedIdentity(null);

	const handleSelect = (identity: string, x: number, y: number) => {
		setSelectedIdentity((prev) =>
			prev?.identity === identity ? null : { identity, x, y },
		);
	};

	const members = useSocketStore((s) => s.rooms);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const myName = useUserStore((s) => s.user?.name);

	const sortedMembers = () => {
		const domainUUID = useDomainStore.getState().currentDomainUUID ?? null;
		const list =
			members.find(
				(r) =>
					r.name === currentRoom &&
					(r.domain_uuid || "") === (domainUUID || ""),
			)?.members ?? [];
		return [...list].sort((a, b) => {
			if (a.identity === myName) return -1;
			if (b.identity === myName) return 1;
			return 0;
		});
	};

	return (
		<div
			className={`relative flex h-full flex-col overflow-hidden shrink-0 ${
				fullWidth ? "w-full border-0" : "border-border w-52 border-l"
			}`}
		>
			<div className="border-border text-muted-foreground flex h-10 items-center border-b px-3 text-xs font-bold">
				成员 ({sortedMembers().length})
			</div>

			<div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
				{sortedMembers().length > 0 ? (
					sortedMembers().map((member) => (
						<MemberListItem
							key={member.identity}
							identity={member.identity}
							name={member.name}
							displayName={member.displayName ?? ""}
							avatar={member.avatar ?? ""}
							isMicMuted={!!member.isMicMuted}
							isMuted={!!member.isMuted}
							isMe={member.identity === myName}
							onSelect={handleSelect}
							isSelected={selectedIdentity?.identity === member.identity}
						/>
					))
				) : (
					<div className="text-faint-foreground flex h-20 items-center justify-center text-xs">
						暂无成员
					</div>
				)}
			</div>
			{selectedIdentity && (
				<UserInfoPopover
					identity={selectedIdentity.identity}
					pos={{ x: selectedIdentity.x, y: selectedIdentity.y }}
					onClose={closePopover}
				/>
			)}
		</div>
	);
}
