import { describe, expect, it } from "vitest";

describe("server smoke", () => {
  it("health endpoint returns pong", async () => {
    const base = process.env.GOSPEAK_TEST_URL ?? "http://127.0.0.1:8998";
    const res = await fetch(`${base}/ping`);
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("pong");
  });
});
