import { describe, expect, it } from "vitest";
import { getBaseURL } from "./helpers/api";

describe("server smoke", () => {
  it("health endpoint returns pong", async () => {
    const res = await fetch(`${getBaseURL()}/ping`);
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("pong");
  });
});
