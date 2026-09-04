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
    expect((await ready.json()) as { status: string }).toMatchObject({ status: "ok" });
  });
});
