import { useNavigate } from "@tanstack/solid-router";
import Headphones from "lucide-solid/icons/headphones";
import MessageSquare from "lucide-solid/icons/message-square";
import Home from "lucide-solid/icons/home";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import Plus from "lucide-solid/icons/plus";
import UserPlus from "lucide-solid/icons/user-plus";
import { createResource, createSignal, For, Show } from "solid-js";
import { type Guild, getGuild } from "@/api/guild";
import CreateGuildModal from "@/components/guild/CreateGuildModal";
import GuildIcon from "@/components/guild/GuildIcon";
import JoinGuildModal from "@/components/guild/JoinGuildModal";
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
	const [createRef, setCreateRef] = createSignal<HTMLDialogElement>();
	const [joinRef, setJoinRef] = createSignal<HTMLDialogElement>();

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
		<div class="flex flex-col h-full w-16 select-none overflow-y-auto">
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
				<OptionSquare
					label="创建服务器"
					onClick={() => createRef()?.showModal()}
				>
					<Plus {...iconProps} />
				</OptionSquare>
				<OptionSquare label="加入服务器" onClick={() => joinRef()?.showModal()}>
					<UserPlus {...iconProps} />
				</OptionSquare>
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
			<CreateGuildModal
				ref={setCreateRef}
				onClose={() => createRef()?.close()}
			/>
			<JoinGuildModal ref={setJoinRef} onClose={() => joinRef()?.close()} />
		</div>
	);
};
export default Sidebar;
