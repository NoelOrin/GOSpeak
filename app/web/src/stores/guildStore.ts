import { createRoot } from "solid-js";
import { createStore } from "solid-js/store";
import {
	type Guild,
	type GuildMember,
	getGuild,
	guildMembers,
	myGuilds,
} from "@/api/guild";

interface GuildState {
	myGuildUUIDs: string[];
	currentGuildUUID: string | null;
	guildCache: Record<string, Guild>;
	memberCache: Record<string, GuildMember[]>;
	loading: boolean;
}

function createGuildStore() {
	const [state, setState] = createStore<GuildState>({
		myGuildUUIDs: [],
		currentGuildUUID: null,
		guildCache: {},
		memberCache: {},
		loading: false,
	});

	const loadMyGuilds = async () => {
		setState("loading", true);
		try {
			const uuids = await myGuilds();
			setState("myGuildUUIDs", uuids);
		} finally {
			setState("loading", false);
		}
	};

	const ensureGuildLoaded = async (uuid: string) => {
		if (state.guildCache[uuid]) return;
		const guild = await getGuild(uuid);
		setState("guildCache", uuid, guild);
	};

	const setCurrentGuild = (uuid: string | null) => {
		setState("currentGuildUUID", uuid);
		if (uuid) ensureGuildLoaded(uuid);
	};

	const loadMembers = async (guildUUID: string) => {
		const members = await guildMembers(guildUUID);
		setState("memberCache", guildUUID, members);
	};

	const addGuild = (guild: Guild) => {
		setState("guildCache", guild.uuid, guild);
		setState("myGuildUUIDs", (prev) =>
			prev.includes(guild.uuid) ? prev : [...prev, guild.uuid],
		);
	};

	const removeGuild = (uuid: string) => {
		setState("myGuildUUIDs", (prev) => prev.filter((u) => u !== uuid));
		if (state.currentGuildUUID === uuid) {
			setState("currentGuildUUID", null);
		}
	};

	return {
		state,
		setState,
		loadMyGuilds,
		ensureGuildLoaded,
		setCurrentGuild,
		loadMembers,
		addGuild,
		removeGuild,
	};
}

export default createRoot(createGuildStore);
