import { describe, expect, it } from "vitest";
import { getBaseURL } from "../helpers";

describe("system module", () => {
	it("serves ping and readyz", async () => {
		const base = getBaseURL();
		const ping = await fetch(`${base}/ping`);
		expect(ping.status).toBe(200);
		expect(await ping.text()).toContain("pong");

		const ready = await fetch(`${base}/readyz`);
		expect(ready.status).toBe(200);
		expect((await ready.json()) as { status: string }).toMatchObject({
			status: "ok",
		});
	});

	// 「关于」页要展示后端版本，且登录前就要能看到，所以版本端点必须公开。
	it("serves the version endpoint without auth", async () => {
		const res = await fetch(`${getBaseURL()}/api/v1/system/version`);
		expect(res.status).toBe(200);

		const body = (await res.json()) as {
			code: number;
			data: { version: string; commit: string };
		};
		expect(body.code).toBe(0);
		expect(body.data.version).toBeTruthy();
	});
});
