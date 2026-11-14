import type { SFUProvider } from "@gospeak/sfu-client/types";

export type ProviderReloadDeps = {
	showToast: (
		msg: string,
		opts?: { type?: "success" | "error" | "warning" | "info" },
	) => unknown;
	preloadSfuClient: (provider: SFUProvider) => Promise<unknown>;
	reload?: () => void;
	delayMs?: number;
};

export function createProviderReloadHandler(deps: ProviderReloadDeps) {
	const delayMs = deps.delayMs ?? 500;
	const reload = deps.reload ?? (() => window.location.reload());

	return (provider?: string) => {
		console.log("[Socket] sfu:provider-changed", provider);
		deps.showToast("语音后端已切换，即将刷新页面", { type: "warning" });
		if (provider) {
			void deps.preloadSfuClient(provider as SFUProvider).catch(() => {});
		}
		window.setTimeout(() => {
			reload();
		}, delayMs);
	};
}
