import { isSFUProvider } from "@gospeak/sfu-client/types";
import type { SFUProvider } from "@gospeak/sfu-client/types";

export type ProviderReloadDeps = {
	showToast: (
		msg: string,
		opts?: { type?: "success" | "error" | "warning" | "info" },
	) => unknown;
	preloadSfuClient: (provider: SFUProvider) => Promise<unknown>;
	/** rejoin 能力探测：用户仍想留在某个房间（rejoin intent 存在）且已登录 */
	canRejoin?: () => boolean;
	/** 执行原地 rejoin：重连 socket，交给 rejoin intent / voice session 状态机接管 */
	attemptRejoin?: () => void;
	/** rejoin 兜底探测：超时仍未恢复连接则退回整页刷新 */
	isConnected?: () => boolean;
	reload?: () => void;
	delayMs?: number;
	rejoinTimeoutMs?: number;
};

export function createProviderReloadHandler(deps: ProviderReloadDeps) {
	const delayMs = deps.delayMs ?? 500;
	const rejoinTimeoutMs = deps.rejoinTimeoutMs ?? 10000;
	const reload = deps.reload ?? (() => window.location.reload());

	return (provider?: string) => {
		console.log("[Socket] sfu:provider-changed", provider);
		const rejoinable = deps.canRejoin?.() ?? false;
		deps.showToast(
			rejoinable
				? "语音后端已切换，正在重新连接"
				: "语音后端已切换，即将刷新页面",
			{ type: "warning" },
		);
		// provider 来自网络事件，白名单运行时校验后再当作 SFUProvider 使用
		if (isSFUProvider(provider)) {
			void deps.preloadSfuClient(provider).catch(() => {});
		}
		// rejoin 优先：保持房间意图不清，重连 socket 后由 intent 派生的 rejoin
		// 与 voice session 自愈在新 provider 上重建会话，避免整页刷新丢状态。
		if (rejoinable && deps.attemptRejoin) {
			deps.attemptRejoin();
			if (deps.isConnected) {
				// 兜底：rejoin 超时仍未恢复连接（如重连持续失败），退回整页刷新
				window.setTimeout(() => {
					if (!deps.isConnected?.()) {
						console.warn(
							"[Socket] provider rejoin timeout, falling back to reload",
						);
						reload();
					}
				}, rejoinTimeoutMs);
			}
			return;
		}
		// rejoin 不可用（不在任何房间 / 未登录 / 会话无法原地重建）：整页刷新兜底
		window.setTimeout(() => {
			reload();
		}, delayMs);
	};
}
