import { createServerConfig } from "@gospeak/desktop";
import { useSyncExternalStore } from "react";

/**
 * 可对外分享的站点基址。
 *
 * 桌面端页面的 origin 是宿主自己的回环代理（http://127.0.0.1:<port>），直接
 * 拿去分享等于把别人的链接指到他们自己的本机，所以用已配置的后端服务器地址；
 * 浏览器里页面的 origin 就是服务器本身，配置地址通常为空，自然退回 origin。
 *
 * 同步读取：调用方（分享弹窗、二维码）都在渲染期就要这个值。启动时先取
 * localStorage 镜像（`getBaseUrlSync`，零 IPC、首帧就有值），再由
 * `loadPublicBaseUrl()` 向宿主要权威值并校正镜像；`setBaseUrl` 后也会更新缓存。
 * 需要等它就绪的组件用 `usePublicBaseUrl()` 订阅，就绪后自动重渲染。
 */
let cachedBaseUrl = readMirrorBaseUrl();
const listeners = new Set<() => void>();

/** 同步读取本地镜像；桌面端与浏览器共用同一实现（见 @gospeak/desktop）。 */
function readMirrorBaseUrl(): string {
	try {
		return createServerConfig().getBaseUrlSync();
	} catch {
		return "";
	}
}

function notify() {
	for (const listener of listeners) listener();
}

export function publicBaseUrl(): string {
	const configured = cachedBaseUrl.trim().replace(/\/+$/, "");
	return configured || window.location.origin;
}

/**
 * 读取一次权威的后端地址（桌面端=宿主文件，浏览器=localStorage），填充缓存。
 * 失败时保持已有值 / origin 兜底。
 */
export async function loadPublicBaseUrl(): Promise<void> {
	try {
		const url = await createServerConfig().getBaseUrl();
		const next = typeof url === "string" ? url.trim() : "";
		if (next === cachedBaseUrl) return;
		cachedBaseUrl = next;
		notify();
	} catch {
		// 宿主未就绪或桥接失败：保持已有值兜底。
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
