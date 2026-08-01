import { createRoot } from "solid-js";
import { createStore } from "solid-js/store";
import {
	type Guild,
	type GuildMember,
	getGuild,
	guildMembers,
	deleteGuild,
	leaveGuild,
	myGuilds,
} from "@/api/guild";

interface GuildState {
	myGuildUUIDs: string[];
	currentGuildUUID: string | null;
	guildCache: Record<string, Guild>;
	memberCache: Record<string, GuildMember[]>;
	guildLoading: Record<string, boolean>;
	memberLoading: Record<string, boolean>;
	guildErrors: Record<string, string | null>;
	memberErrors: Record<string, string | null>;
	loading: boolean;
}

function errorMessage(error: unknown): string {
	if (error instanceof Error) return error.message;
	return String(error);
}

export function createGuildStore() {
	const [state, setState] = createStore<GuildState>({
		myGuildUUIDs: [],
		currentGuildUUID: null,
		guildCache: {},
		memberCache: {},
		guildLoading: {},
		memberLoading: {},
		guildErrors: {},
		memberErrors: {},
		loading: false,
	});

	const removedUUIDs = new Set<string>();
	const loadGenerations = new Map<string, number>();
	let myGuildsVersion = 0;
	let myGuildsPending = 0;

	const invalidateGuildLoads = (uuid: string) => {
		loadGenerations.set(uuid, (loadGenerations.get(uuid) ?? 0) + 1);
	};

	const currentGeneration = (uuid: string) => loadGenerations.get(uuid) ?? 0;

	const loadMyGuilds = async () => {
		const version = ++myGuildsVersion;
		myGuildsPending += 1;
		setState("loading", true);
		try {
			const uuids = await myGuilds();
			if (version === myGuildsVersion) {
				setState("myGuildUUIDs", uuids);
			}
		} finally {
			myGuildsPending -= 1;
			if (myGuildsPending === 0) {
				setState("loading", false);
			}
		}
	};

	const ensureGuildLoaded = async (uuid: string) => {
		if (state.guildCache[uuid]) return state.guildCache[uuid];
		if (removedUUIDs.has(uuid)) return state.guildCache[uuid];

		const generation = currentGeneration(uuid);
		setState("guildLoading", uuid, true);
		setState("guildErrors", uuid, null);
		try {
			const guild = await getGuild(uuid);
			if (currentGeneration(uuid) === generation) {
				setState("guildCache", uuid, guild);
				setState("guildErrors", uuid, null);
				setState("guildLoading", uuid, false);
			}
			return guild;
		} catch (error) {
			if (currentGeneration(uuid) === generation) {
				setState("guildErrors", uuid, errorMessage(error));
				setState("guildLoading", uuid, false);
			}
			throw error;
		}
	};

	const setCurrentGuild = (uuid: string | null) => {
		setState("currentGuildUUID", uuid);
		if (uuid) {
			void ensureGuildLoaded(uuid).catch(() => {});
		}
	};

	const loadMembers = async (guildUUID: string) => {
		if (removedUUIDs.has(guildUUID)) return state.memberCache[guildUUID];
		const generation = currentGeneration(guildUUID);
		setState("memberLoading", guildUUID, true);
		setState("memberErrors", guildUUID, null);
		try {
			const members = await guildMembers(guildUUID);
			if (currentGeneration(guildUUID) === generation) {
				setState("memberCache", guildUUID, members);
				setState("memberErrors", guildUUID, null);
				setState("memberLoading", guildUUID, false);
			}
			return members;
		} catch (error) {
			if (currentGeneration(guildUUID) === generation) {
				setState("memberErrors", guildUUID, errorMessage(error));
				setState("memberLoading", guildUUID, false);
			}
			throw error;
		}
	};

	const addGuild = (guild: Guild) => {
		invalidateGuildLoads(guild.uuid);
		myGuildsVersion += 1;
		removedUUIDs.delete(guild.uuid);
		setState("guildCache", guild.uuid, guild);
		setState("guildErrors", guild.uuid, null);
		setState("guildLoading", guild.uuid, false);
		setState("myGuildUUIDs", (prev) =>
			prev.includes(guild.uuid) ? prev : [...prev, guild.uuid],
		);
	};

	const updateCachedGuild = (guild: Guild) => {
		if (removedUUIDs.has(guild.uuid) || !state.guildCache[guild.uuid]) {
			return;
		}
		invalidateGuildLoads(guild.uuid);
		setState("guildCache", guild.uuid, guild);
		setState("guildErrors", guild.uuid, null);
		setState("guildLoading", guild.uuid, false);
	};

	const removeGuild = (uuid: string) => {
		invalidateGuildLoads(uuid);
		myGuildsVersion += 1;
		removedUUIDs.add(uuid);
		setState("myGuildUUIDs", (prev) => prev.filter((u) => u !== uuid));
		setState("guildCache", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberCache", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("guildLoading", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberLoading", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("guildErrors", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberErrors", (prev) => ({ ...prev, [uuid]: undefined }));
		if (state.currentGuildUUID === uuid) {
			setState("currentGuildUUID", null);
		}
	};

	const leaveAndClear = async (uuid: string) => {
		await leaveGuild(uuid);
		removeGuild(uuid);
	};

	const deleteAndClear = async (uuid: string) => {
		await deleteGuild(uuid);
		removeGuild(uuid);
	};

	return {
		state,
		setState,
		loadMyGuilds,
		ensureGuildLoaded,
		setCurrentGuild,
		loadMembers,
		updateCachedGuild,
		addGuild,
		removeGuild,
		leaveAndClear,
		deleteAndClear,
	};
}

export default createRoot(createGuildStore);
