import { describe, expect, it, vi } from "vitest";
import type { Guild } from "@/api/guild";
import {
	getGuildInviteAction,
	getGuildInvitePreviewStatus,
} from "./GuildInvitePreview";

const guild: Guild = {
	id: 1,
	uuid: "g-1",
	name: "Test Guild",
	icon_url: "",
	description: "",
	owner_uuid: "u-owner",
	invite_code: "ABCDEFGH",
	max_rooms: 10,
	is_public: true,
	created_at: "2026-01-01",
};

describe("GuildInvitePreview status", () => {
	it("shows loading while the preview has not loaded", () => {
		expect(getGuildInvitePreviewStatus(true, null, null)).toBe("loading");
	});

	it("shows error when preview fails without a guild", () => {
		expect(getGuildInvitePreviewStatus(false, "邀请码无效", null)).toBe(
			"error",
		);
	});

	it("keeps the preview visible when joining fails", () => {
		expect(getGuildInvitePreviewStatus(false, "加入失败", guild)).toBe(
			"ready-with-error",
		);
	});
});

describe("GuildInvitePreview action", () => {
	it("offers confirm when the user has not joined", () => {
		const onConfirm = vi.fn();
		const action = getGuildInviteAction(guild, false, false, onConfirm);

		expect(action?.label).toBe("确认加入");
		expect(action?.disabled).toBe(false);
		action?.onClick();
		expect(onConfirm).toHaveBeenCalledTimes(1);
	});

	it("offers enter when the guild is already joined", () => {
		const action = getGuildInviteAction(guild, true, false, vi.fn());

		expect(action?.label).toBe("进入服务器");
		expect(action?.disabled).toBe(false);
	});

	it("disables the confirm button while joining", () => {
		const action = getGuildInviteAction(guild, false, true, vi.fn());

		expect(action?.label).toBe("加入中...");
		expect(action?.disabled).toBe(true);
	});

	it("returns no action before preview is ready", () => {
		expect(getGuildInviteAction(null, false, false, vi.fn())).toBeNull();
	});
});
