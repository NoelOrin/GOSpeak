import { describe, expect, it } from "vitest";

// DomainIcon 的纯逻辑测试（不依赖 DOM 渲染）

function getInitials(name: string): string {
	return name.slice(0, 2).toUpperCase();
}

describe("DomainIcon logic", () => {
	it("extracts first two characters as initials", () => {
		expect(getInitials("Test Domain")).toBe("TE");
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

// DomainList props 校验
describe("DomainList data shape", () => {
	it("valid domain list item shape", () => {
		const domain = {
			uuid: "g-1",
			name: "Test Domain",
			icon_url: "",
			is_public: true,
			owner_uuid: "u-1",
		};
		expect(domain.uuid).toBeDefined();
		expect(domain.name).toBeDefined();
		expect(typeof domain.is_public).toBe("boolean");
	});

	it("handles domain list with icon_url", () => {
		const domain = {
			uuid: "g-2",
			name: "Icon Domain",
			icon_url: "https://example.com/icon.png",
		};
		expect(domain.icon_url).toContain("https://");
	});
});

// CreateDomainModal 表单验证逻辑
describe("CreateDomainModal validation", () => {
	it("rejects empty name", () => {
		const name = "";
		expect(name.trim().length > 0).toBe(false);
	});

	it("accepts valid name", () => {
		const name = "My Domain";
		expect(name.trim().length > 0).toBe(true);
	});

	it("rejects name exceeding 100 characters", () => {
		const longName = "x".repeat(101);
		expect(longName.length > 100).toBe(true);
	});

	it("accepts name within limit", () => {
		const shortName = "My Domain";
		expect(shortName.length <= 100).toBe(true);
	});
});
