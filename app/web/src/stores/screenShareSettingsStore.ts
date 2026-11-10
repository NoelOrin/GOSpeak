import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { create } from "zustand";
import {
	DEFAULT_SCREEN_SHARE_QUALITY,
	normalizeScreenShareQuality,
	type ScreenShareQuality,
} from "@gospeak/sfu-client";

interface PersistedScreenShareSettings {
	quality?: unknown;
}

interface ScreenShareSettingsState {
	quality: ScreenShareQuality;
}

const STORAGE_KEY = "screenShareSettingsStore";

const initialState: ScreenShareSettingsState = {
	quality: DEFAULT_SCREEN_SHARE_QUALITY,
};

const loadPersistedState = async (): Promise<ScreenShareSettingsState> => {
	const [err, savedState] = await to(
		get<PersistedScreenShareSettings>(STORAGE_KEY),
	);
	if (err) throw err;
	return { quality: normalizeScreenShareQuality(savedState?.quality) };
};

export const useScreenShareSettingsStore = create<ScreenShareSettingsState>(
	() => initialState,
);

const persisted = await loadPersistedState().catch((error: unknown) => {
	console.error("Failed to load persisted screen share settings:", error);
	return initialState;
});
useScreenShareSettingsStore.setState(persisted);

const debouncedPersist = debounce((state: ScreenShareSettingsState) => {
	set(STORAGE_KEY, { quality: state.quality }).catch((error: unknown) => {
		console.error("Failed to persist screen share settings:", error);
	});
}, 200);

useScreenShareSettingsStore.subscribe((state) => {
	debouncedPersist(state);
});

const ScreenShareSettingsStore = {
	get state() {
		return useScreenShareSettingsStore.getState();
	},
	setQuality: (quality: ScreenShareQuality) => {
		useScreenShareSettingsStore.setState({
			quality: normalizeScreenShareQuality(quality),
		});
	},
};

export default ScreenShareSettingsStore;
