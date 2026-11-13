/**
 * 只允许 http/https 的 URL 白名单化。
 *
 * 发布页链接来自 GitHub Releases API（后端透传），属于外部输入，不能直接塞进
 * href——`javascript:` / `data:` / `file:` 都可能被执行或被当成文件读取。
 */
export function safeHttpUrl(raw: string | undefined): string | null {
	if (!raw) return null;
	try {
		const url = new URL(raw);
		return url.protocol === "http:" || url.protocol === "https:"
			? url.href
			: null;
	} catch {
		return null;
	}
}
