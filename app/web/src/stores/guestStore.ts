import { createSignal } from "solid-js";
import userStore from "./userStore";

export interface GuestCaps {
	listen: boolean;
	speak: boolean;
	message: boolean;
}

const DEFAULT_CAPS: GuestCaps = { listen: true, speak: true, message: false };

const [caps, setCaps] = createSignal<GuestCaps>(DEFAULT_CAPS);

/** 当前登录身份是否为访客（依据 users.is_guest）。 */
export function isGuest(): boolean {
	return !!userStore.user()?.is_guest;
}

/** 域访客能力开关；未加载前给默认值（听/说开、消息关）。 */
export function guestCaps(): GuestCaps {
	return caps();
}

export function setGuestCaps(next: Partial<GuestCaps>) {
	setCaps({ ...caps(), ...next });
}

export function resetGuestCaps() {
	setCaps(DEFAULT_CAPS);
}

const guestStore = { isGuest, guestCaps, setGuestCaps, resetGuestCaps };
export default guestStore;
