import apiClient from "./apiClient";
import type { Result } from "./apiClient";

export interface OAuthProvider {
  id: number;
  name: string;
  display_name: string;
  icon_url: string;
  client_id: string;
  client_secret: string;
  auth_url: string;
  token_url: string;
  userinfo_url: string;
  redirect_url: string;
  scopes: string;
  uid_field: string;
  username_field: string;
  avatar_field: string;
  email_field: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface EnabledProvider {
  name: string;
  display_name: string;
  icon_url: string;
}

export interface CreateProviderInput {
  name: string;
  display_name?: string;
  icon_url?: string;
  client_id?: string;
  client_secret?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  redirect_url?: string;
  scopes?: string;
  uid_field?: string;
  username_field?: string;
  avatar_field?: string;
  email_field?: string;
  enabled?: boolean;
}

export interface UpdateProviderInput {
  id: number;
  name?: string;
  display_name?: string;
  icon_url?: string;
  client_id?: string;
  client_secret?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  redirect_url?: string;
  scopes?: string;
  uid_field?: string;
  username_field?: string;
  avatar_field?: string;
  email_field?: string;
  enabled?: boolean;
}

export function listProviders(): Promise<Result<OAuthProvider[]>> {
  return apiClient.get({ url: "/api/v1/oauth/admin/providers" }).then((r) => r.data);
}

export function listEnabledProviders(): Promise<Result<EnabledProvider[]>> {
  return apiClient.get({ url: "/api/v1/oauth/providers" }).then((r) => r.data);
}

export function createProvider(input: CreateProviderInput): Promise<Result<OAuthProvider>> {
  return apiClient.post({ url: "/api/v1/oauth/admin/providers", data: input }).then((r) => r.data);
}

export function updateProvider(input: UpdateProviderInput): Promise<Result<OAuthProvider>> {
  return apiClient.put({ url: "/api/v1/oauth/admin/providers", data: input }).then((r) => r.data);
}

export function deleteProvider(id: number): Promise<Result<null>> {
  return apiClient.delete({ url: `/api/v1/oauth/admin/providers/${id}` }).then((r) => r.data);
}
