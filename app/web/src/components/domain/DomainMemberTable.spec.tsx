import { describe, expect, it, vi } from "vitest";
import type { DomainMember } from "@/api/domain";
import {
	canKickMember,
	executeKickMember,
	getKickAction,
	getMemberTableStatus,
	isMemberTableBusy,
} from "./DomainMemberTable";
import { validateDomainForm } from "../../pages/(app)/manage/domains/$domainUUID/index";

const owner: DomainMember = {
	id: 1,
	domain_uuid: "g-1",
	user_uuid: "u-owner",
	nickname: "owner",
	role_name: "owner",
	joined_at: "2026-01-01",
};

const member: DomainMember = {
	id: 2,
	domain_uuid: "g-1",
	user_uuid: "u-member",
	nickname: "member",
	role_name: "member",
	joined_at: "2026-01-02",
};

describe("DomainMemberTable kick logic", () => {
	it("does not allow kicking the domain owner", () => {
		expect(canKickMember(owner, "u-owner", "u-member", true)).toBe(false);
		expect(
			getKickAction(owner, "u-owner", "u-member", true, vi.fn()),
		).toBeNull();
	});

	it("does not allow kicking the current user", () => {
		expect(canKickMember(member, "u-owner", "u-member", true)).toBe(false);
		expect(
			getKickAction(member, "u-owner", "u-member", true, vi.fn()),
		).toBeNull();
	});

	it("hides kick actions when kick permission is missing", () => {
		expect(canKickMember(member, "u-owner", "u-member", false)).toBe(false);
		expect(
			getKickAction(member, "u-owner", "u-member", false, vi.fn()),
		).toBeNull();
	});

	it("builds a kick action that calls back with the target UUID", () => {
		const onKick = vi.fn();
		const action = getKickAction(member, "u-owner", "u-other", true, onKick);

		expect(action).not.toBeNull();
		expect(action?.ariaLabel).toBe("踢出 u-member");
		action?.onClick();
		expect(onKick).toHaveBeenCalledWith("u-member");
	});

	it("marks the kick action disabled while a kick is pending", () => {
		const action = getKickAction(member, "u-owner", "u-other", true, vi.fn(), {
			disabled: true,
		});

		expect(action?.disabled).toBe(true);
	});
});

describe("DomainMemberTable status logic", () => {
	it("shows loading while no members have loaded", () => {
		expect(getMemberTableStatus([], true, null)).toBe("loading");
	});

	it("shows error when loading fails without cached members", () => {
		expect(getMemberTableStatus([], false, "network")).toBe("error");
	});

	it("keeps rows visible and reports an error when refresh fails", () => {
		expect(getMemberTableStatus([member], false, "network")).toBe(
			"ready-with-error",
		);
	});

	it("shows empty state when there are no members and no error", () => {
		expect(getMemberTableStatus([], false, null)).toBe("empty");
	});

	it("treats initial loading and refresh as busy states", () => {
		expect(isMemberTableBusy(true, false)).toBe(true);
		expect(isMemberTableBusy(false, true)).toBe(true);
		expect(isMemberTableBusy(false, false)).toBe(false);
	});
});

describe("DomainMemberTable kick flow", () => {
	it("calls kickDomainMember and refreshes members with the same domain UUID", async () => {
		const kick = vi.fn().mockResolvedValue(undefined);
		const refresh = vi.fn().mockResolvedValue([]);

		await executeKickMember("g-1", "u-member", kick, refresh);

		expect(kick).toHaveBeenCalledWith("g-1", "u-member");
		expect(refresh).toHaveBeenCalledWith("g-1");
	});

	it("does not refresh members when the kick request fails", async () => {
		const kick = vi.fn().mockRejectedValue(new Error("kick failed"));
		const refresh = vi.fn();

		await expect(
			executeKickMember("g-1", "u-member", kick, refresh),
		).rejects.toThrow("kick failed");
		expect(refresh).not.toHaveBeenCalled();
	});
});

describe("Domain manage form validation", () => {
	it("rejects an empty domain name", () => {
		expect(validateDomainForm("   ", 20)).toEqual({
			name: "域名称不能为空",
		});
	});

	it("rejects max_rooms below one", () => {
		expect(validateDomainForm("Domain", 0)).toEqual({
			maxRooms: "房间上限必须是大于等于 1 的整数",
		});
	});

	it("rejects non-integer max_rooms", () => {
		expect(validateDomainForm("Domain", 1.5)).toEqual({
			maxRooms: "房间上限必须是大于等于 1 的整数",
		});
	});

	it("rejects empty max_rooms", () => {
		expect(validateDomainForm("Domain", "")).toEqual({
			maxRooms: "房间上限必须是大于等于 1 的整数",
		});
	});

	it("accepts a valid name and integer room limit", () => {
		expect(validateDomainForm("  My Domain  ", 20)).toEqual({});
	});
});
