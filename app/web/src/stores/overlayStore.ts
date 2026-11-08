import { create } from "zustand";
import type { OverlaySnapshot } from "@gospeak/desktop";

/**
 * Local HUD state for the floating overlay window.
 *
 * The overlay is a SEPARATE WebView with its own (empty) socket/SFU state, so it
 * does NOT read the app's socketStore. Instead the main window relays a
 * lightweight snapshot here via the desktop host (see useOverlayRelay), and the
 * overlay window only renders it. This keeps the overlay from ever joining the
 * voice room a second time ("double-join").
 */
interface OverlayStoreState extends OverlaySnapshot {
	setSnapshot: (s: OverlaySnapshot) => void;
}

export const useOverlayStore = create<OverlayStoreState>((set) => ({
	roomName: null,
	members: [],
	speakingIdentities: [],
	connected: false,
	setSnapshot: (s) => set(s),
}));

export const overlayStore = {
	get: (): OverlaySnapshot => {
		const s = useOverlayStore.getState();
		return {
			roomName: s.roomName,
			members: s.members,
			speakingIdentities: s.speakingIdentities,
			connected: s.connected,
		};
	},
	set: (s: OverlaySnapshot) => useOverlayStore.getState().setSnapshot(s),
};
