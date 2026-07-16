import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export type PluginKind = "builtin" | "external";
export type PluginStatus =
	| "registered"
	| "starting"
	| "running"
	| "stopped"
	| "failed";

export interface SideServerInfo {
	name: string;
	addr: string;
	url: string;
}

export interface PluginInfo {
	name: string;
	display_name: string;
	version: string;
	author: string;
	desc: string;
	kind: PluginKind;
	config_schema?: Record<string, unknown>;
	enabled: boolean;
	status: PluginStatus;
	error?: string;
	config?: Record<string, any>;
	side_servers?: SideServerInfo[];
}

export interface UpdatePluginInput {
	name: string;
	enabled?: boolean;
	config?: Record<string, any>;
	restart?: boolean;
}

export async function listPlugins(): Promise<PluginInfo[]> {
	const res = (await apiClient.post({
		url: "/api/v1/plugins/list",
	})) as AxiosResponse<Result<PluginInfo[]>>;
	return res.data.data ?? [];
}

export async function getPlugin(name: string): Promise<PluginInfo> {
	const res = (await apiClient.post({
		url: "/api/v1/plugins/get",
		data: { name },
	})) as AxiosResponse<Result<PluginInfo>>;
	if (!res.data.data) throw new Error("plugin not found");
	return res.data.data;
}

export async function updatePlugin(
	input: UpdatePluginInput,
): Promise<PluginInfo> {
	const res = (await apiClient.post({
		url: "/api/v1/plugins/update",
		data: input,
	})) as AxiosResponse<Result<PluginInfo>>;
	if (!res.data.data) throw new Error("empty response");
	return res.data.data;
}
