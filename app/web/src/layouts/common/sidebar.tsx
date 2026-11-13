import { ScrollArea } from "@gospeak/components";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Bell, Home, MessageSquare, Settings, ShieldCheck } from "lucide-react";
import { useEffect } from "react";
import OptionSquare from "@/components/common/optionSquare";
import { LoadingRegion, SkeletonListRows } from "@/components/common/Skeletons";
import DomainIcon from "@/components/domain/DomainIcon";
import { firstAccessibleManagePath } from "@/components/manage/manageNav";
import { getMyDomainsDetailed } from "@/api/domain";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useNotificationStore } from "@/stores/notificationStore";
import { hasManageAccess } from "@/utils/permissions";

interface SidebarProps {
	onOpenSettings?: () => void;
	onOpenNotifications?: () => void;
}

/** 侧栏图标统一 20px / 默认描边（与全站图标规范一致） */
const iconProps = {
	size: 20,
} as const;

export default function Sidebar({
	onOpenSettings,
	onOpenNotifications,
}: SidebarProps) {
	const navigate = useNavigate();
	const unreadCount = useNotificationStore((s) => s.unreadCount);
	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const currentDomainUUID = useDomainStore((s) => s.currentDomainUUID);

	useEffect(() => {
		void domainStore.loadMyDomains().catch(() => {});
	}, []);

	const { data: domains, isLoading } = useQuery({
		queryKey: ["my-domains-detailed", myDomainUUIDs.join(",")],
		queryFn: async () => {
			const rows = await getMyDomainsDetailed();
			return rows.filter((row) =>
				useDomainStore.getState().myDomainUUIDs.includes(row.uuid),
			);
		},
	});

	const handleSelect = async (uuid: string) => {
		const previousUUID = useDomainStore.getState().currentDomainUUID;
		domainStore.setCurrentDomain(uuid);
		try {
			await navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: uuid },
			});
		} catch {
			domainStore.setCurrentDomain(previousUUID);
		}
	};

	return (
		<div className="flex h-full min-h-0 w-16 flex-col select-none">
			{/* 快捷入口与域列表共用一个内部滚动区：窗口再矮也只是内部滚动，
			    不会把图标挤出侧栏、压到左下用户栏上 */}
			<ScrollArea
				className="min-h-0 flex-1"
				viewportClassName="px-2 pb-3"
				type="auto"
			>
				<div className="flex flex-col items-center gap-2">
					{/* 直达最终路由：/ 与 /manage 的 beforeLoad 是重定向，跳转中转站会让整个布局卸载一帧（白闪） */}
					<OptionSquare
						label="首页"
						onClick={() => navigate({ to: "/discover" })}
					>
						<Home {...iconProps} />
					</OptionSquare>
					<OptionSquare label="聊天" onClick={() => navigate({ to: "/chat" })}>
						<MessageSquare {...iconProps} />
					</OptionSquare>
					<OptionSquare
						label="通知"
						badge={unreadCount}
						onClick={() => onOpenNotifications?.()}
					>
						<Bell {...iconProps} />
					</OptionSquare>
					<OptionSquare label="设置" onClick={() => onOpenSettings?.()}>
						<Settings {...iconProps} />
					</OptionSquare>
					{hasManageAccess() && (
						<OptionSquare
							label="管理"
							onClick={() =>
								navigate({
									to: `/manage/${firstAccessibleManagePath() ?? "users"}`,
								})
							}
						>
							<ShieldCheck {...iconProps} />
						</OptionSquare>
					)}
					<div className="bg-border my-1 h-px w-full shrink-0" />
					{isLoading ? (
						<LoadingRegion label="正在加载域">
							<SkeletonListRows rows={4} className="w-full" />
						</LoadingRegion>
					) : (
						(domains ?? []).map((domain) => (
							<DomainIcon
								key={domain.uuid}
								name={domain.name}
								iconUrl={domain.icon_url}
								active={currentDomainUUID === domain.uuid}
								onClick={() => void handleSelect(domain.uuid)}
								requiresDoubleClick
							/>
						))
					)}
				</div>
			</ScrollArea>
		</div>
	);
}
