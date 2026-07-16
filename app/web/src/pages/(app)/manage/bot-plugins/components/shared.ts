export const LLM_PROTOCOLS = [
	{ value: "openai-compatible", label: "OpenAI Compatible" },
	{ value: "anthropic", label: "Anthropic" },
	{ value: "gemini", label: "Gemini" },
	{ value: "gemini-response", label: "Gemini Response API" },
	{ value: "ollama", label: "Ollama" },
	{ value: "custom-http", label: "Custom HTTP" },
] as const;

export type LLMProviderForm = {
	name: string;
	display_name: string;
	protocol: string;
	base_url: string;
	api_key: string;
	model: string;
	enabled: boolean;
};

export type ViewMode = "cards" | "list";

export const PAGE_SIZE_OPTIONS = [8, 12, 24, 48] as const;

export function emptyProvider(): LLMProviderForm {
	return {
		name: "",
		display_name: "",
		protocol: "openai-compatible",
		base_url: "",
		api_key: "",
		// 不预设模型，由用户从 API 列表选择或手工填写
		model: "",
		enabled: true,
	};
}

/** 朴素中性色状态标签，避免高饱和彩色 */
export function statusMeta(status?: string) {
	const labelMap: Record<string, string> = {
		running: "运行中",
		failed: "失败",
		stopped: "已停止",
		starting: "启动中",
		registered: "已注册",
	};
	return {
		label: labelMap[status || ""] || status || "未知",
		dot: "bg-base-content/40",
		chip: "border-base-300 bg-base-200/50 text-base-content/70",
	};
}

export function protocolLabel(value: string) {
	return LLM_PROTOCOLS.find((p) => p.value === value)?.label || value;
}

export function kindLabel(kind?: string) {
	if (kind === "builtin") return "内置";
	if (kind === "external") return "外部";
	return kind || "插件";
}
