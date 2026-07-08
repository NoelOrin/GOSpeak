import type { Logger } from "../core/context";

export interface AuthCredentials {
	username: string;
	password: string;
}

export interface AuthResult {
	accessToken: string;
	refreshToken: string;
	userId: number;
	uuid: string;
	username: string;
	displayName: string;
	role: string;
	needChangePassword: boolean;
}

export interface AuthClientOptions {
	baseUrl: string;
	logger: Logger;
}

interface ApiResponse<T> {
	code: number;
	msg: string;
	data: T;
}

/**
 * JWT authentication client for GOSpeak server.
 *
 * Handles login, token refresh, and logout, aligned with the server's
 * /api/v1/auth/* endpoints.
 */
export class AuthClient {
	private opts: AuthClientOptions;
	private accessToken: string | null = null;
	private refreshToken: string | null = null;

	constructor(opts: AuthClientOptions) {
		this.opts = opts;
	}

	get token(): string | null {
		return this.accessToken;
	}

	get refresh(): string | null {
		return this.refreshToken;
	}

	get isLoggedIn(): boolean {
		return this.accessToken !== null;
	}

	async login(credentials: AuthCredentials): Promise<AuthResult> {
		const res = await fetch(`${this.opts.baseUrl}/api/v1/auth/login`, {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({
				username: credentials.username,
				password: credentials.password,
			}),
		});
		const json = (await res.json()) as ApiResponse<{
			access_token: string;
			refresh_token: string;
			user: {
				ID: number;
				UUID: string;
				Name: string;
				DisplayName: string;
				Role: string;
			};
			need_change_password: boolean;
		}>;

		if (json.code !== 0) {
			throw new Error(`Login failed (${json.code}): ${json.msg}`);
		}

		this.accessToken = json.data.access_token;
		this.refreshToken = json.data.refresh_token;

		return {
			accessToken: json.data.access_token,
			refreshToken: json.data.refresh_token,
			userId: json.data.user.ID,
			uuid: json.data.user.UUID,
			username: json.data.user.Name,
			displayName: json.data.user.DisplayName,
			role: json.data.user.Role,
			needChangePassword: json.data.need_change_password,
		};
	}

	async refreshAccessToken(): Promise<string> {
		if (!this.refreshToken) {
			throw new Error("No refresh token available");
		}

		const res = await fetch(`${this.opts.baseUrl}/api/v1/auth/refresh_token`, {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ refresh_token: this.refreshToken }),
		});
		const json = (await res.json()) as ApiResponse<{ access_token: string }>;

		if (json.code !== 0) {
			this.accessToken = null;
			this.refreshToken = null;
			throw new Error(`Token refresh failed (${json.code}): ${json.msg}`);
		}

		this.accessToken = json.data.access_token;
		return this.accessToken;
	}

	async logout(): Promise<void> {
		if (!this.accessToken) return;

		try {
			await fetch(`${this.opts.baseUrl}/api/v1/auth/logout`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: `Bearer ${this.accessToken}`,
				},
			});
		} catch (err) {
			this.opts.logger.warn("Logout request failed:", err);
		} finally {
			this.accessToken = null;
			this.refreshToken = null;
		}
	}

	/** Schedule automatic token refresh before expiry */
	startAutoRefresh(intervalMs: number, onRefreshed?: (token: string) => void): NodeJS.Timeout {
		return setInterval(async () => {
			try {
				const newToken = await this.refreshAccessToken();
				onRefreshed?.(newToken);
				this.opts.logger.info("Access token refreshed");
			} catch (err) {
				this.opts.logger.error("Auto refresh failed:", err);
			}
		}, intervalMs);
	}
}
