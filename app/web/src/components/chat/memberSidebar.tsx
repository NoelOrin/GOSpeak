import { useSocketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import { useState } from "react";
import Avatar from "@/components/common/avatar";
import { LoadingRegion, SkeletonListRows } from "@/components/common/Skeletons";

interface MemberSidebarProps {
	onStartChat: (identity: string, display_name: string) => void;
}

export default function MemberSidebar({ onStartChat }: MemberSidebarProps) {
	const [, setSelectedIdentity] = useState<string | null>(null);

	const members = useSocketStore((s) => s.rooms);
	const connected = useSocketStore((s) => s.connected);
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const myName = useUserStore((s) => s.user?.name);

	const roomMembers =
		members.find((r) => r.name === currentRoom)?.members ?? [];

	const sortedMembers = [...roomMembers].sort((a, b) => {
		if (a.identity === myName) return -1;
		if (b.identity === myName) return 1;
		return 0;
	});

	const handleClick = (identity: string, display_name: string) => {
		setSelectedIdentity(identity);
		onStartChat(identity, display_name);
	};

	return (
		<div className="border-border ml-auto flex h-full w-52 shrink-0 flex-col overflow-hidden border-l">
			<div className="border-border text-muted-foreground flex h-10 items-center border-b px-3 text-xs font-bold">
				服务器成员 ({roomMembers.length})
			</div>
			<div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
				{!connected && sortedMembers.length === 0 ? (
					<LoadingRegion label="正在加载成员">
						<SkeletonListRows rows={6} />
					</LoadingRegion>
				) : sortedMembers.length > 0 ? (
					sortedMembers.map((member) => (
						<button
							key={member.identity}
							type="button"
							className="hover:bg-border/50 flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors"
							onClick={() =>
								handleClick(
									member.identity,
									member.display_name || member.name || member.identity,
								)
							}
						>
							<Avatar
								className="size-6 shrink-0"
								textClass="text-2xs"
								src={member.avatar}
								name={member.display_name || member.name || member.identity}
							/>
							<span className="flex-1 truncate text-xs font-medium">
								{member.display_name || member.name || member.identity}
							</span>
							{member.identity === myName && (
								<span className="text-faint-foreground text-2xs">你</span>
							)}
						</button>
					))
				) : (
					<div className="text-faint-foreground flex h-20 items-center justify-center text-xs">
						{connected ? "暂无成员" : "正在连接服务器，成员列表加载中…"}
					</div>
				)}
			</div>
		</div>
	);
}
