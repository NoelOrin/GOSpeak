import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
	Bell,
	Home,
	Link,
	MessageSquare,
	Settings,
	ShieldCheck,
} from "lucide-react";
import { useEffect } from "react";
import OptionSquare from "@/components/common/optionSquare";
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
		<div className="flex h-full w-16 flex-col select-none">
			<div className="flex flex-col items-center gap-2 pb-3">
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
					label="分享链接"
					onClick={() =>
						navigate({ to: "/link", search: { domain: undefined } })
					}
				>
					<Link {...iconProps} />
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
			</div>
			<div className="min-h-0 flex-1 overflow-y-auto pb-3">
				<div className="flex flex-col items-center gap-2">
					{isLoading ? (
						<span className="border-primary border-t-primary my-2 size-5 animate-spin rounded-full border-2" />
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
			</div>
		</div>
	);
}
