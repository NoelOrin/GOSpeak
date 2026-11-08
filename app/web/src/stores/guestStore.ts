import { create } from "zustand";
import { useUserStore } from "./userStore";

export interface GuestCaps {
	listen: boolean;
	speak: boolean;
	message: boolean;
}

const DEFAULT_CAPS: GuestCaps = { listen: true, speak: true, message: false };

interface GuestState {
	caps: GuestCaps;
	setCaps: (next: Partial<GuestCaps>) => void;
	resetCaps: () => void;
}

export const useGuestStore = create<GuestState>((set) => ({
	caps: DEFAULT_CAPS,
	setCaps: (next) => set((prev) => ({ caps: { ...prev.caps, ...next } })),
	resetCaps: () => set({ caps: DEFAULT_CAPS }),
}));

/** 当前登录身份是否为访客（依据 users.is_guest）。 */
export function isGuest(): boolean {
	return !!useUserStore.getState().user?.is_guest;
}

/** 域访客能力开关；未加载前给默认值（听/说开、消息关）。 */
export function guestCaps(): GuestCaps {
	return useGuestStore.getState().caps;
}

export function setGuestCaps(next: Partial<GuestCaps>) {
	useGuestStore.getState().setCaps(next);
}

export function resetGuestCaps() {
	useGuestStore.getState().resetCaps();
}

const guestStore = { isGuest, guestCaps, setGuestCaps, resetGuestCaps };
export default guestStore;
