import { useNavigate } from "@tanstack/solid-router";
import Compass from "lucide-solid/icons/compass";
import Headphones from "lucide-solid/icons/headphones";
import MessageSquare from "lucide-solid/icons/message-square";
import Home from "lucide-solid/icons/home";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { createResource, For, Show } from "solid-js";
import { type Guild, getGuild } from "@/api/guild";
import GuildIcon from "@/components/guild/GuildIcon";
import OptionSquare from "@/components/common/optionSquare";
import guildStore from "@/stores/guildStore";
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
	const { state, loadMyGuilds, setCurrentGuild } = guildStore;

	loadMyGuilds();

	const [guilds] = createResource<Guild[], string[]>(
		() => state.myGuildUUIDs,
		async (uuids: string[]) => {
			const results = await Promise.allSettled(uuids.map((u) => getGuild(u)));
			return results
				.filter(
					(r): r is PromiseFulfilledResult<Guild> => r.status === "fulfilled",
				)
				.map((r) => r.value);
		},
	);

	const handleSelect = (uuid: string) => {
		setCurrentGuild(uuid);
		navigate({ to: "/guild/$guildUUID", params: { guildUUID: uuid } });
	};

	return (
		<div class="flex flex-col h-full w-16 select-none">
			<div class="flex flex-col items-center gap-2 pb-3">
				<OptionSquare label="首页" onClick={() => navigate({ to: "/" })}>
					<Home {...iconProps} />
				</OptionSquare>
				<OptionSquare
					label="频道"
					onClick={() => navigate({ to: "/channel", search: { id: 12413 } })}
				>
					<Headphones {...iconProps} />
				</OptionSquare>
				<OptionSquare label="聊天" onClick={() => navigate({ to: "/chat" })}>
					<MessageSquare {...iconProps} />
				</OptionSquare>
				<OptionSquare
					label="发现服务器"
					onClick={() => navigate({ to: "/discover" })}
				>
					<Compass {...iconProps} />
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
					<For each={guilds() || []}>
						{(guild) => (
							<GuildIcon
								name={guild.name}
								iconUrl={guild.icon_url}
								active={state.currentGuildUUID === guild.uuid}
								onClick={() => handleSelect(guild.uuid)}
							/>
						)}
					</For>
				</div>
			</div>
		</div>
	);
};
export default Sidebar;
