import { useNavigate } from "@tanstack/solid-router";
// import Search from "lucide-solid/icons/search";
import MessageSquare from "lucide-solid/icons/message-square";
import Home from "lucide-solid/icons/home";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { createResource, For, Show } from "solid-js";
import { type Domain, getDomain } from "@/api/domain";
import DomainIcon from "@/components/domain/DomainIcon";
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

	const [domains] = createResource<Domain[], string[]>(
		() => state.myDomainUUIDs,
		async (uuids: string[]) => {
			const results = await Promise.allSettled(uuids.map((u) => getDomain(u)));
			return results
				.filter(
					(r): r is PromiseFulfilledResult<Domain> => r.status === "fulfilled",
				)
				.map((r) => r.value);
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
				<OptionSquare label="首页" onClick={() => navigate({ to: "/" })}>
					<Home {...iconProps} />
				</OptionSquare>
				<OptionSquare label="聊天" onClick={() => navigate({ to: "/chat" })}>
					<MessageSquare {...iconProps} />
				</OptionSquare>
				<OptionSquare label="设置" onClick={() => props.onOpenSettings?.()}>
					<Settings {...iconProps} />
				</OptionSquare>
				<Show when={hasManageAccess()}>
					<OptionSquare
						label="管理"
						onClick={() => navigate({ to: "/manage" })}
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
				</div>
			</div>
		</div>
	);
};
export default Sidebar;
