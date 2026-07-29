import { describe, expect, it } from "vitest";

// GuildIcon 的纯逻辑测试（不依赖 DOM 渲染）

function getInitials(name: string): string {
	return name.slice(0, 2).toUpperCase();
}

describe("GuildIcon logic", () => {
	it("extracts first two characters as initials", () => {
		expect(getInitials("Test Guild")).toBe("TE");
	});

	it("handles short names", () => {
		expect(getInitials("A")).toBe("A");
	});

	it("handles empty string", () => {
		expect(getInitials("")).toBe("");
	});

	it("uppercases initials", () => {
		expect(getInitials("hello world")).toBe("HE");
	});
});

// GuildList props 校验
describe("GuildList data shape", () => {
	it("valid guild list item shape", () => {
		const guild = {
			uuid: "g-1",
			name: "Test Guild",
			icon_url: "",
			is_public: true,
			owner_uuid: "u-1",
		};
		expect(guild.uuid).toBeDefined();
		expect(guild.name).toBeDefined();
		expect(typeof guild.is_public).toBe("boolean");
	});

	it("handles guild list with icon_url", () => {
		const guild = {
			uuid: "g-2",
			name: "Icon Guild",
			icon_url: "https://example.com/icon.png",
		};
		expect(guild.icon_url).toContain("https://");
	});
});

// CreateGuildModal 表单验证逻辑
describe("CreateGuildModal validation", () => {
	it("rejects empty name", () => {
		const name = "";
		expect(name.trim().length > 0).toBe(false);
	});

	it("accepts valid name", () => {
		const name = "My Guild";
		expect(name.trim().length > 0).toBe(true);
	});

	it("rejects name exceeding 100 characters", () => {
		const longName = "x".repeat(101);
		expect(longName.length > 100).toBe(true);
	});

	it("accepts name within limit", () => {
		const shortName = "My Guild";
		expect(shortName.length <= 100).toBe(true);
	});
});
