import { createServerConfig, inDesktop } from "@gospeak/desktop";
import { useSyncExternalStore } from "react";

/**
 * 可对外分享的站点基址。
 *
 * 浏览器里就是当前 origin。桌面端页面的 origin 是宿主自己的回环代理
 * （http://127.0.0.1:<port>），直接拿去分享等于把别人的链接指到他们自己的
 * 本机，所以改用已配置的后端服务器地址；还没配置时退回 origin，至少能用。
 *
 * 同步读取：调用方（分享弹窗、二维码）都在渲染期就要这个值。缓存在启动时
 * 由 loadPublicBaseUrl() 预热，之后 setBaseUrl 会通过 setPublicBaseUrl 更新；
 * 需要等它就绪的组件用 usePublicBaseUrl() 订阅，就绪后自动重渲染。
 */
let cachedBaseUrl = "";
const listeners = new Set<() => void>();

function notify() {
	for (const listener of listeners) listener();
}

export function publicBaseUrl(): string {
	if (!inDesktop()) return window.location.origin;
	const configured = cachedBaseUrl.trim().replace(/\/+$/, "");
	return configured || window.location.origin;
}

/** 读取一次宿主持久化的后端地址，填充缓存。失败时保持 origin 兜底。 */
export async function loadPublicBaseUrl(): Promise<void> {
	if (!inDesktop()) return;
	try {
		const url = await createServerConfig().getBaseUrl();
		const next = typeof url === "string" ? url : "";
		if (next === cachedBaseUrl) return;
		cachedBaseUrl = next;
		notify();
	} catch {
		// 宿主未就绪或桥接失败：保持 origin 兜底。
	}
}

/** 服务器地址切换后同步缓存（页面即将被宿主重载，这里只是保持一致）。 */
export function setPublicBaseUrl(url: string): void {
	const next = url.trim();
	if (next === cachedBaseUrl) return;
	cachedBaseUrl = next;
	notify();
}

/**
 * 订阅分享基址：宿主地址异步加载完成时返回新值，让依赖它的链接/二维码重算。
 */
export function usePublicBaseUrl(): string {
	return useSyncExternalStore(
		(listener) => {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},
		publicBaseUrl,
		publicBaseUrl,
	);
}
