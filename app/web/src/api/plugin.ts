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
	const data = await apiClient.post<PluginInfo[]>({
		url: "/api/v1/plugins/list",
	});
	return data ?? [];
}

export async function getPlugin(name: string): Promise<PluginInfo> {
	const data = await apiClient.post<PluginInfo>({
		url: "/api/v1/plugins/get",
		data: { name },
	});
	if (!data) throw new Error("plugin not found");
	return data;
}

export async function updatePlugin(
	input: UpdatePluginInput,
): Promise<PluginInfo> {
	const data = await apiClient.post<PluginInfo>({
		url: "/api/v1/plugins/update",
		data: input,
	});
	if (!data) throw new Error("empty response");
	return data;
}

/** 通过供应商 Base URL 拉取可用模型列表，供用户选择 */
export async function listPluginModels(
	pluginName: string,
	input: ListPluginModelsInput,
): Promise<string[]> {
	const data = await apiClient.post<{ models?: string[] }>({
		url: `/api/v1/plugins/${encodeURIComponent(pluginName)}/llm/models`,
		data: input,
	});
	return data?.models ?? [];
}
