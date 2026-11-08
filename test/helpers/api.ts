import { randomBytes } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export interface AuthUser {
  id: number;
  uuid: string;
  name: string;
  display_name: string;
  role: string;
}

export interface AuthData {
  access_token: string;
  refresh_token: string;
  user: AuthUser;
  need_change_password?: boolean;
}

export interface ApiResult<T = unknown> {
  status: number;
  code: number | null;
  msg: string | null;
  data: T | null;
  raw: string;
}

export interface ApiOptions {
  token?: string;
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  formData?: FormData;
  redirect?: "follow" | "manual" | "error";
}

const helperDir = dirname(fileURLToPath(import.meta.url));
const serverInfoPath = resolve(helperDir, "../.runtime/server.json");

function readServerUrl(): string {
  const fromEnv = process.env.GOSPEAK_TEST_URL;
  if (fromEnv) {
    return fromEnv.replace(/\/$/, "");
  }
  if (existsSync(serverInfoPath)) {
    try {
      const parsed = JSON.parse(readFileSync(serverInfoPath, "utf8")) as { url?: string };
      if (parsed.url) {
        return parsed.url.replace(/\/$/, "");
      }
    } catch {
      // fall through to a clear error
    }
  }
  throw new Error("integration server is not running; run pnpm test:server");
}

export function getBaseURL(): string {
  return readServerUrl();
}

export function unique(prefix = "item"): string {
  return `${prefix}_${Date.now().toString(36)}_${randomBytes(4).toString("hex")}`;
}

function randomClientIP(): string {
  const part = () => 2 + Math.floor(Math.random() * 253);
  return `10.${part()}.${part()}.${part()}`;
}

export async function api<T = unknown>(
  path: string,
  options: ApiOptions = {},
): Promise<ApiResult<T>> {
  const headers = new Headers(options.headers);
  headers.set("X-Forwarded-For", randomClientIP());
  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }

  let body: BodyInit | undefined;
  if (options.formData) {
    body = options.formData;
  } else if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(options.body);
  }

  const response = await fetch(`${readServerUrl()}${path}`, {
    method: options.method ?? "POST",
    headers,
    body,
    redirect: options.redirect ?? "follow",
  });
  const raw = await response.text();
  let parsed: { code?: number; msg?: string; data?: T | null } | null = null;
  try {
    parsed = JSON.parse(raw) as { code?: number; msg?: string; data?: T | null };
  } catch {
    parsed = null;
  }
  return {
    status: response.status,
    code: parsed?.code ?? null,
    msg: parsed?.msg ?? null,
    data: parsed?.data ?? null,
    raw,
  };
}

export function assertSuccess<T = unknown>(result: ApiResult<T>): T {
  if (result.code !== 0) {
    throw new Error(`expected success, got code=${result.code} msg=${result.msg} body=${result.raw}`);
  }
  return result.data as T;
}

export async function login(username: string, password: string): Promise<AuthData> {
  const result = await api<AuthData>("/api/v1/auth/login", {
    body: { username, password },
  });
  return assertSuccess(result);
}

export async function registerUser(prefix = "user"): Promise<AuthData & { username: string; password: string }> {
  const username = unique(prefix);
  const password = `Passw0rd!${randomBytes(6).toString("hex")}`;
  const result = await api<AuthData>("/api/v1/auth/register", {
    body: { username, password },
  });
  const data = assertSuccess(result);
  return { ...data, username, password };
}

let cachedAdminToken: string | undefined;

export async function getAdminToken(): Promise<string> {
  if (cachedAdminToken) {
    return cachedAdminToken;
  }
  const data = await login("admin", "admin123");
  cachedAdminToken = data.access_token;
  return cachedAdminToken;
}

export async function createDomain(token: string, name: string): Promise<{ uuid: string; invite_code: string; name: string }> {
  const result = await api<{ uuid: string; invite_code: string; name: string }>("/api/v1/domain/create", {
    token,
    body: { name, description: "integration domain", is_public: false },
  });
  return assertSuccess(result);
}

export async function listDomainRoles(
  token: string,
  domainUUID: string,
): Promise<{ roles: Array<{ name: string; is_system: boolean; permissions: string[] }>; assignable: string[] }> {
  const result = await api(`/api/v1/domain/roles/list`, {
    token,
    body: { domain_uuid: domainUUID },
  });
  return assertSuccess(result);
}

export async function createDomainRole(
  token: string,
  domainUUID: string,
  name: string,
  permissions: string[],
): Promise<unknown> {
  const result = await api(`/api/v1/domain/roles/create`, {
    token,
    body: { domain_uuid: domainUUID, name, permissions },
  });
  return assertSuccess(result);
}

export async function updateDomainMemberRole(
  token: string,
  domainUUID: string,
  userUUID: string,
  roleName: string,
): Promise<unknown> {
  const result = await api(`/api/v1/domain/members/update-role`, {
    token,
    body: { domain_uuid: domainUUID, user_uuid: userUUID, role_name: roleName },
  });
  return assertSuccess(result);
}

export async function joinDomain(token: string, inviteCode: string): Promise<{ uuid: string }> {
  const result = await api<{ uuid: string }>("/api/v1/domain/join", {
    token,
    body: { invite_code: inviteCode },
  });
  return assertSuccess(result);
}

export async function createRoom(
  token: string,
  domainUUID: string,
  name: string,
  type = "voice",
): Promise<{ id: number; uuid: string; name: string; type: string; domain_uuid: string }> {
  const result = await api<{ id: number; uuid: string; name: string; type: string; domain_uuid: string }>(
    "/api/v1/room/create",
    {
      token,
      body: {
        name,
        domain_uuid: domainUUID,
        type,
        description: "integration room",
        limit: 10,
        audio_only: true,
        allow_audience: true,
      },
    },
  );
  return assertSuccess(result);
}
