import { type Component, createResource, For } from "solid-js";
import { type Guild, getGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";
import GuildIcon from "./GuildIcon";

const GuildList: Component = () => {
	const { state, loadMyGuilds, setCurrentGuild } = guildStore;

	// Load my guilds on mount
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

	return (
		<div class="w-16 bg-base-300 flex flex-col items-center py-3 gap-2 overflow-y-auto">
			<For each={guilds() || []}>
				{(guild) => (
					<GuildIcon
						name={guild.name}
						iconUrl={guild.icon_url}
						active={state.currentGuildUUID === guild.uuid}
						onClick={() => setCurrentGuild(guild.uuid)}
					/>
				)}
			</For>
		</div>
	);
};

export default GuildList;
