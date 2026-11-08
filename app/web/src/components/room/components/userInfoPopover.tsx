import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { fetchUserInfo } from "@/api/user";
import Avatar from "@/components/common/avatar";

interface UserInfoPopoverProps {
	identity: string;
	pos: { x: number; y: number };
	onClose: () => void;
}

const POPOVER_WIDTH = 224;
const POPOVER_HEIGHT = 210;
const OFFSET = 8;

export default function UserInfoPopover({
	identity,
	pos,
	onClose,
}: UserInfoPopoverProps) {
	const {
		data: userData,
		isLoading,
		isError,
	} = useQuery({
		queryKey: ["user-info", identity],
		queryFn: () => fetchUserInfo(identity),
		retry: false,
	});

	const [cardPos, setCardPos] = useState({ left: pos.x, top: pos.y });

	useEffect(() => {
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		let left = pos.x + OFFSET;
		let top = pos.y + OFFSET;
		if (pos.x + OFFSET + POPOVER_WIDTH > vw) {
			left = pos.x - POPOVER_WIDTH - OFFSET;
		}
		if (pos.y + OFFSET + POPOVER_HEIGHT > vh) {
			top = pos.y - POPOVER_HEIGHT - OFFSET;
		}
		setCardPos({ left, top });
	}, [pos.x, pos.y]);

	useEffect(() => {
		const onScroll = () => onClose();
		window.addEventListener("scroll", onScroll, true);
		return () => window.removeEventListener("scroll", onScroll, true);
	}, [onClose]);

	return (
		<>
			{/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop click to close */}
			<div className="fixed inset-0 z-40" onClick={onClose} />
			<div
				className="border-border bg-card fixed z-50 w-56 overflow-hidden rounded-lg border shadow-xl"
				style={{ left: `${cardPos.left}px`, top: `${cardPos.top}px` }}
			>
				{isLoading ? (
					<div className="flex h-24 items-center justify-center">
						<span className="border-primary border-t-primary animate-spin size-6 rounded-full border-2" />
					</div>
				) : isError || !userData ? (
					<div className="text-faint-foreground p-4 text-sm">
						{isError ? "加载失败" : "用户不存在"}
					</div>
				) : (
					<div className="flex flex-col items-center gap-3 p-4">
						<Avatar
							src={userData.avatar}
							name={userData.display_name || userData.name}
							alt={userData.name}
							className="size-16"
							textClass="text-xl"
						/>
						<div className="text-center">
							<div className="text-sm font-bold">
								{userData.display_name || userData.name}
							</div>
							<div className="text-faint-foreground text-xs">
								@{userData.name}
							</div>
							<div className="bg-muted text-foreground mt-1 inline-block rounded-full px-2 py-0.5 text-2xs">
								{userData.role}
							</div>
						</div>
					</div>
				)}
			</div>
		</>
	);
}
