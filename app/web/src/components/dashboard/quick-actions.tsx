import { toast } from "@gospeak/components";
import { useNavigate } from "@tanstack/react-router";
import { ArrowRight, CirclePlus, Radio } from "lucide-react";
import { useState } from "react";
import CreateRoomModal from "@/components/modal/createRoomModal";
import { socketStore } from "@/stores/socketStore";

interface QuickActionsProps {
	compact?: boolean;
}

export default function QuickActions({ compact }: QuickActionsProps) {
	const navigate = useNavigate();
	const [createRoomOpen, setCreateRoomOpen] = useState(false);
	const [createRoomDomain, setCreateRoomDomain] = useState("");

	const handleCreateRoom = () => {
		const domainUUID = socketStore.currentDomainUUID();
		if (!domainUUID) {
			toast.warning("请先选择域");
			return;
		}
		setCreateRoomDomain(domainUUID);
		setCreateRoomOpen(true);
	};

	const handleJoinRandom = () => {
		const domainUUID = socketStore.currentDomainUUID();
		if (!domainUUID) {
			toast.warning("请先选择域");
			return;
		}
		const rooms = socketStore
			.rooms()
			.filter(
				(room) =>
					room.count < room.limit && (room.domain_uuid || "") === domainUUID,
			);
		if (rooms.length === 0) {
			toast.warning("当前域没有可加入的房间");
			return;
		}
		const target = [...rooms].sort((a, b) => b.count - a.count)[0];
		socketStore.selectRoom(target);
		const targetDomainUUID = target.domain_uuid;
		if (targetDomainUUID)
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: targetDomainUUID },
			});
		else navigate({ to: "/discover" });
	};

	const cardClass = `bg-muted hover:bg-border flex min-w-0 h-18 cursor-pointer items-center justify-between gap-3 overflow-hidden rounded-lg border-0 px-4 text-left transition-colors`;

	return (
		<>
			<section className="min-w-0 rounded-lg bg-transparent px-2 py-2">
				<div className="mb-5 min-w-0">
					<h2 className="text-lg font-semibold">快捷入口</h2>
					<p className="text-muted-foreground text-sm">
						{compact ? "常用操作" : "把常用操作放在首页，减少跳转成本"}
					</p>
				</div>
				<div
					className={
						compact
							? "grid min-w-0 gap-2"
							: "grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4"
					}
				>
					<button
						type="button"
						className={cardClass}
						onClick={handleCreateRoom}
					>
						<div className="min-w-0 flex-1">
							<div className="truncate font-medium">快速创建房间</div>
							<div className="text-muted-foreground mt-1 truncate text-xs">
								填写配置后创建新房间
							</div>
						</div>
						<CirclePlus size={20} className="shrink-0" />
					</button>
					<button
						type="button"
						className={cardClass}
						onClick={handleJoinRandom}
					>
						<div className="min-w-0 flex-1">
							<div className="truncate font-medium">随机加入房间</div>
							<div className="text-muted-foreground mt-1 truncate text-xs">
								优先选择当前最活跃房间
							</div>
						</div>
						<Radio size={20} className="shrink-0" />
					</button>
					<button
						type="button"
						className={cardClass}
						onClick={() => {
							const uuid = socketStore.currentDomainUUID();
							if (uuid)
								navigate({
									to: "/domain/$domainUUID",
									params: { domainUUID: uuid },
								});
							else navigate({ to: "/discover" });
						}}
					>
						<div className="min-w-0 flex-1">
							<div className="truncate font-medium">前往域</div>
							<div className="text-muted-foreground mt-1 truncate text-xs">
								查看全部房间与在线成员
							</div>
						</div>
						<ArrowRight size={20} className="shrink-0" />
					</button>
				</div>
			</section>
			<CreateRoomModal
				open={createRoomOpen}
				domainUUID={createRoomDomain}
				onClose={() => setCreateRoomOpen(false)}
			/>
		</>
	);
}
