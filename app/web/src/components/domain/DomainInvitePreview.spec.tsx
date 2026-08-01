import { describe, expect, it, vi } from "vitest";
import type { Domain } from "@/api/domain";
import {
	getDomainInviteAction,
	getDomainInvitePreviewStatus,
} from "./DomainInvitePreview";

const domain: Domain = {
	id: 1,
	uuid: "g-1",
	name: "Test Domain",
	icon_url: "",
	description: "",
	owner_uuid: "u-owner",
	invite_code: "ABCDEFGH",
	max_rooms: 10,
	is_public: true,
	created_at: "2026-01-01",
};

describe("DomainInvitePreview status", () => {
	it("shows loading while the preview has not loaded", () => {
		expect(getDomainInvitePreviewStatus(true, null, null)).toBe("loading");
	});

	it("shows error when preview fails without a domain", () => {
		expect(getDomainInvitePreviewStatus(false, "邀请码无效", null)).toBe(
			"error",
		);
	});

	it("keeps the preview visible when joining fails", () => {
		expect(getDomainInvitePreviewStatus(false, "加入失败", domain)).toBe(
			"ready-with-error",
		);
	});
});

describe("DomainInvitePreview action", () => {
	it("offers confirm when the user has not joined", () => {
		const onConfirm = vi.fn();
		const action = getDomainInviteAction(domain, false, false, onConfirm);

		expect(action?.label).toBe("确认加入");
		expect(action?.disabled).toBe(false);
		action?.onClick();
		expect(onConfirm).toHaveBeenCalledTimes(1);
	});

	it("offers enter when the domain is already joined", () => {
		const action = getDomainInviteAction(domain, true, false, vi.fn());

		expect(action?.label).toBe("进入域");
		expect(action?.disabled).toBe(false);
	});

	it("disables the confirm button while joining", () => {
		const action = getDomainInviteAction(domain, false, true, vi.fn());

		expect(action?.label).toBe("加入中...");
		expect(action?.disabled).toBe(true);
	});

	it("returns no action before preview is ready", () => {
		expect(getDomainInviteAction(null, false, false, vi.fn())).toBeNull();
	});
});
