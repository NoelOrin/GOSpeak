/**
 * 切换服务器时暂存的邀请码。
 *
 * 服务器地址变更会让宿主清会话并重载页面（app/desktop/server_config.go），
 * 重载前拿到的邀请码必须先落到 localStorage，重载后由根路由取走并跳转。
 * 用 sessionStorage 语义的过期时间兜底，避免几天后打开应用突然跳旧邀请。
 */
const KEY = "gospeak_pending_invite";
const TTL_MS = 10 * 60 * 1000;

interface PendingInvite {
	code: string;
	at: number;
}

export function setPendingInvite(code: string): void {
	try {
		const payload: PendingInvite = { code, at: Date.now() };
		localStorage.setItem(KEY, JSON.stringify(payload));
	} catch {
		// 隐私模式等场景写入失败：重载后只是不会自动跳转，用户可手动打开链接。
	}
}

/** 取走待处理邀请码（一次性）；过期或损坏时返回 null。 */
export function takePendingInvite(): string | null {
	try {
		const raw = localStorage.getItem(KEY);
		if (!raw) return null;
		localStorage.removeItem(KEY);
		const parsed = JSON.parse(raw) as Partial<PendingInvite>;
		if (typeof parsed.code !== "string" || typeof parsed.at !== "number") {
			return null;
		}
		if (Date.now() - parsed.at > TTL_MS) return null;
		return parsed.code;
	} catch {
		return null;
	}
}

export function clearPendingInvite(): void {
	try {
		localStorage.removeItem(KEY);
	} catch {
		// ignore
	}
}
