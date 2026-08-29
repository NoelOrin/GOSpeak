import { useNavigate } from "@tanstack/solid-router";
// import Search from "lucide-solid/icons/search";
import MessageSquare from "lucide-solid/icons/message-square";
import Home from "lucide-solid/icons/home";
import Link from "lucide-solid/icons/link";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { createResource, For, Show, Suspense } from "solid-js";
import { type DomainDetail, getMyDomainsDetailed } from "@/api/domain";
import DomainIcon from "@/components/domain/DomainIcon";
import { firstAccessibleManagePath } from "@/components/manage/manageNav";
import OptionSquare from "@/components/common/optionSquare";
import domainStore from "@/stores/domainStore";
import { hasManageAccess } from "@/utils/permissions";

interface SidebarProps {
	onOpenSettings?: () => void;
}

const iconProps = {
	size: 22,
	strokeWidth: 2.1,
} as const;

const Sidebar = (props: SidebarProps) => {
	const navigate = useNavigate();
	const { state, loadMyDomains, setCurrentDomain } = domainStore;

	void loadMyDomains().catch(() => {});

	const [domains] = createResource<DomainDetail[], string>(
		// 用拼接字符串做 source：数组内容不变时引用即便被替换也不会重新挂起
		() => state.myDomainUUIDs.join(","),
		async () => {
			const rows = await getMyDomainsDetailed();
			return rows.filter((row) => state.myDomainUUIDs.includes(row.uuid));
		},
	);

	const handleSelect = async (uuid: string) => {
		const previousUUID = state.currentDomainUUID;
		setCurrentDomain(uuid);
		try {
			await navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: uuid },
			});
		} catch {
			setCurrentDomain(previousUUID);
		}
	};

	return (
		<div class="flex flex-col h-full w-16 select-none">
			<div class="flex flex-col items-center gap-2 pb-3">
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
				<OptionSquare label="设置" onClick={() => props.onOpenSettings?.()}>
					<Settings {...iconProps} />
				</OptionSquare>
				<Show when={hasManageAccess()}>
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
				</Show>
				<div class="divider my-1 shrink-0" />
			</div>
			<div class="flex-1 min-h-0 overflow-y-auto pb-3">
				<div class="flex flex-col items-center gap-2">
					{/* 发现服务器
					<OptionSquare
						label="发现服务器"
						onClick={() => navigate({ to: "/discover" })}
					>
						<Search {...iconProps} />
					</OptionSquare>
					*/}
					{/* 局部 Suspense：resource 重新拉取时只降级列表，避免无边界挂起波及整树 */}
					<Suspense
						fallback={<span class="loading loading-spinner loading-sm my-2" />}
					>
						<For each={domains() || []}>
							{(domain) => (
								<DomainIcon
									name={domain.name}
									iconUrl={domain.icon_url}
									active={state.currentDomainUUID === domain.uuid}
									onClick={() => void handleSelect(domain.uuid)}
									requiresDoubleClick
								/>
							)}
						</For>
					</Suspense>
				</div>
			</div>
		</div>
	);
};
export default Sidebar;
