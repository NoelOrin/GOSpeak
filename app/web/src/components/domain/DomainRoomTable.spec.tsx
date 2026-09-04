import { describe, expect, it, vi } from "vitest";
import { validateEditRoomForm } from "../modal/editRoomModal";
import type { RoomRecord } from "@/api/room";
import {
	canManageRoomAction,
	getRoomActions,
	getRoomTableStatus,
	isRoomTableBusy,
	roomTypeLabel,
} from "./DomainRoomTable";

const room: RoomRecord = {
	id: 1,
	uuid: "r-1",
	name: "lobby",
	description: "",
	limit: 12,
	audio_only: true,
	allow_audience: true,
	type: "voice",
	created_by: "creator-1",
	created_at: "2026-08-02T00:00:00Z",
	domain_uuid: "g-1",
};

describe("EditRoomModal form validation", () => {
	it("rejects an empty room name", () => {
		expect(validateEditRoomForm("   ", "desc", 12)).toEqual({
			name: "房间名称至少需要 2 个字符",
		});
	});

	it("rejects limit below two", () => {
		expect(validateEditRoomForm("lobby", "desc", 1)).toEqual({
			limit: "人数上限至少为 2",
		});
	});

	it("rejects empty limit", () => {
		expect(validateEditRoomForm("lobby", "desc", "")).toEqual({
			limit: "人数上限至少为 2",
		});
	});

	it("rejects description over 120 characters", () => {
		expect(validateEditRoomForm("lobby", "x".repeat(121), 12)).toEqual({
			description: "房间说明不能超过 120 个字符",
		});
	});

	it("accepts a valid room config", () => {
		expect(validateEditRoomForm("lobby", "desc", 12)).toEqual({});
	});
});

describe("DomainRoomTable type labels", () => {
	it("labels text rooms and voice rooms", () => {
		expect(roomTypeLabel("text")).toBe("文字");
		expect(roomTypeLabel("voice")).toBe("语音");
		expect(roomTypeLabel(undefined)).toBe("语音");
	});
});

describe("DomainRoomTable action logic", () => {
	it("hides actions when the user lacks manage rights and did not create the room", () => {
		expect(canManageRoomAction(room, "other-1", false)).toBe(false);
		expect(getRoomActions(room, "other-1", false, vi.fn(), vi.fn())).toBeNull();
	});

	it("shows actions to the room creator", () => {
		expect(canManageRoomAction(room, "creator-1", false)).toBe(true);
		expect(
			getRoomActions(room, "creator-1", false, vi.fn(), vi.fn()),
		).not.toBeNull();
	});

	it("shows actions when the user can manage the domain", () => {
		expect(canManageRoomAction(room, "other-1", true)).toBe(true);
	});

	it("builds edit and delete actions that call back with the room", () => {
		const onEdit = vi.fn();
		const onDelete = vi.fn();
		const actions = getRoomActions(room, "creator-1", false, onEdit, onDelete);

		expect(actions).not.toBeNull();
		expect(actions?.map((a) => a.label)).toEqual(["编辑", "删除"]);
		actions?.[0].onClick();
		expect(onEdit).toHaveBeenCalledWith(room);
		actions?.[1].onClick();
		expect(onDelete).toHaveBeenCalledWith(room);
	});
});

describe("DomainRoomTable status logic", () => {
	it("shows loading while no rooms have loaded", () => {
		expect(getRoomTableStatus([], true, null)).toBe("loading");
	});

	it("shows error when loading fails without cached rooms", () => {
		expect(getRoomTableStatus([], false, "network")).toBe("error");
	});

	it("keeps rows visible and reports an error when refresh fails", () => {
		expect(getRoomTableStatus([room], false, "network")).toBe(
			"ready-with-error",
		);
	});

	it("shows empty state when there are no rooms and no error", () => {
		expect(getRoomTableStatus([], false, null)).toBe("empty");
	});

	it("treats initial loading and refresh as busy states", () => {
		expect(isRoomTableBusy(true, false)).toBe(true);
		expect(isRoomTableBusy(false, true)).toBe(true);
		expect(isRoomTableBusy(false, false)).toBe(false);
	});
});
