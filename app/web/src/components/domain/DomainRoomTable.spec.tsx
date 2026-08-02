import { describe, expect, it } from "vitest";
import { validateEditRoomForm } from "../modal/EditRoomModal";

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
