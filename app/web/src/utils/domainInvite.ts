import { publicBaseUrl } from "./publicBaseUrl";

const INVITE_CODE_PATTERN = /^[A-HJ-NP-Z2-9]{36}$/i;
const INVITE_LINK_PATTERN = /\/invite\/d\/([A-HJ-NP-Z2-9]{36})/i;

/** Deep link scheme registered by the desktop host. */
export const DEEPLINK_SCHEME = "gospeak";

/** Loopback hosts that identify the desktop host itself, never a real server. */
const LOOPBACK_HOSTS = new Set(["127.0.0.1", "localhost", "::1", "[::1]"]);

export function extractDomainInviteCode(input: string): string | null {
	const text = input.trim();
	if (!text) return null;

	if (INVITE_CODE_PATTERN.test(text)) {
		return text.toUpperCase();
	}

	const linkMatch = text.match(INVITE_LINK_PATTERN);
	if (linkMatch?.[1]) {
		return linkMatch[1].toUpperCase();
	}

	try {
		const url = new URL(text);
		const code =
			url.searchParams.get("code") || url.searchParams.get("invite_code");
		if (code && INVITE_CODE_PATTERN.test(code)) {
			return code.toUpperCase();
		}
	} catch {
		// 非 URL 文本，直接按邀请码规则处理。
	}

	return null;
}

export interface ParsedInviteLink {
	/** 归一化后的邀请码（大写）。 */
	code: string;
	/**
	 * 链接指向的后端服务器 origin（`scheme://host[:port]`）。
	 * 回环地址（桌面端自身的 127.0.0.1 代理）、gospeak:// 深链与裸邀请码都没有
	 * 可用的服务器，为 undefined。
	 */
	server?: string;
}

/**
 * 解析邀请链接：接受 `gospeak://invite/d/CODE`、`https://host/invite/d/CODE`、
 * 裸邀请码与 `/invite/d/CODE` 路径。
 *
 * 回环 host 属于桌面端自己的代理，不是真正的服务器，因此不推导 server —— 否则
 * 会把服务器地址指向本机回环，切到它等于没切。
 */
export function parseInviteLink(input: string): ParsedInviteLink | null {
	const text = input.trim();
	if (!text) return null;

	const code = extractDomainInviteCode(text);
	if (!code) return null;

	let server: string | undefined;
	try {
		const url = new URL(text);
		if (
			(url.protocol === "http:" || url.protocol === "https:") &&
			!LOOPBACK_HOSTS.has(url.hostname)
		) {
			server = url.origin;
		}
	} catch {
		// 裸邀请码 / 相对路径：没有服务器信息。
	}

	return server ? { code, server } : { code };
}

/** 桌面端可点击的深链：唤起/聚焦应用并跳到邀请页。 */
export function domainDeepLinkUrl(code: string): string {
	return `${DEEPLINK_SCHEME}://invite/d/${encodeURIComponent(code)}`;
}

/**
 * 浏览器邀请链接；桌面端用后端服务器地址而不是回环 origin。
 *
 * `base` 由调用方传入（通常是 usePublicBaseUrl()）时，宿主地址异步加载完成会
 * 触发重渲染，链接与二维码跟着更新。
 */
export function domainInviteUrl(code: string, base?: string): string {
	const origin = (base ?? publicBaseUrl()).replace(/\/+$/, "");
	return `${origin}/invite/d/${encodeURIComponent(code)}`;
}
