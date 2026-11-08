import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	clearPendingInvite,
	setPendingInvite,
	takePendingInvite,
} from "./pendingInvite";

describe("pendingInvite", () => {
	beforeEach(() => {
		localStorage.clear();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("returns the stored code once", () => {
		setPendingInvite("B2JF5EDV");
		expect(takePendingInvite()).toBe("B2JF5EDV");
		expect(takePendingInvite()).toBeNull();
	});

	it("drops an expired entry", () => {
		vi.useFakeTimers();
		setPendingInvite("B2JF5EDV");
		vi.advanceTimersByTime(11 * 60 * 1000);
		expect(takePendingInvite()).toBeNull();
	});

	it("ignores damaged payloads", () => {
		localStorage.setItem("gospeak_pending_invite", "{not json");
		expect(takePendingInvite()).toBeNull();
		localStorage.setItem(
			"gospeak_pending_invite",
			JSON.stringify({ code: 1, at: 2 }),
		);
		expect(takePendingInvite()).toBeNull();
	});

	it("clears on demand", () => {
		setPendingInvite("B2JF5EDV");
		clearPendingInvite();
		expect(takePendingInvite()).toBeNull();
	});
});
