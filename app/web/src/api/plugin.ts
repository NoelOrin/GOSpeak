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

export interface ListPluginModelsInput {
	/** 已保存供应商 name；可与草稿字段一起传 */
	provider?: string;
	protocol?: string;
	base_url?: string;
	api_key?: string;
	model?: string;
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

/** 通过供应商 Base URL 拉取可用模型列表，供用户选择 */
export async function listPluginModels(
	pluginName: string,
	input: ListPluginModelsInput,
): Promise<string[]> {
	const res = (await apiClient.post({
		url: `/api/v1/plugins/${encodeURIComponent(pluginName)}/llm/models`,
		data: input,
	})) as AxiosResponse<Result<{ models?: string[] }>>;
	return res.data.data?.models ?? [];
}
