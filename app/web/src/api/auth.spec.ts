import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/apiClient", () => {
	const mockPost = vi.fn();
	return { default: { post: mockPost }, mockPost };
});

import apiClient from "@/api/apiClient";
import { login, register } from "@/api/auth";

const postMock = vi.mocked(apiClient.post);

describe("authApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("login posts username/password to /auth/login", async () => {
		postMock.mockResolvedValueOnce({ user: { name: "u" } } as never);
		await login({ username: "u", password: "p" });
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/auth/login",
			data: { username: "u", password: "p" },
		});
	});

	it("register posts credentials to /auth/register", async () => {
		postMock.mockResolvedValueOnce({ user: { name: "u" } } as never);
		await register({ username: "u", password: "p".repeat(8) });
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/auth/register",
			data: {
				username: "u",
				password: "p".repeat(8),
				email: undefined,
				email_code: undefined,
			},
		});
	});

	it("register forwards email verification fields", async () => {
		postMock.mockResolvedValueOnce({} as never);
		await register({
			username: "u",
			password: "p".repeat(8),
			email: "a@b.c",
			email_code: "123456",
		});
		expect(postMock).toHaveBeenCalledWith({
			url: "/api/v1/auth/register",
			data: {
				username: "u",
				password: "p".repeat(8),
				email: "a@b.c",
				email_code: "123456",
			},
		});
	});

	it("register rejects when backend returns no data", async () => {
		postMock.mockResolvedValueOnce(undefined as never);
		await expect(
			register({ username: "u", password: "p".repeat(8) }),
		).rejects.toThrow("register data is missing");
	});
});
