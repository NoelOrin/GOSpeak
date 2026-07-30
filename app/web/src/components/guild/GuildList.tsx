import { useNavigate } from "@tanstack/solid-router";
import Plus from "lucide-solid/icons/plus";
import UserPlus from "lucide-solid/icons/user-plus";
import { type Component, createResource, createSignal, For } from "solid-js";
import { type Guild, getGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";
import CreateGuildModal from "./CreateGuildModal";
import GuildIcon from "./GuildIcon";
import JoinGuildModal from "./JoinGuildModal";

const GuildList: Component = () => {
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
		<>
			<div class="w-16 bg-base-300 flex flex-col items-center py-3 gap-2 overflow-y-auto">
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
				<div class="divider my-1" />
				<button
					type="button"
					class="w-12 h-12 rounded-2xl flex items-center justify-center bg-base-200 hover:bg-base-100 transition-colors text-base-content/60"
					title="创建服务器"
					onClick={() => createRef()?.showModal()}
				>
					<Plus size={22} />
				</button>
				<button
					type="button"
					class="w-12 h-12 rounded-2xl flex items-center justify-center bg-base-200 hover:bg-base-100 transition-colors text-base-content/60"
					title="加入服务器"
					onClick={() => joinRef()?.showModal()}
				>
					<UserPlus size={22} />
				</button>
			</div>
			<CreateGuildModal
				ref={setCreateRef}
				onClose={() => createRef()?.close()}
			/>
			<JoinGuildModal ref={setJoinRef} onClose={() => joinRef()?.close()} />
		</>
	);
};

export default GuildList;
