import { describe, expect, it } from "vitest";
import { displayVersion } from "./display";

describe("displayVersion", () => {
	it("空值与 unknown 都回退成「未知」", () => {
		for (const raw of ["", "unknown", undefined, null, "  "]) {
			expect(displayVersion(raw)).toBe("未知");
		}
	});

	it("dev 构建原样展示，去掉两端空白", () => {
		expect(displayVersion(" dev ")).toBe("dev");
		expect(displayVersion("1.0.1")).toBe("1.0.1");
	});
});
