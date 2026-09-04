import { describe, expect, it } from "vitest";
import { api, getAdminToken, registerUser } from "../helpers";

describe("storage module", () => {
  it("reads local storage config as admin", async () => {
    const admin = await getAdminToken();
    const result = await api<{ provider_type: string; max_file_size: number }>("/api/v1/storage/config", {
      token: admin,
    });
    expect(result.code).toBe(0);
    expect(result.data?.provider_type).toBe("local");
    expect(result.data?.max_file_size).toBeGreaterThan(0);
  });

  it("presigns and uploads a local file", async () => {
    const user = await registerUser("storage");

    const presigned = await api<{ object_key: string; provider_type: string }>("/api/v1/storage/presign", {
      token: user.access_token,
      body: {
        file_name: "hello.txt",
        content_type: "text/plain",
        file_size: 11,
        category: "avatar",
      },
    });
    expect(presigned.code).toBe(0);
    expect(presigned.data?.provider_type).toBe("local");
    const objectKey = presigned.data!.object_key;
    expect(objectKey).toContain(user.user.uuid);

    const form = new FormData();
    form.append("file", new Blob(["integration"], { type: "text/plain" }), "hello.txt");
    form.append("object_key", objectKey);
    const uploaded = await api<{ public_url: string }>("/api/v1/storage/upload", {
      token: user.access_token,
      formData: form,
    });
    expect(uploaded.code).toBe(0);
    expect(uploaded.data?.public_url).toBeTruthy();

    const admin = await getAdminToken();
    const deleted = await api("/api/v1/storage/delete", {
      token: admin,
      body: { key: objectKey },
    });
    expect(deleted.code).toBe(0);
  });

  it("rejects disallowed file types", async () => {
    const user = await registerUser("storage");
    const result = await api("/api/v1/storage/presign", {
      token: user.access_token,
      body: {
        file_name: "payload.exe",
        content_type: "application/octet-stream",
        file_size: 10,
        category: "avatar",
      },
    });
    expect(result.code).toBe(8104);
  });
});
