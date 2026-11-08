import { beforeEach, describe, expect, it } from "vitest";
import guestStore, { setGuestCaps } from "./guestStore";
import userStore from "./userStore";

describe("guestStore", () => {
	beforeEach(() => {
		userStore.clearAuth();
	});

	it("isGuest false for anonymous visitor", () => {
		expect(guestStore.isGuest()).toBe(false);
	});

	it("isGuest true when logged user has is_guest flag", async () => {
		await userStore.login({
			id: 1,
			uuid: "u-1",
			name: "guest_abc",
			display_name: "访客",
			avatar: "",
			role: "user",
			is_guest: true,
		});
		expect(guestStore.isGuest()).toBe(true);
	});

	it("caps default listen/speak on and message off", () => {
		expect(guestStore.guestCaps()).toEqual({
			listen: true,
			speak: true,
			message: false,
		});
	});

	it("setGuestCaps merges partial updates", () => {
		setGuestCaps({ message: true });
		expect(guestStore.guestCaps().message).toBe(true);
		guestStore.resetGuestCaps();
		expect(guestStore.guestCaps().message).toBe(false);
	});
});
